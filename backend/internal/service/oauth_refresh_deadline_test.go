package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Advance only the instance clock: the real hour-long context timer never fires.
// This synchronously models an elapsed deadline with its cancellation callback pending.
type deadlineRefreshExecutor struct {
	beforeReturn func(context.Context)
	calls        int
}

func (*deadlineRefreshExecutor) CanRefresh(*Account) bool                  { return true }
func (*deadlineRefreshExecutor) NeedsRefresh(*Account, time.Duration) bool { return true }
func (*deadlineRefreshExecutor) CacheKey(*Account) string                  { return "test:deadline:refresh" }
func (r *deadlineRefreshExecutor) Refresh(ctx context.Context, _ *Account) (map[string]any, error) {
	r.calls++
	if r.beforeReturn != nil {
		r.beforeReturn(ctx)
	}
	return map[string]any{"access_token": "synthetic-new", "refresh_token": "synthetic-refresh"}, nil
}

type deadlineRefreshRepo struct {
	*poolHealthAccountRepo
	account Account
}

func (r *deadlineRefreshRepo) GetByID(context.Context, int64) (*Account, error) {
	account := r.account
	account.Credentials = shallowCopyMap(r.account.Credentials)
	return &account, nil
}
func (r *deadlineRefreshRepo) UpdateCredentials(ctx context.Context, id int64, credentials map[string]any) error {
	r.account.Credentials = shallowCopyMap(credentials)
	return r.poolHealthAccountRepo.UpdateCredentials(ctx, id, credentials)
}
func (r *deadlineRefreshRepo) UpdateGrokOAuthCredentialsIfUnchanged(ctx context.Context, id int64, expected map[string]any, proxy *int64, credentials map[string]any) (bool, error) {
	r.account.Credentials = shallowCopyMap(credentials)
	return r.poolHealthAccountRepo.UpdateGrokOAuthCredentialsIfUnchanged(ctx, id, expected, proxy, credentials)
}

type deadlineRefreshCache struct {
	GeminiTokenCache
	release func(context.Context)
}

func (*deadlineRefreshCache) AcquireRefreshLock(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}
func (*deadlineRefreshCache) DeleteAccessToken(context.Context, string) error { return nil }
func (c *deadlineRefreshCache) ReleaseRefreshLock(ctx context.Context, _ string) error {
	c.release(ctx)
	return nil
}

func TestRefreshDeadline_PendingCancellationRejectsLateCredentials(t *testing.T) {
	for _, unified := range []bool{false, true} {
		for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
			t.Run(map[bool]string{false: "fallback", true: "unified"}[unified]+"/"+offset.String(), func(t *testing.T) {
				now := time.Now()
				repo := &deadlineRefreshRepo{poolHealthAccountRepo: &poolHealthAccountRepo{}, account: grokPoolAccount(501)}
				original := shallowCopyMap(repo.account.Credentials)
				svc := newDeadlineRefreshService(repo, func() time.Time { return now })
				executor := &deadlineRefreshExecutor{beforeReturn: func(ctx context.Context) {
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					now = deadline.Add(offset)
					require.NoError(t, ctx.Err(), "deadline callback is still pending")
				}}
				var unifiedExecutor OAuthRefreshExecutor
				if unified {
					svc.refreshAPI = NewOAuthRefreshAPI(repo, nil)
					svc.refreshAPI.deadlineNow = svc.deadlineNow
					unifiedExecutor = executor
				}
				account, err := repo.GetByID(context.Background(), 501)
				require.NoError(t, err)
				err = svc.refreshWithRetry(context.Background(), account, executor, unifiedExecutor, time.Hour)
				_, writes, permanent, cooldowns := repo.snapshot()
				if offset < 0 {
					require.NoError(t, err)
					require.Len(t, writes, 1)
					require.Zero(t, permanent)
					require.Zero(t, cooldowns)
					require.Equal(t, 1, executor.calls)
					return
				}
				var timeout *refreshAttemptTimeoutError
				require.ErrorAs(t, err, &timeout)
				require.Empty(t, writes)
				require.Equal(t, original, repo.account.Credentials)
				require.Zero(t, permanent)
				require.Equal(t, 1, cooldowns)
				require.Equal(t, 1, executor.calls)
			})
		}
	}
}

func TestRefreshDeadline_ParentExpiryDoesNotMutateAccount(t *testing.T) {
	for _, unified := range []bool{false, true} {
		t.Run(map[bool]string{false: "fallback", true: "unified"}[unified], func(t *testing.T) {
			now := time.Now()
			parent, cancel := context.WithTimeout(context.Background(), time.Hour)
			defer cancel()
			repo := &deadlineRefreshRepo{poolHealthAccountRepo: &poolHealthAccountRepo{}, account: grokPoolAccount(502)}
			svc := newDeadlineRefreshService(repo, func() time.Time { return now })
			svc.cfg.MaxRetries = 2
			svc.attemptTimeoutOverride = 2 * time.Hour
			executor := &deadlineRefreshExecutor{beforeReturn: func(ctx context.Context) {
				deadline, ok := parent.Deadline()
				require.True(t, ok)
				now = deadline
				require.NoError(t, parent.Err())
				require.NoError(t, ctx.Err())
			}}
			var unifiedExecutor OAuthRefreshExecutor
			if unified {
				svc.refreshAPI = NewOAuthRefreshAPI(repo, nil)
				svc.refreshAPI.deadlineNow = svc.deadlineNow
				unifiedExecutor = executor
			}
			account, err := repo.GetByID(parent, 502)
			require.NoError(t, err)
			err = svc.refreshWithRetry(parent, account, executor, unifiedExecutor, time.Hour)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			_, writes, permanent, cooldowns := repo.snapshot()
			require.Empty(t, writes)
			require.Zero(t, permanent)
			require.Zero(t, cooldowns)
			require.Equal(t, 1, executor.calls, "parent expiry must not retry")
		})
	}
}

func TestRefreshDeadline_DurableSuccessSurvivesCleanupExpiry(t *testing.T) {
	now := time.Now()
	repo := &deadlineRefreshRepo{poolHealthAccountRepo: &poolHealthAccountRepo{}, account: grokPoolAccount(503)}
	var attemptDeadline time.Time
	var attemptCtx context.Context
	cache := &deadlineRefreshCache{release: func(ctx context.Context) {
		_, writes, _, _ := repo.snapshot()
		require.Len(t, writes, 1, "advance only after durable persistence")
		now = attemptDeadline
		require.NoError(t, attemptCtx.Err())
		require.NoError(t, ctx.Err(), "lock release uses detached cleanup context")
	}}
	svc := newDeadlineRefreshService(repo, func() time.Time { return now })
	svc.cfg.MaxRetries = 2
	svc.cfg.ProviderFailureThreshold = 1
	svc.refreshAPI = NewOAuthRefreshAPI(repo, cache)
	svc.refreshAPI.deadlineNow = svc.deadlineNow
	executor := &deadlineRefreshExecutor{beforeReturn: func(ctx context.Context) {
		attemptCtx = ctx
		var ok bool
		attemptDeadline, ok = ctx.Deadline()
		require.True(t, ok)
	}}
	account, err := repo.GetByID(context.Background(), 503)
	require.NoError(t, err)
	state := &tokenRefreshProviderState{service: svc, rateGate: newTokenRefreshRateGate(10000), poolGate: newTokenRefreshConcurrencyGate(1)}
	err = svc.refreshWithRetryWithRateGate(context.Background(), account, executor, executor, time.Hour, state)
	state.recordResult(err)
	require.NoError(t, err)
	_, writes, permanent, cooldowns := repo.snapshot()
	require.Len(t, writes, 1)
	require.Zero(t, permanent)
	require.Zero(t, cooldowns)
	require.Equal(t, 1, executor.calls)
	require.False(t, state.isTripped())
}

func newDeadlineRefreshService(repo AccountRepository, now func() time.Time) *TokenRefreshService {
	return &TokenRefreshService{
		accountRepo:            repo,
		refreshPolicy:          DefaultBackgroundRefreshPolicy(),
		cfg:                    &config.TokenRefreshConfig{MaxRetries: 1},
		attemptTimeoutOverride: time.Hour,
		deadlineNow:            now,
	}
}

func TestRefreshDeadline_RequestAPIRejectsLateCredentials(t *testing.T) {
	for _, platform := range []string{PlatformGrok, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			now := time.Now()
			account := grokPoolAccount(504)
			account.Platform = platform
			account.Schedulable = true
			repo := &deadlineRefreshRepo{poolHealthAccountRepo: &poolHealthAccountRepo{}, account: account}
			original := shallowCopyMap(repo.account.Credentials)
			api := NewOAuthRefreshAPI(repo, nil)
			api.deadlineNow = func() time.Time { return now }
			ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
			defer cancel()
			executor := &deadlineRefreshExecutor{beforeReturn: func(ctx context.Context) {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				now = deadline
				require.NoError(t, ctx.Err())
			}}
			result, err := api.RefreshIfNeeded(withOAuthRefreshRequestPath(ctx), &account, executor, time.Hour)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Nil(t, result)
			_, writes, permanent, cooldowns := repo.snapshot()
			require.Empty(t, writes)
			require.Equal(t, original, repo.account.Credentials)
			require.Zero(t, permanent)
			require.Zero(t, cooldowns)
			require.Equal(t, 1, executor.calls)
		})
	}
}
