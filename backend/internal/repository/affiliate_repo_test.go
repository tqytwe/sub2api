package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestAffiliateDefaultTeamJoinSQLUsesOnlyActiveInviterTeam(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateDefaultTeamJoinSQL), " ")

	require.Contains(t, query, "JOIN play_teams t ON t.id = m.team_id")
	require.Contains(t, query, "m.user_id = $1")
	require.Contains(t, query, "m.left_at IS NULL")
	require.Contains(t, query, "t.archived_at IS NULL")
	require.Contains(t, query, "INSERT INTO play_team_members (team_id, user_id)")
	require.Contains(t, query, "ON CONFLICT DO NOTHING")
	require.Contains(t, query, "INSERT INTO play_team_events")
	require.NotContains(t, query, "invite_code")
}
