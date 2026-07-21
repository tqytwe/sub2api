package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayArenaDailyRewardSummaryMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("209_play_arena_daily_reward_summary.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	normalized := strings.Join(strings.Fields(sql), " ")

	require.Contains(t, normalized, "alter table play_arena_periods add column if not exists settled_at timestamptz")
	require.Contains(t, normalized, "set settled_at = updated_at")
	require.Contains(t, normalized, "where status = 'settled' and settled_at is null")
	require.Contains(t, normalized, "create index if not exists idx_play_arena_periods_daily_settled")
	require.Contains(t, normalized, "period_type = 'daily'")
	require.NotContains(t, normalized, "update play_reward_ledger")
	require.NotContains(t, normalized, "delete from play_reward_ledger")
	require.NotContains(t, normalized, "begin")
	require.NotContains(t, normalized, "commit")
}
