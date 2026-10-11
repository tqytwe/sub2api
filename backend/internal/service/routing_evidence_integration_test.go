//go:build integration

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerRoutingActualSchedulersHTTP(t *testing.T) {
	db := ledgertest.New(t)
	for _, mode := range []string{"advanced", "legacy", "legacy_batch"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			accounts := []Account{{ID: 701, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1}, {ID: 702, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_batch"
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: accounts}, cache: &schedulerTestGatewayCache{}, cfg: cfg, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			if mode == "advanced" {
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			}
			l := requestledger.New(db)
			defer l.Stop()
			ids := make(chan string, 1)
			router := gin.New()
			router.Use(requestledger.Middleware(l))
			router.POST("/v1/responses", func(c *gin.Context) {
				ids <- requestledger.FromContext(c.Request.Context()).ID
				selection, _, err := svc.SelectAccountWithScheduler(c.Request.Context(), nil, "", "", "grok-4.5", nil, OpenAIUpstreamTransportHTTPSSE, false)
				require.NoError(t, err)
				require.EqualValues(t, 702, selection.Account.ID)
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				c.JSON(200, gin.H{"ok": true})
			})
			server := httptest.NewServer(router)
			defer server.Close()
			resp, err := http.Post(server.URL+"/v1/responses", "application/json", strings.NewReader(`{"secret":"routing-sentinel"}`))
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, 200, resp.StatusCode)
			id := <-ids
			var evidence *requestledger.RoutingEvidence
			require.Eventually(t, func() bool {
				evidence, err = l.RoutingEvidence(context.Background(), requestledger.Viewer{Admin: true, UserID: 1}, id)
				return err == nil && evidence.Status == "complete"
			}, time.Second, 10*time.Millisecond)
			require.Equal(t, 1, evidence.SelectionCount)
			found := false
			for _, event := range evidence.Events {
				if event.AccountID == 701 && event.Reason == "model_not_supported" {
					found = true
				}
			}
			require.True(t, found, "failed candidate must survive final success")
			data, err := json.Marshal(evidence)
			require.NoError(t, err)
			require.NotContains(t, string(data), "routing-sentinel")
			var attempts int
			require.NoError(t, db.QueryRow(`SELECT attempt_count FROM gateway_requests WHERE id=$1`, id).Scan(&attempts))
			require.Zero(t, attempts)
		})
	}
}

func TestRequestLedgerRoutingRecheckFallbackActionsUnchanged(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, fallback := range []bool{false, true} {
		var baselineAcquires, baselineReleases []int64
		for _, observed := range []bool{false, true} {
			group, other := int64(10), int64(20)
			first := &Account{ID: 801, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{group}}
			second := &Account{ID: 802, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10, GroupIDs: []int64{group}}
			moved := *first
			moved.GroupIDs = []int64{other}
			acquired, released := []int64{}, []int64{}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			cfg.Gateway.Scheduling.LoadBatchEnabled = true
			cache := schedulerTestConcurrencyCache{acquiredIDs: &acquired, releasedIDs: &released}
			if fallback {
				cache.acquireResults = map[int64]bool{801: false, 802: false}
			}
			svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{moved, *second}}, schedulerSnapshot: &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{first, second}, accountsByID: map[int64]*Account{801: first, 802: second}}}, concurrencyService: NewConcurrencyService(cache)}
			ctx := context.Background()
			var h *requestledger.Handle
			var end func(int64, bool, bool)
			if observed {
				var err error
				h, err = l.Begin(ctx, "/v1/messages", "POST", "http")
				require.NoError(t, err)
				ctx, end = requestledger.BeginRoutingSelection(requestledger.WithHandle(ctx, h), true)
			}
			var selection *AccountSelectionResult
			var err error
			if fallback {
				selection, err = svc.selectAccountWithLoadAwareness(ctx, &group, PlatformOpenAI, "", "gpt-5.1", nil, false, "", false)
			} else {
				scheduler := &defaultOpenAIAccountScheduler{service: svc}
				selection, _, err = scheduler.tryAcquireOpenAISelectionOrder(ctx, OpenAIAccountScheduleRequest{GroupID: &group, Platform: PlatformOpenAI, RequestedModel: "gpt-5.1"}, []openAIAccountCandidateScore{{account: first, loadInfo: &AccountLoadInfo{}}, {account: second, loadInfo: &AccountLoadInfo{}}})
			}
			require.NoError(t, err)
			require.EqualValues(t, 802, selection.Account.ID)
			require.Equal(t, fallback, selection.WaitPlan != nil)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if !observed {
				baselineAcquires = acquired
				baselineReleases = released
				continue
			}
			require.Equal(t, baselineAcquires, acquired)
			require.Equal(t, baselineReleases, released)
			end(selection.Account.ID, selection.WaitPlan != nil, false)
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			evidence := awaitRoutingEvidence(t, l, h.ID)
			found := false
			for _, event := range evidence.Events {
				if event.AccountID == 801 && event.Stage == "db_recheck" && event.Reason == "group_mismatch" {
					found = true
				}
			}
			require.True(t, found)
			var attempts, billing int
			require.NoError(t, db.QueryRow(`SELECT attempt_count FROM gateway_requests WHERE id=$1`, h.ID).Scan(&attempts))
			require.Zero(t, attempts)
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links WHERE request_id=$1`, h.ID).Scan(&billing))
			require.Zero(t, billing)
		}
	}
}

func TestRequestLedgerRoutingFirstVetoPoolAndPreviousResponse(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, scenario := range []string{"empty_pool", "first_veto", "previous_response", "sticky", "weighted_fallback"} {
		t.Run(scenario, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			h, err := l.Begin(context.Background(), "/v1/chat/completions", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			account := Account{ID: 901, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
			cfg := newSchedulerTestOpenAIWSV2Config()
			svc := &OpenAIGatewayService{cfg: cfg, accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:sentinel-session": 901}}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			var selection *AccountSelectionResult
			if scenario == "empty_pool" || scenario == "first_veto" {
				accounts := []Account{}
				if scenario == "first_veto" {
					account.Schedulable = false
					account.Type = AccountTypeOAuth
					accounts = []Account{account}
				}
				svc.accountRepo = schedulerTestOpenAIAccountRepo{accounts: accounts}
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
				selection, _, err = svc.SelectAccountWithScheduler(ctx, nil, "", "", "grok-4.5", nil, OpenAIUpstreamTransportHTTPSSE, false)
				require.ErrorIs(t, err, ErrNoAvailableAccounts)
				require.Nil(t, selection)
			} else {
				var end func(int64, bool, bool)
				ctx, end = requestledger.BeginRoutingSelection(ctx, true)
				scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
				req := OpenAIAccountScheduleRequest{Platform: PlatformOpenAI, SessionHash: "sentinel-session", StickyAccountID: 901, RequestedModel: "gpt-5.1", RequiredTransport: OpenAIUpstreamTransportHTTPSSE}
				switch scenario {
				case "previous_response":
					require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, 0, "resp_secret_sentinel", 901, time.Hour))
					req.ExcludedIDs = map[int64]struct{}{901: {}}
					selection, err = svc.selectAccountByPreviousResponseIDForCapability(ctx, nil, "resp_secret_sentinel", "gpt-5.1", req.ExcludedIDs, "", false)
				case "sticky":
					req.ExcludedIDs = map[int64]struct{}{901: {}}
					selection, _, err = scheduler.selectBySessionHash(ctx, req)
				case "weighted_fallback":
					req.StickyWeighted = true
					req.ExcludedIDs = map[int64]struct{}{901: {}}
					selection, err = scheduler.tryFallbackToWeightedSticky(ctx, req)
				}
				require.NoError(t, err)
				require.Nil(t, selection)
				end(0, false, true)
			}
			require.NoError(t, h.Finish(ctx, "failed", 503, "internal_error"))
			evidence := awaitRoutingEvidence(t, l, h.ID)
			if scenario == "empty_pool" {
				found := false
				for _, event := range evidence.Events {
					if event.Reason == "prefiltered_schedulable_pool" {
						require.Zero(t, event.Value)
						found = true
					}
				}
				require.True(t, found)
				require.False(t, evidence.SourcePopulationComplete)
			} else if scenario == "first_veto" {
				require.Equal(t, 1, evidence.VetoCount)
				require.Equal(t, "not_schedulable", evidence.Events[2].Reason)
			} else {
				require.Equal(t, 1, evidence.VetoCount)
			}
			data, err := json.Marshal(evidence)
			require.NoError(t, err)
			require.NotContains(t, string(data), "sentinel")
		})
	}
}

func awaitRoutingEvidence(t *testing.T, l *requestledger.Ledger, id string) *requestledger.RoutingEvidence {
	t.Helper()
	var evidence *requestledger.RoutingEvidence
	require.Eventually(t, func() bool {
		var err error
		evidence, err = l.RoutingEvidence(context.Background(), requestledger.Viewer{Admin: true, UserID: 1}, id)
		return err == nil && evidence.Status == "complete"
	}, 2*time.Second, 10*time.Millisecond)
	return evidence
}

func TestRequestLedgerRoutingCompactAndQuarantineSelections(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, scenario := range []string{"compact", "quarantine"} {
		t.Run(scenario, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			proxy := int64(71)
			account := Account{ID: 1001, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, ProxyID: &proxy, Extra: map[string]any{"openai_compact_supported": false}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"), concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			if scenario == "quarantine" {
				svc.openaiProxyStreamCircuit = newOpenAIProxyStreamCircuit(openAIProxyStreamCircuitSettings{failureThreshold: 1, failureWindow: time.Minute, quarantineTTL: 10 * time.Minute, maxEntries: 16})
				svc.openaiProxyStreamCircuit.recordFailure(proxy, time.Now())
			}
			h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			selection, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5.1", nil, OpenAIUpstreamTransportHTTPSSE, scenario == "compact")
			if scenario == "compact" {
				require.ErrorIs(t, err, ErrNoAvailableCompactAccounts)
				require.Nil(t, selection)
			} else {
				require.NoError(t, err)
				require.EqualValues(t, 1001, selection.Account.ID)
				selection.ReleaseFunc()
			}
			require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
			e := awaitRoutingEvidence(t, l, h.ID)
			wanted := "compact_unsupported"
			wantSelections := 1
			if scenario == "quarantine" {
				wanted = "proxy_stream_quarantined"
				wantSelections = 2
			}
			require.Equal(t, wantSelections, e.SelectionCount)
			found := false
			for _, event := range e.Events {
				if event.Reason == wanted {
					require.Equal(t, 1, event.SelectionNo)
					found = true
				}
			}
			require.True(t, found)
		})
	}
}

func TestRequestLedgerRoutingLegacyMissingStickyIsOneVeto(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	defer l.Stop()
	for _, batch := range []bool{false, true} {
		resetOpenAIAdvancedSchedulerSettingCacheForTest()
		cfg := &config.Config{}
		cfg.Gateway.Scheduling.LoadBatchEnabled = batch
		good := Account{ID: 1102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
		repo := &routingMissingAccountRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{good}}}
		svc := &OpenAIGatewayService{cfg: cfg, accountRepo: repo, cache: &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:missing": 1101}}, concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
		h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
		require.NoError(t, err)
		ctx := requestledger.WithHandle(context.Background(), h)
		selection, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "missing", "gpt-5.1", nil, OpenAIUpstreamTransportHTTPSSE, false)
		require.NoError(t, err)
		require.EqualValues(t, 1102, selection.Account.ID)
		selection.ReleaseFunc()
		require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
		e := awaitRoutingEvidence(t, l, h.ID)
		require.Equal(t, 1, e.VetoCount, "events: %+v", e.Events)
		found := false
		for _, event := range e.Events {
			if event.Reason == "account_missing" {
				require.Equal(t, "sticky", event.Stage)
				require.EqualValues(t, 1101, event.AccountID)
				found = true
			}
		}
		require.True(t, found)
	}
}

type routingMissingAccountRepo struct{ schedulerTestOpenAIAccountRepo }

func (r *routingMissingAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	if id == 1101 {
		return nil, nil
	}
	return r.schedulerTestOpenAIAccountRepo.GetByID(ctx, id)
}
