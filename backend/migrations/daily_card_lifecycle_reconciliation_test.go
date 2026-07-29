package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCardLifecycleReconciliationMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("230_daily_card_lifecycle_reconciliation.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "GREATEST(")
	require.Contains(t, sql, "entitlement.quota_used_usd")
	require.Contains(t, sql, "subscription.daily_usage_usd")
	require.Contains(t, sql, "usage_logs")
	require.Contains(t, sql, "LEAST(")
	require.Contains(t, sql, "entitlement.quota_limit_usd")
	require.Contains(t, sql, "THEN 'exhausted'")
	require.Contains(t, sql, "status = 'active'")
	require.Contains(t, sql, "status = 'pending'")
	require.NotContains(t, sql, "quota_used_usd = 0")
}

func TestAllowZeroDailyCardHoldsMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("231_allow_zero_daily_card_holds.sql")
	require.NoError(t, err)
	sql := string(raw)

	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS subscription_entitlement_holds_amount_check")
	require.Contains(t, sql, "reserved_usd >= 0")
	require.Contains(t, sql, "captured_usd >= 0")
	require.Contains(t, sql, "reserved_usd = 0")
	require.Contains(t, sql, "captured_usd <= reserved_usd")
}
