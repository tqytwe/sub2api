//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPaymentProviderFeeMigrationPreservesLegacyRowsAndExplicitOverrides(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	schema := "payment_fee_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = tx.ExecContext(ctx, "CREATE SCHEMA "+pq.QuoteIdentifier(schema))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO "+pq.QuoteIdentifier(schema))
	require.NoError(t, err)

	legacyDDL, err := migrations.FS.ReadFile("096_payment_provider_instances.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(legacyDDL))
	require.NoError(t, err)
	var instanceID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO payment_provider_instances
		(provider_key,name,config,supported_types,sort_order,limits)
		VALUES('easypay','Legacy provider','{}','alipay,wxpay',7,'{"alipay":{"singleMax":200}}')
		RETURNING id`).Scan(&instanceID))
	var original []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT to_jsonb(instance)
		FROM payment_provider_instances instance WHERE id=$1`, instanceID).Scan(&original))
	var feeColumns int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=$1 AND table_name='payment_provider_instances'
		AND column_name IN ('fee_rate','method_fee_rates')`, schema).Scan(&feeColumns))
	require.Zero(t, feeColumns, "the upgrade fixture must begin at the actual legacy provider schema")

	feeDDL, err := migrations.FS.ReadFile("355_payment_provider_fee_rates.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SAVEPOINT provider_fee_ddl")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(feeDDL))
	require.NoError(t, err)
	var inherited sql.NullFloat64
	var overrides []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT fee_rate,method_fee_rates
		FROM payment_provider_instances WHERE id=$1`, instanceID).Scan(&inherited, &overrides))
	require.False(t, inherited.Valid, "existing instances must inherit the global rate")
	require.JSONEq(t, `{}`, string(overrides))

	// DDL rollback is isolated to this transaction's schema, never shared tables.
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT provider_fee_ddl")
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=$1 AND table_name='payment_provider_instances'
		AND column_name IN ('fee_rate','method_fee_rates')`, schema).Scan(&feeColumns))
	require.Zero(t, feeColumns, "rolling back the additive DDL must restore the legacy table")
	var afterRollback []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT to_jsonb(instance)
		FROM payment_provider_instances instance WHERE id=$1`, instanceID).Scan(&afterRollback))
	require.JSONEq(t, string(original), string(afterRollback))

	_, err = tx.ExecContext(ctx, string(feeDDL))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO payment_provider_instances(provider_key,name,config)
		VALUES('easypay','Old writer after upgrade','{}')`)
	require.NoError(t, err, "old writers may continue omitting both new columns")
	var defaultRows int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_provider_instances
		WHERE fee_rate IS NULL AND method_fee_rates='{}'::jsonb`).Scan(&defaultRows))
	require.Equal(t, 2, defaultRows)
	_, err = tx.ExecContext(ctx, `UPDATE payment_provider_instances
		SET fee_rate=0,method_fee_rates='{"alipay":0,"wxpay":2.75}'::jsonb WHERE id=$1`, instanceID)
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(feeDDL))
		require.NoError(t, err, "reapplying the migration must preserve explicit fees")
	}
	var rateText string
	var oldFields []byte
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT fee_rate::text,method_fee_rates,
		to_jsonb(instance)-'fee_rate'-'method_fee_rates'
		FROM payment_provider_instances instance WHERE id=$1`, instanceID).Scan(&rateText, &overrides, &oldFields))
	require.Equal(t, "0.00", rateText, "explicit zero must not become inheritance")
	require.JSONEq(t, `{"alipay":0,"wxpay":2.75}`, string(overrides))
	require.JSONEq(t, string(original), string(oldFields), "existing provider fields must remain unchanged")

	require.NoError(t, tx.Rollback())
	var schemaExists bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM pg_namespace WHERE nspname=$1)`, schema).Scan(&schemaExists))
	require.False(t, schemaExists, "transaction rollback must leave no fixture schema behind")
}

func TestPaymentProviderFeeEntPersistsDecimalOverridesAndInheritance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := testEntClient(t)
	instance, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeEasyPay).
		SetName("fee-persistence-" + uuid.NewString()).
		SetConfig(`{}`).
		SetSupportedTypes("alipay,wxpay").
		SetFeeRate(2.35).
		SetMethodFeeRates(map[string]float64{"alipay": 0, "wxpay": 1.25}).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.PaymentProviderInstance.DeleteOneID(instance.ID).Exec(context.Background()))
	})
	reloaded, err := client.PaymentProviderInstance.Get(ctx, instance.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.FeeRate)
	require.Equal(t, 2.35, *reloaded.FeeRate)
	require.Equal(t, map[string]float64{"alipay": 0, "wxpay": 1.25}, reloaded.MethodFeeRates)
	var rateText string
	var raw []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT fee_rate::text,method_fee_rates
		FROM payment_provider_instances WHERE id=$1`, instance.ID).Scan(&rateText, &raw))
	require.Equal(t, "2.35", rateText)
	var persisted map[string]float64
	require.NoError(t, json.Unmarshal(raw, &persisted))
	require.Equal(t, reloaded.MethodFeeRates, persisted, "overrides must be JSONB numbers, including explicit zero")

	_, err = client.PaymentProviderInstance.UpdateOneID(instance.ID).
		SetFeeRate(0).SetMethodFeeRates(map[string]float64{"alipay": 0}).Save(ctx)
	require.NoError(t, err)
	reloaded, err = client.PaymentProviderInstance.Get(ctx, instance.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.FeeRate)
	require.Zero(t, *reloaded.FeeRate)
	require.Equal(t, map[string]float64{"alipay": 0}, reloaded.MethodFeeRates)

	_, err = client.PaymentProviderInstance.UpdateOneID(instance.ID).
		ClearFeeRate().SetMethodFeeRates(map[string]float64{}).Save(ctx)
	require.NoError(t, err)
	reloaded, err = client.PaymentProviderInstance.Get(ctx, instance.ID)
	require.NoError(t, err)
	require.Nil(t, reloaded.FeeRate)
	require.NotNil(t, reloaded.MethodFeeRates)
	require.Empty(t, reloaded.MethodFeeRates)
}

func TestPaymentProviderFeeChangesPreserveFrozenOrderSettlement(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{})
	t.Cleanup(func() { require.NoError(t, client.User.DeleteOneID(user.ID).Exec(context.Background())) })
	instance, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).SetName("frozen-fee-provider").
		SetConfig(`{"currency":"CNY"}`).SetSupportedTypes("card,alipay").
		SetFeeRate(2.35).SetMethodFeeRates(map[string]float64{"card": 2.35}).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.PaymentProviderInstance.DeleteOneID(instance.ID).Exec(context.Background()))
	})
	instanceID := strconv.FormatInt(instance.ID, 10)
	reference := "frozen-fee-" + uuid.NewString()
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName("frozen-fee-user").
		SetAmount(100).SetListAmount(100).SetGatewayBaseAmount(80).
		SetDiscountAmount(20).SetFeeAmount(1.88).SetQualifyingRechargeAmount(80).
		SetPayAmount(81.88).SetFeeRate(2.35).SetPaymentCurrency("CNY").
		SetRechargeCode(reference).SetOutTradeNo(reference).
		SetPaymentType(payment.TypeStripe).SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalance).SetStatus(payment.OrderStatusPending).
		SetProviderInstanceID(instanceID).SetProviderKey(payment.TypeStripe).
		SetProviderSnapshot(map[string]any{
			"schema_version": 2, "provider_instance_id": instanceID, "provider_key": payment.TypeStripe,
			"currency": "CNY", "fee_source": "method", "payment_subtype": "card",
		}).
		SetExpiresAt(time.Now().UTC().Add(time.Hour)).
		SetClientIP("127.0.0.1").SetSrcHost("payments.example.test").Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.PaymentOrder.DeleteOneID(order.ID).Exec(context.Background())) })
	before, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)

	changed, err := client.PaymentProviderInstance.UpdateOneID(instance.ID).
		SetFeeRate(9.99).SetMethodFeeRates(map[string]float64{"card": 0, "alipay": 5}).Save(ctx)
	require.NoError(t, err)
	require.NotNil(t, changed.FeeRate)
	require.Equal(t, 9.99, *changed.FeeRate)
	require.Equal(t, map[string]float64{"card": 0, "alipay": 5}, changed.MethodFeeRates)
	after, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, before.ProviderSnapshot, after.ProviderSnapshot)
	require.Equal(t, before.ProviderInstanceID, after.ProviderInstanceID)
	require.Equal(t, before.ProviderKey, after.ProviderKey)
	require.Equal(t, before.ListAmount, after.ListAmount)
	require.Equal(t, before.GatewayBaseAmount, after.GatewayBaseAmount)
	require.Equal(t, before.DiscountAmount, after.DiscountAmount)
	require.Equal(t, before.FeeRate, after.FeeRate)
	require.Equal(t, before.FeeAmount, after.FeeAmount)
	require.Equal(t, before.PayAmount, after.PayAmount)
	require.Equal(t, before.PaymentCurrency, after.PaymentCurrency)
	require.Equal(t, before.QualifyingRechargeAmount, after.QualifyingRechargeAmount)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt, "config changes must not rewrite historical orders")
}
