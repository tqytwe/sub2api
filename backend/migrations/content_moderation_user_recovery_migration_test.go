package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationUserRecoveryMigrationPreservesUpstreamSchemaAndCountSnapshot(t *testing.T) {
	raw, err := FS.ReadFile("296_content_moderation_user_recovery.sql")
	require.NoError(t, err)
	sql := strings.ToUpper(string(raw))
	require.Contains(t, sql, "BEFORE UPDATE OF STATUS ON USERS")
	require.Contains(t, sql, "NEW.MODERATION_STATUS_REVISION := OLD.MODERATION_STATUS_REVISION + 1")
	require.Contains(t, sql, "REFERENCES USERS(ID) ON DELETE CASCADE")
	require.Contains(t, sql, "INSERT INTO CONTENT_MODERATION_USER_HITS(USER_ID,GENERATION,REQUEST_ID,ACTION,CREATED_AT)")
	require.Contains(t, sql, "ON CONFLICT(USER_ID,GENERATION,REQUEST_ID) DO NOTHING")
	require.NotContains(t, sql, "UPDATE USERS SET STATUS")
	require.NotContains(t, sql, "TRUNCATE")
}

func TestContentModerationUserActionAuditSurvivesUserDeletion(t *testing.T) {
	raw, err := FS.ReadFile("296_content_moderation_user_recovery.sql")
	require.NoError(t, err)
	sql := strings.ToUpper(string(raw))
	actions := strings.SplitN(sql, "CREATE TABLE IF NOT EXISTS CONTENT_MODERATION_USER_ACTIONS", 2)
	require.Len(t, actions, 2)
	actions = strings.SplitN(actions[1], ";", 2)
	require.Len(t, actions, 2)
	require.NotContains(t, actions[0], "REFERENCES USERS")
}

func TestContentModerationExemptionMigrationsAddIndependentSwitchesWithoutReset(t *testing.T) {
	for file, columns := range map[string][]string{
		"298_content_moderation_exemption_types.sql": {"KEYWORD_EXEMPT", "API_AUDIT_EXEMPT"},
		"301_content_moderation_email_exemption.sql": {"EMAIL_EXEMPT"},
	} {
		raw, err := FS.ReadFile(file)
		require.NoError(t, err)
		sql := strings.ToUpper(string(raw))
		for _, column := range columns {
			require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS "+column+" BOOLEAN NOT NULL DEFAULT FALSE")
		}
		require.NotContains(t, sql, "UPDATE CONTENT_MODERATION_USER_STATE")
		require.NotContains(t, sql, "UPDATE CONTENT_MODERATION_USER_ACTIONS")
	}
}
