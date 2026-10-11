//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runSubscriptionProjectionOwner(t *testing.T, db *sql.DB, client *dbent.Client) {
	f := newAdmissionFixture(t, db, client, true)
	before := f.readCounters(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	open := func() { once.Do(func() { close(release) }) }
	t.Cleanup(open)
	f.billing.UsageBillingRepository = blockedInflightBilling{UsageBillingRepository: f.billing.UsageBillingRepository, entered: entered, release: release}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	response := make(chan admissionHTTPResult, 1)
	go func() {
		status, body, err := f.requestWithContext(ctx)
		response <- admissionHTTPResult{status, body, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("sealed observation did not reach Apply")
	}
	var pendingLogs, sealed int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1 AND command_payload IS NOT NULL AND completed_at IS NULL`, f.key.ID).Scan(&sealed))
	require.Equal(t, 1, sealed)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1 AND NOT billing_settled AND actual_cost=0 AND billed_cost=0 AND input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens=120`, f.key.ID).Scan(&pendingLogs))
	assert.Equal(t, 1, pendingLogs, "the original pending projection must commit with the claimable observation, before Apply or cancellation")
	assertCounters(t, before, f.readCounters(t))
	cancel()
	require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
	repo := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
	var worker *service.OpenAIGatewayService
	startWorker := func() {
		worker = service.NewOpenAIGatewayServiceWithLiveBilling(nil, nil, repo, nil, nil, nil, nil, &config.Config{}, nil, nil, service.NewBillingService(&config.Config{}, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, requestledger.New(db))
		t.Cleanup(worker.StopLiveSettlementWorker)
	}
	if pendingLogs == 0 {
		// Diagnostic branch for the old dual writer: expire its abandoned lease,
		// stop real recovery after it holds the entitlement, then let the original
		// caller enqueue its legacy INSERT. No production lock is replaced by a mock.
		gatePID, releaseApply := blockAdmissionApply(t, f)
		_, err := db.Exec(`UPDATE gateway_subscription_outbox SET lease_until=clock_timestamp()-interval '1 second',next_attempt_at=clock_timestamp() WHERE api_key_id=$1`, f.key.ID)
		require.NoError(t, err)
		startWorker()
		recoveryPID := awaitPGBlockedBy(t, db, gatePID)
		open()
		writerPID := awaitPGBlockedBy(t, db, recoveryPID)
		var writerQuery string
		require.NoError(t, db.QueryRow(`SELECT query FROM pg_stat_activity WHERE pid=$1`, writerPID).Scan(&writerQuery))
		require.Contains(t, writerQuery, "WITH input")
		var locks string
		require.NoError(t, db.QueryRow(`SELECT json_agg(row_to_json(x))::text FROM (SELECT pid,locktype,mode,granted,relation::regclass::text AS relation,page,tuple,transactionid::text FROM pg_locks WHERE pid IN ($1,$2) AND (NOT granted OR relation IN ('usage_logs'::regclass,'user_subscriptions'::regclass,'subscription_package_entitlements'::regclass)) ORDER BY pid,locktype,mode) x`, recoveryPID, writerPID).Scan(&locks))
		var correlation string
		require.NoError(t, db.QueryRow(`SELECT json_build_object('request',request_id,'key',api_key_id,'billing_request',billing_request_id,'receipt',receipt_id)::text FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&correlation))
		t.Logf("controlled_wait_graph gate_pid=%d recovery_pid=%d legacy_writer_pid=%d locks=%s correlation=%s", gatePID, recoveryPID, writerPID, locks, correlation)
		releaseApply()
	} else {
		open()
		_ = receiveAdmissionResponse(t, response)
		f.awaitSettlements(t, 1)
		require.Eventually(t, func() bool {
			var ready bool
			err := db.QueryRow(`SELECT lease_token IS NULL FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&ready)
			return err == nil && ready
		}, 5*time.Second, 10*time.Millisecond)
		_, err := db.Exec(`UPDATE gateway_subscription_outbox SET next_attempt_at=clock_timestamp() WHERE api_key_id=$1`, f.key.ID)
		require.NoError(t, err)
		startWorker()
	}
	require.Eventually(t, func() bool {
		var done bool
		err := db.QueryRow(`SELECT completed_at IS NOT NULL FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&done)
		return err == nil && done
	}, 5*time.Second, 10*time.Millisecond)
	worker.StopLiveSettlementWorker()
	calls := f.awaitSettlements(t, 1)
	assertCounters(t, inflightCharge(t, f, calls[0].command), f.readCounters(t).minus(before))
	assertCounterProofCounts(t, f.counterFixture, 1, 1)
	assertAdmissionUsageLink(t, f, true)
	assert.EqualValues(t, 1, f.sends.Load())
	require.Eventually(t, func() bool {
		var closed bool
		err := db.QueryRow(`SELECT execution_state<>'inflight' FROM gateway_requests WHERE api_key_id=$1`, f.key.ID).Scan(&closed)
		return err == nil && closed
	}, 5*time.Second, 10*time.Millisecond)
	assert.Zero(t, f.usageLogs.legacyWrites.Load(), "receipt projection has one transaction owner; no legacy tail writer")
	t.Logf("projection_owner pending_before_apply=%d legacy_writes=%d sends=%d", pendingLogs, f.usageLogs.legacyWrites.Load(), f.sends.Load())
}

func runSubscriptionProjectionFailure(t *testing.T, db *sql.DB, client *dbent.Client) {
	f := newAdmissionFixture(t, db, client, true)
	before := f.readCounters(t)
	name := fmt.Sprintf("pending_projection_fault_%d", f.key.ID)
	_, err := db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.api_key_id=%d THEN RAISE EXCEPTION 'synthetic pending projection fault'; END IF; RETURN NEW; END $$`, name, f.key.ID))
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION %s()`, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP TRIGGER IF EXISTS ` + name + ` ON usage_logs`)
		_, _ = db.Exec(`DROP FUNCTION IF EXISTS ` + name + `()`)
	})
	status, body, err := f.request()
	require.NoError(t, err)
	require.Equal(t, 200, status, body)
	f.workers.Stop()
	require.Empty(t, f.billing.snapshot(), "projection failure must precede any Apply")
	assertCounters(t, before, f.readCounters(t))
	assertCounterProofCounts(t, f.counterFixture, 0, 0)
	var outboxes, logs int
	var usageState string
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&outboxes))
	require.Zero(t, outboxes)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1`, f.key.ID).Scan(&logs))
	require.Zero(t, logs)
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE api_key_id=$1`, f.key.ID).Scan(&usageState))
	require.Equal(t, "usage_unknown", usageState)
	assert.Zero(t, f.usageLogs.legacyWrites.Load())
	assert.EqualValues(t, 1, f.sends.Load())
}

func runSubscriptionProjectionOwnership(t *testing.T, db *sql.DB, client *dbent.Client) {
	for _, field := range []string{"user", "key", "account", "request", "subscription", "group", "entitlement", "payload_hash", "log_group", "receipt_control"} {
		t.Run(field, func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, true)
			other := newCounterFixture(t, db, client, true, 0.5, false, false)
			before, otherBefore := f.readCounters(t), other.readCounters(t)
			f.usageLogs.rewriteObservation = func(in requestledger.SubscriptionObservation) requestledger.SubscriptionObservation {
				var cmd service.UsageBillingCommand
				var log service.UsageLog
				assert.NoError(t, json.Unmarshal([]byte(in.Command), &cmd))
				assert.NoError(t, json.Unmarshal([]byte(in.Log), &log))
				switch field {
				case "user":
					cmd.UserID, log.UserID = other.user.ID, other.user.ID
				case "key":
					cmd.APIKeyID, log.APIKeyID = other.key.ID, other.key.ID
				case "account":
					cmd.AccountID, log.AccountID = other.account.ID, other.account.ID
				case "request":
					cmd.RequestID, log.RequestID = "gateway:wrong-private-request", "gateway:wrong-private-request"
				case "subscription":
					cmd.SubscriptionID, log.SubscriptionID = &other.sub.ID, &other.sub.ID
				case "group":
					cmd.APIKeyGroupID, log.GroupID = other.key.GroupID, other.key.GroupID
				case "entitlement":
					cmd.PackageEntitlementID = &other.sub.PackageEntitlement.ID
				case "payload_hash":
					cmd.RequestPayloadHash = "wrong-payload-hash"
				case "log_group":
					log.GroupID = other.key.GroupID
				case "receipt_control":
					cmd.SubscriptionReceiptID = "00000000-0000-0000-0000-000000000000"
				}
				c, err := json.Marshal(cmd)
				assert.NoError(t, err)
				l, err := json.Marshal(log)
				assert.NoError(t, err)
				in.Command, in.Log = string(c), string(l)
				return in
			}
			status, body, err := f.request()
			require.NoError(t, err)
			require.Equal(t, 200, status, body)
			f.workers.Stop()
			var outboxes, logs int
			var state string
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id IN ($1,$2)`, f.key.ID, other.key.ID).Scan(&outboxes))
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id IN ($1,$2)`, f.key.ID, other.key.ID).Scan(&logs))
			require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE api_key_id=$1`, f.key.ID).Scan(&state))
			assert.Zero(t, outboxes, "wrong identity must never publish a recoverable command")
			assert.Zero(t, logs, "wrong identity must never publish a pending projection")
			assert.Equal(t, "usage_unknown", state)
			assert.Empty(t, f.billing.snapshot(), "ownership validation must precede Apply")
			assertCounters(t, before, f.readCounters(t))
			assertCounters(t, otherBefore, other.readCounters(t))
			assertCounterProofCounts(t, f.counterFixture, 0, 0)
			assert.Zero(t, f.usageLogs.legacyWrites.Load())
			assert.EqualValues(t, 1, f.sends.Load())
			assert.Zero(t, other.sends.Load())
		})
	}
}
