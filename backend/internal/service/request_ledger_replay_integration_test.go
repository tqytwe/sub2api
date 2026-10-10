//go:build integration

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

type ledgerReplayBatchRepo struct {
	BatchImageRepository
	job *BatchImageJob
}

func (r *ledgerReplayBatchRepo) GetBatchImageJobByIdempotencyKey(context.Context, int64, int64, string) (*BatchImageJob, error) {
	return r.job, nil
}

func TestRequestLedgerIdempotentTaskReplayKeepsCurrentAdmission(t *testing.T) {
	db := ledgertest.New(t)
	ledger := requestledger.New(db)
	keyID := int64(201)
	for _, kind := range []string{"image_studio", "image_batch"} {
		t.Run(kind, func(t *testing.T) {
			h, err := ledger.Begin(context.Background(), "/v1/images/batches", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			taskID := kind + "-existing-task"
			if kind == "image_studio" {
				job := &ImageStudioJob{ID: taskID, UserID: 101, APIKeyID: &keyID, IdempotencyKeyHash: "synthetic-replay", IdempotencyFingerprint: "synthetic-fingerprint"}
				svc := &ImageStudioService{repo: &imageStudioIdempotentReplayRepoStub{job: job}}
				got, _, err := svc.CreatePendingJob(ctx, 101, ImageStudioGenerateRequest{IdempotencyKeyHash: job.IdempotencyKeyHash, IdempotencyFingerprint: job.IdempotencyFingerprint})
				require.NoError(t, err)
				require.Equal(t, taskID, got.ID)
			} else {
				fingerprint := "synthetic-fingerprint"
				svc := &BatchImagePublicService{Repo: &ledgerReplayBatchRepo{job: &BatchImageJob{BatchID: taskID, UserID: 101, APIKeyID: &keyID, RequestHash: &fingerprint}}}
				_, found, err := svc.findIdempotentBatch(ctx, BatchImageOwner{UserID: 101, APIKeyID: keyID}, "synthetic-replay", fingerprint)
				require.NoError(t, err)
				require.True(t, found)
			}
			var refs int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_tasks WHERE request_id=$1 AND task_kind=$2 AND task_ref=$3 AND user_id=101 AND api_key_id=201`, h.ID, kind, taskID).Scan(&refs))
			require.Equal(t, 1, refs, "a successful replay must still attribute this admission to the actual task")
		})
	}
}
