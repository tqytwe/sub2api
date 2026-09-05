//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMembershipReconciliationReadsPendingRowsFromMigratedSchema(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("membership-reconciliation-%d@example.com", time.Now().UnixNano()),
	})

	var orderID int64
	err := integrationDB.QueryRowContext(ctx, `
		INSERT INTO payment_orders (
			user_id, amount, pay_amount, expires_at, status, order_type,
			payment_currency, list_amount, gateway_base_amount,
			qualifying_recharge_amount, subscription_snapshot
		) VALUES ($1, 100, 90, NOW() + INTERVAL '1 hour', 'COMPLETED', 'balance',
		          'CNY', 100, 90, 90, NULL)
		RETURNING id`, user.ID).Scan(&orderID)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO play_membership_order_contributions (
			order_id, user_id, order_type, paid_amount, refund_amount, net_amount,
			paid_at, status, qualification_state, qualification_source
		) VALUES ($1, $2, 'balance', 0, 0, 0, NOW(), 'active',
		          'pending_review', 'integration_test')`, orderID, user.ID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM play_membership_order_contributions WHERE order_id = $1", orderID)
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM payment_orders WHERE id = $1", orderID)
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM users WHERE id = $1", user.ID)
	})

	var verifiedViewRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM play_membership_verified_contributions
		WHERE order_id = $1`, orderID).Scan(&verifiedViewRows))
	require.Zero(t, verifiedViewRows, "the verified-only view must exclude pending review rows")

	report, err := service.NewMembershipReconciliationService(integrationDB).Reconcile(ctx,
		service.MembershipReconciliationOptions{AfterOrderID: orderID - 1, Limit: 1})
	require.NoError(t, err)
	require.Equal(t, 1, report.Scanned)
	require.Equal(t, 1, report.Verified)
	require.Equal(t, orderID, report.Items[0].OrderID)
}
