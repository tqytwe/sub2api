//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAutomaticModelSyncClaimIsSharedAndSurvivesStaleAccountEdit(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	otherInstance := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	account := mustCreateAccount(t, tx.Client(), &service.Account{
		Name: "model-sync-claim", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"api_key": "key-keep"},
	})
	stale, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	claimed, err := repo.ClaimAutomaticUpstreamModelSync(ctx, account.ID, now, now.Add(5*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = otherInstance.ClaimAutomaticUpstreamModelSync(ctx, account.ID, now.Add(time.Minute), now.Add(6*time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "a different application instance must observe the same attempt window")

	stale.Name = "edited-after-attempt"
	stale.Extra = map[string]any{"note": "from-stale-form", service.UpstreamModelSyncExtraKey: map[string]any{"next_attempt_unix": 0}}
	require.NoError(t, repo.Update(ctx, stale))
	claimed, err = otherInstance.ClaimAutomaticUpstreamModelSync(ctx, account.ID, now.Add(2*time.Minute), now.Add(7*time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "ordinary edits cannot reset or forge the service-owned attempt window")
	claimed, err = otherInstance.ClaimAutomaticUpstreamModelSync(ctx, account.ID, now.Add(5*time.Minute), now.Add(10*time.Minute))
	require.NoError(t, err)
	require.True(t, claimed)
}

func TestApplyUpstreamModelsPreservesCredentialsAndInvalidatesScheduler(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
	account := mustCreateAccount(t, tx.Client(), &service.Account{
		Name: "model-sync-apply", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"api_key": "key-keep", "base_url": "https://provider.example/v1", "model_mapping": map[string]any{"old": "old"}},
		Extra:       map[string]any{"unrelated": "keep"},
	})
	cache := &schedulerCacheRecorder{}
	repo.schedulerCache = cache
	expected, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	applied, err := repo.UpdateUpstreamModelMappingIfUnchanged(ctx, expected, map[string]string{"live": "live"})
	require.NoError(t, err)
	require.True(t, applied)
	latest, err := repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "key-keep", latest.GetCredential("api_key"))
	require.Equal(t, "https://provider.example/v1", latest.GetCredential("base_url"))
	require.Equal(t, map[string]any{"live": "live"}, latest.Credentials["model_mapping"])
	require.Equal(t, "keep", latest.Extra["unrelated"])
	require.Equal(t, service.StatusActive, latest.Status)
	require.True(t, latest.Schedulable)
	require.Len(t, cache.setAccounts, 1)
	require.Equal(t, "live", cache.setAccounts[0].GetMappedModel("live"))
	var outboxCount int
	require.NoError(t, scanSingleRow(ctx, repo.sql,
		"SELECT COUNT(*) FROM scheduler_outbox WHERE account_id=$1 AND event_type=$2",
		[]any{account.ID, service.SchedulerOutboxEventAccountChanged}, &outboxCount))
	require.Equal(t, 1, outboxCount)

	latest.Credentials["api_key"] = "rotated-key"
	_, err = tx.Client().Account.UpdateOneID(account.ID).SetCredentials(latest.Credentials).Save(ctx)
	require.NoError(t, err)
	applied, err = repo.UpdateUpstreamModelMappingIfUnchanged(ctx, expected, map[string]string{"stale": "stale"})
	require.NoError(t, err)
	require.False(t, applied, "a late catalog must not overwrite a credential rotation")
	latest, err = repo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "rotated-key", latest.GetCredential("api_key"))
	require.Equal(t, map[string]any{"live": "live"}, latest.Credentials["model_mapping"])
	require.Len(t, cache.setAccounts, 1, "a skipped CAS must not publish a stale scheduler snapshot")
}

func TestOAuthDegradationNativeModelSyncFencesAccountAvailabilityChanges(t *testing.T) {
	for _, change := range []string{"scheduling paused", "status changed", "proxy changed"} {
		t.Run(change, func(t *testing.T) {
			ctx := t.Context()
			tx := testEntTx(t)
			repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)
			account := mustCreateAccount(t, tx.Client(), &service.Account{
				Name: "degradation-native-model-sync-fence", Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth,
				Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"access_token": "fixture-token", "model_mapping": map[string]any{"old": "old"}},
			})
			expected, err := repo.GetByID(ctx, account.ID)
			require.NoError(t, err)
			update := tx.Client().Account.UpdateOneID(account.ID)
			switch change {
			case "scheduling paused":
				update.SetSchedulable(false)
			case "status changed":
				update.SetStatus(service.StatusDisabled)
			case "proxy changed":
				proxy := mustCreateProxy(t, tx.Client(), &service.Proxy{Name: "model-sync-proxy"})
				update.SetProxyID(proxy.ID)
			}
			_, err = update.Save(ctx)
			require.NoError(t, err)
			cache := &schedulerCacheRecorder{}
			repo.schedulerCache = cache
			applied, err := repo.UpdateUpstreamModelMappingIfUnchanged(ctx, expected, map[string]string{"new": "new"})
			require.NoError(t, err)
			require.False(t, applied, "a catalog fetched before native pause must not overwrite the model mapping")
			latest, err := repo.GetByID(ctx, account.ID)
			require.NoError(t, err)
			require.Equal(t, expected.Credentials, latest.Credentials)
			require.Empty(t, cache.setAccounts, "the skipped catalog must not publish scheduler state")
			var outboxCount int
			require.NoError(t, scanSingleRow(ctx, repo.sql, "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id=$1", []any{account.ID}, &outboxCount))
			require.Zero(t, outboxCount)
		})
	}
}

func TestApplyUpstreamModelsAtomicOutboxFailureAndCancellation(t *testing.T) {
	for _, failure := range []string{"outbox failure", "request canceled"} {
		t.Run(failure, func(t *testing.T) {
			client := testEntClient(t)
			account := mustCreateAccount(t, client, &service.Account{
				Name: "model-sync-atomic-failure", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Status: service.StatusActive, Schedulable: true,
				Credentials: map[string]any{"api_key": "fixture-key", "model_mapping": map[string]any{"old": "old"}},
			})
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id=$1", account.ID)
				_ = client.Account.DeleteOneID(account.ID).Exec(context.Background())
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var executor sqlExecutor = integrationDB
			if failure == "outbox failure" {
				executor = &failAtomicSchedulerOutboxSQLExecutor{sqlExecutor: integrationDB}
			} else {
				cancel()
			}
			cache := &schedulerCacheRecorder{}
			repo := newAccountRepositoryWithSQL(client, executor, cache)
			applied, err := repo.UpdateUpstreamModelMappingIfUnchanged(ctx, account, map[string]string{"new": "new"})
			require.Error(t, err)
			require.False(t, applied)
			latest, err := repo.GetByID(context.Background(), account.ID)
			require.NoError(t, err)
			require.Equal(t, account.Credentials, latest.Credentials, "failed atomic statement cannot leave model mapping changed")
			require.Empty(t, cache.setAccounts)
			var count int
			require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM scheduler_outbox WHERE account_id=$1", account.ID).Scan(&count))
			require.Zero(t, count)
		})
	}
}
