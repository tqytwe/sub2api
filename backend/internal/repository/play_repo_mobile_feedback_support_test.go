//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCreateUserMobileFeedbackMessageEnforcesOwnershipInAtomicQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	createdAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)WITH owned AS .*UPDATE mobile_feedback.*WHERE id = \$1 AND user_id = \$2 AND status <> 'ignored'.*INSERT INTO mobile_feedback_messages`).
		WithArgs(int64(77), int64(42), "补充网络信息").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feedback_id", "sender_type", "content", "created_at"}).
			AddRow(int64(9), int64(77), "user", "补充网络信息", createdAt))
	repo := NewPlayRepository(nil, db)

	message, err := repo.CreateUserMobileFeedbackMessage(context.Background(), 42, 77, "补充网络信息")

	require.NoError(t, err)
	require.Equal(t, int64(9), message.ID)
	require.Equal(t, int64(77), message.FeedbackID)
	require.Equal(t, "user", message.SenderType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUserMobileFeedbackMessageRejectsClosedOrUnownedTicket(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery(`(?s)WITH owned AS .*WHERE id = \$1 AND user_id = \$2 AND status <> 'ignored'`).
		WithArgs(int64(77), int64(99), "越权回复").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feedback_id", "sender_type", "content", "created_at"}))
	repo := NewPlayRepository(nil, db)

	_, err = repo.CreateUserMobileFeedbackMessage(context.Background(), 99, 77, "越权回复")

	require.ErrorIs(t, err, service.ErrMobileFeedbackClosed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAdminMobileFeedbackAppendsOnlyChangedReply(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	createdAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	columns := []string{"id", "user_id", "user_email", "user_name", "title", "category", "content", "status", "app_version", "installation_id", "app_channel", "install_referrer", "platform", "device_model", "android_version", "system_version", "group_name", "group_id", "backend_url", "last_error", "crash_log", "device_info", "screenshots", "admin_note", "version", "updated_by", "status_changed_at", "created_at", "updated_at"}
	mock.ExpectQuery(`(?s)UPDATE mobile_feedback`).
		WithArgs(int64(77), "viewed", "正在处理", int64(4), int64(99)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int64(77), int64(42), "", "", "网络问题", "bug", "无法连接", "viewed", "2.0.36", "install-77", "google-play", "utm_source=x", "android", "Redmi", "13", "", "", int64(0), "", "", "", []byte(`{}`), []byte(`[]`), "正在处理", int64(5), int64(99), createdAt, createdAt, createdAt,
		))
	mock.ExpectExec(`(?s)INSERT INTO mobile_feedback_messages .*WHERE NOT EXISTS`).
		WithArgs(int64(77), "正在处理").
		WillReturnResult(sqlmock.NewResult(0, 1))
	repo := NewPlayRepository(nil, db)

	record, err := repo.UpdateAdminMobileFeedback(context.Background(), 77, service.MobileFeedbackStatusUpdate{Status: "viewed", AdminNote: "正在处理", ExpectedVersion: 4, ActorAdminID: 99})

	require.NoError(t, err)
	require.Equal(t, "正在处理", record.AdminNote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAdminMobileFeedbackStillSavesWhenReplyAppendFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	createdAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	columns := []string{"id", "user_id", "user_email", "user_name", "title", "category", "content", "status", "app_version", "installation_id", "app_channel", "install_referrer", "platform", "device_model", "android_version", "system_version", "group_name", "group_id", "backend_url", "last_error", "crash_log", "device_info", "screenshots", "admin_note", "version", "updated_by", "status_changed_at", "created_at", "updated_at"}
	mock.ExpectQuery(`(?s)UPDATE mobile_feedback`).
		WithArgs(int64(77), "handled", "已处理", int64(4), int64(99)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int64(77), int64(42), "", "", "网络问题", "bug", "无法连接", "handled", "2.0.36", "install-77", "google-play", "utm_source=x", "android", "Redmi", "13", "", "", int64(0), "", "", "", []byte(`{}`), []byte(`[]`), "已处理", int64(5), int64(99), createdAt, createdAt, createdAt,
		))
	mock.ExpectExec(`(?s)INSERT INTO mobile_feedback_messages .*WHERE NOT EXISTS`).
		WithArgs(int64(77), "已处理").
		WillReturnError(errors.New("messages table unavailable"))
	repo := NewPlayRepository(nil, db)

	record, err := repo.UpdateAdminMobileFeedback(context.Background(), 77, service.MobileFeedbackStatusUpdate{Status: "handled", AdminNote: "已处理", ExpectedVersion: 4, ActorAdminID: 99})

	require.NoError(t, err)
	require.Equal(t, "handled", record.Status)
	require.Equal(t, "已处理", record.AdminNote)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateAdminMobileFeedbackRejectsStaleVersionWithoutAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery(`(?s)UPDATE mobile_feedback`).
		WithArgs(int64(77), "handled", "已处理", int64(3), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	repo := NewPlayRepository(nil, db)

	_, err = repo.UpdateAdminMobileFeedback(context.Background(), 77, service.MobileFeedbackStatusUpdate{Status: "handled", AdminNote: "已处理", ExpectedVersion: 3, ActorAdminID: 99})

	require.ErrorIs(t, err, service.ErrMobileFeedbackVersionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
