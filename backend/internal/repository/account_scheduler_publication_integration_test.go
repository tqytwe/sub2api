//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSchedulerPublicationPostgresReadsFreshStateAndHonorsSharedFence(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
	account := mustCreateAccount(t, client, &service.Account{
		Name: "publication-fence", Platform: service.PlatformOpenAI,
		Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "synthetic-old"},
	})
	t.Cleanup(func() { require.NoError(t, client.Account.DeleteOneID(account.ID).Exec(ctx)) })

	owner, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = owner.Rollback() })
	key := int64(uint64(account.ID) ^ uint64(0x5780000000000000))
	_, err = owner.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", key)
	require.NoError(t, err)
	short, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	called := false
	err = repo.WithLatestSchedulerAccounts(short, []int64{account.ID}, func([]service.Account) error {
		called = true
		return nil
	})
	require.Error(t, err, "a competing publisher must respect the same account fence")
	require.ErrorIs(t, short.Err(), context.DeadlineExceeded)
	require.False(t, called, "timed-out lock acquisition must not publish")
	_, err = owner.ExecContext(ctx, `UPDATE accounts SET schedulable=FALSE, credentials='{"api_key":"synthetic-new"}'::jsonb WHERE id=$1`, account.ID)
	require.NoError(t, err)
	require.NoError(t, owner.Commit())

	require.NoError(t, repo.WithLatestSchedulerAccounts(ctx, []int64{account.ID, account.ID}, func(accounts []service.Account) error {
		require.Len(t, accounts, 1)
		require.False(t, accounts[0].Schedulable)
		require.Equal(t, "synthetic-new", accounts[0].Credentials["api_key"])
		return nil
	}))
}
