package migrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthReauthMigrationsUseForkNamespace(t *testing.T) {
	for _, legacy := range []string{
		"254_openai_oauth_reauth.sql",
		"255_account_token_guard_v2.sql",
		"256_openai_oauth_reauth_proxy_override.sql",
		"257_openai_oauth_reauth_proxy_source.sql",
	} {
		_, err := os.Stat(filepath.Join(legacy))
		require.ErrorIs(t, err, os.ErrNotExist, "legacy upstream migration must not be restored")
	}

	reauth, err := os.ReadFile(filepath.Join("330_openai_oauth_reauth.sql"))
	require.NoError(t, err)
	require.Contains(t, string(reauth), "CREATE TABLE IF NOT EXISTS openai_oauth_reauth_tasks")

	guard, err := os.ReadFile(filepath.Join("331_account_token_guard_v2.sql"))
	require.NoError(t, err)
	require.Contains(t, string(guard), "CREATE TABLE IF NOT EXISTS account_token_guard_v2_accounts")
	require.Contains(t, string(guard), "openai_oauth_reauth_tasks_stage_check")

	proxy, err := os.ReadFile(filepath.Join("333_openai_oauth_reauth_proxy_source.sql"))
	require.NoError(t, err)
	require.Contains(t, string(proxy), "openai_oauth_reauth_configs_proxy_source_check")
}

func TestAccountReloginBudgetMigrationDoesNotReferenceMissingOAuthProfile(t *testing.T) {
	reauth, err := os.ReadFile(filepath.Join("330_openai_oauth_reauth.sql"))
	require.NoError(t, err)
	budget, err := os.ReadFile(filepath.Join("335_account_relogin_budget.sql"))
	require.NoError(t, err)
	require.NotContains(t, string(reauth), "oauth_profile")
	require.NotContains(t, string(budget), "oauth_profile")
	require.Contains(t, string(budget), "WHERE task.account_id = guard.account_id")
}
