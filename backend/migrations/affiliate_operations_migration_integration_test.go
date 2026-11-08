//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAffiliateOperationsFactsKeepOriginalMonthAcrossRefundAndRepayment(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err)
		}
		t.Skip("Docker is unavailable")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("sub2api_test"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE users(id bigint PRIMARY KEY);
CREATE TABLE user_affiliates(user_id bigint PRIMARY KEY,inviter_id bigint,created_at timestamptz DEFAULT now(),updated_at timestamptz DEFAULT now());
CREATE TABLE payment_orders(id bigint PRIMARY KEY);
CREATE TABLE play_membership_order_contributions(order_id bigint PRIMARY KEY,user_id bigint,order_type text,paid_amount numeric,refund_amount numeric,net_amount numeric,paid_at timestamptz,status text,qualification_state text);
CREATE TABLE referral_campaign_attributions(id bigint PRIMARY KEY,campaign_id bigint,inviter_id bigint,invitee_id bigint,registered_at timestamptz);
INSERT INTO users(id) VALUES(1),(2),(3),(4);
INSERT INTO user_affiliates(user_id,inviter_id) VALUES(1,NULL),(2,1);
INSERT INTO payment_orders(id) VALUES(10),(11),(12);`)
	require.NoError(t, err)
	raw, err := dbmigrations.FS.ReadFile("350_affiliate_operations.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	var evidence string
	require.NoError(t, db.QueryRow(`SELECT evidence_status FROM affiliate_paid_invite_facts WHERE invitee_id=2`).Scan(&evidence))
	require.Equal(t, "pending", evidence)
	_, err = db.Exec(`INSERT INTO user_affiliates(user_id,inviter_id,bound_at) VALUES(3,1,'2026-01-01T00:00:00Z');
INSERT INTO play_membership_order_contributions VALUES(10,3,'balance',20,0,20,'2026-01-31T15:59:59Z','COMPLETED','verified');`)
	require.NoError(t, err)
	var month string
	var net float64
	read := func() {
		require.NoError(t, db.QueryRow(`SELECT first_paid_month,net_paid::double precision FROM affiliate_paid_invite_facts WHERE invitee_id=3`).Scan(&month, &net))
	}
	read()
	require.Equal(t, "2026-01", month)
	require.Equal(t, 20.0, net)
	_, err = db.Exec(`UPDATE play_membership_order_contributions SET refund_amount=20,net_amount=0,status='REFUNDED' WHERE order_id=10`)
	require.NoError(t, err)
	read()
	require.Equal(t, "2026-01", month)
	require.Zero(t, net)
	_, err = db.Exec(`INSERT INTO play_membership_order_contributions VALUES(11,3,'subscription',30,0,30,'2026-02-02T00:00:00Z','COMPLETED','verified')`)
	require.NoError(t, err)
	read()
	require.Equal(t, "2026-01", month)
	require.Equal(t, 30.0, net)
	_, err = db.Exec(`INSERT INTO user_affiliates(user_id,inviter_id,bound_at) VALUES(4,1,'2026-01-01T00:00:00Z');
INSERT INTO play_membership_order_contributions VALUES(12,4,'balance',20,0,20,'2026-01-31T16:00:00Z','COMPLETED','verified');`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT first_paid_month FROM affiliate_paid_invite_facts WHERE invitee_id=4`).Scan(&month))
	require.Equal(t, "2026-02", month)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	read()
	require.Equal(t, "2026-01", month)
	require.Equal(t, 30.0, net)
	_, err = db.Exec(`INSERT INTO users(id) VALUES(5),(6),(7);
INSERT INTO payment_orders(id) VALUES(13),(14);
INSERT INTO play_membership_order_contributions VALUES(13,5,'balance',10,0,10,'2025-12-01','COMPLETED','verified');
INSERT INTO user_affiliates(user_id,inviter_id,bound_at) VALUES(5,1,'2026-01-01');
INSERT INTO play_membership_order_contributions VALUES(14,5,'balance',10,0,10,'2026-02-01','COMPLETED','verified');`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT evidence_status FROM affiliate_paid_invite_facts WHERE invitee_id=5`).Scan(&evidence))
	require.Equal(t, "pending", evidence, "a pre-existing paying customer cannot become a new paid invite")
	_, err = db.Exec(`INSERT INTO referral_campaign_attributions VALUES(1,1,1,6,'2026-01-01');`)
	require.NoError(t, err)
	var inviter int64
	require.NoError(t, db.QueryRow(`SELECT inviter_id,evidence_status FROM affiliate_paid_invite_facts WHERE invitee_id=6`).Scan(&inviter, &evidence))
	require.EqualValues(t, 1, inviter)
	require.Equal(t, "verified", evidence)
	_, err = db.Exec(`INSERT INTO referral_campaign_attributions VALUES(2,2,7,6,'2026-01-01');`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT evidence_status FROM affiliate_paid_invite_facts WHERE invitee_id=6`).Scan(&evidence))
	require.Equal(t, "conflict", evidence)
	_, err = db.Exec(`DELETE FROM play_membership_order_contributions WHERE order_id=10;DELETE FROM payment_orders WHERE id=10;`)
	require.NoError(t, err, "the honor board must not add a financial order deletion restriction")
	require.NoError(t, db.QueryRow(`SELECT evidence_status,first_paid_month FROM affiliate_paid_invite_facts WHERE invitee_id=3`).Scan(&evidence, &month))
	require.Equal(t, "pending", evidence)
	require.Equal(t, "2026-01", month, "removed evidence must not re-anchor the user to their later repayment month")
	_, err = db.Exec(`INSERT INTO users(id) VALUES(8);INSERT INTO payment_orders(id) VALUES(15),(16);
INSERT INTO user_affiliates(user_id,inviter_id,bound_at) VALUES(8,1,'2026-01-01');
INSERT INTO play_membership_order_contributions VALUES(16,8,'balance',10,0,10,'2026-02-01','COMPLETED','verified');
INSERT INTO play_membership_order_contributions VALUES(15,8,'balance',10,0,10,'2026-01-30','COMPLETED','verified');`)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT first_paid_month FROM affiliate_paid_invite_facts WHERE invitee_id=8`).Scan(&month))
	require.Equal(t, "2026-01", month, "out-of-order callbacks must use the earlier actual payment, never arrival order")
	var ruleVersion string
	require.NoError(t, db.QueryRow(`SELECT rule_version FROM affiliate_paid_invite_facts WHERE invitee_id=8`).Scan(&ruleVersion))
	require.Equal(t, "paid-invite-v1", ruleVersion)
}
