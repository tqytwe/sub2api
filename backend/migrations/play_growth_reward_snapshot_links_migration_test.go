package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayGrowthRewardSnapshotLinksMigrationPreservesHistoryAndAddsStrongRelations(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("266_play_growth_reward_snapshot_links.sql"))
	require.NoError(t, err)
	sql := strings.ToUpper(string(content))

	require.Contains(t, sql, "ALTER TABLE PLAY_BLINDBOX_OPENS")
	require.Contains(t, sql, "ALTER TABLE PLAY_REWARD_LEDGER")
	require.Contains(t, sql, "GROWTH_ELIGIBILITY_SNAPSHOT_ID BIGINT NULL")
	require.Contains(t, sql, "REFERENCES PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS(ID) ON DELETE RESTRICT")
	require.Contains(t, sql, "GROWTH_RULE_VERSION VARCHAR(32) NULL")
	require.Contains(t, sql, "CHK_PLAY_REWARD_LEDGER_GROWTH_SNAPSHOT_RULE_VERSION")
	require.NotContains(t, sql, "UPDATE PLAY_REWARD_LEDGER")
	require.NotContains(t, sql, "DELETE FROM PLAY_REWARD_LEDGER")
	require.NotContains(t, sql, "UPDATE PLAY_BLINDBOX_OPENS")
	require.NotContains(t, sql, "DELETE FROM PLAY_BLINDBOX_OPENS")
}
