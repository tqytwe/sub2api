//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func admittedOAuthTokenTestAccount() *Account {
	return &Account{
		ID: 901, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{
			"access_token": "admitted-test-bearer", "refresh_token": "admitted-test-refresh",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339), "_token_version": int64(200),
			"chatgpt_account_id": "admitted-test-identity",
		},
	}
}

type admittedOAuthRefreshExecutor struct{ refreshAPIExecutorStub }

func (*admittedOAuthRefreshExecutor) CacheKey(account *Account) string {
	return OpenAITokenCacheKey(account)
}

func TestOpenAITokenProvider_AdmittedSnapshotOverridesStaleCache(t *testing.T) {
	for _, shadow := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "shadow_parent"}[shadow], func(t *testing.T) {
			parent := admittedOAuthTokenTestAccount()
			selected := bindOpenAITurnCredentialParent(parent, nil)
			if shadow {
				selected = bindOpenAITurnCredentialParent(&Account{
					ID: 902, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent.ID,
				}, parent)
			}
			cache := newOpenAITokenCacheStub()
			cache.tokens[OpenAITokenCacheKey(parent)] = "stale-cache-test-bearer"
			// An ordinary repository read must not replace the admitted snapshot.
			repo := &refreshAPIAccountRepo{getByIDErr: errors.New("unexpected ordinary reread")}
			gateway := &OpenAIGatewayService{accountRepo: repo, openAITokenProvider: NewOpenAITokenProvider(repo, cache, nil)}
			token, kind, err := gateway.GetAccessToken(context.Background(), selected)
			require.NoError(t, err)
			require.Equal(t, "admitted-test-bearer", token)
			require.Equal(t, "oauth", kind)
			require.Zero(t, repo.getByIDCalls)
			require.Zero(t, atomic.LoadInt32(&cache.setCalled), "a snapshot must not overwrite a concurrent cache rotation")
		})
	}
}

func TestOpenAITokenProvider_AdmittedExpiredTokenStillRefreshes(t *testing.T) {
	account := admittedOAuthTokenTestAccount()
	account.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
	admitted := bindOpenAITurnCredentialParent(account, nil)
	cache := newOpenAITokenCacheStub()
	cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
	repo := &refreshAPIAccountRepo{account: snapshotOAuthRefreshAccount(account)}
	executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: MergeCredentials(account.Credentials, map[string]any{
			"access_token": "refreshed-test-bearer", "refresh_token": "refreshed-test-refresh",
			"expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		}),
	}}
	provider := NewOpenAITokenProvider(repo, cache, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
	token, err := provider.GetAccessToken(context.Background(), admitted)
	require.NoError(t, err)
	require.Equal(t, "refreshed-test-bearer", token)
	require.Equal(t, 1, executor.refreshCalls)
	require.Equal(t, 1, repo.updateCredentialsCalls)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.lockCalled))
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.unlockCalled))
	require.Equal(t, "admitted-test-bearer", admitted.GetOpenAIAccessToken())
}

func TestOpenAITokenProvider_AdmittedRefreshFailureNeverUsesStaleCache(t *testing.T) {
	account := admittedOAuthTokenTestAccount()
	account.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
	cache := newOpenAITokenCacheStub()
	cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
	repo := &refreshAPIAccountRepo{account: snapshotOAuthRefreshAccount(account)}
	executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{needsRefresh: true, err: errors.New("refresh unavailable")}}
	provider := NewOpenAITokenProvider(repo, cache, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
	token, err := provider.GetAccessToken(context.Background(), bindOpenAITurnCredentialParent(account, nil))
	require.Error(t, err)
	require.Empty(t, token)
	require.Equal(t, 1, executor.refreshCalls)
	require.Equal(t, int32(1), atomic.LoadInt32(&cache.unlockCalled))
}

func TestOpenAITokenProvider_AdmittedRefreshRejectsChangedOrOlderState(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func(*Account)
	}{
		{"older_generation", func(a *Account) { a.Credentials["_token_version"] = int64(199) }},
		{"different_unversioned_credentials", func(a *Account) { a.Credentials["access_token"] = "older-test-bearer" }},
		{"changed_identity", func(a *Account) { a.Credentials["chatgpt_account_id"] = "other-test-identity" }},
		{"disabled", func(a *Account) { a.Status = StatusDisabled }},
		{"unschedulable", func(a *Account) { a.Schedulable = false }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			account := admittedOAuthTokenTestAccount()
			account.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
			fresh := snapshotOAuthRefreshAccount(account)
			mutate.apply(fresh)
			cache := newOpenAITokenCacheStub()
			cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
			repo := &refreshAPIAccountRepo{account: fresh}
			executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{needsRefresh: true}}
			provider := NewOpenAITokenProvider(repo, cache, nil)
			provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
			token, err := provider.GetAccessToken(context.Background(), bindOpenAITurnCredentialParent(account, nil))
			require.Error(t, err)
			require.True(t, IsOpenAITurnAdmissionError(err), "credential-state denial stays local: %v", err)
			require.Empty(t, token)
			require.Zero(t, executor.refreshCalls, "do not refresh a different or older credential binding")
			require.Equal(t, int32(1), atomic.LoadInt32(&cache.unlockCalled))
		})
	}
}

func TestOpenAITokenProvider_AdmittedNearExpiryRefreshFailureUsesOnlyValidatedToken(t *testing.T) {
	for _, returnError := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep_valid_token", true: "return_policy_error"}[returnError], func(t *testing.T) {
			account := admittedOAuthTokenTestAccount()
			account.Credentials["expires_at"] = time.Now().Add(time.Minute).Format(time.RFC3339)
			cache := newOpenAITokenCacheStub()
			cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
			repo := &refreshAPIAccountRepo{account: snapshotOAuthRefreshAccount(account)}
			executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{needsRefresh: true, err: errors.New("refresh unavailable")}}
			provider := NewOpenAITokenProvider(repo, cache, nil)
			provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
			if returnError {
				policy := OpenAIProviderRefreshPolicy()
				policy.OnRefreshError = ProviderRefreshErrorReturn
				provider.SetRefreshPolicy(policy)
			}
			token, err := provider.GetAccessToken(context.Background(), bindOpenAITurnCredentialParent(account, nil))
			if returnError {
				require.Error(t, err)
				require.Empty(t, token)
			} else {
				require.NoError(t, err)
				require.Equal(t, "admitted-test-bearer", token)
			}
			require.Equal(t, 1, executor.refreshCalls)
			require.Zero(t, atomic.LoadInt32(&cache.setCalled))
		})
	}
}

func TestOpenAITokenProvider_AdmittedRefreshLockRaceRequiresDurableToken(t *testing.T) {
	for _, refreshed := range []bool{false, true} {
		t.Run(map[bool]string{false: "stale_cache_only", true: "concurrent_durable_refresh"}[refreshed], func(t *testing.T) {
			account := admittedOAuthTokenTestAccount()
			account.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
			fresh := snapshotOAuthRefreshAccount(account)
			if refreshed {
				fresh.Credentials["access_token"] = "concurrent-refreshed-test-bearer"
				fresh.Credentials["_token_version"] = int64(201)
				fresh.Credentials["expires_at"] = time.Now().Add(time.Hour).Format(time.RFC3339)
			}
			cache := newOpenAITokenCacheStub()
			cache.lockAcquired = false
			cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
			repo := &refreshAPIAccountRepo{account: fresh}
			executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{needsRefresh: true}}
			provider := NewOpenAITokenProvider(repo, cache, nil)
			provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
			token, err := provider.GetAccessToken(context.Background(), bindOpenAITurnCredentialParent(account, nil))
			if refreshed {
				require.NoError(t, err)
				require.Equal(t, "concurrent-refreshed-test-bearer", token)
			} else {
				require.Error(t, err)
				require.Empty(t, token)
			}
			require.Zero(t, executor.refreshCalls)
			require.Zero(t, atomic.LoadInt32(&cache.unlockCalled), "never release another refresher's lock")
		})
	}
}

func TestOpenAITokenProvider_AdmittedShadowRefreshKeepsParentSchedulingIndependent(t *testing.T) {
	parent := admittedOAuthTokenTestAccount()
	parent.Schedulable = false
	parent.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
	selected := bindOpenAITurnCredentialParent(&Account{ID: 902, Platform: PlatformOpenAI, Type: AccountTypeOAuth, ParentAccountID: &parent.ID}, parent)
	cache := newOpenAITokenCacheStub()
	cache.tokens[OpenAITokenCacheKey(parent)] = "stale-cache-test-bearer"
	repo := &refreshAPIAccountRepo{account: snapshotOAuthRefreshAccount(parent)}
	executor := &admittedOAuthRefreshExecutor{refreshAPIExecutorStub: refreshAPIExecutorStub{
		needsRefresh: true,
		credentials: MergeCredentials(parent.Credentials, map[string]any{
			"access_token": "refreshed-test-bearer", "expires_at": time.Now().Add(time.Hour).Format(time.RFC3339),
		}),
	}}
	provider := NewOpenAITokenProvider(repo, cache, nil)
	provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
	gateway := &OpenAIGatewayService{accountRepo: repo, openAITokenProvider: provider}
	token, _, err := gateway.GetAccessToken(context.Background(), selected)
	require.NoError(t, err)
	require.Equal(t, "refreshed-test-bearer", token)
	require.Equal(t, 1, executor.refreshCalls)
}

func TestOpenAITokenProvider_AdmittedImportedBearerWithoutExpiry(t *testing.T) {
	for _, expiry := range []string{"missing", "malformed"} {
		for _, refreshable := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/refreshable=%v", expiry, refreshable), func(t *testing.T) {
				account := admittedOAuthTokenTestAccount()
				delete(account.Credentials, "expires_at")
				if expiry == "malformed" {
					account.Credentials["expires_at"] = "not-a-timestamp"
				}
				if !refreshable {
					delete(account.Credentials, "refresh_token")
				}
				cache := newOpenAITokenCacheStub()
				cache.tokens[OpenAITokenCacheKey(account)] = "stale-cache-test-bearer"
				repo := &refreshAPIAccountRepo{account: snapshotOAuthRefreshAccount(account)}
				provider := NewOpenAITokenProvider(repo, cache, nil)
				// The real production policy skips provider refresh when expiry is
				// unknown and the account is not rate-limited. No OAuth transport
				// is configured, so an accidental refresh also fails the test.
				provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), NewOpenAITokenRefresher(nil, repo))
				token, err := provider.GetAccessToken(context.Background(), bindOpenAITurnCredentialParent(account, nil))
				require.NoError(t, err, "imported OAuth bearers retain their existing unknown-expiry contract")
				require.Equal(t, "admitted-test-bearer", token)
				require.Zero(t, repo.updateCredentialsCalls)
				require.Zero(t, atomic.LoadInt32(&cache.setCalled))
				if refreshable {
					require.Equal(t, 1, repo.getByIDCalls)
				} else {
					require.Zero(t, repo.getByIDCalls)
				}
			})
		}
	}
}

type cancelingAdmittedTokenRepo struct {
	refreshAPIAccountRepo
	cancel          context.CancelFunc
	waitForDeadline bool
}

func (r *cancelingAdmittedTokenRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	account, err := r.refreshAPIAccountRepo.GetByID(ctx, id)
	if r.waitForDeadline {
		<-ctx.Done()
	} else {
		r.cancel()
	}
	return account, err
}

func TestOpenAITokenProvider_AdmittedRefreshRejectsCancellationAfterDurableRead(t *testing.T) {
	for _, boundary := range []string{"cancel", "deadline"} {
		for _, locked := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/owns_lock=%v", boundary, locked), func(t *testing.T) {
				account := admittedOAuthTokenTestAccount()
				account.Credentials["expires_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339)
				fresh := snapshotOAuthRefreshAccount(account)
				fresh.Credentials["expires_at"] = time.Now().Add(time.Hour).Format(time.RFC3339)
				fresh.Credentials["access_token"] = "concurrent-refreshed-test-bearer"
				fresh.Credentials["_token_version"] = int64(201)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				repo := &cancelingAdmittedTokenRepo{refreshAPIAccountRepo: refreshAPIAccountRepo{account: fresh}, cancel: cancel, waitForDeadline: boundary == "deadline"}
				cache := newOpenAITokenCacheStub()
				cache.lockAcquired = locked
				executor := &admittedOAuthRefreshExecutor{}
				provider := NewOpenAITokenProvider(repo, cache, nil)
				provider.SetRefreshAPI(NewOAuthRefreshAPI(repo, cache), executor)
				token, err := provider.GetAccessToken(ctx, bindOpenAITurnCredentialParent(account, nil))
				require.True(t, IsOpenAITurnAdmissionError(err), "a late success after the request boundary must be rejected locally: %v", err)
				require.Empty(t, token)
				require.Zero(t, executor.refreshCalls)
			})
		}
	}
}
