package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayGrowthGovernanceMigrationIsAppendOnlyAndFailClosed(t *testing.T) {
	raw, err := os.ReadFile("268_play_growth_governance.sql")
	require.NoError(t, err)
	sql := strings.ToUpper(string(raw))

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS PLAY_GROWTH_GOVERNANCE_APPROVALS")
	require.Contains(t, sql, "DECISION IN ('APPROVED', 'REVOKED')")
	require.Contains(t, sql, "BUDGET_AMOUNT > 0")
	require.Contains(t, sql, "ROLLOUT_PERCENT BETWEEN 10 AND 20")
	require.Contains(t, sql, "COHORT_END >= COHORT_START + INTERVAL '14 DAYS'")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS PLAY_GROWTH_REWARD_BUDGET_LEDGER")
	require.Contains(t, sql, "UNIQUE (SOURCE, ACTION_ID)")
	require.Contains(t, sql, "USER_ID       BIGINT NOT NULL REFERENCES USERS(ID) ON DELETE RESTRICT")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION REJECT_PLAY_GROWTH_GOVERNANCE_MUTATION")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON PLAY_GROWTH_GOVERNANCE_APPROVALS")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON PLAY_GROWTH_REWARD_BUDGET_LEDGER")
	require.Contains(t, sql, "IDX_PLAY_GROWTH_GOVERNANCE_LATEST")
	require.Contains(t, sql, "ON PLAY_GROWTH_GOVERNANCE_APPROVALS (ID DESC)")
	require.Contains(t, sql, "IDX_PLAY_GROWTH_REWARD_BUDGET_APPROVAL_CREATED")
}
