//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func setupOAuthAutomationFixture(t *testing.T) service.AccountRepository {
	t.Helper()
	ctx := context.Background()
	for _, name := range []string{"376_oauth_auto_config_events.sql", "377_openai_apikey_fallback_state.sql"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		for range 2 {
			_, err = integrationDB.ExecContext(ctx, string(raw))
			require.NoError(t, err)
		}
	}
	var previous string
	err := integrationDB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.SettingKeyOAuthAutoConfig).Scan(&previous)
	hadPrevious := err == nil
	t.Cleanup(func() {
		if hadPrevious {
			_, e := integrationDB.ExecContext(ctx, `INSERT INTO settings(key,value)VALUES($1,$2)ON CONFLICT(key)DO UPDATE SET value=EXCLUDED.value`, service.SettingKeyOAuthAutoConfig, previous)
			require.NoError(t, e)
		} else {
			_, e := integrationDB.ExecContext(ctx, `DELETE FROM settings WHERE key=$1`, service.SettingKeyOAuthAutoConfig)
			require.NoError(t, e)
		}
	})
	return NewAccountRepository(integrationEntClient, integrationDB, nil)
}
func saveOAuthAutomationFixture(t *testing.T, c service.OAuthAutoConfig) {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(context.Background(), `INSERT INTO settings(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, service.SettingKeyOAuthAutoConfig, string(raw))
	require.NoError(t, err)
}
func automationAccount(t *testing.T, repo service.AccountRepository, kind string) *service.Account {
	t.Helper()
	a := &service.Account{Name: "automation fixture", Platform: service.PlatformOpenAI, Type: kind, Status: service.StatusActive, Schedulable: false, Concurrency: 8, Credentials: map[string]any{"api_key": "fixture-only", "refresh_token": "fixture-refresh"}, Extra: map[string]any{}}
	require.NoError(t, repo.Create(context.Background(), a))
	t.Cleanup(func() {
		_, e := integrationDB.Exec(`DELETE FROM account_auto_config_events WHERE account_id=$1`, a.ID)
		require.NoError(t, e)
		_, e = integrationDB.Exec(`DELETE FROM scheduler_outbox WHERE account_id=$1`, a.ID)
		require.NoError(t, e)
		_, e = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, a.ID)
		require.NoError(t, e)
	})
	return a
}
func TestOAuthAutomationFallbackPostgreSQLAtomicStateAuditAndRetention(t *testing.T) {
	ctx := context.Background()
	repo := setupOAuthAutomationFixture(t)
	a := automationAccount(t, repo, service.AccountTypeAPIKey)
	cfg := service.DefaultOAuthAutoConfig()
	cfg.Revision = "fixture-fallback"
	cfg.OpenAIAPIKeyFallback.Enabled = true
	cfg.OpenAIAPIKeyFallback.AccountIDs = []int64{a.ID}
	saveOAuthAutomationFixture(t, cfg)
	fallback := repo.(service.OpenAIAPIKeyFallbackRepository)
	type outcome struct {
		changed bool
		err     error
	}
	outcomes := make(chan outcome, 3)
	for range 3 {
		go func() {
			changed, err := fallback.EnableOpenAIAPIKeyFallback(ctx, a.ID, cfg.Revision, "oauth_unavailable")
			outcomes <- outcome{changed, err}
		}()
	}
	changedCount := 0
	for range 3 {
		result := <-outcomes
		require.NoError(t, result.err)
		if result.changed {
			changedCount++
		}
	}
	require.Equal(t, 1, changedCount)
	opened, err := fallback.EnableOpenAIAPIKeyFallback(ctx, a.ID, cfg.Revision, "oauth_unavailable")
	require.NoError(t, err)
	require.False(t, opened)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM openai_apikey_fallback_events WHERE account_id=$1`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM account_auto_config_events WHERE account_id=$1 AND kind='apikey_fallback_opened'`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
	live, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, live.Schedulable)
	require.Equal(t, "fixture-only", live.Credentials["api_key"])
	openedAt, reason, err := repo.(service.OpenAIAPIKeyFallbackStateRepository).LoadOpenAIAPIKeyFallbackState(ctx)
	require.NoError(t, err)
	require.NotNil(t, openedAt)
	require.Equal(t, "oauth_unavailable", reason)
	// No stale policy revision or unselected account may open scheduling.
	_, err = integrationDB.Exec(`UPDATE accounts SET schedulable=false WHERE id=$1`, a.ID)
	require.NoError(t, err)
	opened, err = fallback.EnableOpenAIAPIKeyFallback(ctx, a.ID, "stale-revision", "oauth_unavailable")
	require.NoError(t, err)
	require.False(t, opened)
	cfg.OpenAIAPIKeyFallback.Enabled = false
	saveOAuthAutomationFixture(t, cfg)
	opened, err = fallback.EnableOpenAIAPIKeyFallback(ctx, a.ID, cfg.Revision, "metric_below_threshold")
	require.NoError(t, err)
	require.False(t, opened)
	_, err = integrationDB.Exec(`UPDATE openai_apikey_fallback_events SET created_at=NOW()-INTERVAL '31 days' WHERE account_id=$1`, a.ID)
	require.NoError(t, err)
	require.NoError(t, repo.(interface {
		PruneOAuthAutomationHistory(context.Context, time.Time) error
	}).PruneOAuthAutomationHistory(ctx, time.Now().Add(-30*24*time.Hour)))
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM openai_apikey_fallback_events WHERE account_id=$1`, a.ID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM openai_apikey_fallback_state WHERE account_id=$1`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
}
func TestOAuthAutomationStaleRefreshCannotUndoNativeDegradationAndManualEditRecovers(t *testing.T) {
	ctx := context.Background()
	repo := setupOAuthAutomationFixture(t)
	a := automationAccount(t, repo, service.AccountTypeOAuth)
	stale, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	marker := service.OAuthDegradationState{NativeControls: true, NativeSchedulingPaused: true, Revision: "degraded"}
	hold := service.AutoConfigConcurrencyState{Concurrency: 2, DegradationHold: true}
	extra, _ := json.Marshal(map[string]any{service.OAuthDegradationActionExtraKey: marker, service.AutoConfigConcurrencyExtraKey: hold})
	_, err = integrationDB.Exec(`UPDATE accounts SET concurrency=2,credentials=credentials||'{"model_mapping":{"allowed":"allowed"},"model_mapping_mode":"allowlist"}',extra=$2::jsonb WHERE id=$1`, a.ID, string(extra))
	require.NoError(t, err)
	stale.Credentials["refresh_token"] = "rotated-fixture"
	require.NoError(t, repo.Update(ctx, stale))
	require.Equal(t, 2, stale.Concurrency)
	require.Equal(t, "allowlist", stale.Credentials["model_mapping_mode"])
	require.Equal(t, "rotated-fixture", stale.Credentials["refresh_token"])
	require.Contains(t, stale.Extra, service.AutoConfigConcurrencyExtraKey)
	require.NoError(t, repo.(interface {
		UpdateCredentials(context.Context, int64, map[string]any) error
	}).UpdateCredentials(ctx, a.ID, map[string]any{"refresh_token": "second-rotation", "model_mapping": map[string]any{"unrestricted": "unrestricted"}}))
	refreshed, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "allowlist", refreshed.Credentials["model_mapping_mode"])
	require.Equal(t, map[string]any{"allowed": "allowed"}, refreshed.Credentials["model_mapping"])
	require.Equal(t, "second-rotation", refreshed.Credentials["refresh_token"])
	manual := service.WithOAuthAutomationManualEdit(ctx, true, true, true)
	stale.Concurrency = 7
	stale.Credentials["model_mapping"] = map[string]any{"new-model": "new-model"}
	require.NoError(t, repo.Update(manual, stale))
	require.Equal(t, 7, stale.Concurrency)
	require.NotContains(t, stale.Extra, service.AutoConfigConcurrencyExtraKey)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM account_auto_config_events WHERE account_id=$1 AND kind='manual_recovery'`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
}
