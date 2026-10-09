package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountGroupAllowedModelsMigration(t *testing.T) {
	content, err := FS.ReadFile("273_account_group_allowed_models.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.True(t, strings.HasPrefix(sql, "SET LOCAL lock_timeout = '2s'; SET LOCAL statement_timeout = '30s';"),
		"transaction-local timeout guards must precede all migration DDL")
	require.Contains(t, sql, "ALTER TABLE account_groups ADD COLUMN IF NOT EXISTS allowed_models JSONB")
	require.NotContains(t, sql, "NOT NULL")
	require.NotContains(t, sql, "UPDATE ")
	require.NotContains(t, sql, "DROP ")
}
