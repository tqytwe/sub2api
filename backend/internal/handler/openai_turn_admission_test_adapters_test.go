package handler

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Keep admission reads tied to the fixture's current repository state. This
// adapter supplies no eligibility defaults: status, groups, proxies and parent
// availability remain the responsibility of each test fixture.
func readHandlerTestOpenAITurnAdmission(ctx context.Context, repo service.AccountRepository, id int64) (*service.Account, *service.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	account, err := repo.GetByID(ctx, id)
	if err != nil || account == nil || account.ParentAccountID == nil {
		return account, nil, err
	}
	parent, err := repo.GetByID(ctx, *account.ParentAccountID)
	return account, parent, err
}

func (r *openAIWSUsageHandlerAccountRepoStub) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	return readHandlerTestOpenAITurnAdmission(ctx, r, id)
}

func (r *openAIWSFailoverHandlerAccountRepoStub) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	return readHandlerTestOpenAITurnAdmission(ctx, r, id)
}

func (r codexModelsFailoverAccountRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*service.Account, *service.Account, error) {
	return readHandlerTestOpenAITurnAdmission(ctx, r, id)
}

var (
	_ service.OpenAITurnAdmissionReader = (*openAIWSUsageHandlerAccountRepoStub)(nil)
	_ service.OpenAITurnAdmissionReader = (*openAIWSFailoverHandlerAccountRepoStub)(nil)
	_ service.OpenAITurnAdmissionReader = codexModelsFailoverAccountRepo{}
)

func TestHandlerTurnAdmissionAdapterPreservesLatestState(t *testing.T) {
	repo := &openAIWSFailoverHandlerAccountRepoStub{accounts: []service.Account{{ID: 1, Status: service.StatusActive, Schedulable: true}}}
	account, parent, err := repo.GetOpenAITurnAdmission(t.Context(), 1)
	require.NoError(t, err)
	require.Nil(t, parent)
	require.Nil(t, account.RateLimitResetAt)
	reset := time.Now().Add(time.Minute)
	require.NoError(t, repo.SetRateLimited(t.Context(), 1, reset))
	repo.accounts[0].Status = service.StatusError
	repo.accounts[0].Schedulable = false
	account, _, err = repo.GetOpenAITurnAdmission(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, service.StatusError, account.Status)
	require.False(t, account.Schedulable)
	require.Equal(t, &reset, account.RateLimitResetAt)
	require.Empty(t, account.GroupIDs)
	require.Nil(t, account.Proxy)
}

func TestHandlerTurnAdmissionAdapterPreservesParentAndMissingState(t *testing.T) {
	parentID := int64(2)
	repo := codexModelsFailoverAccountRepo{accounts: []service.Account{
		{ID: 1, ParentAccountID: &parentID},
		{ID: 2, Status: service.StatusError},
	}}
	account, parent, err := repo.GetOpenAITurnAdmission(t.Context(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), account.ID)
	require.Equal(t, service.StatusError, parent.Status)
	require.Empty(t, account.Status)
	require.False(t, account.Schedulable)
	repo.accounts = repo.accounts[:1]
	account, parent, err = repo.GetOpenAITurnAdmission(t.Context(), 1)
	require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
	require.NotNil(t, account)
	require.Nil(t, parent)
	account, parent, err = repo.GetOpenAITurnAdmission(t.Context(), 99)
	require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
	require.Nil(t, account)
	require.Nil(t, parent)
}

func TestHandlerTurnAdmissionAdapterPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	account, parent, err := (&openAIWSUsageHandlerAccountRepoStub{}).GetOpenAITurnAdmission(ctx, 1)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, account)
	require.Nil(t, parent)
}
