package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithdrawalsCP5MigrationContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("211_withdrawals.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "withdrawal_system_settings")
	require.Contains(t, sql, "global_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "minimum_amount NUMERIC(20,8) NOT NULL DEFAULT 10.00000000")
	require.Contains(t, sql, "daily_limit_amount NUMERIC(20,8) NOT NULL DEFAULT 500.00000000")
	require.Contains(t, sql, "double_review_threshold NUMERIC(20,8) NOT NULL DEFAULT 100.00000000")

	require.Contains(t, sql, "user_withdrawal_settings")
	require.Contains(t, sql, "enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "withdrawal_recalc_status = 'ready'")

	require.Contains(t, sql, "withdrawal_payout_accounts")
	require.Contains(t, sql, "account_encrypted TEXT NOT NULL")
	require.Contains(t, sql, "account_mask TEXT NOT NULL")
	require.Contains(t, sql, "method IN ('alipay', 'bank_transfer', 'other')")
	require.Contains(t, sql, "currency IN ('CNY', 'USD')")

	require.Contains(t, sql, "withdrawal_requests")
	require.Contains(t, sql, "account_snapshot_encrypted TEXT NOT NULL")
	require.Contains(t, sql, "status IN ('pending_review', 'second_review', 'payout_pending', 'paid', 'rejected', 'canceled')")
	require.Contains(t, sql, "amount_scale_two_decimals")
	require.Contains(t, sql, "external_fee_amount NUMERIC(20,8) NOT NULL DEFAULT 0")

	require.Contains(t, sql, "withdrawal_status_events")
	require.Contains(t, sql, "withdrawal_request_entitlements")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS uq_withdrawal_requests_user_in_progress")
	require.Contains(t, sql, "COMMENT ON TABLE withdrawal_requests")
}
