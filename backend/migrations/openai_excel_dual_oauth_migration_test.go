package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIExcelDualOAuthMigration(t *testing.T) {
	content, err := FS.ReadFile("283_openai_excel_dual_oauth.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE openai_oauth_reauth_tasks")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS oauth_profile TEXT NOT NULL DEFAULT 'codex'")
	require.Contains(t, sql, "CHECK (oauth_profile IN ('codex', 'excel'))")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS openai_excel_oauth_credentials")
	require.Contains(t, sql, "account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "credentials_ciphertext TEXT NOT NULL")
	require.Contains(t, sql, "updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()")
}
