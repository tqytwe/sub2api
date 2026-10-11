package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// VerifyBatchImageCaptureProof only controls new usage evidence annotations.
// It neither claims dedup keys nor changes the existing capture transaction.
// Hot and archived proof are read together in one statement snapshot.
func (r *usageBillingRepository) VerifyBatchImageCaptureProof(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (bool, error) {
	if r == nil || r.db == nil || cmd == nil || cmd.APIKeyID <= 0 || cmd.UserID <= 0 || cmd.RequestFingerprint == "" ||
		cmd.BatchID == "" || cmd.RequestID != service.BatchImageCaptureRequestID(cmd.BatchID) {
		return false, nil
	}
	var verified bool
	err := r.db.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM batch_image_jobs WHERE batch_id=$4 AND user_id=$5 AND api_key_id=$2)
 AND (SELECT COUNT(*)>0 AND BOOL_AND(proof.request_fingerprint=$3) FROM (
 SELECT request_fingerprint FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2
 UNION ALL SELECT request_fingerprint FROM usage_billing_dedup_archive WHERE request_id=$1 AND api_key_id=$2
 ) proof)`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint, cmd.BatchID, cmd.UserID).Scan(&verified)
	return verified, err
}

var _ service.BatchImageCaptureProofReader = (*usageBillingRepository)(nil)
