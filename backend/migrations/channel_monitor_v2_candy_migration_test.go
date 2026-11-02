package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2CandyMigrationIsAdditiveAndIdempotent(t *testing.T) {
	sql, err := os.ReadFile("340_channel_monitor_v2_candy.sql")
	require.NoError(t, err)
	script := string(sql)
	require.Contains(t, script, "ADD COLUMN IF NOT EXISTS candy_probes")
	require.Contains(t, script, "ADD COLUMN IF NOT EXISTS candy_history_limit")
	require.Contains(t, script, "ADD COLUMN IF NOT EXISTS card_timeline_mode")
	require.Contains(t, script, "ADD COLUMN IF NOT EXISTS group_order")
	require.Contains(t, script, "CREATE TABLE IF NOT EXISTS channel_monitor_v2_candy_results")
	require.Contains(t, script, "UNIQUE (group_id, config_key, schedule_slot)")
	require.Contains(t, script, "BETWEEN 15 AND 100")
	require.NotContains(t, strings.ToLower(script), "drop table")
	require.NotContains(t, strings.ToLower(script), "delete from")
}
