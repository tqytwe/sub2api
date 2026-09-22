package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestManualConfirmPaymentDurablyRecordsPaidTransitionBeforeFulfillment(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")

	// No fulfillment dependencies are installed. The operation must still make
	// the paid transition durable and leave the ordinary retry path available.
	result, err := (&PaymentService{entClient: client}).ManualConfirmPayment(ctx, order.ID, ManualConfirmPaymentInput{
		GatewayTransactionReference: "  manual-gateway-123  ",
		Operator:                    "admin:42",
	})
	require.NoError(t, err)
	require.True(t, result.FulfillmentPending)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, reloaded.PaidAt)
	require.Equal(t, "manual-gateway-123", reloaded.PaymentTradeNo)
	require.Equal(t, OrderStatusPaid, reloaded.Status)

	audits, err := client.PaymentAuditLog.Query().Where(
		paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)),
		paymentauditlog.ActionEQ(ManualPaymentConfirmedAuditAction),
	).All(ctx)
	require.NoError(t, err)
	require.Len(t, audits, 1)
	require.Equal(t, "admin:42", audits[0].Operator)
	require.Contains(t, audits[0].Detail, "manual-gateway-123")
	require.NotContains(t, audits[0].Detail, "amount")
}

func TestManualConfirmPaymentRejectsIneligibleStatesAndReferences(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	for _, status := range []string{OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted, OrderStatusRefunding, OrderStatusRefunded} {
		t.Run(status, func(t *testing.T) {
			order := createManualConfirmOrder(t, ctx, client, status, "")
			_, err := svc.ManualConfirmPayment(ctx, order.ID, ManualConfirmPaymentInput{GatewayTransactionReference: "blocked-" + status, Operator: "admin:1"})
			require.Error(t, err)
			require.Equal(t, "INVALID_STATUS", infraerrors.Reason(err))
		})
	}

	order := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "known-reference")
	_, err := svc.ManualConfirmPayment(ctx, order.ID, ManualConfirmPaymentInput{GatewayTransactionReference: "other-reference", Operator: "admin:1"})
	require.Error(t, err)
	require.Equal(t, "PAYMENT_REFERENCE_MISMATCH", infraerrors.Reason(err))

	_, err = svc.ManualConfirmPayment(ctx, order.ID, ManualConfirmPaymentInput{GatewayTransactionReference: " ", Operator: "admin:1"})
	require.Error(t, err)
	require.Equal(t, "INVALID_PAYMENT_REFERENCE", infraerrors.Reason(err))
}

func TestManualConfirmPaymentRejectsDuplicateProviderReference(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	first := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")
	_, err := svc.ManualConfirmPayment(ctx, first.ID, ManualConfirmPaymentInput{GatewayTransactionReference: "shared-reference", Operator: "admin:1"})
	require.NoError(t, err)

	second := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")
	_, err = svc.ManualConfirmPayment(ctx, second.ID, ManualConfirmPaymentInput{GatewayTransactionReference: "shared-reference", Operator: "admin:2"})
	require.Error(t, err)
	require.Equal(t, "DUPLICATE_PAYMENT_REFERENCE", infraerrors.Reason(err))
}

func TestManualConfirmPaymentScopesReferenceToTheResolvedProviderIdentity(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}

	first := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")
	_, err := first.Update().SetProviderKey("primary-gateway").Save(ctx)
	require.NoError(t, err)
	_, err = svc.ManualConfirmPayment(ctx, first.ID, ManualConfirmPaymentInput{
		GatewayTransactionReference: "provider-scoped-reference",
		Operator:                    "admin:1",
	})
	require.NoError(t, err)

	// An external gateway can legitimately reuse a transaction string in another
	// provider namespace. The database expression and service check must agree.
	second := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")
	_, err = second.Update().SetProviderKey("backup-gateway").Save(ctx)
	require.NoError(t, err)
	_, err = svc.ManualConfirmPayment(ctx, second.ID, ManualConfirmPaymentInput{
		GatewayTransactionReference: "provider-scoped-reference",
		Operator:                    "admin:2",
	})
	require.NoError(t, err)

	third := createManualConfirmOrder(t, ctx, client, OrderStatusPending, "")
	_, err = third.Update().SetProviderKey("PRIMARY-GATEWAY").Save(ctx)
	require.NoError(t, err)
	_, err = svc.ManualConfirmPayment(ctx, third.ID, ManualConfirmPaymentInput{
		GatewayTransactionReference: "provider-scoped-reference",
		Operator:                    "admin:3",
	})
	require.Error(t, err)
	require.Equal(t, "DUPLICATE_PAYMENT_REFERENCE", infraerrors.Reason(err))
}

func createManualConfirmOrder(t *testing.T, ctx context.Context, client *dbent.Client, status, tradeNo string) *dbent.PaymentOrder {
	t.Helper()
	seed := strconv.FormatInt(time.Now().UnixNano(), 10)
	user, err := client.User.Create().
		SetEmail("manual-confirm-" + seed + "@example.com").
		SetPasswordHash("hash").
		SetUsername("manual-confirm-user-" + seed).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(50.5).
		SetPayAmount(50.5).
		SetListAmount(50.5).
		SetGatewayBaseAmount(50).
		SetFeeAmount(0.5).
		SetQualifyingRechargeAmount(50).
		SetPaymentCurrency(payment.DefaultPaymentCurrency).
		SetRechargeCode("PAY-MANUAL-" + seed).
		SetOutTradeNo("sub2_manual_" + seed).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo(tradeNo).
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(status).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	return order
}
