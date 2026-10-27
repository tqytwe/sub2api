package service

// Root review gap coverage for the quota-read authentication preparation path.
// Deterministic, no network beyond loopback: expired contexts must fail
// immediately instead of surfacing as silent skips or hanging without a bound.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Gap 1: the background cleanup context used when an OAuth account is disabled
// for a missing refresh token must carry a deadline. Without one a hung cache
// backend blocks DeleteAccessToken forever.
func TestDisableAccountMissingRefreshTokenBackgroundContextBounded(t *testing.T) {
	expired := time.Now().Add(-time.Hour)
	account := &Account{ID: 9234, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "fake-token", "chatgpt_account_id": "fake-account", "expires_at": expired.Format(time.RFC3339)}}
	repo := &setErrorCapturingRepo{stubQuotaAccountRepo: stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	cache := &ctxCapturingTokenCache{}
	provider := NewOpenAITokenProvider(repo, cache, nil)

	_, err := provider.GetAccessToken(context.Background(), account)
	require.Error(t, err, "expired token without refresh_token must fail")

	require.NotNil(t, cache.deleteCtx, "cache delete must be invoked")
	deadline, ok := cache.deleteCtx.Deadline()
	require.True(t, ok, "background cleanup context must have a deadline")
	require.Less(t, time.Until(deadline), 30*time.Second, "deadline must be a short cleanup bound")
}

// Gap 2: an already-expired work context must not silently skip agent-identity
// task registration. ensureAgentIdentityTaskForAccount must report the expired
// context explicitly rather than failing later inside the registration client.
func TestEnsureAgentIdentityTaskExpiredContextFailsFast(t *testing.T) {
	key, privateKey := newTestAgentIdentityKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"task_id":"task-late"}`))
	}))
	defer server.Close()
	oldBase := openAIAgentIdentityAuthAPIBaseURL
	openAIAgentIdentityAuthAPIBaseURL = server.URL
	t.Cleanup(func() { openAIAgentIdentityAuthAPIBaseURL = oldBase })

	account := &Account{ID: 41, Type: AccountTypeOAuth, Platform: PlatformOpenAI, Credentials: map[string]any{
		"auth_mode":         OpenAIAuthModeAgentIdentity,
		"agent_runtime_id":  key.runtimeID,
		"agent_private_key": privateKey,
	}}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	start := time.Now()
	var taskMu sync.Mutex
	err := ensureAgentIdentityTaskForAccount(ctx, repo, nil, &taskMu, account, "")
	require.ErrorIs(t, err, context.DeadlineExceeded, "expired work context must surface immediately")
	require.Less(t, time.Since(start), 5*time.Second, "must not wait for the registration client to time out")
	require.Empty(t, account.GetCredential("task_id"), "no task may be persisted for an expired context")
}

// Gap 3: closing invalidated agent-identity WebSocket connections must not
// hold the per-account task mutex. A blocked close would otherwise stall that
// account's next recovery behind its own synchronous cleanup. The invalidator
// runs with the lock released; the in-flight recovery itself keeps running.
func TestAgentIdentityWSInvalidationOutsideTaskLock(t *testing.T) {
	key, privateKey := newTestAgentIdentityKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"task_id":"task-ws"}`))
	}))
	defer server.Close()
	oldBase := openAIAgentIdentityAuthAPIBaseURL
	openAIAgentIdentityAuthAPIBaseURL = server.URL
	t.Cleanup(func() { openAIAgentIdentityAuthAPIBaseURL = oldBase })

	account := &Account{ID: 42, Type: AccountTypeOAuth, Platform: PlatformOpenAI, Credentials: map[string]any{
		"auth_mode":         OpenAIAuthModeAgentIdentity,
		"agent_runtime_id":  key.runtimeID,
		"agent_private_key": privateKey,
	}}
	repo := &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}

	entered := make(chan struct{})
	unblock := make(chan struct{})
	invalidator := &blockingWSInvalidator{entered: entered, unblock: unblock}

	var taskMu sync.Mutex
	done := make(chan error, 1)
	go func() {
		done <- ensureAgentIdentityTaskForAccount(context.Background(), repo, invalidator, &taskMu, account, "")
	}()

	// Wait until the first registration is inside the blocked WS close, then
	// recover a second account. It must not queue behind the first close.
	<-entered
	other := &Account{ID: 43, Type: AccountTypeOAuth, Platform: PlatformOpenAI, Credentials: map[string]any{
		"auth_mode":         OpenAIAuthModeAgentIdentity,
		"agent_runtime_id":  key.runtimeID,
		"agent_private_key": privateKey,
	}}
	repo.accounts[other.ID] = other
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- ensureAgentIdentityTaskForAccount(context.Background(), repo, nil, &taskMu, other, "")
	}()

	secondCompleted := false
	var secondErr error
	select {
	case secondErr = <-secondDone:
		secondCompleted = true
	case <-time.After(3 * time.Second):
	}
	close(unblock)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("first registration did not finish after its WS close was released")
	}
	if !secondCompleted {
		select {
		case secondErr = <-secondDone:
		case <-time.After(3 * time.Second):
			t.Fatal("second registration did not finish")
		}
	}
	require.NoError(t, secondErr)
	require.True(t, secondCompleted, "second account recovery must complete before the first caller's WS close finishes")
}

type setErrorCapturingRepo struct {
	stubQuotaAccountRepo
	setErrorCalls int
}

func (r *setErrorCapturingRepo) SetError(_ context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	return nil
}

type ctxCapturingTokenCache struct {
	stubQuotaTokenCache
	deleteCtx context.Context
}

func (c *ctxCapturingTokenCache) DeleteAccessToken(ctx context.Context, cacheKey string) error {
	c.deleteCtx = ctx
	return nil
}

type blockingWSInvalidator struct {
	entered chan struct{}
	unblock chan struct{}
	once    sync.Once
}

func (b *blockingWSInvalidator) InvalidateAgentIdentityWSConnections(accountID int64) {
	b.once.Do(func() { close(b.entered) })
	<-b.unblock
}
