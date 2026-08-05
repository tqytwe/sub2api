package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestoreUpstreamDailySubscriptionUsageMigrationIsNonDestructive(t *testing.T) {
	content, err := FS.ReadFile("248_restore_upstream_daily_subscription_usage.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(normalizeMigrationSQL(string(content)))
	require.Contains(t, sql, "TO_REGCLASS('PUBLIC.SUBSCRIPTION_ENTITLEMENTS')")
	require.Contains(t, sql, "GREATEST(")
	require.Contains(t, sql, "INTERVAL '1 DAY'")
	require.NotContains(t, sql, "DROP TABLE")
	require.NotContains(t, sql, "TRUNCATE")
	require.NotContains(t, sql, "DELETE FROM")
}
