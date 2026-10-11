//go:build integration && billing_acceptance

package billingacceptance

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCounterOwnership(t *testing.T, db *sql.DB, client *dbent.Client) {
	t.Run("zero_multiplier_wrong_group_rolls_back", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0, true, false)
		other := newCounterFixture(t, db, client, false, 0, false, false)
		_, err := db.Exec(`UPDATE subscription_package_entitlements SET group_id=$2 WHERE id=$1`, f.sub.PackageEntitlement.ID, other.sub.GroupID)
		require.NoError(t, err)
		before := f.readCounters(t)
		f.send(t)
		assert.ErrorIs(t, f.settlementErrors[0], service.ErrUsageBillingOwnershipMismatch)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f, 0, 0)
	})
	t.Run("zero_multiplier_wrong_subscription_rolls_back", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0, true, false)
		other := newCounterFixture(t, db, client, false, 0, false, false)
		f.sub.ID = other.sub.ID
		before := f.readCounters(t)
		f.send(t)
		assert.ErrorIs(t, f.settlementErrors[0], service.ErrUsageBillingOwnershipMismatch)
		assertCounters(t, before, f.readCounters(t))
		assertCounterProofCounts(t, f, 0, 0)
	})
	t.Run("frozen_group_survives_key_reassignment", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		other := newCounterFixture(t, db, client, false, 0.5, false, false)
		_, err := db.Exec(`UPDATE api_keys SET group_id=$2 WHERE id=$1`, f.key.ID, other.sub.GroupID)
		require.NoError(t, err)
		before := f.readCounters(t)
		f.send(t)
		require.NoError(t, f.settlementErrors[0])
		f.assertSettlement(t, before, 1, 0.5, false, false)
	})
	t.Run("dedup_does_not_authorize_changed_frozen_group", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0, true, false)
		other := newCounterFixture(t, db, client, false, 0, false, false)
		f.send(t)
		require.NoError(t, f.settlementErrors[0])
		after := f.readCounters(t)
		// Group is deliberately absent from the legacy fingerprint. Ownership
		// must be checked even when that fingerprint and private request match.
		original := f.key.GroupID
		f.key.GroupID = &other.sub.GroupID
		err := f.gateway.RecordUsage(f.retainedCtx, f.retainedInput)
		assert.ErrorIs(t, err, service.ErrUsageBillingOwnershipMismatch)
		calls := f.billing.snapshot()
		require.Equal(t, calls[0].command.RequestFingerprint, calls[1].command.RequestFingerprint)
		f.key.GroupID = original
		assertCounters(t, after, f.readCounters(t))
		assertCounterProofCounts(t, f, 1, 1)
	})
	for _, wrong := range []string{"group", "user"} {
		t.Run("prior_dedup_does_not_authorize_new_wrong_"+wrong, func(t *testing.T) {
			f := newCounterFixture(t, db, client, true, 0.5, false, false)
			other := newCounterFixture(t, db, client, true, 0.5, false, false)
			f.send(t)
			require.NoError(t, f.settlementErrors[0])
			after := f.readCounters(t)
			if wrong == "group" {
				_, err := db.Exec(`UPDATE subscription_package_entitlements SET group_id=$2 WHERE id=$1`, f.sub.PackageEntitlement.ID, other.sub.GroupID)
				require.NoError(t, err)
			} else {
				_, err := db.Exec(`UPDATE subscription_package_entitlements SET user_id=$2 WHERE id=$1`, f.sub.PackageEntitlement.ID, other.user.ID)
				require.NoError(t, err)
			}
			f.send(t)
			assert.ErrorIs(t, f.settlementErrors[1], service.ErrUsageBillingOwnershipMismatch)
			assertCounters(t, after, f.readCounters(t))
			assertCounterProofCounts(t, f, 1, 1)
			var requests int
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM gateway_requests WHERE user_id=$1 AND route='/v1/responses'`, f.user.ID).Scan(&requests))
			require.Equal(t, 2, requests, "two distinct private HTTP requests")
		})
	}
	t.Run("concurrent_independent_private_requests", func(t *testing.T) {
		f := newCounterFixture(t, db, client, true, 0.5, false, false)
		f.callbacks = 4
		f.concurrentCallbacks = true
		before := f.readCounters(t)
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, err := http.NewRequest("POST", f.server.URL+"/v1/responses", strings.NewReader(`{"model":"gpt-5.1","input":"synthetic counter acceptance","stream":false}`))
				if err != nil {
					errs <- err
					return
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Client-Request-ID", "same-client-header")
				req.Header.Set("X-Request-ID", "same-request-header")
				response, err := f.server.Client().Do(req)
				if err != nil {
					errs <- err
					return
				}
				defer func() { _ = response.Body.Close() }()
				_, err = io.Copy(io.Discard, response.Body)
				if err == nil && response.StatusCode != http.StatusOK {
					err = fmt.Errorf("HTTP status %d", response.StatusCode)
				}
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		require.EqualValues(t, 2, f.sends.Load())
		f.mu.Lock()
		settlements := append([]error(nil), f.settlementErrors...)
		f.mu.Unlock()
		require.Len(t, settlements, 8)
		for _, err := range settlements {
			require.NoError(t, err)
		}
		f.assertSettlement(t, before, 2, 0.5, false, false)
	})
	for _, status := range []string{"exhausted", "expired", "revoked"} {
		for _, archive := range []bool{false, true} {
			t.Run(fmt.Sprintf("historical_%s_archive_%t", status, archive), func(t *testing.T) {
				f := newCounterFixture(t, db, client, true, 0.5, false, false)
				before := f.readCounters(t)
				f.send(t)
				require.NoError(t, f.settlementErrors[0])
				f.assertSettlement(t, before, 1, 0.5, false, false)
				after := f.readCounters(t)
				if status == "revoked" {
					// Real revocation soft-deletes the subscription and retires its
					// entitlements atomically. Historical proof remains valid.
					require.NoError(t, f.subscriptions.RevokeSubscription(context.Background(), f.sub.ID))
				} else {
					_, err := db.Exec(`UPDATE subscription_package_entitlements SET status=$2,starts_at=NOW()-INTERVAL '2 days',expires_at=NOW()-INTERVAL '1 day' WHERE id=$1`, f.sub.PackageEntitlement.ID, status)
					require.NoError(t, err)
				}
				if archive {
					tx, err := db.BeginTx(context.Background(), nil)
					require.NoError(t, err)
					defer func() { _ = tx.Rollback() }()
					_, err = tx.Exec(`INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) SELECT request_id,api_key_id,request_fingerprint,created_at FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					_, err = tx.Exec(`DELETE FROM usage_billing_dedup WHERE api_key_id=$1`, f.key.ID)
					require.NoError(t, err)
					require.NoError(t, tx.Commit())
				}
				var wg sync.WaitGroup
				errs := make(chan error, 8)
				for range 8 {
					wg.Add(1)
					go func() {
						defer wg.Done()
						input := *f.retainedInput
						result := *input.Result
						input.Result = &result
						errs <- f.gateway.RecordUsage(f.retainedCtx, &input)
					}()
				}
				wg.Wait()
				close(errs)
				for err := range errs {
					assert.NoError(t, err, "historical exact replay")
				}
				assertCounters(t, after, f.readCounters(t))
				for _, call := range f.billing.snapshot()[1:] {
					if assert.NoError(t, call.err) && assert.NotNil(t, call.result) {
						assert.False(t, call.result.Applied)
						assert.True(t, call.result.SettlementVerified)
						assert.Equal(t, call.command.RequestFingerprint, call.result.SettlementFingerprint)
					}
				}
				assertCounterProofCounts(t, f, 1, 1)
				var usages int
				require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE api_key_id=$1 AND billing_settled`, f.key.ID).Scan(&usages))
				require.Equal(t, 1, usages)
			})
		}
	}
}

func assertCounterProofCounts(t *testing.T, f *counterFixture, dedup, verified int) {
	t.Helper()
	var got int
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM (SELECT request_id FROM usage_billing_dedup WHERE api_key_id=$1 UNION SELECT request_id FROM usage_billing_dedup_archive WHERE api_key_id=$1) proof`, f.key.ID).Scan(&got))
	assert.Equal(t, dedup, got)
	require.NoError(t, f.db.QueryRow(`SELECT COUNT(*) FROM gateway_request_billing_links WHERE api_key_id=$1 AND settlement_verified`, f.key.ID).Scan(&got))
	assert.Equal(t, verified, got)
}
