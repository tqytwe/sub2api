package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestManageModerationUserUnbanIsVersionedAndTransactional(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, role, moderation_status_revision FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"status", "role", "revision"}).AddRow("disabled", "user", int64(8)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version,generation,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt,owned_status_revision FROM content_moderation_user_state WHERE user_id=$1 FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_status_revision"}).AddRow(int64(3), int64(2), false, false, false, false, int64(8)))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE users SET status='active',updated_at=NOW() WHERE id=$1 AND status='disabled' AND moderation_status_revision=$2 RETURNING status")).WithArgs(int64(27), int64(8)).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE content_moderation_user_state SET version=version+1,generation=$2,auto_ban_exempt=$3,owned_status_revision=$4,reason=$5,actor_id=$6,keyword_exempt=$7,api_audit_exempt=$8,email_exempt=$9,updated_at=NOW() WHERE user_id=$1 AND version=$10")).WithArgs(int64(27), int64(2), false, nil, "reviewed", int64(4), false, false, false, int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO content_moderation_user_actions(user_id,actor_id,action,reason,version,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)")).WithArgs(int64(27), int64(4), "unban", "reviewed", int64(4), false, false, false, false).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = repo.ManageModerationUser(context.Background(), 27, service.ModerationUserAction{
		Action: "unban", Version: 3, Reason: "reviewed", Confirm: true, ActorID: 4,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManageModerationUserRejectsUnownedDisableBeforeStatusWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, role, moderation_status_revision FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"status", "role", "revision"}).AddRow("disabled", "user", int64(8)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version,generation,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt,owned_status_revision FROM content_moderation_user_state WHERE user_id=$1 FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_status_revision"}).AddRow(int64(3), int64(0), false, false, false, false, nil))
	mock.ExpectRollback()

	err = repo.ManageModerationUser(context.Background(), 27, service.ModerationUserAction{
		Action: "unban", Version: 3, Reason: "reviewed", Confirm: true, ActorID: 4,
	})

	require.ErrorContains(t, err, "MODERATION_DISABLE_NOT_OWNED")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListModerationUsersIncludesLegacyHistoryWithoutRecoveryState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	countQuery := `SELECT COUNT\(\*\) FROM users u LEFT JOIN content_moderation_user_state s ON s.user_id=u.id WHERE .*u.deleted_at IS NULL.*content_moderation_user_hits.*generation=0.*`
	listQuery := `SELECT u\.id, COALESCE\(s\.version,0\).*FROM users u LEFT JOIN content_moderation_user_state s ON s\.user_id=u\.id WHERE .*u.deleted_at IS NULL.*content_moderation_user_hits.*generation=0.*ORDER BY .*LIMIT \$5 OFFSET \$6`
	for _, filter := range []string{"all", "enforced"} {
		mock.ExpectQuery(countQuery).WithArgs(since, true, filter, int64(0)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(listQuery).WithArgs(since, true, filter, int64(0), 20, 0).WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_disable", "status", "current_count", "reason", "actor_id", "updated_at",
		}).AddRow(int64(27), int64(0), int64(0), false, false, false, false, false, "disabled", 2, "", nil, nil))

		items, total, err := repo.ListModerationUsers(context.Background(), 1, 20, filter, 0, since, true)

		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, items, 1)
		require.Equal(t, int64(27), items[0].UserID)
		require.Zero(t, items[0].Version)
		require.Zero(t, items[0].Generation)
		require.False(t, items[0].AutoBanExempt)
		require.False(t, items[0].KeywordExempt)
		require.False(t, items[0].APIAuditExempt)
		require.False(t, items[0].EmailExempt)
		require.False(t, items[0].OwnedDisable, "legacy disabled state must not be attributed to moderation")
		require.Equal(t, 2, items[0].CurrentCount)
		require.Nil(t, items[0].UpdatedAt)
	}

	// An old hit without recovery state has no recorded exemptions and must not
	// leak into exemption-specific lists.
	mock.ExpectQuery(countQuery).WithArgs(since, true, "exempt", int64(0)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(listQuery).WithArgs(since, true, "exempt", int64(0), 20, 0).WillReturnRows(sqlmock.NewRows([]string{
		"user_id", "version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_disable", "status", "current_count", "reason", "actor_id", "updated_at",
	}))
	items, total, err := repo.ListModerationUsers(context.Background(), 1, 20, "exempt", 0, since, true)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetModerationUserReadsLegacyHistoryWithoutCreatingRecoveryState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewContentModerationRepository(db).(*contentModerationRepository)
	since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT u\.id, COALESCE\(s\.version,0\).*FROM users u LEFT JOIN content_moderation_user_state s ON s\.user_id=u\.id WHERE u\.id=\$3 AND u\.deleted_at IS NULL`).
		WithArgs(since, true, int64(27)).WillReturnRows(sqlmock.NewRows([]string{
		"user_id", "version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_disable", "status", "current_count", "reason", "actor_id", "updated_at",
	}).AddRow(int64(27), int64(0), int64(0), false, false, false, false, false, "disabled", 1, "", nil, nil))

	state, err := repo.GetModerationUser(context.Background(), 27, since, true)

	require.NoError(t, err)
	require.Equal(t, int64(27), state.UserID)
	require.Equal(t, "disabled", state.Status)
	require.Zero(t, state.Version)
	require.Zero(t, state.Generation)
	require.False(t, state.OwnedDisable)
	require.Equal(t, 1, state.CurrentCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyModerationHitDuplicateRequestIsNoOp(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewContentModerationRepository(db).(*contentModerationRepository)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status, role, moderation_status_revision FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"status", "role", "revision"}).AddRow("active", "user", int64(2)))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO content_moderation_user_state(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING")).WithArgs(int64(27)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version,generation,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt,owned_status_revision FROM content_moderation_user_state WHERE user_id=$1 FOR UPDATE")).WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"version", "generation", "auto_ban_exempt", "keyword_exempt", "api_audit_exempt", "email_exempt", "owned_status_revision"}).AddRow(int64(4), int64(1), false, false, false, false, nil))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO content_moderation_user_hits(user_id,generation,request_id,action) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,generation,request_id) DO NOTHING")).WithArgs(int64(27), int64(1), "req-1", "block").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := repo.ApplyModerationHit(context.Background(), service.ModerationHit{
		UserID: 27, Version: 4, RequestID: "req-1", Action: "block", AutoBan: true, Threshold: 1,
	})

	require.NoError(t, err)
	require.False(t, result.Accepted)
	require.Zero(t, result.Count)
	require.False(t, result.BanApplied)
	require.NoError(t, mock.ExpectationsWereMet())
}
