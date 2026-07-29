package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCardRedeemEntitlementSourcesMigrationContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("232_daily_card_redeem_entitlement_sources.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_type")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS source_id")
	require.Contains(t, sql, "ALTER COLUMN payment_order_id DROP NOT NULL")
	require.Contains(t, sql, "idx_subscription_entitlements_source")
	require.Contains(t, sql, "source_type IN ('payment_order', 'redeem_code', 'admin_grant', 'backfill')")
	require.Contains(t, sql, "canonical_daily_plan")
	require.Contains(t, sql, "user_subscriptions AS us")
	require.Contains(t, sql, "sp.quota_mode = 'one_time'")
	require.Contains(t, sql, "us.status = 'active'")
	require.Contains(t, sql, "existing.user_id = us.user_id")
	require.Contains(t, sql, "existing.status IN ('active', 'pending')")
	require.Contains(t, sql, "existing.created_at >= us.starts_at")
	require.Contains(t, sql, "existing.starts_at < us.expires_at")
	require.Contains(t, sql, "'backfill'")
	require.Contains(t, sql, "'user_subscription:' || subscription_id::text")
	require.Contains(t, sql, "quota_used_usd")
	require.Contains(t, sql, "0")
	require.NotContains(t, sql, "usage_logs")
	require.NotContains(t, sql, "daily_usage_usd = 0")
	require.NotContains(t, sql, "UPDATE user_subscriptions")
}
