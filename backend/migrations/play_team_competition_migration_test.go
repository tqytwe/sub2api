package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlayTeamCompetitionMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("243_play_team_competition.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upper := strings.ToUpper(sql)

	require.Contains(t, upper, "ALTER TABLE PLAY_TEAM_MEMBERS ADD COLUMN IF NOT EXISTS REWARD_ELIGIBLE_AT TIMESTAMPTZ")
	require.Contains(t, upper, "ALTER TABLE PLAY_TEAMS ALTER COLUMN INVITE_CODE TYPE VARCHAR(64)")
	require.Contains(t, upper, "INVITE_CODE_EXPIRES_AT TIMESTAMPTZ")
	require.Contains(t, upper, "IS_RECRUITING BOOLEAN NOT NULL DEFAULT TRUE")
	require.Contains(t, upper, "CREATE TABLE IF NOT EXISTS PLAY_TEAM_JOIN_APPLICATIONS")
	require.Contains(t, upper, "CREATE TABLE IF NOT EXISTS PLAY_TEAM_JOIN_APPLICATION_EVENTS")
	require.Contains(t, upper, "CREATE TABLE IF NOT EXISTS PLAY_TEAM_SEASONS")
	require.Contains(t, upper, "CREATE TABLE IF NOT EXISTS PLAY_TEAM_SEASON_RANKINGS")
	require.Contains(t, upper, "PAYOUT_LEASE_EXPIRES_AT TIMESTAMPTZ")
	require.Contains(t, upper, "PAYOUT_ATTEMPTS INT NOT NULL DEFAULT 0")
	// PostgreSQL partial uniqueness is expressed as a unique index rather than
	// a table constraint with a WHERE clause.
	require.Contains(t, upper, "CREATE UNIQUE INDEX")
	require.Contains(t, upper, "ON PLAY_TEAM_JOIN_APPLICATIONS(TEAM_ID, APPLICANT_USER_ID)")
	require.Contains(t, upper, "WHERE STATUS = 'PENDING'")
	require.Contains(t, upper, "UNIQUE (SEASON_ID, TEAM_ID)")
	require.Contains(t, upper, "CHECK (STATUS IN ('PENDING', 'APPROVED', 'REJECTED', 'WITHDRAWN', 'EXPIRED'))")
	require.Contains(t, upper, "CHECK (STATUS IN ('ACTIVE', 'SETTLING', 'SETTLED', 'FAILED', 'LEGACY'))")
	require.Contains(t, upper, "AT TIME ZONE 'ASIA/SHANGHAI'")
	require.Contains(t, upper, "WHERE RANK <= 10")
	// Only fully completed legacy settlements can become public historical proof.
	require.Contains(t, upper, "FROM PLAY_TEAM_SETTLEMENTS S WHERE S.STATUS = 'COMPLETED'")
	require.Contains(t, upper, "LEFT JOIN PLAY_TEAM_REWARD_ALLOCATIONS A ON A.SETTLEMENT_ID = S.ID WHERE S.STATUS = 'COMPLETED'")

	for _, destructive := range []string{"DROP TABLE", "DROP COLUMN", "DELETE FROM", "TRUNCATE"} {
		require.NotContains(t, upper, destructive)
	}
}
