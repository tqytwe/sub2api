package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualityBPSCoexistMigration(t *testing.T) {
	content, err := FS.ReadFile("282_quality_bps_coexist.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP INDEX IF EXISTS scheduled_test_quality_account_unique")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS scheduled_test_quality_account_scope_unique")
	require.Contains(t, sql, "ON scheduled_test_plans (account_id,")
	require.Contains(t, sql, "pelican_config->'quality'->>'action' = 'enable_bps' THEN 'bps' ELSE 'quarantine' END")
	require.Contains(t, sql, "WHERE pelican_config->'quality' IS NOT NULL")
}
