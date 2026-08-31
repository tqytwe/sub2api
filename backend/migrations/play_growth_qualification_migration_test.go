package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayGrowthQualificationMigrationContract(t *testing.T) {
	raw, err := FS.ReadFile("262_play_growth_qualification.sql")
	require.NoError(t, err)

	sql := strings.ToUpper(string(raw))
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS")
	require.Contains(t, sql, "SOURCE IN ('CHECKIN', 'QUIZ', 'BLINDBOX')")
	require.Contains(t, sql, "ACTION_ID VARCHAR(128) NOT NULL")
	require.Equal(t, 2, strings.Count(sql, "CHECK (BTRIM(ACTION_ID) <> '')"))
	require.Contains(t, sql, "UNIQUE (USER_ID, SOURCE, ACTION_ID)")
	require.Contains(t, sql, "UQ_PLAY_GROWTH_ELIGIBILITY_SNAPSHOT_DAILY_ACTIVITY")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS PLAY_GROWTH_ENERGY_LEDGER")
	require.Contains(t, sql, "UNIQUE (ACTION_ID)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS GROWTH_ELIGIBILITY_SNAPSHOT_ID")
	require.Contains(t, sql, "REFERENCES PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS(ID) ON DELETE RESTRICT")
	require.Contains(t, sql, "IDX_PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS_USER_CREATED")
	require.Contains(t, sql, "IDX_PLAY_GROWTH_ENERGY_LEDGER_USER_CREATED")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION REJECT_PLAY_GROWTH_QUALIFICATION_MUTATION")
	require.Contains(t, sql, "RAISE EXCEPTION '% ROWS ARE IMMUTABLE AFTER INSERTION', TG_TABLE_NAME")
	require.Contains(t, sql, "CREATE TRIGGER PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS_IMMUTABLE")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON PLAY_GROWTH_ELIGIBILITY_SNAPSHOTS")
	require.Contains(t, sql, "CREATE TRIGGER PLAY_GROWTH_ENERGY_LEDGER_IMMUTABLE")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON PLAY_GROWTH_ENERGY_LEDGER")
	require.NotContains(t, sql, "DROP TABLE")
	require.NotContains(t, sql, "TRUNCATE")
	require.NotContains(t, sql, "DELETE FROM")
}
