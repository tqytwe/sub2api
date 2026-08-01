package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArenaSeasonMigrationFreezesRulesAndPersistsHistoricalProof(t *testing.T) {
	raw, err := FS.ReadFile("241_play_growth_competition.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "reward_rules_json")
	require.Contains(t, sql, "create table if not exists play_arena_season_snapshots")
	require.Contains(t, sql, "unique (period_id, rank)")
	require.Contains(t, sql, "unique (period_id, user_id)")
	require.Contains(t, sql, "payout_status")
	require.Contains(t, sql, "asia/shanghai")
}
