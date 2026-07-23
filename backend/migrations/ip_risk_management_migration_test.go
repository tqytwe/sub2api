package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPRiskManagementMigrationKeepsAutomationOffAndAddsRollbackAudit(t *testing.T) {
	t.Parallel()

	content, err := FS.ReadFile("215_ip_risk_management.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Contains(t, sql, "create table if not exists ip_risk_config")
	require.Contains(t, sql, `"auto_block_enabled": false`)
	require.Contains(t, sql, `"auto_block_duration_minutes": 30`)
	require.Contains(t, sql, `"event_retention_days": 90`)
	require.Contains(t, sql, `"case_retention_days": 365`)
	require.Contains(t, sql, "add column if not exists case_version")
	require.Contains(t, sql, "add column if not exists rollback_of_action_id")
	require.Contains(t, sql, "add column if not exists last_notified_level")
	require.Contains(t, sql, "add column if not exists last_notified_at")
	require.Contains(t, sql, "add column if not exists source_action_id")
	require.Contains(t, sql, "create unique index if not exists uq_ip_risk_actions_preview_token")
	require.Contains(t, sql, "where preview_token_hash is not null")
	require.Contains(t, sql, "create unique index if not exists uq_ip_risk_actions_rollback_once")
	require.Contains(t, sql, "where rollback_of_action_id is not null")
	require.NotContains(t, sql, `"auto_block_enabled": true`)
	require.NotContains(t, sql, "drop table")
	require.NotContains(t, sql, "truncate")
}
