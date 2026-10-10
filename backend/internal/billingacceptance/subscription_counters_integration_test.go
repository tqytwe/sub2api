//go:build integration && billing_acceptance

package billingacceptance

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// No provider endpoint is reachable: this adapter permits only the exact
// httptest origin. The real gateway parser and ledger transport remain in use.
type counterUpstream struct {
	origin string
	client *http.Client
}

func (u counterUpstream) Do(r *http.Request, _ string, account int64, _ int) (*http.Response, error) {
	if r.URL.Scheme+"://"+r.URL.Host != u.origin {
		return nil, fmt.Errorf("fixture refused nonlocal upstream")
	}
	client := *u.client
	client.Transport = requestledger.Transport(client.Transport, account)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return client.Do(r)
}
func (u counterUpstream) DoWithTLS(r *http.Request, p string, a int64, c int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, a, c)
}

type counterApply struct {
	command service.UsageBillingCommand
	result  *service.UsageBillingApplyResult
	err     error
}
type counterBilling struct {
	service.UsageBillingRepository
	mu    sync.Mutex
	calls []counterApply
}

func (r *counterBilling) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	result, err := r.UsageBillingRepository.Apply(ctx, cmd)
	r.mu.Lock()
	r.calls = append(r.calls, counterApply{*cmd, result, err})
	r.mu.Unlock()
	return result, err
}
func (r *counterBilling) snapshot() []counterApply {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]counterApply(nil), r.calls...)
}

type counterSnapshot struct {
	Daily    float64 `json:"daily_usage_usd"`
	Weekly   float64 `json:"weekly_usage_usd"`
	Monthly  float64 `json:"monthly_usage_usd"`
	Requests int64   `json:"request_used"`
	Amount   float64 `json:"amount_used_usd"`
	Tokens   int64   `json:"token_used"`
}

func (s counterSnapshot) minus(b counterSnapshot) counterSnapshot {
	return counterSnapshot{s.Daily - b.Daily, s.Weekly - b.Weekly, s.Monthly - b.Monthly, s.Requests - b.Requests, s.Amount - b.Amount, s.Tokens - b.Tokens}
}
func assertCounters(t *testing.T, want, got counterSnapshot) {
	t.Helper()
	assert.InDelta(t, want.Daily, got.Daily, 1e-10, "daily")
	assert.InDelta(t, want.Weekly, got.Weekly, 1e-10, "weekly")
	assert.InDelta(t, want.Monthly, got.Monthly, 1e-10, "monthly")
	assert.Equal(t, want.Requests, got.Requests, "requests")
	assert.InDelta(t, want.Amount, got.Amount, 1e-10, "amount")
	assert.Equal(t, want.Tokens, got.Tokens, "tokens")
}

type counterFixture struct {
	db                  *sql.DB
	client              *dbent.Client
	user                *service.User
	key                 *service.APIKey
	account             *service.Account
	sub                 *service.UserSubscription
	subscriptions       *service.SubscriptionService
	billing             *counterBilling
	gateway             *service.OpenAIGatewayService
	server              *httptest.Server
	upstream            *httptest.Server
	sends               atomic.Int64
	pricingAt           time.Time
	callbacks           int
	concurrentCallbacks bool
	cancelBeforeBilling bool
	partial             bool
	disconnect          bool
	retainedCtx         context.Context
	retainedInput       *service.OpenAIRecordUsageInput
	mu                  sync.Mutex
	settlementErrors    []error
	forwardErrors       []error
}

func newCounterFixture(t *testing.T, db *sql.DB, client *dbent.Client, packaged bool, multiplier float64, surcharge, frozen bool) *counterFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	f := &counterFixture{db: db, client: client, callbacks: 1, pricingAt: now}
	u, err := client.User.Create().SetEmail("counter-" + uuid.NewString() + "@example.invalid").SetPasswordHash("disabled-fixture").SetRole(service.RoleUser).SetStatus(service.StatusActive).SetBalance(100).SetConcurrency(16).Save(ctx)
	require.NoError(t, err)
	g, err := client.Group.Create().SetName("counter-" + uuid.NewString()).SetPlatform(service.PlatformOpenAI).SetSubscriptionType(service.SubscriptionTypeSubscription).SetStatus(service.StatusActive).SetRateMultiplier(multiplier).Save(ctx)
	require.NoError(t, err)
	k, err := client.APIKey.Create().SetUserID(u.ID).SetGroupID(g.ID).SetKey("disabled-fixture-" + uuid.NewString()).SetName("counter fixture").SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	s, err := client.UserSubscription.Create().SetUserID(u.ID).SetGroupID(g.ID).SetStatus(service.StatusActive).SetStartsAt(now.Add(-time.Hour)).SetExpiresAt(now.AddDate(0, 2, 0)).SetDailyWindowStart(day).SetWeeklyWindowStart(day).SetMonthlyWindowStart(day).SetDailyUsageUsd(1.125).SetWeeklyUsageUsd(2.25).SetMonthlyUsageUsd(3.5).Save(ctx)
	require.NoError(t, err)
	groupRepo := repository.NewGroupRepository(client, db)
	f.subscriptions = service.NewSubscriptionService(groupRepo, repository.NewUserSubscriptionRepository(client), nil, client, nil)
	t.Cleanup(f.subscriptions.Stop)
	f.user = &service.User{ID: u.ID, Balance: 100, Status: service.StatusActive}
	group, err := groupRepo.GetByID(ctx, g.ID)
	require.NoError(t, err)
	group.BillingSurchargeOverrideEnabled = true
	group.BillingSurchargeEnabled = surcharge
	group.BillingSurchargeMode = service.BillingSurchargeModeAdditiveMultiplier
	group.BillingSurchargeValue = 0.3
	if frozen {
		group.PeakRateEnabled = true
		peakHour := (now.Hour() + 6) % 23
		group.PeakStart = fmt.Sprintf("%02d:00", peakHour)
		group.PeakEnd = fmt.Sprintf("%02d:00", peakHour+1)
		group.PeakRateMultiplier = 3
		f.pricingAt = time.Date(2026, 1, 15, peakHour, 30, 0, 0, time.UTC)
		require.Equal(t, 1.0, group.PeakMultiplierAt(now), "record-time price must differ from frozen price")
	}
	f.key = &service.APIKey{ID: k.ID, UserID: u.ID, GroupID: &g.ID, Group: group}
	if packaged {
		_, err = db.Exec(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,request_used,amount_limit_usd,amount_used_usd,token_limit,token_used) VALUES($1,$2,'admin_grant','counter-' || $1::bigint::text,NOW()-INTERVAL '1 hour',NOW()+INTERVAL '30 days',100,7,1000,4.75,1000000,123)`, u.ID, g.ID)
		require.NoError(t, err)
	}
	f.sub, err = f.subscriptions.GetActiveSubscription(ctx, u.ID, g.ID)
	require.NoError(t, err)
	require.Equal(t, s.ID, f.sub.ID)
	f.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.sends.Add(1)
		var admitted int
		err := db.QueryRow(`SELECT COUNT(*) FROM gateway_request_attempts a JOIN gateway_requests q ON q.id=a.request_id WHERE a.account_id=$1 AND a.execution_state='inflight' AND q.user_id=$2 AND q.api_key_id=$3`, f.account.ID, u.ID, k.ID).Scan(&admitted)
		if err != nil || admitted == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if f.disconnect {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"synthetic partial"}`+"\n\n")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.in_progress","response":{"id":"synthetic-canceled","model":"gpt-5.1","usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}}`+"\n\n")
			if err := http.NewResponseController(w).Flush(); err != nil {
				return
			}
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		if f.partial {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"synthetic partial"}`+"\n\n")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.failed","response":{"id":"synthetic-partial","model":"gpt-5.1","status":"failed","error":{"code":"rate_limit_exceeded","message":"synthetic failure"},"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}}`+"\n\n")
			if err := http.NewResponseController(w).Flush(); err != nil {
				return
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "synthetic-upstream-id")
		_, _ = io.WriteString(w, `{"id":"synthetic-response","object":"response","model":"gpt-5.1","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"synthetic"}]}],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
	}))
	t.Cleanup(f.upstream.Close)
	a, err := client.Account.Create().SetName("counter-" + uuid.NewString()).SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).SetCredentials(map[string]any{"base_url": f.upstream.URL, "api_key": "disabled-fixture"}).SetExtra(map[string]any{"openai_responses_supported": true}).SetConcurrency(16).SetPriority(50).SetStatus(service.StatusActive).SetSchedulable(true).Save(ctx)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,50)`, a.ID, g.ID)
	require.NoError(t, err)
	accounts := repository.NewAccountRepository(client, db, nil)
	f.account, err = accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	f.billing = &counterBilling{UsageBillingRepository: repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))}
	f.gateway = service.NewOpenAIGatewayService(accounts, repository.NewUsageLogRepository(client, db), f.billing, nil, repository.NewUserSubscriptionRepository(client), nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, &service.BillingCacheService{}, counterUpstream{f.upstream.URL, f.upstream.Client()}, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.Use(requestledger.Middleware(requestledger.New(db)), middleware.GatewayClientRequestID())
	// Synthetic authentication is deliberately scoped to this test router.
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: u.ID})
		role := "user"
		if strings.HasPrefix(c.Request.URL.Path, "/api/v1/admin/") {
			role = "admin"
		}
		c.Set(string(middleware.ContextKeyUserRole), role)
		c.Next()
	})
	router.POST("/v1/responses", func(c *gin.Context) {
		ctx := c.Request.Context()
		if err := requestledger.BindIdentity(ctx, u.ID, k.ID); err != nil {
			c.Status(503)
			return
		}
		c.Set("api_key", f.key)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Status(400)
			return
		}
		result, forwardErr := f.gateway.Forward(ctx, c, f.account, body)
		f.mu.Lock()
		f.forwardErrors = append(f.forwardErrors, forwardErr)
		f.mu.Unlock()
		if result == nil {
			if !c.Writer.Written() {
				c.Status(502)
			}
			return
		}
		billingCtx := ctx
		if f.cancelBeforeBilling {
			var cancel context.CancelFunc
			billingCtx, cancel = context.WithCancel(ctx)
			cancel()
		}
		input := &service.OpenAIRecordUsageInput{Result: result, APIKey: f.key, User: f.user, Account: f.account, Subscription: f.sub, PricingAt: f.pricingAt, RequestPayloadHash: service.HashUsageRequestPayload(body)}
		f.mu.Lock()
		f.retainedCtx = context.WithoutCancel(billingCtx)
		f.retainedInput = input
		f.mu.Unlock()
		settle := func() {
			// RecordUsage normalizes its result; callback copies must not race on it.
			copyInput := *input
			copyResult := *result
			copyInput.Result = &copyResult
			err := f.gateway.RecordUsage(billingCtx, &copyInput)
			f.mu.Lock()
			f.settlementErrors = append(f.settlementErrors, err)
			f.mu.Unlock()
		}
		var wg sync.WaitGroup
		for range f.callbacks {
			if f.concurrentCallbacks {
				wg.Add(1)
				go func() { defer wg.Done(); settle() }()
			} else {
				settle()
			}
		}
		wg.Wait()
	})
	userHandler := handler.NewSubscriptionHandler(f.subscriptions)
	adminHandler := adminhandler.NewSubscriptionHandler(f.subscriptions)
	router.GET("/api/v1/subscriptions", userHandler.List)
	router.GET("/api/v1/admin/subscriptions", adminHandler.List)
	ledgerHandler := &handler.RequestLedgerHandler{Ledger: requestledger.New(db)}
	router.GET("/api/v1/requests/:id", ledgerHandler.GetUser)
	router.GET("/api/v1/admin/requests/:id", ledgerHandler.GetAdmin)
	router.GET("/api/v1/requests/:id/usage/:usage_id", ledgerHandler.UsageUser)
	router.GET("/api/v1/admin/requests/:id/usage/:usage_id", ledgerHandler.UsageAdmin)
	f.server = httptest.NewServer(router)
	t.Cleanup(f.server.Close)
	return f
}

func (f *counterFixture) readCounters(t *testing.T) counterSnapshot {
	t.Helper()
	var s counterSnapshot
	require.NoError(t, f.db.QueryRow(`SELECT daily_usage_usd,weekly_usage_usd,monthly_usage_usd FROM user_subscriptions WHERE id=$1`, f.sub.ID).Scan(&s.Daily, &s.Weekly, &s.Monthly))
	if f.sub.PackageEntitlement != nil {
		require.NoError(t, f.db.QueryRow(`SELECT request_used,amount_used_usd,token_used FROM subscription_package_entitlements WHERE id=$1`, f.sub.PackageEntitlement.ID).Scan(&s.Requests, &s.Amount, &s.Tokens))
	}
	return s
}
func (f *counterFixture) send(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	beforeCallbacks := len(f.settlementErrors)
	f.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", f.server.URL+"/v1/responses", strings.NewReader(fmt.Sprintf(`{"model":"gpt-5.1","input":"synthetic counter acceptance","stream":%t}`, f.partial || f.disconnect)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Request-ID", "same-client-header")
	req.Header.Set("X-Request-ID", "same-request-header")
	response, err := f.server.Client().Do(req)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	if f.disconnect {
		reader := bufio.NewReader(response.Body)
		for {
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			if strings.Contains(line, "cache_creation_tokens") {
				break
			}
		}
		cancel()
		require.Eventually(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.settlementErrors) == f.callbacks }, 10*time.Second, 10*time.Millisecond, "client disconnect must settle observed usage")
		return
	}
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode, string(body))
	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.settlementErrors) == beforeCallbacks+f.callbacks
	}, 10*time.Second, 10*time.Millisecond, "HTTP callback completion")
}
func (f *counterFixture) readJSON(t *testing.T, path string, target any) {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, 200, resp.StatusCode)
	require.NoError(t, json.NewDecoder(resp.Body).Decode(target))
}
func (f *counterFixture) assertDisplays(t *testing.T, want counterSnapshot) {
	t.Helper()
	type displayed struct {
		counterSnapshot
		ID      int64            `json:"id"`
		Package *counterSnapshot `json:"package_entitlement"`
	}
	var user struct {
		Data []displayed `json:"data"`
	}
	f.readJSON(t, "/api/v1/subscriptions", &user)
	require.Len(t, user.Data, 1)
	var admin struct {
		Data struct {
			Items []displayed `json:"items"`
		} `json:"data"`
	}
	f.readJSON(t, fmt.Sprintf("/api/v1/admin/subscriptions?user_id=%d", f.user.ID), &admin)
	require.Len(t, admin.Data.Items, 1)
	for _, row := range []displayed{user.Data[0], admin.Data.Items[0]} {
		require.Equal(t, f.sub.ID, row.ID)
		got := row.counterSnapshot
		if f.sub.PackageEntitlement != nil {
			require.NotNil(t, row.Package)
			got.Requests = row.Package.Requests
			got.Amount = row.Package.Amount
			got.Tokens = row.Package.Tokens
		} else {
			require.Nil(t, row.Package)
		}
		assertCounters(t, want, got)
	}
}

func TestSubscriptionCounterHTTP(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	var migrationCount int
	var lastMigration, pgVersion string
	require.NoError(t, db.QueryRow(`SELECT COUNT(*),MAX(filename) FROM schema_migrations`).Scan(&migrationCount, &lastMigration))
	require.NoError(t, db.QueryRow(`SHOW server_version`).Scan(&pgVersion))
	t.Logf("full_migrations count=%d last=%s postgres=%s", migrationCount, lastMigration, pgVersion)
	for _, packaged := range []bool{false, true} {
		kind := "subscription"
		if packaged {
			kind = "package"
		}
		for _, scenario := range []struct {
			name                                                             string
			requests, callbacks                                              int
			concurrent, cancel, zero, surcharge, frozen, partial, disconnect bool
		}{
			{name: "sequential_replay", requests: 1, callbacks: 2},
			{name: "concurrent_duplicate", requests: 1, callbacks: 8, concurrent: true},
			{name: "independent_private_requests", requests: 2, callbacks: 2},
			{name: "canceled_context_observed_usage", requests: 1, callbacks: 2, cancel: true},
			{name: "http_client_disconnect_observed_usage", requests: 1, callbacks: 2, disconnect: true},
			{name: "partial_output_observed_usage", requests: 1, callbacks: 2, partial: true},
			{name: "zero_multiplier", requests: 1, callbacks: 2, zero: true, surcharge: true},
			{name: "surcharge_frozen_price", requests: 1, callbacks: 2, surcharge: true, frozen: true},
		} {
			t.Run(kind+"/"+scenario.name, func(t *testing.T) {
				multiplier := 0.5
				if scenario.zero {
					multiplier = 0
				}
				f := newCounterFixture(t, db, client, packaged, multiplier, scenario.surcharge, scenario.frozen)
				f.callbacks = scenario.callbacks
				f.concurrentCallbacks = scenario.concurrent
				f.cancelBeforeBilling = scenario.cancel
				f.partial = scenario.partial
				f.disconnect = scenario.disconnect
				before := f.readCounters(t)
				f.assertDisplays(t, before)
				for range scenario.requests {
					f.send(t)
				}
				f.mu.Lock()
				forwardErrors := append([]error(nil), f.forwardErrors...)
				settlementErrors := append([]error(nil), f.settlementErrors...)
				f.mu.Unlock()
				require.Len(t, forwardErrors, scenario.requests)
				for _, err := range forwardErrors {
					if scenario.partial || scenario.disconnect {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
					}
				}
				require.Len(t, settlementErrors, scenario.requests*scenario.callbacks)
				for _, err := range settlementErrors {
					require.NoError(t, err)
				}
				require.EqualValues(t, scenario.requests, f.sends.Load())
				f.assertSettlement(t, before, scenario.requests, multiplier, scenario.surcharge, scenario.frozen)
			})
		}
	}
	t.Run("edges", func(t *testing.T) { runCounterEdges(t, db, client) })
}

func (f *counterFixture) assertSettlement(t *testing.T, before counterSnapshot, requests int, multiplier float64, surcharge, frozen bool) {
	t.Helper()
	// Expected price derives from a separate pricing call at multiplier 1; the
	// three customer prices deliberately differ in the surcharge case.
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	cost, err := service.NewBillingService(cfg, nil).CalculateCost("gpt-5.1", service.UsageTokens{InputTokens: 82, OutputTokens: 20, CacheCreationTokens: 7, CacheReadTokens: 11}, 1)
	require.NoError(t, err)
	if frozen {
		multiplier *= 3
	}
	actual := cost.TotalCost * multiplier
	billed := actual
	if surcharge && actual > 0 {
		billed += cost.TotalCost * 0.3
	}
	// SubscriptionCost is rounded to the documented NUMERIC(20,8) scale;
	// package amount uses BilledCost at its NUMERIC(20,10) scale.
	subscriptionCharge := math.Round(billed*1e8) / 1e8
	applied := 0
	for _, call := range f.billing.snapshot() {
		if call.err != nil {
			continue
		}
		require.NotNil(t, call.result)
		assert.InDelta(t, subscriptionCharge, call.command.SubscriptionCost, 1e-10)
		assert.InDelta(t, billed, call.command.BilledCost, 1e-10)
		assert.InDelta(t, actual, call.command.ActualCost, 1e-10)
		assert.Zero(t, call.command.BalanceCost)
		require.Equal(t, f.sub.ID, *call.command.SubscriptionID)
		require.Equal(t, 82, call.command.InputTokens)
		require.Equal(t, 20, call.command.OutputTokens)
		require.Equal(t, 7, call.command.CacheCreationTokens)
		require.Equal(t, 11, call.command.CacheReadTokens)
		if f.sub.PackageEntitlement != nil {
			require.Equal(t, f.sub.PackageEntitlement.ID, *call.command.PackageEntitlementID)
		} else {
			require.Nil(t, call.command.PackageEntitlementID)
		}
		if call.result.Applied {
			applied++
		}
	}
	require.Equal(t, requests, applied)
	delta := counterSnapshot{Daily: subscriptionCharge * float64(requests), Weekly: subscriptionCharge * float64(requests), Monthly: subscriptionCharge * float64(requests)}
	if f.sub.PackageEntitlement != nil {
		delta.Requests = int64(requests)
		delta.Amount = billed * float64(requests)
		delta.Tokens = int64(requests * 120)
	}
	after := f.readCounters(t)
	var daily, weekly, monthly time.Time
	require.NoError(t, f.db.QueryRow(`SELECT daily_window_start,weekly_window_start,monthly_window_start FROM user_subscriptions WHERE id=$1`, f.sub.ID).Scan(&daily, &weekly, &monthly))
	for i, pair := range [][2]time.Time{{*f.sub.DailyWindowStart, daily}, {*f.sub.WeeklyWindowStart, weekly}, {*f.sub.MonthlyWindowStart, monthly}} {
		require.True(t, pair[0].Equal(pair[1]), "unexpected window reset on axis %d", i)
	}
	assertCounters(t, delta, after.minus(before))
	f.assertDisplays(t, after)
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=$1`, f.user.ID).Scan(&balance))
	require.Equal(t, 100.0, balance)
	for _, table := range []string{"usage_billing_dedup", "usage_logs", "gateway_request_billing_links"} {
		var n int
		require.Eventually(t, func() bool {
			err := f.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE api_key_id=$1`, f.key.ID).Scan(&n)
			return err == nil && n == requests
		}, 5*time.Second, 10*time.Millisecond, table)
	}
	rows, err := f.db.Query(`SELECT r.id,b.billing_request_id,b.subscription_id,b.package_entitlement_id,b.settlement_verified,b.settlement_applied,u.id,u.input_tokens,u.output_tokens,u.cache_creation_tokens,u.cache_read_tokens,u.total_cost,u.actual_cost,u.billed_cost,u.billing_settled,r.usage_state,r.settlement_state FROM gateway_requests r JOIN gateway_request_billing_links b ON b.request_id=r.id JOIN usage_billing_dedup d ON d.request_id=b.billing_request_id AND d.api_key_id=b.api_key_id AND d.request_fingerprint=b.request_fingerprint JOIN usage_logs u ON u.request_id=b.billing_request_id AND u.api_key_id=b.api_key_id AND u.billing_request_fingerprint=b.request_fingerprint WHERE r.user_id=$1`, f.user.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	seen := map[string]bool{}
	n := 0
	for rows.Next() {
		var id, billingID, usageState, settlementState string
		var sid, usageID int64
		var eid sql.NullInt64
		var verified, wasApplied, settled bool
		var in, out, creation, read int
		var total, gotActual, gotBilled float64
		require.NoError(t, rows.Scan(&id, &billingID, &sid, &eid, &verified, &wasApplied, &usageID, &in, &out, &creation, &read, &total, &gotActual, &gotBilled, &settled, &usageState, &settlementState))
		require.False(t, seen[id])
		seen[id] = true
		require.True(t, strings.HasPrefix(billingID, "gateway:"))
		require.Equal(t, f.sub.ID, sid)
		require.True(t, verified)
		require.True(t, wasApplied)
		require.True(t, settled)
		require.Equal(t, "known", usageState)
		require.Equal(t, "settled", settlementState)
		require.Equal(t, []int{82, 20, 7, 11}, []int{in, out, creation, read})
		assert.InDelta(t, cost.TotalCost, total, 1e-9)
		assert.InDelta(t, actual, gotActual, 1e-9)
		assert.InDelta(t, billed, gotBilled, 1e-9)
		if f.sub.PackageEntitlement != nil {
			require.True(t, eid.Valid)
			require.Equal(t, f.sub.PackageEntitlement.ID, eid.Int64)
		} else {
			require.False(t, eid.Valid)
		}
		for _, prefix := range []string{"/api/v1/requests/", "/api/v1/admin/requests/"} {
			var payload struct {
				Data struct {
					Billed *float64 `json:"billed_cost"`
					Input  int      `json:"input_tokens"`
				} `json:"data"`
			}
			f.readJSON(t, fmt.Sprintf("%s%s/usage/%d", prefix, id, usageID), &payload)
			require.NotNil(t, payload.Data.Billed)
			assert.InDelta(t, billed, *payload.Data.Billed, 1e-9)
			assert.Equal(t, 82, payload.Data.Input)
		}
		n++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, requests, n)
	t.Logf("counter_evidence kind_package=%t requests=%d callbacks=%d before=%+v after=%+v delta=%+v total=%.10f actual=%.10f billed=%.10f dedup=usage=links=%d", f.sub.PackageEntitlement != nil, requests, len(f.billing.snapshot()), before, after, after.minus(before), cost.TotalCost, actual, billed, n)
}
