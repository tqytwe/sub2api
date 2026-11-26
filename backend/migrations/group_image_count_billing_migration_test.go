package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupImageCountBillingMigrationIsAdditiveAndDefaultsOff(t *testing.T) {
	content, err := os.ReadFile("432_group_image_count_billing.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE groups")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS image_count_billing BOOLEAN NOT NULL DEFAULT false")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.image_count_billing")
	require.NotContains(t, strings.ToUpper(sql), "UPDATE GROUPS")
	require.NotContains(t, strings.ToUpper(sql), "DELETE FROM")
}
