//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"io"
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
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type admissionFixture struct {
	*counterFixture
	server         *httptest.Server
	keyValue       string
	stream         bool
	requestBody    string
	requestPath    string
	channels       *service.ChannelService
	workers        *service.UsageRecordWorkerPool
	billingContext *admissionBillingContext
	usageLogs      *admissionUsageLogs
}

type admissionUsageLogs struct {
	service.UsageLogRepository
	legacyWrites       atomic.Int64
	rewriteObservation func(requestledger.SubscriptionObservation) requestledger.SubscriptionObservation
}

func (r *admissionUsageLogs) CreateBestEffort(ctx context.Context, log *service.UsageLog) error {
	r.legacyWrites.Add(1)
	writer, ok := r.UsageLogRepository.(interface {
		CreateBestEffort(context.Context, *service.UsageLog) error
	})
	if !ok {
		return fmt.Errorf("test fixture does not implement CreateBestEffort")
	}
	return writer.CreateBestEffort(ctx, log)
}

func (r *admissionUsageLogs) PersistSubscriptionUsage(ctx context.Context, in requestledger.SubscriptionObservation) error {
	if r.rewriteObservation != nil {
		in = r.rewriteObservation(in)
	}
	writer, ok := r.UsageLogRepository.(interface {
		PersistSubscriptionUsage(context.Context, requestledger.SubscriptionObservation) error
	})
	if !ok {
		return fmt.Errorf("test fixture does not implement PersistSubscriptionUsage")
	}
	return writer.PersistSubscriptionUsage(ctx, in)
}

type admissionBillingContext struct {
	service.UsageBillingRepository
	mu  sync.Mutex
	ctx context.Context
}

func (r *admissionBillingContext) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.mu.Lock()
	r.ctx = context.WithoutCancel(ctx)
	r.mu.Unlock()
	return r.UsageBillingRepository.Apply(ctx, cmd)
}

func (r *admissionBillingContext) retained() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ctx
}

type admissionResponseGate struct {
	entered     chan struct{}
	release     chan struct{}
	enterOnce   sync.Once
	releaseOnce sync.Once
}

func newAdmissionResponseGate(t *testing.T) *admissionResponseGate {
	t.Helper()
	g := &admissionResponseGate{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(g.open)
	return g
}

func (g *admissionResponseGate) open() { g.releaseOnce.Do(func() { close(g.release) }) }

type gatedAdmissionUpstream struct {
	counterUpstream
	gate   *admissionResponseGate
	before *admissionResponseGate
}

func (u gatedAdmissionUpstream) Do(r *http.Request, proxy string, account int64, concurrency int) (*http.Response, error) {
	if u.before != nil {
		u.before.enterOnce.Do(func() { close(u.before.entered) })
		select {
		case <-u.before.release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	response, err := u.counterUpstream.Do(r, proxy, account, concurrency)
	if err != nil || u.gate == nil {
		return response, err
	}
	u.gate.enterOnce.Do(func() { close(u.gate.entered) })
	select {
	case <-u.gate.release:
		return response, nil
	case <-r.Context().Done():
		_ = response.Body.Close()
		return nil, r.Context().Err()
	}
}

func (u gatedAdmissionUpstream) DoWithTLS(r *http.Request, proxy string, account int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, proxy, account, concurrency)
}

// Unlike counterFixture.send, this fixture executes production API-key auth,
// subscription admission, registered Responses handler, DB account selection,
// and the real bounded usage worker. Concurrency limits are disabled in the
// synthetic rows; Redis behavior is not part of this PostgreSQL lock experiment.
func newAdmissionFixture(t *testing.T, db *sql.DB, client *dbent.Client, packaged bool, gates ...*admissionResponseGate) *admissionFixture {
	t.Helper()
	f := newCounterFixture(t, db, client, packaged, 0.5, false, false)
	_, err := db.Exec(`UPDATE users SET concurrency=0 WHERE id=$1`, f.user.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE accounts SET concurrency=0 WHERE id=$1`, f.account.ID)
	require.NoError(t, err)
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.MaxBodySize = 1 << 20
	cfg.Gateway.TextMaxBodySize = 1 << 20
	users := repository.NewUserRepository(client, db)
	keys := repository.NewAPIKeyRepository(client, db)
	groups := repository.NewGroupRepository(client, db)
	subs := repository.NewUserSubscriptionRepository(client)
	rates := repository.NewUserGroupRateRepository(db)
	keyService := service.NewAPIKeyService(keys, users, groups, subs, rates, nil, cfg)
	billingCache := service.NewBillingCacheService(nil, users, subs, keys, nil, rates, cfg, nil)
	t.Cleanup(billingCache.Stop)
	concurrency := service.NewConcurrencyService(nil)
	workers := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 16, TaskTimeout: 30 * time.Second, OverflowPolicy: config.UsageRecordOverflowPolicySync})
	t.Cleanup(workers.Stop)
	billingContext := &admissionBillingContext{UsageBillingRepository: f.billing}
	usageLogs := &admissionUsageLogs{UsageLogRepository: repository.NewUsageLogRepository(client, db)}
	upstream := gatedAdmissionUpstream{counterUpstream: counterUpstream{f.upstream.URL, f.upstream.Client()}}
	if len(gates) > 0 {
		upstream.gate = gates[0]
	}
	if len(gates) > 1 {
		upstream.before = gates[1]
	}
	channels := service.NewChannelService(repository.NewChannelRepository(db), groups, nil, nil, nil)
	billing := service.NewBillingService(cfg, nil)
	f.gateway = service.NewOpenAIGatewayService(repository.NewAccountRepository(client, db, nil), usageLogs, billingContext, users, subs, rates, nil, cfg, nil, concurrency, billing, nil, billingCache, upstream, &service.DeferredService{}, nil, nil, service.NewModelPricingResolver(channels, billing), channels, nil, nil, nil)
	h := handler.NewOpenAIGatewayHandler(f.gateway, concurrency, billingCache, keyService, workers, nil, nil, nil, cfg)
	router := gin.New()
	router.Use(requestledger.Middleware(requestledger.New(db)))
	routes.RegisterGatewayRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: h}, middleware.NewAPIKeyAuthMiddleware(keyService, f.subscriptions, cfg), keyService, f.subscriptions, nil, nil, nil, cfg)
	a := &admissionFixture{counterFixture: f, server: httptest.NewServer(router), workers: workers, billingContext: billingContext, channels: channels, usageLogs: usageLogs}
	t.Cleanup(a.server.Close)
	if upstream.gate != nil {
		t.Cleanup(upstream.gate.open)
	}
	if upstream.before != nil {
		t.Cleanup(upstream.before.open)
	}
	require.NoError(t, db.QueryRow(`SELECT key FROM api_keys WHERE id=$1`, f.key.ID).Scan(&a.keyValue))
	return a
}

func (f *admissionFixture) request() (int, string, error) {
	return f.requestWithContext(context.Background())
}

func (f *admissionFixture) requestWithContext(parent context.Context) (int, string, error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	bodyJSON := f.requestBody
	if bodyJSON == "" {
		bodyJSON = fmt.Sprintf(`{"model":"gpt-5.1","input":"synthetic admission test","stream":%t}`, f.stream)
	}
	path := f.requestPath
	if path == "" {
		path = "/v1/responses"
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, f.server.URL+path, strings.NewReader(bodyJSON))
	if err != nil {
		return 0, "", err
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Client-Request-ID", "same-client-header")
	r.Header.Set("X-Request-ID", "same-request-header")
	r.Header.Set("Authorization", "Bearer "+f.keyValue)
	response, err := f.server.Client().Do(r)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	return response.StatusCode, string(body), err
}

func (f *admissionFixture) awaitSettlements(t *testing.T, count int) []counterApply {
	t.Helper()
	require.Eventually(t, func() bool {
		return len(f.billing.snapshot()) == count
	}, 10*time.Second, 10*time.Millisecond, "real worker must finish settlement")
	return f.billing.snapshot()
}

type admissionHTTPResult struct {
	status int
	body   string
	err    error
}

func startAdmissionRequest(f *admissionFixture) <-chan admissionHTTPResult {
	ch := make(chan admissionHTTPResult, 1)
	go func() { status, body, err := f.request(); ch <- admissionHTTPResult{status, body, err} }()
	return ch
}

func receiveAdmissionResponse(t *testing.T, ch <-chan admissionHTTPResult) admissionHTTPResult {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(10 * time.Second):
		t.Fatal("admitted HTTP response did not complete")
		return admissionHTTPResult{}
	}
}

func awaitResponseGate(t *testing.T, g *admissionResponseGate) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("request did not pass real admission and reach synthetic upstream")
	}
}

func assertAdmissionUsageLink(t *testing.T, f *admissionFixture, settled bool) {
	t.Helper()
	var total, matching int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*),COUNT(*) FILTER(WHERE billing_settled=$2) FROM usage_logs WHERE api_key_id=$1`, f.key.ID, settled).Scan(&total, &matching))
	require.Equal(t, 1, total)
	require.Equal(t, 1, matching)
	var sub int64
	var entitlement sql.NullInt64
	var verified bool
	require.NoError(t, f.db.QueryRow(`SELECT subscription_id,package_entitlement_id,settlement_verified FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&sub, &entitlement, &verified))
	require.Equal(t, f.sub.ID, sub)
	require.Equal(t, settled, verified)
	if f.sub.PackageEntitlement != nil {
		require.True(t, entitlement.Valid)
		require.Equal(t, f.sub.PackageEntitlement.ID, entitlement.Int64)
	} else {
		require.False(t, entitlement.Valid)
	}
}

// This is the approved new contract, not a characterization of the old refusal.
// It deliberately uses the production API-key auth, registered native Responses
// handler, actual ledger transport, and bounded usage worker against full PG.
func TestSubscriptionInflightRevoke(t *testing.T) {
	db, client := openIsolatedBillingDB(t)
	gin.SetMode(gin.TestMode)
	var migrations int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&migrations))
	t.Logf("full_migrations count=%d", migrations)
	t.Run("routing_279_before_receipt_280", func(t *testing.T) { runInflightRoutingMigrationCoexistence(t, db, client) })
	for _, packaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("active_positive_control_package_%t", packaged), func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, packaged)
			before := f.readCounters(t)
			status, body, err := f.request()
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, status, body)
			calls := f.awaitSettlements(t, 1)
			require.NoError(t, calls[0].err)
			require.True(t, calls[0].result.Applied)
			assertCounters(t, inflightCharge(t, f, calls[0].command), f.readCounters(t).minus(before))
			assertAdmissionUsageLink(t, f, true)
			assertInflightDetail(t, f, calls[0].command, true)
		})
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("admitted_before_revoke_package_%t_stream_%t", packaged, stream), func(t *testing.T) {
				gate := newAdmissionResponseGate(t)
				f := newAdmissionFixture(t, db, client, packaged, gate)
				configureInflightStream(f, stream)
				before := f.readCounters(t)
				wallet := readInflightWallet(t, f)
				response := startAdmissionRequest(f)
				awaitResponseGate(t, gate)
				require.EqualValues(t, 1, f.sends.Load(), "upstream has actually received the admitted request")
				require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
				gate.open()
				r := receiveAdmissionResponse(t, response)
				require.NoError(t, r.err)
				require.Equal(t, http.StatusOK, r.status, r.body)
				calls := f.awaitSettlements(t, 1)
				cmd := calls[0].command
				t.Logf("admitted_before_revoke package=%t stream=%t actual_sends=%d apply_error=%v delta=%+v", packaged, stream, f.sends.Load(), calls[0].err, f.readCounters(t).minus(before))
				assert.NoError(t, calls[0].err, "previously admitted observed usage must settle after revoke")
				if assert.NotNil(t, calls[0].result) {
					assert.True(t, calls[0].result.Applied)
					assert.True(t, calls[0].result.SettlementVerified)
				}
				wantDelta := inflightCharge(t, f, cmd)
				after := f.readCounters(t)
				assertCounters(t, wantDelta, after.minus(before))
				assertCounterProofCounts(t, f.counterFixture, 1, 1)
				assertAdmissionUsageLink(t, f, true)
				assertInflightClosed(t, f)
				assert.Equal(t, wallet, readInflightWallet(t, f), "no balance fallback")
				assertInflightDetail(t, f, cmd, true)
				// Replay the identical persisted input, without a new HTTP send.
				result, err := f.billing.UsageBillingRepository.Apply(f.billingContext.retained(), &cmd)
				assert.NoError(t, err)
				if assert.NotNil(t, result) {
					assert.False(t, result.Applied)
					assert.True(t, result.SettlementVerified)
				}
				assertCounters(t, after, f.readCounters(t))
				status, body, err := f.request()
				require.NoError(t, err)
				assert.Equal(t, http.StatusForbidden, status, body)
				assert.EqualValues(t, 1, f.sends.Load(), "same client headers cannot reuse prior admission")
			})
		}
		t.Run(fmt.Sprintf("revoke_between_auth_and_actual_send_package_%t", packaged), func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, packaged, nil, gate)
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			require.Zero(t, f.sends.Load(), "barrier is before the real RoundTrip gate")
			require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
			gate.open()
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			assert.GreaterOrEqual(t, r.status, 400, r.body)
			assert.Zero(t, f.sends.Load(), "cached auth is insufficient; actual sending must revalidate")
			if f.sends.Load() > 0 {
				f.awaitSettlements(t, 1) // Drain baseline's incorrectly sent request before fixture cleanup.
			}
			assertCounters(t, before, f.readCounters(t))
			assertCounterProofCounts(t, f.counterFixture, 0, 0)
			assertInflightClosed(t, f)
			t.Logf("revoke_before_send package=%t status=%d actual_sends=%d", packaged, r.status, f.sends.Load())
		})
		t.Run(fmt.Sprintf("frozen_user_rate_package_%t", packaged), func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, packaged, gate)
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			// The original group multiplier is 0.5. A new personal override after
			// actual sending must not change this request's frozen price.
			_, err := db.Exec(`INSERT INTO user_group_rate_multipliers(user_id,group_id,rate_multiplier) VALUES($1,$2,7)`, f.user.ID, *f.key.GroupID)
			require.NoError(t, err)
			gate.open()
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, http.StatusOK, r.status, r.body)
			calls := f.awaitSettlements(t, 1)
			require.NoError(t, calls[0].err)
			cmd := calls[0].command
			want := frozenInflightCost(t, f)
			assert.InDelta(t, service.QuantizeUsageBillingAmount(want), cmd.SubscriptionCost, 1e-10, "personal-rate edits after send cannot alter approved price")
			assert.InDelta(t, want, cmd.BilledCost, 1e-10)
			charge := service.QuantizeUsageBillingAmount(want)
			delta := counterSnapshot{Daily: charge, Weekly: charge, Monthly: charge}
			if packaged {
				delta.Requests, delta.Amount, delta.Tokens = 1, want, 120
			}
			assertCounters(t, delta, f.readCounters(t).minus(before))
			assertCounterProofCounts(t, f.counterFixture, 1, 1)
			t.Logf("frozen_price package=%t approved=%.10f observed_command=%.10f", packaged, want, cmd.BilledCost)
		})
		t.Run(fmt.Sprintf("natural_expiry_is_not_revocation_exception_package_%t", packaged), func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, packaged, gate)
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			_, err := db.Exec(`UPDATE user_subscriptions SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, f.sub.ID)
			require.NoError(t, err)
			if packaged {
				_, err = db.Exec(`UPDATE subscription_package_entitlements SET expires_at=NOW()-INTERVAL '1 minute' WHERE id=$1`, f.sub.PackageEntitlement.ID)
				require.NoError(t, err)
			}
			require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
			gate.open()
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, http.StatusOK, r.status, r.body)
			calls := f.awaitSettlements(t, 1)
			assert.Error(t, calls[0].err, "this patch must not grant expired consumption")
			assertCounters(t, before, f.readCounters(t))
			assertCounterProofCounts(t, f.counterFixture, 0, 0)
			assertAdmissionUsageLink(t, f, false)
		})
	}
	runInflightBoundaries(t, db, client)
	runInflightPricing(t, db, client)
	t.Run("NativeScope", func(t *testing.T) { runInflightNativeScope(t, db, client) })
	t.Run("ExpiryWait", func(t *testing.T) { runInflightExpiryWait(t, db, client) })
	t.Run("LegacyScope", func(t *testing.T) { runInflightLegacyScope(t, db, client) })
	t.Run("UnknownOutput", func(t *testing.T) { runInflightUnknownOutput(t, db, client) })
	t.Run("ModelCase", func(t *testing.T) { runInflightModelCase(t, db, client) })
	t.Run("ProjectionOwnership", func(t *testing.T) { runSubscriptionProjectionOwnership(t, db, client) })
	t.Run("projection_single_owner", func(t *testing.T) { runSubscriptionProjectionOwner(t, db, client) })
	t.Run("pending_projection_failure_atomic_rollback", func(t *testing.T) { runSubscriptionProjectionFailure(t, db, client) })
}

func configureInflightStream(f *admissionFixture, stream bool) {
	f.stream = stream
	if !stream {
		return
	}
	// Set before any request: the synthetic server preserves the real parser and
	// transport. A terminal Responses usage frame supplies all four token classes.
	f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.sends.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"synthetic"}`+"\n\n")
		_, _ = io.WriteString(w, "data: "+`{"type":"response.completed","response":{"id":"synthetic-stream","model":"gpt-5.1","status":"completed","usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}}`+"\n\n")
		_ = http.NewResponseController(w).Flush()
	})
}

func frozenInflightCost(t *testing.T, f *admissionFixture) float64 {
	t.Helper()
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	cost, err := service.NewBillingService(cfg, nil).CalculateCost("gpt-5.1", service.UsageTokens{InputTokens: 82, OutputTokens: 20, CacheCreationTokens: 7, CacheReadTokens: 11}, 0.5)
	require.NoError(t, err)
	return cost.ActualCost
}

func inflightCharge(t *testing.T, f *admissionFixture, cmd service.UsageBillingCommand) counterSnapshot {
	t.Helper()
	want := frozenInflightCost(t, f)
	charge := service.QuantizeUsageBillingAmount(want)
	assert.InDelta(t, charge, cmd.SubscriptionCost, 1e-10)
	assert.InDelta(t, want, cmd.BilledCost, 1e-10)
	assert.Zero(t, cmd.BalanceCost)
	assert.Equal(t, 120, cmd.InputTokens+cmd.OutputTokens+cmd.CacheCreationTokens+cmd.CacheReadTokens)
	delta := counterSnapshot{Daily: charge, Weekly: charge, Monthly: charge}
	if f.sub.PackageEntitlement != nil {
		delta.Requests, delta.Amount, delta.Tokens = 1, want, 120
	}
	return delta
}

type inflightWallet struct {
	Balance float64
	Rows    int
}

func readInflightWallet(t *testing.T, f *admissionFixture) inflightWallet {
	t.Helper()
	var w inflightWallet
	require.NoError(t, f.db.QueryRow(`SELECT balance,(SELECT COUNT(*) FROM balance_transactions WHERE user_id=$1) FROM users WHERE id=$1`, f.user.ID).Scan(&w.Balance, &w.Rows))
	return w
}

func assertInflightClosed(t *testing.T, f *admissionFixture) {
	t.Helper()
	var deleted bool
	require.NoError(t, f.db.QueryRow(`SELECT deleted_at IS NOT NULL FROM user_subscriptions WHERE id=$1`, f.sub.ID).Scan(&deleted))
	assert.True(t, deleted, "late settlement must not restore the subscription")
	if f.sub.PackageEntitlement != nil {
		var status string
		require.NoError(t, f.db.QueryRow(`SELECT status FROM subscription_package_entitlements WHERE id=$1`, f.sub.PackageEntitlement.ID).Scan(&status))
		assert.Equal(t, "revoked", status)
	}
}

func assertInflightDetail(t *testing.T, f *admissionFixture, cmd service.UsageBillingCommand, settled bool) {
	t.Helper()
	var id string
	require.NoError(t, f.db.QueryRow(`SELECT request_id FROM gateway_request_billing_links WHERE api_key_id=$1`, f.key.ID).Scan(&id))
	for _, path := range []string{"/api/v1/requests/" + id, "/api/v1/admin/requests/" + id} {
		var envelope struct {
			Data struct {
				Request requestledger.Record             `json:"request"`
				Billing []requestledger.BillingReference `json:"billing"`
			} `json:"data"`
		}
		f.readJSON(t, path, &envelope)
		assert.Equal(t, "known", envelope.Data.Request.UsageState)
		wantState := "settlement_pending"
		if settled {
			wantState = "settled"
		}
		assert.Equal(t, wantState, envelope.Data.Request.SettlementState)
		require.Len(t, envelope.Data.Billing, 1)
		proof := envelope.Data.Billing[0]
		assert.Equal(t, settled, proof.Settled)
		assert.Equal(t, cmd.SubscriptionID, proof.SubscriptionID)
		assert.Equal(t, cmd.PackageEntitlementID, proof.PackageEntitlementID)
		if settled && assert.NotNil(t, proof.BilledCost) {
			assert.InDelta(t, cmd.BilledCost, *proof.BilledCost, 1e-10)
		}
	}
}
