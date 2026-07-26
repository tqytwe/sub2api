package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMobilePushRepositoryUpsertsDeviceAndRevokesDuplicateToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewMobilePushRepository(db, time.Minute, 8)
	now := time.Date(2026, time.July, 26, 10, 0, 0, 0, time.UTC)
	installationID := uuid.NewString()
	deviceID := uuid.NewString()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
		WithArgs("installation:" + installationID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
		WithArgs("token:hash").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mobile_devices")).
		WithArgs(now, "hash", int64(17), installationID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mobile_devices")).
		WithArgs(now, installationID, int64(17)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO mobile_devices")).
		WithArgs(int64(17), installationID, "android", "fcm", "ciphertext", "hash", "2.0.34", "zh-CN", now).
		WillReturnRows(sqlmock.NewRows(mobileDeviceColumnNames()).AddRow(
			deviceID, int64(17), installationID, "android", "fcm", "ciphertext", "hash",
			"2.0.34", "zh-CN", true, now, nil, now, now,
		))
	mock.ExpectCommit()

	device, err := repo.UpsertDevice(context.Background(), service.MobileDeviceWrite{
		UserID: 17, InstallationID: installationID, Platform: "android", PushProvider: "fcm",
		TokenCiphertext: "ciphertext", TokenHash: "hash", AppVersion: "2.0.34", Locale: "zh-CN", LastSeenAt: now,
	})

	require.NoError(t, err)
	require.Equal(t, deviceID, device.ID)
	require.Equal(t, int64(17), device.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobilePushRepositoryRevokeUsesUserAndInstallation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewMobilePushRepository(db, time.Minute, 8)
	now := time.Now().UTC()
	installationID := uuid.NewString()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE mobile_devices")).
		WithArgs(now, int64(23), installationID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	deleted, err := repo.RevokeDevice(context.Background(), 23, installationID, now)

	require.NoError(t, err)
	require.True(t, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobilePushRepositoryClaimUsesLeaseAndSkipLocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	lease := 90 * time.Second
	repo := NewMobilePushRepository(db, lease, 8)
	now := time.Now().UTC()
	mock.ExpectQuery(`(?s)FROM mobile_push_outbox.*FOR UPDATE SKIP LOCKED.*UPDATE mobile_push_outbox`).
		WithArgs(now, now.Add(lease)).
		WillReturnError(sql.ErrNoRows)

	item, err := repo.ClaimPendingPush(context.Background(), now)

	require.NoError(t, err)
	require.Nil(t, item)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobilePushRepositoryClaimReturnsOwnerToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	lease := 90 * time.Second
	repo := NewMobilePushRepository(db, lease, 8)
	now := time.Now().UTC()
	claimToken := uuid.NewString()
	mock.ExpectQuery(`(?s)status = 'processing'.*claim_token = gen_random_uuid\(\).*lease_expires_at`).
		WithArgs(now, now.Add(lease)).
		WillReturnRows(sqlmock.NewRows(mobilePushOutboxColumnNames()).AddRow(
			int64(7), int64(31), "dedupe", "task.completed", "task", "1", "任务完成", "任务已经完成",
			[]byte(`{"task_id":"1"}`), "processing", 0, nil, now, nil, claimToken, now.Add(lease), now, now,
		))

	item, err := repo.ClaimPendingPush(context.Background(), now)

	require.NoError(t, err)
	require.Equal(t, claimToken, item.ClaimToken)
	require.Equal(t, now.Add(lease), *item.LeaseExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobilePushRepositoryClaimCASRejectsStaleWorker(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewMobilePushRepository(db, time.Minute, 8)
	now := time.Now().UTC()
	claimToken := uuid.NewString()
	mock.ExpectExec(`(?s)UPDATE mobile_push_outbox.*claim_token = NULL.*WHERE id = \$1 AND status = 'processing' AND claim_token = \$6::uuid`).
		WithArgs(int64(7), 1, 8, "DELIVERY_FAILED", now, claimToken).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.RequeuePush(context.Background(), 7, claimToken, "DELIVERY_FAILED", now, true)

	require.ErrorIs(t, err, service.ErrMobilePushClaimLost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMobilePushRepositoryEnqueueReturnsExistingDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewMobilePushRepository(db, time.Minute, 8)
	now := time.Now().UTC()
	item := &service.MobilePushOutboxItem{
		UserID: 31, DedupeKeyHash: "dedupe", EventType: "task.completed", SourceType: "task", SourceID: "1",
		TitleZh: "任务完成", BodyZh: "任务已经完成", Data: map[string]string{"task_id": "1"}, AvailableAt: now,
	}
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO mobile_push_outbox")).
		WithArgs(item.UserID, item.DedupeKeyHash, item.EventType, item.SourceType, item.SourceID, item.TitleZh, item.BodyZh, sqlmock.AnyArg(), item.AvailableAt).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + mobilePushOutboxColumns + " FROM mobile_push_outbox WHERE dedupe_key_hash = $1")).
		WithArgs("dedupe").
		WillReturnRows(sqlmock.NewRows(mobilePushOutboxColumnNames()).AddRow(
			int64(7), int64(31), "dedupe", "task.completed", "task", "1", "任务完成", "任务已经完成",
			[]byte(`{"task_id":"1"}`), "pending", 0, nil, now, nil, nil, nil, now, now,
		))

	got, created, err := repo.EnqueuePush(context.Background(), item)

	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, int64(7), got.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func mobileDeviceColumnNames() []string {
	return []string{"id", "user_id", "installation_id", "platform", "push_provider", "token_ciphertext", "token_hash", "app_version", "locale", "enabled", "last_seen_at", "revoked_at", "created_at", "updated_at"}
}

func mobilePushOutboxColumnNames() []string {
	return []string{"id", "user_id", "dedupe_key_hash", "event_type", "source_type", "source_id", "title_zh", "body_zh", "data", "status", "attempts", "last_error_code", "available_at", "sent_at", "claim_token", "lease_expires_at", "created_at", "updated_at"}
}
