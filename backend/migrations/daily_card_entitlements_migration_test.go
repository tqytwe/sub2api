package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCardEntitlementsMigrationContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("228_daily_card_entitlements.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS subscription_entitlements")
	require.Contains(t, sql, "payment_order_id BIGINT NOT NULL")
	require.Contains(t, sql, "quota_mode VARCHAR(24) NOT NULL")
	require.Contains(t, sql, "quota_limit_usd NUMERIC(20,10) NOT NULL")
	require.Contains(t, sql, "quota_used_usd NUMERIC(20,10) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "quota_reserved_usd NUMERIC(20,10) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "status IN ('pending', 'active', 'exhausted', 'expired', 'revoked')")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlements_order")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlements_one_active")
	require.Contains(t, sql, "WHERE status = 'active'")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS subscription_entitlement_holds")
	require.Contains(t, sql, "request_id VARCHAR(200) NOT NULL")
	require.Contains(t, sql, "status IN ('reserved', 'captured', 'released')")
	require.Contains(t, sql, "subscription_snapshot JSONB")
	require.Contains(t, sql, "sp.storefront_category = 'daily'")
	require.Contains(t, sql, "g.daily_limit_usd > 0")
	require.Contains(t, sql, "WITH RECURSIVE eligible_orders")
	require.Contains(t, sql, "usage_logs")
	require.Contains(t, sql, "GREATEST(COALESCE(us.daily_usage_usd, 0), COALESCE(log_usage.used_usd, 0))")
	require.Contains(t, sql, "ON CONFLICT (payment_order_id) DO NOTHING")
	require.NotContains(t, sql, "SET daily_usage_usd = 0")
}
