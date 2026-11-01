//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelBillingPostgresRevisionRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := NewSettingRepository(testEntTx(t).Client()).(*settingRepository)
	key := "test_model_billing_revision"
	revision, err := repo.CompareAndSet(ctx, key, "", "", "first")
	require.NoError(t, err)
	value, storedRevision, err := repo.GetValueWithRevision(ctx, key)
	require.NoError(t, err)
	require.Equal(t, revision, storedRevision, "save must return the exact persisted PostgreSQL revision")
	for i := 0; i < 5; i++ {
		next := fmt.Sprintf("value-%d", i)
		previousRevision := revision
		revision, err = repo.CompareAndSet(ctx, key, revision, value, next)
		require.NotEqual(t, previousRevision, revision)
		require.NoError(t, err, "immediate repeated save must accept the returned revision")
		value, storedRevision, err = repo.GetValueWithRevision(ctx, key)
		require.NoError(t, err)
		require.Equal(t, revision, storedRevision)
	}
	_, err = repo.CompareAndSet(ctx, key, revision, "outdated-value", "bad")
	require.ErrorIs(t, err, service.ErrModelBillingRevisionConflict)
	// Even a clock behind the stored row must produce a new revision when the
	// saved JSON is identical, so replaying the old revision cannot succeed.
	future := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	_, err = repo.client.Setting.Update().Where(setting.KeyEQ(key)).SetUpdatedAt(future).Save(ctx)
	require.NoError(t, err)
	value, revision, err = repo.GetValueWithRevision(ctx, key)
	require.NoError(t, err)
	advanced, err := repo.CompareAndSet(ctx, key, revision, value, value)
	require.NoError(t, err)
	require.NotEqual(t, revision, advanced)
	_, err = repo.CompareAndSet(ctx, key, revision, value, value)
	require.ErrorIs(t, err, service.ErrModelBillingRevisionConflict)

}

func TestModelBillingPostgresConcurrentWriters(t *testing.T) {
	ctx := context.Background()
	repo := NewSettingRepository(integrationEntClient).(*settingRepository)
	key := fmt.Sprintf("test_model_billing_cas_%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = repo.Delete(ctx, key) })
	for _, firstWrite := range []bool{true, false} {
		expectedValue, expectedRevision := "", ""
		if !firstWrite {
			var err error
			expectedValue, expectedRevision, err = repo.GetValueWithRevision(ctx, key)
			require.NoError(t, err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				_, err := repo.CompareAndSet(ctx, key, expectedRevision, expectedValue, fmt.Sprintf("candidate-%t-%d", firstWrite, i))
				results <- err
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		successes, conflicts := 0, 0
		for err := range results {
			if err == nil {
				successes++
			} else {
				require.ErrorIs(t, err, service.ErrModelBillingRevisionConflict)
				conflicts++
			}
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, conflicts)
	}
}

func TestModelBillingPostgresServiceCreateAndSave(t *testing.T) {
	ctx := context.Background()
	repo := NewSettingRepository(testEntTx(t).Client())
	svc := service.NewSettingService(repo, nil)
	initial, err := svc.GetModelBillingConfig(ctx)
	require.NoError(t, err)
	require.Empty(t, initial.Revision)
	first, err := svc.SaveModelBillingConfigJSON(ctx, []byte(`{"enabled":true,"rules":[{"model":"gpt-5","multiplier":2,"group_ids":[]}]}`), initial.Revision)
	require.NoError(t, err)
	second, err := svc.SaveModelBillingConfigJSON(ctx, []byte(`{"enabled":false}`), first.Revision)
	require.NoError(t, err)
	stored, err := svc.GetModelBillingConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, second, stored)
	require.False(t, stored.Config.Enabled)
	require.Len(t, stored.Config.Rules, 1)
	_, err = svc.SaveModelBillingConfigJSON(ctx, []byte(`{"enabled":true}`), first.Revision)
	require.ErrorIs(t, err, service.ErrModelBillingRevisionConflict)
}
