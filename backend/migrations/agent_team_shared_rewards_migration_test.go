package migrations

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const approvedTeamRewardTiersJSON = `[{"threshold":"20","rate":"0.02"},{"threshold":"100","rate":"0.03"},{"threshold":"500","rate":"0.04"},{"threshold":"2000","rate":"0.05"}]`

func TestAgentTeamSharedRewardsMigrationContract(t *testing.T) {
	content, err := FS.ReadFile("191_agent_team_shared_rewards.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upperSQL := strings.ToUpper(sql)

	require.Contains(t, upperSQL, "ALTER TABLE PLAY_TEAMS ADD COLUMN IF NOT EXISTS ARCHIVED_AT TIMESTAMPTZ")
	require.Contains(t, upperSQL, "ALTER TABLE PLAY_TEAM_MEMBERS ADD COLUMN IF NOT EXISTS LEFT_AT TIMESTAMPTZ")
	require.Contains(t, upperSQL, "ALTER TABLE PLAY_TEAM_MEMBERS DROP CONSTRAINT IF EXISTS UQ_PLAY_TEAM_MEMBERS_USER")
	require.Contains(t, upperSQL, "ALTER TABLE PLAY_TEAM_MEMBERS DROP CONSTRAINT IF EXISTS UQ_PLAY_TEAM_MEMBERS_TEAM_USER")
	require.Contains(
		t,
		upperSQL,
		"CREATE UNIQUE INDEX IF NOT EXISTS UQ_PLAY_TEAM_MEMBERS_ACTIVE_USER ON PLAY_TEAM_MEMBERS(USER_ID) WHERE LEFT_AT IS NULL",
	)
	require.Contains(
		t,
		upperSQL,
		"CREATE INDEX IF NOT EXISTS IDX_PLAY_TEAM_REWARD_ALLOCATIONS_USER_SETTLEMENT ON PLAY_TEAM_REWARD_ALLOCATIONS(USER_ID, SETTLEMENT_ID DESC)",
	)
	requireGuardedMigrationConstraint(
		t,
		upperSQL,
		"play_team_members",
		"chk_play_team_members_active_interval",
	)
	require.Contains(
		t,
		upperSQL,
		"CHECK (LEFT_AT IS NULL OR LEFT_AT >= JOINED_AT)",
	)

	eventTable := migrationTableBody(t, sql, "play_team_events")
	requireMigrationColumns(t, eventTable, map[string]string{
		"id":              `BIGSERIAL PRIMARY KEY`,
		"team_id":         `BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT`,
		"actor_user_id":   `BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT`,
		"subject_user_id": `BIGINT REFERENCES users(id) ON DELETE SET NULL`,
		"event_type":      `VARCHAR(32) NOT NULL`,
		"detail":          `JSONB NOT NULL DEFAULT '{}'::jsonb`,
		"created_at":      `TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	})
	require.Contains(
		t,
		upperSQL,
		"CREATE INDEX IF NOT EXISTS IDX_PLAY_TEAM_EVENTS_TEAM_CREATED_AT ON PLAY_TEAM_EVENTS(TEAM_ID, CREATED_AT DESC)",
	)

	settlementTable := migrationTableBody(t, sql, "play_team_settlements")
	requireMigrationColumns(t, settlementTable, map[string]string{
		"id":                    `BIGSERIAL PRIMARY KEY`,
		"team_id":               `BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE RESTRICT`,
		"period_start":          `DATE NOT NULL`,
		"window_start":          `TIMESTAMPTZ NOT NULL`,
		"window_end":            `TIMESTAMPTZ NOT NULL`,
		"team_spend":            `DECIMAL(20, 8) NOT NULL`,
		"reached_threshold":     `DECIMAL(20, 8) NOT NULL`,
		"reward_rate":           `DECIMAL(20, 8) NOT NULL`,
		"pool_amount":           `DECIMAL(20, 8) NOT NULL`,
		"cap_amount":            `DECIMAL(20, 8) NOT NULL`,
		"status":                `VARCHAR(16) NOT NULL DEFAULT 'pending'`,
		"last_error":            `TEXT`,
		"processing_started_at": `TIMESTAMPTZ`,
		"completed_at":          `TIMESTAMPTZ`,
		"created_at":            `TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
		"updated_at":            `TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	})
	require.Contains(
		t,
		strings.ToUpper(settlementTable),
		"CONSTRAINT UQ_PLAY_TEAM_SETTLEMENTS_TEAM_PERIOD UNIQUE (TEAM_ID, PERIOD_START)",
	)
	require.Contains(
		t,
		strings.ToUpper(settlementTable),
		"CONSTRAINT CHK_PLAY_TEAM_SETTLEMENTS_STATUS CHECK (STATUS IN ('PENDING', 'PROCESSING', 'COMPLETED', 'PARTIAL', 'FAILED'))",
	)
	require.Contains(
		t,
		upperSQL,
		"CREATE INDEX IF NOT EXISTS IDX_PLAY_TEAM_SETTLEMENTS_STATUS_PERIOD ON PLAY_TEAM_SETTLEMENTS(STATUS, PERIOD_START)",
	)
	requireGuardedMigrationConstraint(
		t,
		upperSQL,
		"play_team_settlements",
		"chk_play_team_settlements_period_window",
	)
	require.Contains(t, upperSQL, "PERIOD_START = DATE_TRUNC('MONTH', PERIOD_START)::DATE")
	require.Regexp(
		t,
		regexp.MustCompile(
			`WINDOW_START\s*=\s*\(\s*PERIOD_START::TIMESTAMP AT TIME ZONE 'ASIA/SHANGHAI'\s*\)`,
		),
		upperSQL,
	)
	require.Regexp(
		t,
		regexp.MustCompile(
			`WINDOW_END\s*=\s*\(\s*\(PERIOD_START \+ INTERVAL '1 MONTH'\)::TIMESTAMP\s+AT TIME ZONE 'ASIA/SHANGHAI'\s*\)`,
		),
		upperSQL,
	)
	requireGuardedMigrationConstraint(
		t,
		upperSQL,
		"play_team_settlements",
		"chk_play_team_settlements_status_timestamps",
	)
	require.Contains(
		t,
		upperSQL,
		"(STATUS = 'COMPLETED') = (COMPLETED_AT IS NOT NULL)",
	)
	require.Contains(
		t,
		upperSQL,
		"STATUS = 'PENDING' OR PROCESSING_STARTED_AT IS NOT NULL",
	)

	allocationTable := migrationTableBody(t, sql, "play_team_reward_allocations")
	requireMigrationColumns(t, allocationTable, map[string]string{
		"id":              `BIGSERIAL PRIMARY KEY`,
		"settlement_id":   `BIGINT NOT NULL REFERENCES play_team_settlements(id) ON DELETE CASCADE`,
		"user_id":         `BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT`,
		"contribution":    `DECIMAL(20, 8) NOT NULL`,
		"ratio":           `DECIMAL(20, 8) NOT NULL`,
		"reward_amount":   `DECIMAL(20, 8) NOT NULL`,
		"payout_status":   `VARCHAR(16) NOT NULL DEFAULT 'pending'`,
		"idempotency_key": `VARCHAR(128) NOT NULL`,
		"paid_at":         `TIMESTAMPTZ`,
		"last_error":      `TEXT`,
		"created_at":      `TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
		"updated_at":      `TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	})
	allocationUpper := strings.ToUpper(allocationTable)
	require.Contains(
		t,
		allocationUpper,
		"CONSTRAINT UQ_PLAY_TEAM_REWARD_ALLOCATIONS_SETTLEMENT_USER UNIQUE (SETTLEMENT_ID, USER_ID)",
	)
	require.Contains(
		t,
		allocationUpper,
		"CONSTRAINT UQ_PLAY_TEAM_REWARD_ALLOCATIONS_IDEMPOTENCY UNIQUE (IDEMPOTENCY_KEY)",
	)
	require.Contains(
		t,
		allocationUpper,
		"CONSTRAINT CHK_PLAY_TEAM_REWARD_ALLOCATIONS_PAYOUT_STATUS CHECK (PAYOUT_STATUS IN ('PENDING', 'PROCESSING', 'PAID', 'FAILED'))",
	)
	require.Contains(
		t,
		upperSQL,
		"CREATE INDEX IF NOT EXISTS IDX_PLAY_TEAM_REWARD_ALLOCATIONS_SETTLEMENT_STATUS ON PLAY_TEAM_REWARD_ALLOCATIONS(SETTLEMENT_ID, PAYOUT_STATUS)",
	)
	requireGuardedMigrationConstraint(
		t,
		upperSQL,
		"play_team_reward_allocations",
		"chk_play_team_reward_allocations_paid_timestamp",
	)
	require.Contains(
		t,
		upperSQL,
		"(PAYOUT_STATUS = 'PAID') = (PAID_AT IS NOT NULL)",
	)

	require.Contains(t, sql, approvedTeamRewardTiersJSON)
	for _, setting := range []string{
		"('play_team_shared_reward_enabled', 'true')",
		"('play_team_shared_reward_tiers', '" + approvedTeamRewardTiersJSON + "')",
		"('play_team_shared_reward_cap', '250')",
	} {
		require.Contains(t, sql, setting)
	}
	require.Regexp(
		t,
		regexp.MustCompile(`(?i)\(\s*'play_team_shared_reward_start_month',\s*TO_CHAR\(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai',\s*'YYYY-MM'\)\s*\)`),
		sql,
	)
	require.Contains(t, upperSQL, "ON CONFLICT (KEY) DO NOTHING")
	require.NotContains(t, upperSQL, "AFFILIATE")
}

func TestAgentTeamSharedRewardsMigrationIsForwardOnlyAndPreservesMemberships(t *testing.T) {
	content, err := FS.ReadFile("191_agent_team_shared_rewards.sql")
	require.NoError(t, err)

	sql := normalizeMigrationSQL(string(content))
	upperSQL := strings.ToUpper(sql)

	for _, destructive := range []string{
		"DROP TABLE",
		"DROP COLUMN",
		"DELETE FROM",
		"TRUNCATE",
		"CREATE TABLE PLAY_TEAM_MEMBERS",
		"CREATE TABLE IF NOT EXISTS PLAY_TEAM_MEMBERS",
		"ALTER TABLE PLAY_TEAM_MEMBERS RENAME",
		"UPDATE PLAY_TEAM_MEMBERS",
		"INSERT INTO PLAY_TEAM_MEMBERS",
	} {
		require.NotContains(t, upperSQL, destructive)
	}
	require.NotRegexp(t, regexp.MustCompile(`(?i)\bleft_at\s*=\s*`), sql)
}

func normalizeMigrationSQL(sql string) string {
	lineComment := regexp.MustCompile(`(?m)--.*$`)
	return strings.Join(strings.Fields(lineComment.ReplaceAllString(sql, " ")), " ")
}

func migrationTableBody(t *testing.T, sql string, table string) string {
	t.Helper()

	pattern := regexp.MustCompile(`(?is)CREATE TABLE IF NOT EXISTS ` + regexp.QuoteMeta(table) + `\s*\((.*?)\);`)
	match := pattern.FindStringSubmatch(sql)
	require.Len(t, match, 2, "missing CREATE TABLE IF NOT EXISTS %s", table)
	return match[1]
}

func requireMigrationColumns(t *testing.T, tableBody string, columns map[string]string) {
	t.Helper()

	for column, definition := range columns {
		pattern := regexp.MustCompile(
			`(?i)(?:^|,)\s*` + regexp.QuoteMeta(column) + `\s+` + migrationDefinitionPattern(definition) + `(?:\s*,|\s*$)`,
		)
		require.Regexp(t, pattern, tableBody, "missing or invalid column %s", column)
	}
}

func migrationDefinitionPattern(definition string) string {
	quoted := regexp.QuoteMeta(definition)
	quoted = strings.ReplaceAll(quoted, `\ `, `\s+`)
	quoted = strings.ReplaceAll(quoted, `DECIMAL\(20,\ 8\)`, `DECIMAL\s*\(\s*20\s*,\s*8\s*\)`)
	quoted = strings.ReplaceAll(quoted, `play_teams\(id\)`, `play_teams\s*\(\s*id\s*\)`)
	quoted = strings.ReplaceAll(quoted, `play_team_settlements\(id\)`, `play_team_settlements\s*\(\s*id\s*\)`)
	quoted = strings.ReplaceAll(quoted, `users\(id\)`, `users\s*\(\s*id\s*\)`)
	return quoted
}

func requireGuardedMigrationConstraint(
	t *testing.T,
	upperSQL string,
	table string,
	constraint string,
) {
	t.Helper()

	table = strings.ToUpper(table)
	constraint = strings.ToUpper(constraint)
	pattern := regexp.MustCompile(
		`IF NOT EXISTS\s*\(\s*SELECT 1 FROM PG_CONSTRAINT WHERE CONNAME = '` +
			regexp.QuoteMeta(constraint) +
			`'\s+AND CONRELID = '` +
			regexp.QuoteMeta(table) +
			`'::REGCLASS\s*\)\s*THEN ALTER TABLE ` +
			regexp.QuoteMeta(table) +
			` ADD CONSTRAINT ` +
			regexp.QuoteMeta(constraint),
	)
	require.Regexp(t, pattern, upperSQL)
}
