package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPRiskFoundationMigrationDeclaresRequiredPrivacyAndAuditTables(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("214_ip_risk_foundation.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, table := range []string{
		"auth_risk_events",
		"ip_risk_cases",
		"ip_risk_case_users",
		"ip_risk_policies",
		"ip_risk_scans",
		"ip_risk_actions",
		"ip_risk_action_items",
	} {
		require.Contains(t, sql, "create table if not exists "+table)
	}

	require.Contains(t, sql, "ip_address inet")
	require.Contains(t, sql, "primary_ip inet")
	require.Contains(t, sql, "primary_network cidr")
	require.Contains(t, sql, "user_agent_hmac bytea")
	require.Contains(t, sql, "invitation_hmac bytea")
	require.Contains(t, sql, "affiliate_hmac bytea")
	require.Contains(t, sql, "evidence_confidence")
	require.Contains(t, sql, "action_snapshot jsonb")
	require.Contains(t, sql, "rollback_status")
	require.Contains(t, sql, "create or replace function ip_risk_try_parse_inet")
	require.NotContains(t, sql, "pg_input_is_valid")
}
