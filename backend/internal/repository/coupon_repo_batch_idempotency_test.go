package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIssueCouponBatchRejectsReusedKeyForDifferentRequest(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		request service.CouponBatchIssueInput
	}{
		{
			name: "different recipients",
			request: service.CouponBatchIssueInput{
				TemplateID:     9,
				UserIDs:        []int64{7, 10},
				Source:         service.CouponIssueSourceAdminBatch,
				IdempotencyKey: "batch-key",
				Metadata:       map[string]any{"campaign": "summer"},
			},
		},
		{
			name: "different source",
			request: service.CouponBatchIssueInput{
				TemplateID:     9,
				UserIDs:        []int64{7, 8},
				Source:         service.CouponIssueSourceCompensate,
				IdempotencyKey: "batch-key",
				Metadata:       map[string]any{"campaign": "summer"},
			},
		},
		{
			name: "different metadata",
			request: service.CouponBatchIssueInput{
				TemplateID:     9,
				UserIDs:        []int64{7, 8},
				Source:         service.CouponIssueSourceAdminBatch,
				IdempotencyKey: "batch-key",
				Metadata:       map[string]any{"campaign": "autumn"},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db, mock := newCouponRepositoryMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(`(?s)FROM coupon_issue_batches b.*WHERE b\.idempotency_key = \$1.*FOR UPDATE OF b`).
				WithArgs("batch-key").
				WillReturnRows(couponIssueBatchMockRows(now))
			mock.ExpectRollback()

			repo := &couponRepository{db: db}
			_, err := repo.IssueCouponBatch(context.Background(), tc.request, now)

			require.Equal(t, "COUPON_BATCH_IDEMPOTENCY_CONFLICT", infraerrors.Reason(err))
		})
	}
}

func TestIssueCouponBatchReplaysMatchingRequest(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	db, mock := newCouponRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)FROM coupon_issue_batches b.*WHERE b\.idempotency_key = \$1.*FOR UPDATE OF b`).
		WithArgs("batch-key").
		WillReturnRows(couponIssueBatchMockRows(now))
	mock.ExpectCommit()

	repo := &couponRepository{db: db}
	batch, err := repo.IssueCouponBatch(context.Background(), service.CouponBatchIssueInput{
		TemplateID:     9,
		UserIDs:        []int64{7, 8},
		Source:         service.CouponIssueSourceAdminBatch,
		IdempotencyKey: "batch-key",
		Metadata:       map[string]any{"campaign": "summer"},
	}, now)

	require.NoError(t, err)
	require.NotNil(t, batch)
	require.Equal(t, int64(71), batch.ID)
}

func couponIssueBatchMockRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "template_id", "template_name", "source", "requested_count", "issued_count",
		"failed_count", "status", "idempotency_key", "input_snapshot", "created_by",
		"completed_at", "created_at",
	}).AddRow(
		int64(71), int64(9), "Recharge coupon", service.CouponIssueSourceAdminBatch, 2, 2,
		0, service.CouponIssueBatchStatusCompleted, "batch-key", []byte(`{"user_ids":[7,8],"metadata":{"campaign":"summer"}}`), nil,
		now, now,
	)
}
