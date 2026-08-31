package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayGrowthEligibilityOrdersIndexMigrationIsOnlineAndBounded(t *testing.T) {
	raw, err := FS.ReadFile("265_play_growth_eligibility_orders_index_notx.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(string(raw))
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS IDX_PAYMENT_ORDERS_GROWTH_ELIGIBILITY_BALANCE_COMPLETED")
	require.Contains(t, sql, "ON PAYMENT_ORDERS (USER_ID, COMPLETED_AT DESC)")
	require.Contains(t, sql, "ORDER_TYPE = 'BALANCE'")
	require.Contains(t, sql, "STATUS = 'COMPLETED'")
	require.Contains(t, sql, "COMPLETED_AT IS NOT NULL")
	require.NotContains(t, sql, "DROP INDEX")
}
