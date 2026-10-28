//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualityOperationsCoreMigrationPreservesLegacyPlansAndAddsRoundOwnership(t *testing.T) {
	content, err := FS.ReadFile("320_quality_operations_core.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, contract := range []string{
		"ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS pelican_config JSONB",
		"ALTER TABLE scheduled_test_plans ADD COLUMN IF NOT EXISTS running_until TIMESTAMPTZ",
		"ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS pelican_config JSONB",
		"ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_round_id TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_action TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE scheduled_test_results ADD COLUMN IF NOT EXISTS quality_judgment JSONB",
		"CREATE TABLE IF NOT EXISTS account_quality_states",
		"plan_id BIGINT PRIMARY KEY REFERENCES scheduled_test_plans(id) ON DELETE CASCADE",
	} {
		require.Contains(t, sql, contract)
	}
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_scheduled_test_results_quality_round")
	require.NotContains(t, sql, "UPDATE scheduled_test_results")
	require.NotContains(t, sql, "DELETE FROM scheduled_test_results")
}
