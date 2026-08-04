package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyCardRequestReplayMigrationPreservesHistoricalHoldMeaning(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("247_daily_card_request_replays.sql"))
	require.NoError(t, err)
	text := string(data)
	require.Contains(t, text, "UNIQUE (user_id, group_id, client_request_id)")
	require.Contains(t, text, "usage_billing_dedup")
	require.Contains(t, text, "usage_logs")
	require.Contains(t, text, "hold.status = 'captured'")
	require.Contains(t, text, "WHERE status = 'reserved'")
	require.Contains(t, text, "SET quota_reserved_usd = 0")
	require.NotContains(t, text, "WHERE hold.status IN ('captured', 'released', 'reserved')")
}
