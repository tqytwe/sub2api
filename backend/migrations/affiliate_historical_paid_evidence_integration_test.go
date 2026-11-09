//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAffiliateHistoricalFirstPaymentRebateProvesAttributionWithoutInventingBinding(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:18.1-alpine3.23", tcpostgres.WithDatabase("sub2api_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE users(id bigint PRIMARY KEY);
CREATE TABLE user_affiliates(user_id bigint PRIMARY KEY,inviter_id bigint,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now());
CREATE TABLE payment_orders(id bigint PRIMARY KEY,user_id bigint);
CREATE TABLE play_membership_order_contributions(order_id bigint PRIMARY KEY,user_id bigint,order_type text,paid_amount numeric,refund_amount numeric,net_amount numeric,paid_at timestamptz,status text,qualification_state text);
CREATE TABLE referral_campaign_attributions(id bigint PRIMARY KEY,campaign_id bigint,inviter_id bigint,invitee_id bigint,registered_at timestamptz);
CREATE TABLE user_affiliate_ledger(id bigserial PRIMARY KEY,user_id bigint,source_user_id bigint,source_order_id bigint,action text,amount numeric,referral_reward_id bigint);
CREATE TABLE payment_audit_logs(id bigserial PRIMARY KEY,order_id text,action text,detail text);
INSERT INTO users SELECT generate_series(1,9);
INSERT INTO user_affiliates(user_id,inviter_id) SELECT n,1 FROM generate_series(2,8) n;
INSERT INTO payment_orders SELECT n,n/10 FROM generate_series(20,89) n;
INSERT INTO play_membership_order_contributions
 SELECT n*10,n,'balance',20,0,20,'2026-10-31T16:00:00Z','COMPLETED','verified' FROM generate_series(2,8) n;
INSERT INTO user_affiliate_ledger(user_id,source_user_id,source_order_id,action,amount)
 SELECT 1,n,n*10,'accrue',2 FROM generate_series(2,8) n WHERE n<>3;
INSERT INTO payment_audit_logs(order_id,action,detail)
 SELECT (n*10)::text,'AFFILIATE_REBATE_APPLIED',CASE WHEN n=4 THEN '{"rebateAmount":3}' ELSE '{"rebateAmount":2}' END FROM generate_series(2,8) n WHERE n<>5;
-- A rebate only on the second payment cannot prove the first one's attribution.
UPDATE user_affiliate_ledger SET source_order_id=61 WHERE source_user_id=6;
INSERT INTO play_membership_order_contributions VALUES(69,7,'balance',5,0,5,'2026-10-01','COMPLETED','pending_review');
INSERT INTO referral_campaign_attributions VALUES(1,1,9,8,'2026-09-01');`)
	require.NoError(t, err)
	apply := func(name string) {
		raw, readErr := dbmigrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		_, execErr := db.Exec(string(raw))
		require.NoError(t, execErr)
	}
	apply("350_affiliate_operations.sql")
	var evidence, source, month string
	var bound sql.NullTime
	var net float64
	read := func(user int) {
		require.NoError(t, db.QueryRow(`SELECT evidence_status,evidence_source,first_paid_month,bound_at,net_paid::float8 FROM affiliate_paid_invite_facts WHERE invitee_id=$1`, user).Scan(&evidence, &source, &month, &bound, &net))
	}
	read(2)
	require.Equal(t, "pending", evidence, "regression: the original migration discards a proven first-payment rebate")
	apply("352_affiliate_historical_paid_evidence.sql")
	read(2)
	require.Equal(t, "verified", evidence)
	require.Equal(t, "first_payment_rebate", source)
	require.False(t, bound.Valid, "proof of attribution must not manufacture a binding date")
	require.Equal(t, "2026-11", month)
	require.Equal(t, 20.0, net)
	for _, user := range []int{3, 4, 5, 6, 7} {
		read(user)
		require.Equal(t, "pending", evidence, "incomplete or inconsistent evidence must stay pending")
	}
	read(8)
	require.Equal(t, "conflict", evidence)
	// Audit completion happens after contribution creation. Its mutation must
	// refresh the read model without polling all historical orders.
	_, err = db.Exec(`INSERT INTO payment_audit_logs(order_id,action,detail) VALUES('50','AFFILIATE_REBATE_APPLIED','{"rebateAmount":2}')`)
	require.NoError(t, err)
	read(5)
	require.Equal(t, "verified", evidence)
	_, err = db.Exec(`UPDATE payment_audit_logs SET detail='{"rebateAmount":2oops}' WHERE order_id='50'`)
	require.NoError(t, err)
	read(5)
	require.Equal(t, "pending", evidence, "malformed audit JSON cannot verify payment attribution")
	_, err = db.Exec(`UPDATE play_membership_order_contributions SET refund_amount=5,net_amount=15,status='PARTIALLY_REFUNDED' WHERE order_id=20`)
	require.NoError(t, err)
	read(2)
	require.Equal(t, 15.0, net)
	_, err = db.Exec(`UPDATE play_membership_order_contributions SET refund_amount=20,net_amount=0,status='REFUNDED' WHERE order_id=20`)
	require.NoError(t, err)
	read(2)
	require.Equal(t, "2026-11", month)
	require.Zero(t, net)
	_, err = db.Exec(`INSERT INTO play_membership_order_contributions VALUES(21,2,'subscription',30,0,30,'2026-12-01','COMPLETED','verified')`)
	require.NoError(t, err)
	read(2)
	require.Equal(t, "2026-11", month)
	require.Equal(t, 30.0, net)
	apply("352_affiliate_historical_paid_evidence.sql")
	read(2)
	require.Equal(t, 30.0, net)
	var ledgerCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM user_affiliate_ledger`).Scan(&ledgerCount))
	require.Equal(t, 6, ledgerCount, "evidence recovery cannot rewrite financial ledgers")
	_, err = db.Exec(`DELETE FROM payment_audit_logs WHERE order_id='20'`)
	require.NoError(t, err)
	read(2)
	require.Equal(t, "pending", evidence)
	require.Equal(t, "2026-11", month)
}
