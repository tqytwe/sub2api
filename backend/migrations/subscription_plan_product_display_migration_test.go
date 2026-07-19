package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubscriptionPlanProductDisplayMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("207_subscription_plan_product_display.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upperSQL := strings.ToUpper(sql)

	require.Contains(t, upperSQL, "ALTER TABLE SUBSCRIPTION_PLANS")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS COVER_IMAGE_URL TEXT NOT NULL DEFAULT ''")
	require.Contains(t, upperSQL, "ADD COLUMN IF NOT EXISTS DETAIL_DESCRIPTION TEXT NOT NULL DEFAULT ''")
	require.NotContains(t, upperSQL, "DROP TABLE")
	require.NotContains(t, upperSQL, "TRUNCATE")
	require.NotContains(t, upperSQL, "DELETE FROM")
}
