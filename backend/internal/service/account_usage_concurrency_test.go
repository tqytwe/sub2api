package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

// No sockets: every upstream operation is intercepted, including unexpected
// generation/reset requests. Channels and synctest.Wait establish ordering.
type quotaReadAttempt struct {
	ctx     context.Context
	release chan struct{}
	status  int
	used    int
	closed  bool
}
type quotaReadBody struct {
	io.Reader
	attempt *quotaReadAttempt
}

func (b *quotaReadBody) Close() error { b.attempt.closed = true; return nil }

// Timer expiry alone does not establish a race-detector happens-before edge.
// Protect the attempt registry even where virtual time makes ordering explicit.
type quotaReadAttempts struct {
	mu    sync.Mutex
	items []*quotaReadAttempt
}

func (a *quotaReadAttempts) snapshot() []*quotaReadAttempt {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*quotaReadAttempt(nil), a.items...)
}
func concurrentQuotaService(t *testing.T) (*AccountUsageService, *quotaReadAttempts) {
	t.Helper()
	account := &Account{ID: 9234, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "fake-token", "chatgpt_account_id": "fake-account"}}
	repo := &readOnlyUsageRepo{stubQuotaAccountRepo: &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	tokens := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fake-token"}}
	attempts := &quotaReadAttempts{}
	factory := func(string) (*req.Client, error) {
		c := req.C()
		c.GetClient().Transport = readOnlyUsageTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet || r.URL.Path != "/backend-api/wham/usage" {
				t.Errorf("forbidden upstream operation: %s %s", r.Method, r.URL.Path)
				return nil, fmt.Errorf("forbidden operation")
			}
			attempts.mu.Lock()
			a := &quotaReadAttempt{ctx: r.Context(), release: make(chan struct{}), status: 200, used: len(attempts.items) + 1}
			attempts.items = append(attempts.items, a)
			attempts.mu.Unlock()
			select {
			case <-a.release:
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}
			body := fmt.Sprintf(`{"rate_limit":{"primary_window":{"used_percent":%d,"limit_window_seconds":18000}}}`, a.used)
			return &http.Response{StatusCode: a.status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: &quotaReadBody{Reader: strings.NewReader(body), attempt: a}, Request: r}, nil
		})
		return c, nil
	}
	return &AccountUsageService{accountRepo: repo, cache: NewUsageCache(), openAIQuotaService: NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, tokens, nil), factory, nil)}, attempts
}
func startQuotaRead(s *AccountUsageService, ctx context.Context, force bool) <-chan *openAIUsageCacheEntry {
	done := make(chan *openAIUsageCacheEntry, 1)
	go func() { done <- s.readOpenAICodexSnapshot(ctx, 9234, false, force) }()
	synctest.Wait()
	return done
}
func quotaReadReady(t *testing.T, done <-chan *openAIUsageCacheEntry) *openAIUsageCacheEntry {
	t.Helper()
	select {
	case result := <-done:
		return result
	default:
		t.Fatal("waiter did not return independently")
		return nil
	}
}
func TestAccountUsageConcurrency_CancelWaiters(t *testing.T) {
	for _, cancelFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(cancelFirst), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, attempts := concurrentQuotaService(t)
				ctx1, cancel1 := context.WithCancel(context.Background())
				defer cancel1()
				ctx2, cancel2 := context.WithCancel(context.Background())
				defer cancel2()
				first := startQuotaRead(s, ctx1, false)
				second := startQuotaRead(s, ctx2, false)
				require.Len(t, attempts.snapshot(), 1)
				canceled, survivor := first, second
				if cancelFirst {
					cancel1()
				} else {
					cancel2()
					canceled, survivor = second, first
				}
				synctest.Wait()
				require.ErrorIs(t, quotaReadReady(t, canceled).err, context.Canceled)
				require.NoError(t, attempts.snapshot()[0].ctx.Err(), "one caller must not cancel shared read")
				close(attempts.snapshot()[0].release)
				synctest.Wait()
				result := quotaReadReady(t, survivor)
				require.NoError(t, result.err)
				require.Same(t, result, s.readOpenAICodexSnapshot(context.Background(), 9234, false, false))
				require.Len(t, attempts.snapshot(), 1)
				require.True(t, attempts.snapshot()[0].closed)
				require.ErrorIs(t, attempts.snapshot()[0].ctx.Err(), context.Canceled, "shared context released after success")
			})
		})
	}
}
func TestAccountUsageConcurrency_ForceAfterOrdinary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		ordinary := startQuotaRead(s, context.Background(), false)
		force1 := startQuotaRead(s, context.Background(), true)
		force2 := startQuotaRead(s, context.Background(), true)
		require.Len(t, attempts.snapshot(), 1, "serialize refreshes so older persistence cannot overwrite newer")
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		old := quotaReadReady(t, ordinary)
		require.NoError(t, old.err)
		require.Len(t, attempts.snapshot(), 2, "force needs a new read after a preexisting ordinary flight")
		require.Same(t, old, s.readOpenAICodexSnapshot(context.Background(), 9234, false, false), "ordinary cache hit remains available while force runs")
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		fresh := quotaReadReady(t, force1)
		require.NoError(t, fresh.err)
		require.NotSame(t, old, fresh)
		require.Same(t, fresh, quotaReadReady(t, force2))
		require.Same(t, fresh, s.readOpenAICodexSnapshot(context.Background(), 9234, false, false))
		require.Equal(t, float64(2), fresh.updates["codex_5h_used_percent"])
	})
}
func TestAccountUsageConcurrency_OrdinaryJoinsForce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		force := startQuotaRead(s, context.Background(), true)
		ordinary := startQuotaRead(s, context.Background(), false)
		require.Len(t, attempts.snapshot(), 1)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.Same(t, quotaReadReady(t, force), quotaReadReady(t, ordinary))
	})
}
func TestAccountUsageConcurrency_AllCanceledAndTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		ctx, cancel := context.WithCancel(context.Background())
		first := startQuotaRead(s, ctx, false)
		cancel()
		synctest.Wait()
		require.ErrorIs(t, quotaReadReady(t, first).err, context.Canceled)
		require.NoError(t, attempts.snapshot()[0].ctx.Err())
		time.Sleep(openaiQuotaUpstreamTimeout)
		synctest.Wait()
		require.ErrorIs(t, attempts.snapshot()[0].ctx.Err(), context.DeadlineExceeded)
		// A real shared timeout keeps the existing operational-error backoff.
		result := s.readOpenAICodexSnapshot(context.Background(), 9234, false, false)
		require.Error(t, result.err)
		require.Len(t, attempts.snapshot(), 1)
		retry := startQuotaRead(s, context.Background(), true)
		require.Len(t, attempts.snapshot(), 2, "completed timed-out flight must be released")
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, retry).err)
	})
}
func TestAccountUsageConcurrency_ErrorCacheAndExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		first := startQuotaRead(s, context.Background(), false)
		attempts.snapshot()[0].status = 429
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		result := quotaReadReady(t, first)
		require.Error(t, result.err)
		require.Same(t, result, s.readOpenAICodexSnapshot(context.Background(), 9234, false, false))
		time.Sleep(apiErrorCacheTTL)
		next := startQuotaRead(s, context.Background(), false)
		require.Len(t, attempts.snapshot(), 2)
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, next).err)
		time.Sleep(openAIProbeCacheTTL)
		next = startQuotaRead(s, context.Background(), false)
		require.Len(t, attempts.snapshot(), 3)
		close(attempts.snapshot()[2].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, next).err)
	})
}
func TestAccountUsageConcurrency_AlreadyCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result := s.readOpenAICodexSnapshot(ctx, 9234, false, true)
		require.ErrorIs(t, result.err, context.Canceled)
		require.Empty(t, attempts.snapshot())
		_, cached := s.cache.openAIUsageCache.Load(int64(9234))
		require.False(t, cached)
	})
}

func TestAccountUsageConcurrency_CanceledQueuedForce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		ordinary := startQuotaRead(s, context.Background(), false)
		ctx, cancel := context.WithCancel(context.Background())
		force := startQuotaRead(s, ctx, true)
		cancel()
		synctest.Wait()
		require.ErrorIs(t, quotaReadReady(t, force).err, context.Canceled)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, ordinary).err)
		require.Len(t, attempts.snapshot(), 1, "canceled queued force must not start another read")
		require.Empty(t, s.cache.openAIUsageFlights)
	})
}
func TestAccountUsageConcurrency_WithoutCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		s.cache = nil
		ctx, cancel := context.WithCancel(context.Background())
		done := startQuotaRead(s, ctx, false)
		cancel()
		synctest.Wait()
		require.ErrorIs(t, quotaReadReady(t, done).err, context.Canceled)
		require.NoError(t, attempts.snapshot()[0].ctx.Err())
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.True(t, attempts.snapshot()[0].closed)
		require.ErrorIs(t, attempts.snapshot()[0].ctx.Err(), context.Canceled)
	})
}

type quotaPreparationRepo struct {
	AccountRepository
	ctx     context.Context
	release chan struct{}
}

func (r *quotaPreparationRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.ctx = ctx
	select {
	case <-r.release:
		return r.AccountRepository.GetByID(ctx, id)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func TestAccountUsageConcurrency_DeadlineIncludesPreparation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		repo := &quotaPreparationRepo{AccountRepository: s.accountRepo, release: make(chan struct{})}
		defer close(repo.release)
		s.openAIQuotaService.accountRepo = repo
		done := startQuotaRead(s, context.Background(), false)
		require.NotNil(t, repo.ctx)
		deadline, ok := repo.ctx.Deadline()
		require.True(t, ok, "credential preparation must have a shared deadline")
		require.Equal(t, openaiQuotaUpstreamTimeout, time.Until(deadline))
		time.Sleep(openaiQuotaUpstreamTimeout)
		synctest.Wait()
		require.ErrorIs(t, quotaReadReady(t, done).err, context.DeadlineExceeded)
		require.Empty(t, attempts.snapshot())
		require.Empty(t, s.cache.openAIUsageFlights)
	})
}

func TestAccountUsageConcurrency_ShadowIsolation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		repo, ok := s.accountRepo.(*readOnlyUsageRepo)
		require.True(t, ok)
		parentID := int64(9234)
		repo.accounts[9235] = &Account{ID: 9235, ParentAccountID: &parentID, Platform: PlatformOpenAI, Type: AccountTypeOAuth, QuotaDimension: QuotaDimensionSpark}
		parent := startQuotaRead(s, context.Background(), false)
		shadow := make(chan *openAIUsageCacheEntry, 1)
		go func() { shadow <- s.readOpenAICodexSnapshot(context.Background(), 9235, true, false) }()
		synctest.Wait()
		require.Len(t, attempts.snapshot(), 2, "shared credentials must not merge distinct quota dimensions")
		close(attempts.snapshot()[1].release)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		parentResult, shadowResult := quotaReadReady(t, parent), quotaReadReady(t, shadow)
		require.NoError(t, parentResult.err)
		require.NoError(t, shadowResult.err)
		require.Equal(t, float64(1), parentResult.updates["codex_5h_used_percent"])
		require.Nil(t, shadowResult.updates["codex_5h_used_percent"], "absent Spark limits must not reuse parent quota")
		require.Same(t, parentResult, s.readOpenAICodexSnapshot(context.Background(), 9234, false, false))
		require.Same(t, shadowResult, s.readOpenAICodexSnapshot(context.Background(), 9235, true, false))
		require.Contains(t, repo.extraUpdates, int64(9234))
		require.Contains(t, repo.extraUpdates, int64(9235))
		require.Empty(t, s.cache.openAIUsageFlights)
	})
}

// Delay one already-queued caller as it resumes, without sleeps/scheduling luck.
type quotaDelayedContext struct {
	context.Context
	calls  int
	resume <-chan struct{}
}

func (c *quotaDelayedContext) Err() error {
	c.calls++
	if c.calls == 2 {
		<-c.resume
	}
	return c.Context.Err()
}
func TestAccountUsageConcurrency_QueuedForceSharesFastResult(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		defer func() {
			for _, a := range attempts.snapshot() {
				select {
				case <-a.release:
				default:
					close(a.release)
				}
			}
			synctest.Wait()
		}()
		ordinary := startQuotaRead(s, context.Background(), false)
		first := startQuotaRead(s, context.Background(), true)
		resume := make(chan struct{})
		delayed := &quotaDelayedContext{Context: context.Background(), resume: resume}
		second := startQuotaRead(s, delayed, true)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, ordinary).err)
		require.Len(t, attempts.snapshot(), 2)
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		fresh := quotaReadReady(t, first)
		require.NoError(t, fresh.err)
		close(resume)
		synctest.Wait()
		require.Len(t, attempts.snapshot(), 2, "all force callers queued behind one ordinary read share its successor even after completion")
		require.Same(t, fresh, quotaReadReady(t, second))
	})
}

type quotaPersistenceRepo struct {
	AccountRepository
	ctx   context.Context
	calls int
}

func (r *quotaPersistenceRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.calls++
	if r.calls == 1 {
		r.ctx = ctx
		<-ctx.Done()
		return ctx.Err()
	}
	return r.AccountRepository.UpdateExtra(ctx, id, updates)
}
func TestAccountUsageConcurrency_PersistenceBoundAndOrdering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		repo := &quotaPersistenceRepo{AccountRepository: s.accountRepo}
		s.accountRepo = repo
		ordinary := startQuotaRead(s, context.Background(), false)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.NotNil(t, repo.ctx)
		force := startQuotaRead(s, context.Background(), true)
		require.Len(t, attempts.snapshot(), 1, "next read must wait for old persistence")
		time.Sleep(openaiQuotaUpstreamTimeout)
		synctest.Wait()
		require.ErrorIs(t, repo.ctx.Err(), context.DeadlineExceeded)
		// A persistence failure has never invalidated a successful quota observation.
		require.NoError(t, quotaReadReady(t, ordinary).err)
		require.Len(t, attempts.snapshot(), 2)
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, force).err)
		require.Empty(t, s.cache.openAIUsageFlights)
	})
}

func TestAccountUsageConcurrency_QueuedForceCancellationKeepsSurvivor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, attempts := concurrentQuotaService(t)
		ordinary := startQuotaRead(s, context.Background(), false)
		ctx, cancel := context.WithCancel(context.Background())
		canceled := startQuotaRead(s, ctx, true)
		survivor := startQuotaRead(s, context.Background(), true)
		cancel()
		synctest.Wait()
		require.ErrorIs(t, quotaReadReady(t, canceled).err, context.Canceled)
		close(attempts.snapshot()[0].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, ordinary).err)
		require.Len(t, attempts.snapshot(), 2)
		close(attempts.snapshot()[1].release)
		synctest.Wait()
		require.NoError(t, quotaReadReady(t, survivor).err)
		require.Empty(t, s.cache.openAIUsageFlights)
	})
}
