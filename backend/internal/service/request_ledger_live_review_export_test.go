//go:build integration

package service

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RequestLedgerLiveReviewFixture exposes private orchestration only to this
// package's external integration test, so it can inject the real PG/Redis repos
// without a production import cycle or a production test hook.
func RequestLedgerLiveReviewFixture(t *testing.T, db *sql.DB, cache GatewayCache, logs UsageLogRepository, billing UsageBillingRepository, outbox LiveSettlementOutboxRepository) {
	t.Helper()
	l := requestledger.New(db)
	ctx, h, _ := ledgerFollowupContext(t, l, "/v1/live", "async_execution")
	callHash := hashLiveCallID("synthetic-live-ledger-fault")
	groupID := int64(44)
	record := &LiveCallRecord{CallID: "synthetic-live-ledger-fault", CallHash: callHash, AccountID: 234, APIKeyID: 201, UserID: 101, GroupID: groupID, LeaseID: "synthetic-live-lease", Model: "synthetic-live-model", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Minute), Controller: LiveControllerObserver, InboundEndpoint: "/v1/live"}
	require.NoError(t, requestledger.BindTask(ctx, "live_call", callHash, 101, 201))
	store, ok := cache.(LiveCallStore)
	require.True(t, ok)
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	_, err := db.Exec(`ALTER TABLE gateway_request_attempts ADD CONSTRAINT synthetic_live_usage_fault CHECK (usage_state<>'known') NOT VALID`)
	require.NoError(t, err)
	defer func() {
		_, _ = db.Exec(`ALTER TABLE gateway_request_attempts DROP CONSTRAINT IF EXISTS synthetic_live_usage_fault`)
	}()
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, e := coderws.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.CloseNow()
		connections.Add(1)
		for _, p := range []string{
			`{"type":"response.created","response":{"id":"synthetic-live-response"}}`,
			`{"type":"response.done","response":{"id":"synthetic-live-response","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}}`,
			`{"type":"response.done","response":{"id":"synthetic-live-response","status":"completed","usage":{"input_tokens":7,"output_tokens":3}}}`,
			`{"type":"session.ended"}`,
		} {
			if conn.Write(r.Context(), coderws.MessageText, []byte(p)) != nil {
				return
			}
		}
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	connectionAttempt, err := requestledger.BeginConnectionAttempt(ctx, 234, 22)
	require.NoError(t, err)
	conn, _, err := coderws.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	require.NoError(t, connectionAttempt.Finish(ctx, http.StatusSwitchingProtocols, nil))
	raw := &coderOpenAIWSClientConn{conn: conn}
	wrapped := &ledgerLiveFrameConn{inner: raw, audit: newRealtimeLedgerAudit(ctx, &Account{ID: 234}), execution: h, ctx: ctx}
	defer func() { _ = wrapped.Close() }()
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, &openAIRecordUsageUserRepoStub{user: &User{ID: 101}}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.cfg.Default.RateMultiplier = 1
	svc.billingService = NewBillingService(svc.cfg, &PricingService{pricingData: map[string]*LiteLLMModelPricing{"synthetic-live-model": {InputCostPerToken: .01, OutputCostPerToken: .02}}})
	svc.cache = cache
	svc.requestLedger = l
	svc.liveSettlementOutbox = &ledgerReviewLiveOutbox{LiveSettlementOutboxRepository: outbox, t: t, db: db}
	svc.concurrencyService = NewConcurrencyService(&liveTestConcurrencyCache{})
	svc.accountRepo = &liveTestAccountRepo{account: &Account{ID: 234, Platform: PlatformOpenAI, Type: AccountTypeOAuth}}
	svc.liveAPIKeyLoader = &liveTestAPIKeyLoader{key: &APIKey{ID: 201, UserID: 101, GroupID: &groupID, Group: &Group{ID: groupID, RateMultiplier: 1}}}
	svc.liveAPIKeyQuotaUpdater = &openAIRecordUsageAPIKeyQuotaStub{}
	assert.ErrorIs(t, svc.runLiveObserverConnection(record, wrapped), ErrLiveCallNotFound, "audit failure cannot become a transport read error")
	latest, err := store.GetLiveCall(context.Background(), callHash)
	require.NoError(t, err)
	assert.Equal(t, 7, latest.Usage.InputTokens)
	assert.Equal(t, 3, latest.Usage.OutputTokens)
	require.NoError(t, wrapped.Close())
	_, err = db.Exec(`ALTER TABLE gateway_request_attempts DROP CONSTRAINT synthetic_live_usage_fault`)
	require.NoError(t, err)
	require.NoError(t, l.Recover(context.Background()))
	var unknown int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE parent_id=$1 AND usage_state='usage_unknown' AND execution_state<>'inflight'`, h.ID).Scan(&unknown))
	assert.Positive(t, unknown)
	svc.finalizeLiveCall(record)
	svc.finalizeLiveCall(record)
	var remaining int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM live_usage_settlement_outbox WHERE call_hash=$1`, callHash).Scan(&remaining))
	assert.Zero(t, remaining, "existing outbox acknowledges successful settlement")
	var balance, cost float64
	var logInput, logOutput, count int
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=101`).Scan(&balance))
	assert.InDelta(t, 99.87, balance, .000001)
	require.NoError(t, db.QueryRow(`SELECT input_tokens,output_tokens,billed_cost FROM usage_logs WHERE request_id=$1 AND api_key_id=201`, callHash).Scan(&logInput, &logOutput, &cost))
	assert.Equal(t, 7, logInput)
	assert.Equal(t, 3, logOutput)
	assert.InDelta(t, .13, cost, .000001)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM balance_transactions WHERE user_id=101 AND source_type='usage_charge'`).Scan(&count))
	assert.Equal(t, 1, count)
	assert.EqualValues(t, 1, connections.Load(), "no reconnect or repeated upstream consumption")
}

// The delegate is the real PostgreSQL outbox; inspect its durable snapshot at
// the enqueue boundary before its existing Ack removes completed work.
type ledgerReviewLiveOutbox struct {
	LiveSettlementOutboxRepository
	t  *testing.T
	db *sql.DB
}

func (o *ledgerReviewLiveOutbox) Enqueue(ctx context.Context, r *LiveCallRecord) error {
	if err := o.LiveSettlementOutboxRepository.Enqueue(ctx, r); err != nil {
		return err
	}
	var input, output int
	if err := o.db.QueryRow(`SELECT input_tokens,output_tokens FROM live_usage_settlement_outbox WHERE call_hash=$1`, r.CallHash).Scan(&input, &output); err != nil {
		return err
	}
	assert.Equal(o.t, 7, input, "PG outbox preserves the received usage snapshot")
	assert.Equal(o.t, 3, output)
	return nil
}
