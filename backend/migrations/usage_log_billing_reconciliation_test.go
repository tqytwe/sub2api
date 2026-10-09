package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsageLogBillingReconciliationMigrationIsForwardOnly(t *testing.T) {
	content, err := FS.ReadFile("274_usage_log_billing_reconciliation.sql")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(strings.Join(strings.Fields(string(content)), " "),
		"SET LOCAL lock_timeout = '2s'; SET LOCAL statement_timeout = '30s';"),
		"transaction-local timeout guards must precede all migration DDL")
	sql := strings.ToUpper(string(content))
	require.Contains(t, sql, "BILLING_REQUEST_FINGERPRINT TEXT")
	require.Contains(t, sql, "BILLING_SETTLED BOOLEAN NOT NULL DEFAULT FALSE")
	require.NotContains(t, sql, "UPDATE USAGE_LOGS")
	trigger, err := FS.ReadFile("222_group_usage_daily_rollups.sql")
	require.NoError(t, err)
	require.Contains(t, strings.ToLower(string(trigger)), "actual_cost")
}
