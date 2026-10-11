//go:build integration

package acceptance

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBatchImageHTTPConflictingProofBeforeUsageRecovery(t *testing.T) {
	f := newBatchImageFixture(t)
	ctx := context.Background()
	s := f.submit(t, "")
	before := f.balance(t)
	_, err := f.db.Exec(`CREATE FUNCTION reject_batch_usage() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic usage persistence failure'; END $$;
 CREATE TRIGGER reject_batch_usage BEFORE INSERT ON usage_logs FOR EACH ROW EXECUTE FUNCTION reject_batch_usage()`)
	require.NoError(t, err)
	_, err = f.settler.Settle(ctx, s.ID)
	require.ErrorIs(t, err, service.ErrBatchImageSettlementBillingFailed)
	f.requireCapture(t, s.ID, before+4)
	f.requireNoUsage(t, s)
	_, err = f.db.Exec(`DROP TRIGGER reject_batch_usage ON usage_logs; DROP FUNCTION reject_batch_usage()`)
	require.NoError(t, err)
	_, err = f.db.Exec(`INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at)
 SELECT request_id,api_key_id,repeat('x',64),created_at FROM usage_billing_dedup WHERE request_id=$1`, service.BatchImageCaptureRequestID(s.ID))
	require.NoError(t, err)
	// The hot capture fingerprint still matches. Recovery must preserve the
	// already-applied charge and real image usage without certifying conflicting
	// retained evidence as a verified billed amount on a fresh usage INSERT.
	_, err = f.settler.Settle(ctx, s.ID)
	require.NoError(t, err)
	f.requireCapture(t, s.ID, before+4)
	var settled bool
	var billed, actual float64
	var images int
	require.NoError(t, f.db.QueryRow(`SELECT billing_settled,billed_cost,actual_cost,image_count FROM usage_logs WHERE request_id=$1`, service.BatchImageCaptureRequestID(s.ID)).Scan(&settled, &billed, &actual, &images))
	require.Equal(t, 6.0, actual)
	require.Equal(t, 3, images)
	t.Logf("recovered usage: settled=%t billed=%v actual=%v images=%d", settled, billed, actual, images)
	require.False(t, settled, "conflicting hot/archive proof must not certify a fresh recovered usage row")
	require.Zero(t, billed, "unverified billed_cost is not a zero-cost claim")
	status, body := batchImageHTTPRead(t, f.server, "101", "/api/v1/requests/"+s.RequestID)
	require.Equal(t, 200, status, body)
	require.Contains(t, body, `"billing":[]`)
}

// Fault injection changes only the new read-only evidence capability. Capture
// and usage still use the real PostgreSQL repositories.
type batchCaptureProofReadFailure struct {
	service.UsageBillingRepository
	db *sql.DB
}

func (r *batchCaptureProofReadFailure) VerifyBatchImageCaptureProof(ctx context.Context, _ *service.BatchImageBalanceHoldCommand) (bool, error) {
	var unused bool
	err := r.db.QueryRowContext(ctx, `SELECT unavailable_proof FROM fixture_missing_capture_evidence`).Scan(&unused)
	return false, err
}

func TestBatchImageHTTPCaptureProofReadFailureKeepsUsage(t *testing.T) {
	f := newBatchImageFixture(t)
	s := f.submit(t, "")
	before := f.balance(t)
	f.settler.BillingRepo = &batchCaptureProofReadFailure{UsageBillingRepository: f.billing, db: f.db}
	_, err := f.settler.Settle(context.Background(), s.ID)
	require.NoError(t, err)
	f.requireCapture(t, s.ID, before+4)
	var settled bool
	var billed, actual float64
	var images int
	require.NoError(t, f.db.QueryRow(`SELECT billing_settled,billed_cost,actual_cost,image_count FROM usage_logs WHERE request_id=$1`, service.BatchImageCaptureRequestID(s.ID)).Scan(&settled, &billed, &actual, &images))
	require.False(t, settled)
	require.Zero(t, billed)
	require.Equal(t, 6.0, actual)
	require.Equal(t, 3, images)
	job, err := f.jobs.GetBatchImageJobByBatchID(context.Background(), s.ID)
	require.NoError(t, err)
	require.Equal(t, service.BatchImageJobStatusCompleted, job.Status)
}
