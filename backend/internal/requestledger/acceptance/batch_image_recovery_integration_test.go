//go:build integration

package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBatchImageHTTPRecoveryAndIsolation(t *testing.T) {
	f := newBatchImageFixture(t)
	ctx := context.Background()
	t.Run("retained zero capture proof needs no invented wallet row", func(t *testing.T) {
		s := f.submit(t, "?free-proof=1")
		before := f.balance(t)
		_, err := f.settler.Settle(ctx, s.ID)
		require.NoError(t, err)
		require.Equal(t, before, f.balance(t))
		ref := f.reference(t, s.RequestID)
		require.Nil(t, ref.WalletTransactionID)
		require.NotNil(t, ref.BilledCost)
		require.Zero(t, *ref.BilledCost)
		status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"settlement_state":"settled"`)
		require.Contains(t, body, `"wallet":[]`)
	})
	t.Run("capture survives usage database failure and retries without provider replay", func(t *testing.T) {
		s := f.submit(t, "")
		before := f.balance(t)
		_, err := f.db.Exec(`CREATE FUNCTION reject_batch_usage() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic usage persistence failure'; END $$;
 CREATE TRIGGER reject_batch_usage BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION reject_batch_usage()`)
		require.NoError(t, err)
		_, err = f.settler.Settle(ctx, s.ID)
		require.ErrorIs(t, err, service.ErrBatchImageSettlementBillingFailed)
		f.requireCapture(t, s.ID, before+4)
		job, err := f.jobs.GetBatchImageJobByBatchID(ctx, s.ID)
		require.NoError(t, err)
		require.Equal(t, service.BatchImageJobStatusSettling, job.Status)
		require.Equal(t, "USAGE_LOG_PERSIST_FAILED", *job.LastErrorCode)
		f.requireNoUsage(t, s)
		_, err = f.db.Exec(`DROP TRIGGER reject_batch_usage ON usage_logs; DROP FUNCTION reject_batch_usage()`)
		require.NoError(t, err)
		// An unconfigured provider is intentional: settlement recovery must never
		// submit/poll/generate again. Only the existing settlement branch can pass.
		pipeline := &service.BatchImagePipelineProcessor{Ledger: f.ledger, ProviderProcessor: &service.BatchImageProviderProcessor{Repo: f.jobs}, SettlementService: f.settler}
		result, err := pipeline.Process(ctx, s.ID)
		require.NoError(t, err)
		require.True(t, result.Terminal)
		f.requireCapture(t, s.ID, before+4)
		ref := f.reference(t, s.RequestID)
		require.False(t, ref.Applied, "shared batch proof must not claim this admission applied the charge")
		require.Equal(t, &s.ID, ref.BatchID)
		var workerID string
		require.NoError(t, f.db.QueryRow(`SELECT request_id FROM gateway_request_tasks WHERE task_ref=$1 AND request_id<>$2`, s.ID, s.RequestID).Scan(&workerID))
		require.Equal(t, ref, f.reference(t, workerID))
		for range 2 {
			result, err := f.settler.Settle(ctx, s.ID)
			require.NoError(t, err)
			require.True(t, result.AlreadySettled)
		}
		f.requireCapture(t, s.ID, before+4)
		require.Equal(t, ref, f.reference(t, s.RequestID))
	})
	t.Run("concurrent duplicate settlement and separate request outcomes", func(t *testing.T) {
		s := f.submit(t, "")
		before := f.balance(t)
		var wg sync.WaitGroup
		errs := make(chan error, 4)
		for range 4 {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := f.settler.Settle(ctx, s.ID); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		f.requireCapture(t, s.ID, before+4)
		ref := f.reference(t, s.RequestID)
		h, err := f.ledger.Begin(ctx, "/v1/images/batches", "POST", "http")
		require.NoError(t, err)
		require.NoError(t, requestledger.BindTask(requestledger.WithHandle(ctx, h), "image_batch", s.ID, 101, 201))
		require.NoError(t, h.Finish(ctx, "cancelled", 0, "cancelled"))
		require.Equal(t, ref, f.reference(t, h.ID))
		status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+h.ID)
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"execution_state":"cancelled"`)
		require.Contains(t, body, `"settlement_state":"settled"`)
	})
	t.Run("cancellation releases the hold without a usage or zero bill inference", func(t *testing.T) {
		s := f.submit(t, "?queued=1")
		before := f.balance(t)
		require.NoError(t, f.ledger.ReconcileTaskBilling(ctx))
		status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"settlement_state":"settlement_pending"`)
		for range 2 {
			req, err := http.NewRequest("POST", f.server.URL+"/v1/images/batches/"+s.ID+"/cancel", nil)
			require.NoError(t, err)
			req.Header.Set("X-Ledger-Test-User", "101")
			response, err := f.server.Client().Do(req)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, 200, response.StatusCode)
		}
		require.Equal(t, before+10, f.balance(t))
		f.requireNoUsage(t, s)
		require.NoError(t, f.ledger.ReconcileTaskBilling(ctx))
		status, body = batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"image_balance_release"`)
		require.Contains(t, body, `"settlement_state":"settled"`)
		require.Contains(t, body, `"billing":[]`)
		require.NotContains(t, body, `"billed_cost":0`)
	})
	t.Run("real zero output is distinct from absent usage", func(t *testing.T) {
		s := f.submit(t, "?zero=1")
		before := f.balance(t)
		result, err := f.settler.Settle(ctx, s.ID)
		require.NoError(t, err)
		require.Zero(t, result.ActualCost)
		require.Equal(t, before+10, f.balance(t))
		ref := f.reference(t, s.RequestID)
		require.NotNil(t, ref.BilledCost)
		require.Zero(t, *ref.BilledCost)
		status, body := batchImageHTTPRead(t, f.server, "101", fmt.Sprintf("/api/v1/requests/%s/usage/%d", s.RequestID, *ref.UsageLogID))
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"image_count":0`)
	})
	t.Run("archive proof, conflicting proof, retained legacy rows and tenant isolation", func(t *testing.T) {
		s := f.submit(t, "")
		_, err := f.settler.Settle(ctx, s.ID)
		require.NoError(t, err)
		ref := f.reference(t, s.RequestID)
		path := fmt.Sprintf("/api/v1/requests/%s/usage/%d", s.RequestID, *ref.UsageLogID)
		for _, tc := range []struct {
			user, path string
			status     int
		}{
			{"", path, 401}, {"102", path, 404}, {"101", "/api/v1/admin/requests/" + s.RequestID, 403},
			{"999", fmt.Sprintf("/api/v1/admin/requests/%s/usage/%d", s.RequestID, *ref.UsageLogID), 200},
			{"102", "/v1/images/batches/" + s.ID + "/items", 404},
		} {
			status, body := batchImageHTTPRead(t, f.server, tc.user, tc.path)
			require.Equal(t, tc.status, status, body)
		}
		// Another owned key/request cannot borrow this batch's proof or usage.
		h, err := f.ledger.Begin(ctx, "/v1/images/batches", "POST", "http")
		require.NoError(t, err)
		require.NoError(t, requestledger.BindTask(requestledger.WithHandle(ctx, h), "image_batch", s.ID, 101, 202))
		require.NoError(t, h.Finish(ctx, "failed", 400, "upstream_error"))
		status, body := batchImageHTTPRead(t, f.server, "101", fmt.Sprintf("/api/v1/requests/%s/usage/%d", h.ID, *ref.UsageLogID))
		require.Equal(t, 404, status, body)
		capture := service.BatchImageCaptureRequestID(s.ID)
		_, err = f.db.Exec(`WITH moved AS (DELETE FROM usage_billing_dedup WHERE request_id=$1 RETURNING request_id,api_key_id,request_fingerprint,created_at) INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) SELECT * FROM moved`, capture)
		require.NoError(t, err)
		require.Equal(t, ref, f.reference(t, s.RequestID))
		// A conflicting hot/cold proof is not resolved by selecting whichever matches.
		_, err = f.db.Exec(`INSERT INTO usage_billing_dedup(request_id,api_key_id,request_fingerprint) VALUES($1,201,repeat('a',64))`, capture)
		require.NoError(t, err)
		status, body = batchImageHTTPRead(t, f.server, "101", path)
		require.Equal(t, 404, status, body)
		_, err = f.db.Exec(`WITH removed AS (DELETE FROM usage_billing_dedup WHERE request_id=$1) UPDATE usage_logs SET billing_request_fingerprint=NULL,billing_settled=FALSE,billed_cost=0 WHERE request_id=$1`, capture)
		require.NoError(t, err)
		_, err = f.settler.Settle(ctx, s.ID)
		require.NoError(t, err)
		status, body = batchImageHTTPRead(t, f.server, "101", path)
		require.Equal(t, 404, status, body)
		var fingerprint *string
		require.NoError(t, f.db.QueryRow(`SELECT billing_request_fingerprint FROM usage_logs WHERE id=$1`, *ref.UsageLogID).Scan(&fingerprint))
		require.Nil(t, fingerprint, "completed job must not reconstruct historical evidence")
	})
	t.Run("optional diagnostics failure does not block capture or usage persistence", func(t *testing.T) {
		s := f.submit(t, "")
		before := f.balance(t)
		_, err := f.db.Exec(`ALTER TABLE gateway_request_attempt_evidence RENAME TO fixture_unavailable_evidence`)
		require.NoError(t, err)
		_, err = f.settler.Settle(ctx, s.ID)
		require.NoError(t, err)
		f.requireCapture(t, s.ID, before+4)
		status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
		require.Equal(t, 200, status, body)
		require.Contains(t, body, `"attempt_evidence_unavailable":true`)
		require.Contains(t, body, `"billed_cost":6`)
		_, err = f.db.Exec(`ALTER TABLE fixture_unavailable_evidence RENAME TO gateway_request_attempt_evidence`)
		require.NoError(t, err)
	})
}

func (f *batchImageFixture) balance(t *testing.T) float64 {
	t.Helper()
	var balance float64
	require.NoError(t, f.db.QueryRow(`SELECT balance FROM users WHERE id=101`).Scan(&balance))
	return balance
}

func (f *batchImageFixture) requireCapture(t *testing.T, batchID string, balance float64) {
	t.Helper()
	require.Equal(t, balance, f.balance(t))
	var captures, proofs int
	var frozen float64
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM balance_transactions WHERE source_type='image_balance_capture' AND source_id=$1`, batchID).Scan(&captures))
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=201`, service.BatchImageCaptureRequestID(batchID)).Scan(&proofs))
	require.NoError(t, f.db.QueryRow(`SELECT frozen_balance FROM users WHERE id=101`).Scan(&frozen))
	require.Equal(t, 1, captures)
	require.Equal(t, 1, proofs)
	require.Zero(t, frozen)
}

func (f *batchImageFixture) requireNoUsage(t *testing.T, s batchImageSubmission) {
	t.Helper()
	var count int
	require.NoError(t, f.db.QueryRow(`SELECT count(*) FROM usage_logs WHERE request_id=$1`, service.BatchImageCaptureRequestID(s.ID)).Scan(&count))
	require.Zero(t, count)
	status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
	require.Equal(t, 200, status, body)
	require.Contains(t, body, `"billing":[]`)
}

func (f *batchImageFixture) reference(t *testing.T, requestID string) requestledger.BillingReference {
	t.Helper()
	require.NoError(t, f.ledger.ReconcileTaskBilling(context.Background()))
	status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+requestID)
	require.Equal(t, 200, status, body)
	var detail struct {
		Data struct {
			Billing []requestledger.BillingReference `json:"billing"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &detail))
	require.Len(t, detail.Data.Billing, 1, body)
	return detail.Data.Billing[0]
}
