package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogDailyCardEntitlementsMigration(t *testing.T) {
	sql, err := os.ReadFile("233_usage_log_daily_card_entitlements.sql")
	require.NoError(t, err)

	text := string(sql)
	require.Contains(t, text, "ADD COLUMN IF NOT EXISTS subscription_entitlement_id BIGINT")
	require.Contains(t, text, "REFERENCES subscription_entitlements(id) ON DELETE SET NULL")
	require.Contains(t, text, "idx_usage_logs_subscription_entitlement_id")
	require.Contains(t, text, "idx_usage_logs_subscription_entitlement_created")
	require.Contains(t, text, "COUNT(*) OVER (PARTITION BY usage.id) AS match_count")
	require.Contains(t, text, "candidates.match_count = 1")
	require.Contains(t, text, "GREATEST(entitlement.quota_used_usd, entitlement_usage.used_usd)")
	require.Contains(t, text, "LEAST(reconciled.used_usd, entitlement.quota_limit_usd)")
	require.Contains(t, text, "UPDATE subscription_entitlement_holds AS hold")
	require.Contains(t, text, "hold.status = 'reserved'")
}
