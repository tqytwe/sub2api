package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountOpsAlertMigrationsUseReservedFreeFinalSchema(t *testing.T) {
	alerts, err := FS.ReadFile("385_account_ops_alerts.sql")
	require.NoError(t, err)
	episodes, err := FS.ReadFile("386_account_ops_threshold_episodes.sql")
	require.NoError(t, err)
	alertSQL := strings.Join(strings.Fields(string(alerts)), " ")
	episodeSQL := strings.Join(strings.Fields(string(episodes)), " ")

	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS account_ops_alerts",
		"PRIMARY KEY (group_id, kind)",
		"deliveries JSONB NOT NULL DEFAULT '{}'::jsonb",
		"details JSONB NOT NULL DEFAULT 'null'::jsonb",
		"CHECK (kind IN ('balance_low', 'weekly_quota', 'balance_threshold', 'quota_threshold'))",
	} {
		require.Contains(t, alertSQL, contract)
	}
	for _, contract := range []string{
		"CREATE TABLE IF NOT EXISTS account_ops_threshold_monitors",
		"CREATE TABLE IF NOT EXISTS account_ops_threshold_events",
		"UNIQUE (episode_id, phase)",
		"CHECK (phase IN ('alert', 'recovery'))",
		"CREATE INDEX IF NOT EXISTS account_ops_threshold_events_due",
		"CREATE INDEX IF NOT EXISTS account_ops_threshold_events_history",
	} {
		require.Contains(t, episodeSQL, contract)
	}
}
