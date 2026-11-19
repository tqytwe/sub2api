package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIAPIKeyFallbackStateMigrationIsScopedAndIdempotent(t *testing.T) {
	content, err := FS.ReadFile("377_openai_apikey_fallback_state.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS openai_apikey_fallback_state",
		"account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE",
		"config_revision TEXT NOT NULL",
		"CREATE TABLE IF NOT EXISTS openai_apikey_fallback_events",
		"UNIQUE (account_id, config_revision, trigger_reason)",
	} {
		require.Contains(t, sql, contract)
	}
	require.NotContains(t, strings.ToLower(sql), "credentials")
	require.NotContains(t, strings.ToLower(sql), "request_body")
}
