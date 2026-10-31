package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQualityRuleTemplateMigrationIsAdditiveAndPlanScoped(t *testing.T) {
	content, err := FS.ReadFile("260_quality_rule_templates.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS quality_rule_templates",
		"account_filter JSONB NOT NULL DEFAULT '{}'::jsonb",
		"pelican_config JSONB NOT NULL",
		"CREATE TABLE IF NOT EXISTS quality_rule_template_accounts",
		"template_id BIGINT NOT NULL REFERENCES quality_rule_templates(id) ON DELETE CASCADE",
		"account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE",
		"plan_id BIGINT REFERENCES scheduled_test_plans(id) ON DELETE SET NULL",
		"PRIMARY KEY (template_id, account_id)",
	} {
		require.Contains(t, sql, contract)
	}
	require.NotContains(t, strings.ToLower(sql), "request_body")
	require.NotContains(t, strings.ToLower(sql), "response_body")
	require.NotContains(t, strings.ToLower(sql), "full_prompt")
}

func TestQualityRulePresetMigrationHasNoRuntimeOwnershipSideEffects(t *testing.T) {
	content, err := FS.ReadFile("336_quality_rule_presets.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS quality_rule_presets",
		"name VARCHAR(100) NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)",
		"model_id VARCHAR(100) NOT NULL",
		"cron_expression TEXT NOT NULL",
		"pelican_config JSONB NOT NULL",
	} {
		require.Contains(t, sql, contract)
	}
	// Presets are reusable configuration only. Applying one is an explicit
	// application action; the migration must not create plans or change users.
	require.NotContains(t, strings.ToUpper(sql), "INSERT INTO SCHEDULED_TEST_PLANS")
	require.NotContains(t, strings.ToUpper(sql), "UPDATE ACCOUNTS")
	require.NotContains(t, strings.ToLower(sql), "request_body")
	require.NotContains(t, strings.ToLower(sql), "response_body")
	require.NotContains(t, strings.ToLower(sql), "full_prompt")
}

func TestQualityRuleMigrationsAreEmbeddedWithDistinctFilenames(t *testing.T) {
	for _, name := range []string{"260_quality_rule_templates.sql", "336_quality_rule_presets.sql"} {
		_, err := FS.ReadFile(name)
		require.NoError(t, err, name)
	}
	_, err := FS.ReadFile("260_model_catalog_media_capabilities.sql")
	require.NoError(t, err)
}
