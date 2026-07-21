package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlanStorefrontMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("208_subscription_plan_storefront.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upperSQL := strings.ToUpper(sql)

	require.Contains(t, upperSQL, "ALTER TABLE SUBSCRIPTION_PLANS")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS STOREFRONT_PLATFORM VARCHAR(50) NOT NULL DEFAULT ''")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS STOREFRONT_CATEGORY VARCHAR(50) NOT NULL DEFAULT ''")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS STOREFRONT_FEATURED BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS STOREFRONT_BADGE VARCHAR(64) NOT NULL DEFAULT ''")
	require.Contains(t, upperSQL, "UPDATE SUBSCRIPTION_PLANS SET STOREFRONT_CATEGORY = CASE")
	require.Contains(t, upperSQL, "CREATE INDEX IF NOT EXISTS IDX_SUBSCRIPTION_PLANS_STOREFRONT_PLATFORM")
	require.Contains(t, upperSQL, "CREATE INDEX IF NOT EXISTS IDX_SUBSCRIPTION_PLANS_STOREFRONT_CATEGORY")
	require.Contains(t, upperSQL, "CREATE INDEX IF NOT EXISTS IDX_SUBSCRIPTION_PLANS_STOREFRONT_FEATURED")
	require.NotContains(t, upperSQL, "DROP TABLE")
	require.NotContains(t, upperSQL, "TRUNCATE")
	require.NotContains(t, upperSQL, "DELETE FROM")
}
