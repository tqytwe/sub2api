package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIExcelOAuthCredentialsMigrationKeepsCiphertextSeparate(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("339_openai_excel_oauth_credentials.sql"))
	require.NoError(t, err)
	sql := string(b)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS openai_excel_oauth_credentials")
	require.Contains(t, sql, "REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "credentials_encrypted TEXT NOT NULL")
	require.Contains(t, sql, "revision BIGINT NOT NULL DEFAULT 1")
	require.NotContains(t, sql, "access_token TEXT")
	require.NotContains(t, sql, "refresh_token TEXT")
	require.NotContains(t, sql, "ALTER TABLE accounts")
}
