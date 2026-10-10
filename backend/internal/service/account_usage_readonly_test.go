package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	httppool "github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/stretchr/testify/require"
)

type readOnlyUsageRepo struct {
	*stubQuotaAccountRepo
	mu sync.Mutex
}

func (r *readOnlyUsageRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stubQuotaAccountRepo.UpdateExtra(ctx, id, updates)
}

func (r *readOnlyUsageRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	var accounts []*Account
	for _, id := range ids {
		account, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

type readOnlyUsageTransport func(*http.Request) (*http.Response, error)

func (f readOnlyUsageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Both the quota client and the legacy pooled generation client are redirected
// to this fake server. Even the RED regression run cannot reach a real upstream.
func newReadOnlyUsageTestService(t *testing.T, status int, body string) (*AccountUsageService, *Account, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	reads, forbidden := &atomic.Int32{}, &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/backend-api/wham/usage" {
			forbidden.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		reads.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	proxy := &Proxy{ID: 1, Protocol: "http", Host: u.Hostname(), Port: port}
	account := &Account{ID: 9234, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		ProxyID: &proxy.ID, Proxy: proxy, Credentials: map[string]any{"access_token": "fake-token", "chatgpt_account_id": "fake-account"}}
	legacy, err := httppool.GetClient(httppool.Options{ProxyURL: proxy.URL(), Timeout: 15 * time.Second, ResponseHeaderTimeout: 10 * time.Second})
	require.NoError(t, err)
	original := legacy.Transport
	legacy.Transport = readOnlyUsageTransport(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.URL.Scheme = u.Scheme
		r.URL.Host = u.Host
		return http.DefaultTransport.RoundTrip(r)
	})
	t.Cleanup(func() { legacy.Transport = original })
	repo := &readOnlyUsageRepo{stubQuotaAccountRepo: &stubQuotaAccountRepo{accounts: map[int64]*Account{account.ID: account}}}
	tokens := &stubQuotaTokenCache{tokens: map[string]string{OpenAITokenCacheKey(account): "fake-token"}}
	quota := NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, tokens, nil), newQuotaRedirectingFactory(srv), nil)
	return &AccountUsageService{accountRepo: repo, usageLogRepo: &usageBatchLogRepoStub{}, openAIQuotaService: quota, cache: NewUsageCache()}, account, reads, forbidden
}

func TestAccountUsageReadOnly_AllEntrypoints(t *testing.T) {
	for _, entry := range []string{"ordinary", "force", "batch", "batch-force", "channel"} {
		t.Run(entry, func(t *testing.T) {
			reset := time.Now().Add(time.Hour).Unix()
			body := fmt.Sprintf(`{"rate_limit":{"primary_window":{"used_percent":37.5,"limit_window_seconds":18000,"reset_at":%d},"secondary_window":{"used_percent":81,"limit_window_seconds":604800,"reset_after_seconds":86400}},"credits":{"has_credits":true,"unlimited":false,"balance":"12345.1234567890"},"rate_limit_reset_credits":{"available_count":3}}`, reset)
			s, account, reads, forbidden := newReadOnlyUsageTestService(t, http.StatusOK, body)
			var usage *UsageInfo
			var err error
			switch entry {
			case "ordinary", "force":
				usage, err = s.GetUsage(context.Background(), account.ID, entry == "force")
			case "batch", "batch-force":
				var all map[int64]*UsageInfo
				var failures map[int64]string
				all, failures, err = s.GetUsageBatch(context.Background(), []int64{account.ID, account.ID}, entry == "batch-force")
				require.Empty(t, failures)
				usage = all[account.ID]
			case "channel":
				fetcher := &ChannelMonitorQuotaFetcher{usage: s, accounts: s.accountRepo, cache: make(map[int64]monitorQuotaCacheEntry)}
				snapshot := fetcher.Fetch(context.Background(), account.ID)
				require.True(t, snapshot.Success)
				require.Len(t, snapshot.Tiers, 2)
				if len(snapshot.Tiers) == 2 {
					require.Equal(t, 37.5, snapshot.Tiers[0].UsedPercent)
					require.Equal(t, 81.0, snapshot.Tiers[1].UsedPercent)
				}
			}
			require.NoError(t, err)
			require.Zero(t, forbidden.Load(), "quota reads must never generate or consume reset credits")
			require.EqualValues(t, 1, reads.Load())
			if entry != "channel" {
				require.NotNil(t, usage.FiveHour)
				require.Equal(t, 37.5, usage.FiveHour.Utilization)
				require.NotNil(t, usage.FiveHour.ResetsAt)
				require.Equal(t, reset, usage.FiveHour.ResetsAt.Unix())
				require.NotNil(t, usage.SevenDay)
				require.Equal(t, 81.0, usage.SevenDay.Utilization)
			}
		})
	}
}

func TestAccountUsageReadOnly_FailureNeverGeneratesAndThrottles(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			s, account, reads, forbidden := newReadOnlyUsageTestService(t, status, `{"error":"fake-upstream-sensitive-detail"}`)
			for range 2 {
				usage, err := s.GetUsage(context.Background(), account.ID)
				require.NoError(t, err)
				require.NotEmpty(t, usage.ErrorCode)
				require.NotContains(t, usage.Error, "sensitive-detail")
				require.Nil(t, usage.FiveHour)
				require.Nil(t, usage.SevenDay)
			}
			require.EqualValues(t, 1, reads.Load())
			_, err := s.GetUsage(context.Background(), account.ID, true)
			require.NoError(t, err)
			require.EqualValues(t, 2, reads.Load())
			require.Zero(t, forbidden.Load())
		})
	}
}

func TestAccountUsageReadOnly_MissingWindowsStayUnknown(t *testing.T) {
	for _, body := range []string{`{}`, `{"rate_limit":{"primary_window":{},"secondary_window":null}}`, `{"rate_limit":{"primary_window":{"used_percent":null}}}`, `{"rate_limit":{"primary_window":{"used_percent":12}}}`} {
		s, account, _, forbidden := newReadOnlyUsageTestService(t, 200, body)
		account.Extra = map[string]any{"codex_5h_used_percent": 71.0, "codex_7d_used_percent": 92.0, "codex_primary_used_percent": 92.0}
		usage, err := s.GetUsage(context.Background(), account.ID, true)
		require.NoError(t, err)
		require.Nil(t, usage.FiveHour)
		require.Nil(t, usage.SevenDay)
		require.Zero(t, forbidden.Load())
		repo, ok := s.accountRepo.(*readOnlyUsageRepo)
		require.True(t, ok)
		stored := repo.extraUpdates[account.ID]
		five, seven := openAICanonicalQuotaWindows(stored, time.Now())
		require.False(t, five.hasUsed)
		require.False(t, seven.hasUsed, "legacy raw fields must not resurrect an absent window")
	}
}

func TestAccountUsageReadOnly_CacheForceAndExpiry(t *testing.T) {
	s, account, reads, forbidden := newReadOnlyUsageTestService(t, 200, `{"rate_limit":{"primary_window":{"used_percent":0,"limit_window_seconds":18000},"secondary_window":{"used_percent":19,"limit_window_seconds":604800}},"credits":{"has_credits":true,"unlimited":false,"balance":"12345.1234567890"}}`)
	for range 2 {
		usage, err := s.GetUsage(context.Background(), account.ID)
		require.NoError(t, err)
		require.Equal(t, 0.0, usage.FiveHour.Utilization)
		require.Nil(t, usage.FiveHour.ResetsAt, "missing reset must not be synthesized as now")
		require.Equal(t, 19.0, usage.SevenDay.Utilization)
	}
	require.Nil(t, account.Extra, "queries must not mutate shared input accounts")
	require.EqualValues(t, 1, reads.Load())
	repo, ok := s.accountRepo.(*readOnlyUsageRepo)
	require.True(t, ok)
	cachedCredits, ok := repo.extraUpdates[account.ID][openaiQuotaCreditsKey].(openAICreditsSnapshot)
	require.True(t, ok)
	require.Equal(t, "12345.1234567890", *cachedCredits.Credits.Balance)
	require.Positive(t, cachedCredits.FetchedAt)
	require.NotContains(t, repo.extraUpdates[account.ID], openaiQuotaResetCreditsKey)
	_, err := s.GetUsage(context.Background(), account.ID, true)
	require.NoError(t, err)
	require.EqualValues(t, 2, reads.Load())
	cached, _ := s.cache.openAIUsageCache.Load(account.ID)
	cachedEntry, ok := cached.(*openAIUsageCacheEntry)
	require.True(t, ok)
	entry := *cachedEntry
	entry.timestamp = time.Now().Add(-openAIProbeCacheTTL - time.Second)
	s.cache.openAIUsageCache.Store(account.ID, &entry)
	_, err = s.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.EqualValues(t, 3, reads.Load())
	require.Zero(t, forbidden.Load())
}

func TestAccountUsageReadOnly_ConcurrentReadsCoalesce(t *testing.T) {
	s, account, reads, forbidden := newReadOnlyUsageTestService(t, 200, `{"rate_limit":{"primary_window":{"used_percent":12,"limit_window_seconds":18000}}}`)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			usage, err := s.GetUsage(context.Background(), account.ID)
			if err != nil || usage.FiveHour == nil || usage.FiveHour.Utilization != 12 {
				t.Errorf("invalid concurrent usage: %#v %v", usage, err)
			}
		})
	}
	wg.Wait()
	require.EqualValues(t, 1, reads.Load())
	require.Zero(t, forbidden.Load())
}

func TestAccountUsageReadOnly_DoesNotNotifyAutoReset(t *testing.T) {
	auto := NewOpenAIQuotaAutoResetService(nil, nil, nil, nil, nil, nil, nil)
	setOpenAIAutoResetNotifier(auto)
	t.Cleanup(func() { clearOpenAIAutoResetNotifier(auto); auto.cancel() })
	s, account, _, forbidden := newReadOnlyUsageTestService(t, 200, `{"rate_limit":{"primary_window":{"used_percent":100,"limit_window_seconds":18000}},"rate_limit_reset_credits":{"available_count":3}}`)
	_, err := s.GetUsage(context.Background(), account.ID, true)
	require.NoError(t, err)
	require.Empty(t, auto.queue)
	require.Zero(t, forbidden.Load())
}

func TestAccountUsageReadOnly_FailurePreservesObservation(t *testing.T) {
	s, account, reads, forbidden := newReadOnlyUsageTestService(t, 429, `{"error":"limited"}`)
	observed := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	account.Extra = map[string]any{"codex_usage_updated_at": observed.Format(time.RFC3339), "codex_5h_used_percent": 88.0}
	usage, err := s.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, 88.0, usage.FiveHour.Utilization)
	require.Equal(t, observed, *usage.UpdatedAt)
	require.Equal(t, "rate_limited", usage.ErrorCode)
	cached, _ := s.cache.openAIUsageCache.Load(account.ID)
	cachedEntry, ok := cached.(*openAIUsageCacheEntry)
	require.True(t, ok)
	entry := *cachedEntry
	entry.timestamp = time.Now().Add(-apiErrorCacheTTL - time.Second)
	s.cache.openAIUsageCache.Store(account.ID, &entry)
	_, err = s.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.EqualValues(t, 2, reads.Load())
	require.Zero(t, forbidden.Load())
}
