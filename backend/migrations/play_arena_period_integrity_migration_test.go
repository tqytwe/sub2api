package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArenaPeriodIntegrityMigrationProtectsPayoutEvidence(t *testing.T) {
	raw, err := FS.ReadFile("244_play_arena_period_integrity.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	require.Contains(t, sql, "duplicate active play arena periods have payout evidence")
	require.Contains(t, sql, "play_reward_ledger")
	require.Contains(t, sql, "arena_settlement")
	require.Contains(t, sql, "on play_arena_periods (period_type, start_at)")
	require.Contains(t, sql, "where status = 'active'")
	require.Contains(t, sql, "set status = 'draft'")
}
