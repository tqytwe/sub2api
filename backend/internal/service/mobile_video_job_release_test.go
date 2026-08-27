package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestMobileVideoJobReleaseArtifactClearsOnlyThePrivateDeliveryReference(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const (
		userID     int64 = 7
		taskID           = "f7d4d58c-7e1e-45ed-9737-a6febd4a30b3"
		storageKey       = "mobile-video-results/7/f7d4d58c-7e1e-45ed-9737-a6febd4a30b3.mp4"
	)
	now := time.Date(2026, time.August, 20, 3, 0, 0, 0, time.UTC)
	service := NewMobileVideoJobService(db)
	service.now = func() time.Time { return now }

	mock.ExpectQuery("SELECT task_id, user_id, group_id").
		WithArgs(userID, taskID).
		WillReturnRows(mobileVideoJobRows(taskID, userID, storageKey, now))
	mock.ExpectExec("UPDATE mobile_video_jobs").
		WithArgs(now, taskID, userID, storageKey).
		WillReturnResult(sqlmock.NewResult(0, 1))

	got, err := service.ReleaseArtifact(context.Background(), userID, taskID)
	require.NoError(t, err)
	require.Equal(t, storageKey, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobileVideoJobReleaseArtifactIsIdempotentAfterAnotherClientClearsIt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const (
		userID     int64 = 7
		taskID           = "c988f31d-89a6-4e4f-9dbd-301ad5521064"
		storageKey       = "mobile-video-results/7/c988f31d-89a6-4e4f-9dbd-301ad5521064.mp4"
	)
	now := time.Date(2026, time.August, 20, 3, 0, 0, 0, time.UTC)
	service := NewMobileVideoJobService(db)
	service.now = func() time.Time { return now }

	mock.ExpectQuery("SELECT task_id, user_id, group_id").
		WithArgs(userID, taskID).
		WillReturnRows(mobileVideoJobRows(taskID, userID, storageKey, now))
	mock.ExpectExec("UPDATE mobile_video_jobs").
		WithArgs(now, taskID, userID, storageKey).
		WillReturnResult(sqlmock.NewResult(0, 0))

	got, err := service.ReleaseArtifact(context.Background(), userID, taskID)
	require.NoError(t, err)
	require.Empty(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mobileVideoJobRows(taskID string, userID int64, storageKey string, now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"task_id", "user_id", "group_id", "execution_api_key_id", "model", "prompt", "resolution", "ratio",
		"duration_seconds", "generate_audio", "watermark", "reference_asset_ids", "provider", "provider_request_id",
		"provider_status", "state", "attempt_count", "next_poll_at", "lease_owner", "lease_expires_at",
		"artifact_storage_key", "artifact_url", "artifact_content_type", "artifact_byte_size", "last_error", "created_at", "updated_at",
	}).AddRow(
		taskID, userID, int64(9), int64(11), "video-model", "private prompt", "720p", "16:9",
		8, false, false, "[]", "grok", "provider-1", "completed", "completed", 1, now, nil, nil,
		storageKey, "/api/v1/mobile/video/jobs/"+taskID+"/content", "video/mp4", int64(12), nil, now, now,
	)
}
