//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errInflightCrash = errors.New("synthetic process boundary after durable observation")

type interruptedInflightBilling struct {
	service.UsageBillingRepository
	afterCommit bool
}

func (r interruptedInflightBilling) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if r.afterCommit {
		if _, err := r.UsageBillingRepository.Apply(ctx, cmd); err != nil {
			return nil, err
		}
	}
	return nil, errInflightCrash
}

type blockedInflightBilling struct {
	service.UsageBillingRepository
	entered, release chan struct{}
}

func (r blockedInflightBilling) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	close(r.entered)
	<-r.release
	return nil, errInflightCrash
}

func runInflightBoundaries(t *testing.T, db *sql.DB, client *dbent.Client) {
	t.Run("failover_keeps_previous_unknown_usage", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			n := f.sends.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if n == 1 {
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"error":{"type":"server_error","message":"synthetic unknown usage"}}`)
				return
			}
			_, _ = io.WriteString(w, `{"id":"synthetic-second","model":"gpt-5.1","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}`)
		})
		a, err := client.Account.Create().SetName("inflight-second-" + uuid.NewString()).SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).SetCredentials(map[string]any{"base_url": f.upstream.URL, "api_key": "disabled-fixture"}).SetExtra(map[string]any{"openai_responses_supported": true}).SetConcurrency(0).SetPriority(100).SetStatus(service.StatusActive).SetSchedulable(true).Save(context.Background())
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,100)`, a.ID, *f.key.GroupID)
		require.NoError(t, err)
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		require.EqualValues(t, 2, f.sends.Load())
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		assertCounterProofCounts(t, f.counterFixture, 1, 1)
		var state string
		require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE api_key_id=$1`, f.key.ID).Scan(&state))
		require.Equal(t, "usage_unknown", state)
		var receipts, observations int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
		require.Equal(t, 2, receipts)
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&observations))
		require.Equal(t, 1, observations)
	})

	for _, packaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("real_pg_apply_before_revoke_package_%t", packaged), func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, packaged)
			before := f.readCounters(t)
			gatePID, release := blockAdmissionApply(t, f)
			response := startAdmissionRequest(f)
			applyPID := awaitPGBlockedBy(t, db, gatePID)
			revoked := make(chan error, 1)
			go func() { revoked <- f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID) }()
			revokePID := awaitPGBlockedBy(t, db, applyPID)
			release()
			select {
			case err := <-revoked:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("revocation did not complete")
			}
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, 200, r.status, r.body)
			call := f.awaitSettlements(t, 1)[0]
			require.NoError(t, call.err)
			assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
			assertInflightClosed(t, f)
			assertAdmissionUsageLink(t, f, true)
			t.Logf("real_pg_wait_graph gate_pid=%d apply_pid=%d revoke_pid=%d", gatePID, applyPID, revokePID)
		})
		t.Run(fmt.Sprintf("real_pg_revoke_before_apply_package_%t", packaged), func(t *testing.T) {
			gate := newAdmissionResponseGate(t)
			f := newAdmissionFixture(t, db, client, packaged, gate)
			before := f.readCounters(t)
			response := startAdmissionRequest(f)
			awaitResponseGate(t, gate)
			tx, ctx, pid := newAdminTransaction(t, client)
			require.NoError(t, f.subscriptions.RevokeSubscription(ctx, f.sub.ID))
			gate.open()
			applyPID := awaitPGBlockedBy(t, db, pid)
			require.NoError(t, tx.Commit())
			r := receiveAdmissionResponse(t, response)
			require.NoError(t, r.err)
			require.Equal(t, 200, r.status, r.body)
			call := f.awaitSettlements(t, 1)[0]
			require.NoError(t, call.err)
			assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
			assertInflightClosed(t, f)
			assertAdmissionUsageLink(t, f, true)
			t.Logf("real_pg_wait_graph revoke_pid=%d apply_pid=%d", pid, applyPID)
		})
		t.Run(fmt.Sprintf("distinct_private_requests_each_bill_package_%t", packaged), func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, packaged)
			before := f.readCounters(t)
			for range 2 {
				status, body, err := f.request()
				require.NoError(t, err)
				require.Equal(t, 200, status, body)
			}
			calls := f.awaitSettlements(t, 2)
			require.NoError(t, calls[0].err)
			require.NoError(t, calls[1].err)
			require.NotEqual(t, calls[0].command.RequestID, calls[1].command.RequestID)
			one := inflightCharge(t, f, calls[0].command)
			two := counterSnapshot{Daily: one.Daily * 2, Weekly: one.Weekly * 2, Monthly: one.Monthly * 2, Requests: one.Requests * 2, Amount: one.Amount * 2, Tokens: one.Tokens * 2}
			assertCounters(t, two, f.readCounters(t).minus(before))
			assertCounterProofCounts(t, f.counterFixture, 2, 2)
			var outbox, done int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*),COUNT(completed_at) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&outbox, &done))
			require.Equal(t, 2, outbox)
			require.Equal(t, 2, done)
		})
	}
	t.Run("real_pg_restore_waits_for_revoke_commit", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		tx, ctx, pid := newAdminTransaction(t, client)
		require.NoError(t, f.subscriptions.RevokeSubscription(ctx, f.sub.ID))
		restored := make(chan error, 1)
		go func() { _, err := f.subscriptions.RestoreSubscription(context.Background(), f.sub.ID); restored <- err }()
		restorePID := awaitPGBlockedBy(t, db, pid)
		require.NoError(t, tx.Commit())
		select {
		case err := <-restored:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("restore did not complete")
		}
		var deleted bool
		require.NoError(t, db.QueryRow(`SELECT deleted_at IS NOT NULL FROM user_subscriptions WHERE id=$1`, f.sub.ID).Scan(&deleted))
		require.False(t, deleted)
		var kinds string
		require.NoError(t, db.QueryRow(`SELECT string_agg(kind,',' ORDER BY event_order) FROM gateway_subscription_events WHERE subscription_id=$1`, f.sub.ID).Scan(&kinds))
		require.Equal(t, "revoke,restore", kinds)
		t.Logf("real_pg_wait_graph revoke_pid=%d restore_pid=%d", pid, restorePID)
	})

	t.Run("cancel_after_durable_usage_production_worker_recovers", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
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
			t.Fatal("durable-before-Apply barrier not reached")
		}
		var durable int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1 AND command_payload IS NOT NULL AND completed_at IS NULL`, f.key.ID).Scan(&durable))
		require.Equal(t, 1, durable)
		cancel()
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		once.Do(func() { close(release) })
		_ = receiveAdmissionResponse(t, response)
		call := f.awaitSettlements(t, 1)[0]
		require.ErrorIs(t, call.err, errInflightCrash)
		// The billing probe records Apply's return before the caller's deferred
		// lease release. Wait for that durable handoff before advancing backoff;
		// otherwise release can overwrite our test clock and skip the first poll.
		require.Eventually(t, func() bool {
			var released bool
			err := db.QueryRow(`SELECT lease_token IS NULL FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&released)
			return err == nil && released
		}, 5*time.Second, 10*time.Millisecond)
		_, err := db.Exec(`UPDATE gateway_subscription_outbox SET next_attempt_at=clock_timestamp() WHERE api_key_id=$1`, f.key.ID)
		require.NoError(t, err)
		repo := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
		// The actual production constructor starts recovery without a new request.
		worker := service.NewOpenAIGatewayServiceWithLiveBilling(nil, nil, repo, nil, nil, nil, nil, &config.Config{}, nil, nil, service.NewBillingService(&config.Config{}, nil), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, requestledger.New(db))
		t.Cleanup(worker.StopLiveSettlementWorker)
		require.Eventually(t, func() bool {
			var done bool
			err := db.QueryRow(`SELECT completed_at IS NOT NULL FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&done)
			return err == nil && done
		}, 5*time.Second, 10*time.Millisecond)
		worker.StopLiveSettlementWorker()
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		assertCounterProofCounts(t, f.counterFixture, 1, 1)
		assertAdmissionUsageLink(t, f, true)
		require.EqualValues(t, 1, f.sends.Load())
	})

	t.Run("revocation_stops_internal_compaction_retry", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		f.requestBody = `{"model":"gpt-5.1","stream":false,"input":[{"id":"cmp_fixture","type":"compaction","encrypted_content":"opaque-fixture"},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]}`
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.sends.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			_, _ = io.WriteString(w, `{"error":{"code":"invalid_encrypted_content","type":"invalid_request_error","message":"bad encrypted content"}}`)
		})
		before := f.readCounters(t)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.GreaterOrEqual(t, r.status, 400)
		require.EqualValues(t, 1, f.sends.Load())
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		var receipts int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_receipts WHERE api_key_id=$1`, f.key.ID).Scan(&receipts))
		require.Equal(t, 1, receipts)
	})
	t.Run("expired_lease_worker_cannot_commit", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		f.billing.UsageBillingRepository = interruptedInflightBilling{UsageBillingRepository: f.billing.UsageBillingRepository}
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		call := f.awaitSettlements(t, 1)[0]
		require.ErrorIs(t, call.err, errInflightCrash)
		o, err := requestledger.LoadSubscriptionOutbox(context.Background(), db, call.command.SubscriptionReceiptID)
		require.NoError(t, err)
		ctx := requestledger.ResumeSubscriptionOutbox(context.Background(), db, o)
		old, err := requestledger.ClaimSubscriptionOutbox(ctx, db, o.Receipt.ID)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE gateway_subscription_outbox SET lease_until=clock_timestamp()-INTERVAL '1 second' WHERE receipt_id=$1`, o.Receipt.ID)
		require.NoError(t, err)
		current, err := requestledger.ClaimSubscriptionOutbox(ctx, db, o.Receipt.ID)
		require.NoError(t, err)
		repo := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
		_, err = repo.Apply(old, &call.command)
		require.Error(t, err)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		result, err := repo.Apply(current, &call.command)
		require.NoError(t, err)
		require.True(t, result.Applied)
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		assertAdmissionUsageLink(t, f, true)
	})
	t.Run("durable_write_failure_leaves_unknown_and_no_charge", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		name := fmt.Sprintf("inflight_persist_fail_%d", f.sub.ID)
		_, err := db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.api_key_id=%d THEN RAISE EXCEPTION 'synthetic durable-write fault'; END IF; RETURN NEW; END $$`, name, f.key.ID))
		require.NoError(t, err)
		_, err = db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON gateway_subscription_outbox FOR EACH ROW EXECUTE FUNCTION %s()`, name, name))
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = db.Exec(`DROP TRIGGER IF EXISTS ` + name + ` ON gateway_subscription_outbox`)
			_, _ = db.Exec(`DROP FUNCTION IF EXISTS ` + name + `()`)
		})
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		require.Empty(t, f.billing.snapshot())
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		var n int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&n))
		require.Zero(t, n)
		var state string
		require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE api_key_id=$1`, f.key.ID).Scan(&state))
		require.NotEqual(t, "known", state)
	})
	t.Run("ordinary_exhausted_revoke_reset_restore_does_not_gain_exception", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, false, gate)
		_, err := db.Exec(`UPDATE groups SET daily_limit_usd=100 WHERE id=$1`, *f.key.GroupID)
		require.NoError(t, err)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		_, err = db.Exec(`UPDATE user_subscriptions SET daily_usage_usd=200 WHERE id=$1`, f.sub.ID)
		require.NoError(t, err)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		_, err = f.subscriptions.RestoreSubscription(context.Background(), f.sub.ID)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE user_subscriptions SET daily_usage_usd=0 WHERE id=$1`, f.sub.ID)
		require.NoError(t, err)
		before := f.readCounters(t)
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, 200, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.Error(t, call.err)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
	})

	for _, window := range []string{"daily", "weekly", "monthly"} {
		for _, beforeSend := range []bool{true, false} {
			t.Run(fmt.Sprintf("ordinary_%s_concurrent_exhaustion_before_send_%t", window, beforeSend), func(t *testing.T) {
				gate := newAdmissionResponseGate(t)
				var f *admissionFixture
				if beforeSend {
					f = newAdmissionFixture(t, db, client, false, nil, gate)
				} else {
					f = newAdmissionFixture(t, db, client, false, gate)
				}
				_, err := db.Exec(`UPDATE groups SET `+window+`_limit_usd=100 WHERE id=$1`, *f.key.GroupID)
				require.NoError(t, err)
				response := startAdmissionRequest(f)
				awaitResponseGate(t, gate)
				_, err = db.Exec(`UPDATE user_subscriptions SET `+window+`_usage_usd=200 WHERE id=$1`, f.sub.ID)
				require.NoError(t, err)
				before := f.readCounters(t)
				gate.open()
				r := receiveAdmissionResponse(t, response)
				require.NoError(t, r.err)
				if beforeSend {
					require.GreaterOrEqual(t, r.status, 400)
					require.Zero(t, f.sends.Load())
				} else {
					require.Equal(t, 200, r.status, r.body)
					call := f.awaitSettlements(t, 1)[0]
					require.Error(t, call.err)
				}
				assertCounters(t, before, f.readCounters(t))
				assertCounterProofCounts(t, f.counterFixture, 0, 0)
			})
		}
	}

	for _, packaged := range []bool{false, true} {
		t.Run(fmt.Sprintf("durable_before_worker_queue_package_%t", packaged), func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, packaged)
			before := f.readCounters(t)
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }) })
			f.workers.Submit(func(context.Context) { close(started); <-release })
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("worker occupancy barrier failed")
			}
			status, body, err := f.request()
			require.NoError(t, err)
			require.Equal(t, 200, status, body)
			// A blocked in-memory worker cannot hold the only copy of usage.
			var durable, done int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*),COUNT(completed_at) FROM gateway_subscription_outbox WHERE api_key_id=$1 AND command_payload IS NOT NULL`, f.key.ID).Scan(&durable, &done))
			require.Equal(t, 1, durable)
			require.Equal(t, 1, done)
			require.Zero(t, f.workers.Stats().CompletedTasks)
			calls := f.awaitSettlements(t, 1)
			assertCounters(t, inflightCharge(t, f, calls[0].command), f.readCounters(t).minus(before))
			once.Do(func() { close(release) })
		})
		for _, afterCommit := range []bool{false, true} {
			t.Run(fmt.Sprintf("crash_package_%t_after_commit_%t", packaged, afterCommit), func(t *testing.T) {
				f := newAdmissionFixture(t, db, client, packaged)
				before := f.readCounters(t)
				f.billing.UsageBillingRepository = interruptedInflightBilling{f.billing.UsageBillingRepository, afterCommit}
				status, body, err := f.request()
				require.NoError(t, err)
				require.Equal(t, 200, status, body)
				call := f.awaitSettlements(t, 1)[0]
				require.ErrorIs(t, call.err, errInflightCrash)
				var rows, done int
				require.NoError(t, db.QueryRow(`SELECT COUNT(*),COUNT(completed_at) FROM gateway_subscription_outbox WHERE api_key_id=$1`, f.key.ID).Scan(&rows, &done))
				require.Equal(t, 1, rows)
				if afterCommit {
					require.Equal(t, 1, done)
				} else {
					require.Zero(t, done)
					assertCounters(t, before, f.readCounters(t))
					assertCounterProofCounts(t, f.counterFixture, 0, 0)
				}
				require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
				recovered := recoverInflight(t, f)
				if afterCommit {
					require.Zero(t, recovered)
				} else {
					require.Equal(t, 1, recovered)
				}
				assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
				assertCounterProofCounts(t, f.counterFixture, 1, 1)
				assertAdmissionUsageLink(t, f, true)
				assertInflightDetail(t, f, call.command, true)
				assertInflightClosed(t, f)
				require.Zero(t, recoverInflight(t, f), "restart again cannot bill twice")
				require.EqualValues(t, 1, f.sends.Load(), "recovery must not resend upstream")
			})
		}
	}
	t.Run("outbox_completion_failure_rolls_back_all_billing", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		before := f.readCounters(t)
		name := fmt.Sprintf("inflight_completion_fail_%d", f.sub.ID)
		_, err := db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.api_key_id=%d AND NEW.completed_at IS NOT NULL THEN RAISE EXCEPTION 'synthetic completion fault'; END IF; RETURN NEW; END $$`, name, f.key.ID))
		require.NoError(t, err)
		_, err = db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON gateway_subscription_outbox FOR EACH ROW EXECUTE FUNCTION %s()`, name, name))
		require.NoError(t, err)
		remove := func() {
			_, _ = db.Exec(`DROP TRIGGER IF EXISTS ` + name + ` ON gateway_subscription_outbox`)
			_, _ = db.Exec(`DROP FUNCTION IF EXISTS ` + name + `()`)
		}
		t.Cleanup(remove)
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		call := f.awaitSettlements(t, 1)[0]
		require.Error(t, call.err)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		assertAdmissionUsageLink(t, f, false)
		remove()
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		require.Equal(t, 1, recoverInflight(t, f))
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		assertCounterProofCounts(t, f.counterFixture, 1, 1)
		assertAdmissionUsageLink(t, f, true)
	})
	t.Run("revoke_restore_new_entitlement_revoke_keeps_first_target", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		before := f.readCounters(t)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		_, err := f.subscriptions.RestoreSubscription(context.Background(), f.sub.ID)
		require.NoError(t, err)
		var newEnt int64
		require.NoError(t, db.QueryRow(`INSERT INTO subscription_package_entitlements(user_id,group_id,source_type,grant_key,starts_at,expires_at,request_limit,amount_limit_usd,token_limit)
 VALUES($1,$2,'admin_grant',$3,NOW()-INTERVAL '1 hour',NOW()+INTERVAL '20 days',100,1000,1000000) RETURNING id`, f.user.ID, *f.key.GroupID, "inflight-renew-"+uuid.NewString()).Scan(&newEnt))
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, 200, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		var requests, tokens int64
		var amount float64
		var status string
		require.NoError(t, db.QueryRow(`SELECT request_used,amount_used_usd,token_used,status FROM subscription_package_entitlements WHERE id=$1`, newEnt).Scan(&requests, &amount, &tokens, &status))
		require.Zero(t, requests)
		require.Zero(t, amount)
		require.Zero(t, tokens)
		require.Equal(t, "revoked", status)
		assertInflightClosed(t, f)
		assertAdmissionUsageLink(t, f, true)
	})
	t.Run("already_exhausted_before_revoke_stays_pending", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		_, err := db.Exec(`UPDATE subscription_package_entitlements SET request_used=request_limit,status='exhausted',exhausted_reason='request' WHERE id=$1`, f.sub.PackageEntitlement.ID)
		require.NoError(t, err)
		before := f.readCounters(t)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, 200, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.Error(t, call.err)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
		require.Zero(t, recoverInflight(t, f))
		assertAdmissionUsageLink(t, f, false)
	})
	t.Run("archive_replay_and_mutated_commands", func(t *testing.T) {
		f := newAdmissionFixture(t, db, client, true)
		status, body, err := f.request()
		require.NoError(t, err)
		require.Equal(t, 200, status, body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		after := f.readCounters(t)
		archiveBindingProof(t, db, f.key.ID)
		base := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
		ctx := f.billingContext.retained()
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cmd := call.command
				result, err := base.Apply(ctx, &cmd)
				if err == nil && (result == nil || result.Applied || !result.SettlementVerified) {
					err = errors.New("archive duplicate was not verified replay")
				}
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		for _, field := range []string{"receipt", "amount", "usage", "target"} {
			cmd := call.command
			switch field {
			case "receipt":
				cmd.SubscriptionReceiptID = uuid.NewString()
			case "amount":
				cmd.BilledCost++
			case "usage":
				cmd.InputTokens++
			case "target":
				v := *cmd.PackageEntitlementID + 100
				cmd.PackageEntitlementID = &v
			}
			_, err := base.Apply(ctx, &cmd)
			require.Error(t, err, field)
		}
		assertCounters(t, after, f.readCounters(t))
		require.EqualValues(t, 1, f.sends.Load())
	})
	for _, tamper := range []string{"price", "usage"} {
		t.Run("durable_"+tamper+"_tampering_rejected", func(t *testing.T) {
			f := newAdmissionFixture(t, db, client, true)
			before := f.readCounters(t)
			f.billing.UsageBillingRepository = interruptedInflightBilling{UsageBillingRepository: f.billing.UsageBillingRepository}
			status, body, err := f.request()
			require.NoError(t, err)
			require.Equal(t, 200, status, body)
			call := f.awaitSettlements(t, 1)[0]
			require.ErrorIs(t, call.err, errInflightCrash)
			if tamper == "price" {
				_, err = db.Exec(`UPDATE gateway_subscription_receipts SET pricing_payload='{}' WHERE id=$1`, call.command.SubscriptionReceiptID)
			} else {
				_, err = db.Exec(`UPDATE gateway_subscription_outbox SET usage_payload='{}' WHERE receipt_id=$1`, call.command.SubscriptionReceiptID)
			}
			require.NoError(t, err)
			require.Zero(t, recoverInflight(t, f))
			assertCounters(t, before, f.readCounters(t))
			assertCounterProofCounts(t, f.counterFixture, 0, 0)
		})
	}
	t.Run("zero_multiplier_still_counts_package_request_and_tokens", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		_, err := db.Exec(`UPDATE groups SET rate_multiplier=0 WHERE id=$1`, *f.key.GroupID)
		require.NoError(t, err)
		before := f.readCounters(t)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, 200, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		require.Zero(t, call.command.SubscriptionCost)
		require.Zero(t, call.command.BilledCost)
		assertCounters(t, counterSnapshot{Requests: 1, Tokens: 120}, f.readCounters(t).minus(before))
		assertAdmissionUsageLink(t, f, true)
	})
	t.Run("partial_failed_stream_observed_usage_settles_once", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		configureInflightStream(f, true)
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.sends.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.output_text.delta","delta":"synthetic partial"}`+"\n\n")
			_, _ = io.WriteString(w, "data: "+`{"type":"response.failed","response":{"id":"synthetic-partial","model":"gpt-5.1","status":"failed","usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":11,"cache_creation_tokens":7}}}}`+"\n\n")
		})
		before := f.readCounters(t)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.Equal(t, 200, r.status, r.body)
		call := f.awaitSettlements(t, 1)[0]
		require.NoError(t, call.err)
		assertCounters(t, inflightCharge(t, f, call.command), f.readCounters(t).minus(before))
		require.EqualValues(t, 1, f.sends.Load())
	})
	t.Run("revocation_stops_actual_failover_send", func(t *testing.T) {
		gate := newAdmissionResponseGate(t)
		f := newAdmissionFixture(t, db, client, true, gate)
		f.upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			f.sends.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(503)
			_, _ = io.WriteString(w, `{"error":{"type":"server_error","message":"synthetic retryable zero-usage failure"}}`)
		})
		a, err := client.Account.Create().SetName("inflight-failover-" + uuid.NewString()).SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).SetCredentials(map[string]any{"base_url": f.upstream.URL, "api_key": "disabled-fixture"}).SetExtra(map[string]any{"openai_responses_supported": true}).SetConcurrency(0).SetPriority(100).SetStatus(service.StatusActive).SetSchedulable(true).Save(context.Background())
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,100)`, a.ID, *f.key.GroupID)
		require.NoError(t, err)
		before := f.readCounters(t)
		response := startAdmissionRequest(f)
		awaitResponseGate(t, gate)
		require.EqualValues(t, 1, f.sends.Load())
		require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
		gate.open()
		r := receiveAdmissionResponse(t, response)
		require.NoError(t, r.err)
		require.GreaterOrEqual(t, r.status, 400)
		assert.EqualValues(t, 1, f.sends.Load(), "second account cannot receive a new send after revocation")
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f.counterFixture, 0, 0)
	})
}

func recoverInflight(t *testing.T, f *admissionFixture) int {
	t.Helper()
	_, err := f.db.Exec(`UPDATE gateway_subscription_outbox SET next_attempt_at=clock_timestamp() WHERE api_key_id=$1`, f.key.ID)
	require.NoError(t, err)
	repo := repository.NewUsageBillingRepositoryWithLedger(f.client, f.db, service.NewBalanceLedgerService(f.db, nil, nil))
	recovery, ok := repo.(interface {
		RecoverSubscriptionSettlements(context.Context, int) (int, error)
	})
	require.True(t, ok)
	n, err := recovery.RecoverSubscriptionSettlements(context.Background(), 100)
	require.NoError(t, err)
	return n
}

func newAdminTransaction(t *testing.T, client *dbent.Client) (*dbent.Tx, context.Context, int) {
	t.Helper()
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	ctx := dbent.NewTxContext(context.Background(), tx)
	rows, err := tx.QueryContext(ctx, `SELECT pg_backend_pid()`)
	require.NoError(t, err)
	require.True(t, rows.Next())
	var pid int
	require.NoError(t, rows.Scan(&pid))
	require.NoError(t, rows.Close())
	return tx, ctx, pid
}

// Ordering is established by PostgreSQL's actual wait graph, not sleeps or
// goroutine launch order. These checks also prove operations really overlap.
func awaitPGBlockedBy(t *testing.T, db *sql.DB, blocker int) int {
	t.Helper()
	var pid int
	require.Eventually(t, func() bool {
		return db.QueryRow(`SELECT pid FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND datname=current_database() ORDER BY pid LIMIT 1`, blocker).Scan(&pid) == nil
	}, 10*time.Second, 10*time.Millisecond, "no real PostgreSQL waiter for blocker %d", blocker)
	return pid
}

// The temporary trigger stalls the real Apply transaction after its quota row
// lock is held. It changes no value and exists only in the disposable database.
func blockAdmissionApply(t *testing.T, f *admissionFixture) (int, func()) {
	t.Helper()
	conn, err := f.db.Conn(context.Background())
	require.NoError(t, err)
	key := int64(9000000) + f.sub.ID
	var pid int
	require.NoError(t, conn.QueryRowContext(context.Background(), `SELECT pg_backend_pid()`).Scan(&pid))
	_, err = conn.ExecContext(context.Background(), `SELECT pg_advisory_lock($1)`, key)
	require.NoError(t, err)
	var once sync.Once
	release := func() {
		once.Do(func() {
			_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, key)
			_ = conn.Close()
		})
	}
	name := fmt.Sprintf("binding_apply_gate_%d", f.sub.ID)
	table, id, predicate := "user_subscriptions", f.sub.ID, "NEW.daily_usage_usd > OLD.daily_usage_usd"
	if f.sub.PackageEntitlement != nil {
		table = "subscription_package_entitlements"
		id = f.sub.PackageEntitlement.ID
		predicate = "NEW.request_used = OLD.request_used + 1"
	}
	t.Cleanup(func() {
		release()
		_, _ = f.db.Exec(`DROP TRIGGER IF EXISTS ` + name + ` ON ` + table)
		_, _ = f.db.Exec(`DROP FUNCTION IF EXISTS ` + name + `()`)
	})
	_, err = f.db.Exec(fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.id=%d AND (%s) THEN PERFORM pg_advisory_xact_lock(%d); END IF; RETURN NEW; END $$`, name, id, predicate, key))
	require.NoError(t, err)
	_, err = f.db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION %s()`, name, table, name))
	require.NoError(t, err)
	return pid, release
}
