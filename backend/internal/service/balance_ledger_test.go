package service

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type balanceLedgerAuthInvalidatorStub struct {
	userIDs []int64
}

func (s *balanceLedgerAuthInvalidatorStub) InvalidateAuthCacheByKey(context.Context, string) {}

func (s *balanceLedgerAuthInvalidatorStub) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	s.userIDs = append(s.userIDs, userID)
}

func (s *balanceLedgerAuthInvalidatorStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

type balanceLedgerCacheInvalidatorStub struct {
	userIDs []int64
	err     error
}

func (s *balanceLedgerCacheInvalidatorStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.userIDs = append(s.userIDs, userID)
	return s.err
}

func TestBalanceLedgerApplyDeltaCommitsLedgerAndInvalidatesAfterCommit(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	auth := &balanceLedgerAuthInvalidatorStub{}
	cache := &balanceLedgerCacheInvalidatorStub{}
	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{
		db:                      db,
		authCacheInvalidator:    auth,
		balanceCacheInvalidator: cache,
		now:                     func() time.Time { return createdAt },
	}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "checkin:42:2026-07-19").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.0, 1.0))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1, frozen_balance = \\$2").
		WithArgs(7.5, 1.25, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WithArgs(
			int64(42),
			0.5,
			7.0,
			7.5,
			0.25,
			1.0,
			1.25,
			"checkin",
			"2026-07-19",
			"checkin:42:2026-07-19",
			"system",
			nil,
			"签到奖励",
			sqlmock.AnyArg(),
			false,
			"high",
			createdAt,
		).
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9001), int64(42), 0.5, 7.0, 7.5, 0.25, 1.0, 1.25,
			"checkin", "2026-07-19", "checkin:42:2026-07-19", "system", nil,
			"签到奖励", `{"checkin_date":"2026-07-19"}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   0.5,
		FrozenDelta:    0.25,
		SourceType:     "checkin",
		SourceID:       "2026-07-19",
		IdempotencyKey: "checkin:42:2026-07-19",
		Description:    "签到奖励",
		Metadata:       map[string]any{"checkin_date": "2026-07-19"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(9001), got.ID)
	require.Equal(t, 7.5, *got.BalanceAfter)
	require.Equal(t, 1.25, *got.FrozenAfter)
	require.Equal(t, []int64{42}, auth.userIDs)
	require.Equal(t, []int64{42}, cache.userIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerApplyDeltaReplaysExistingIdempotencyKey(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "quiz:42:2026-07-19").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(8), int64(42), 0.5, 1.0, 1.5, 0.0, 0.0, 0.0,
			"quiz", "attempt-77", "quiz:42:2026-07-19", "system", nil,
			"答题奖励", `{}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   0.5,
		SourceType:     "quiz",
		SourceID:       "attempt-77",
		IdempotencyKey: "quiz:42:2026-07-19",
	})
	require.NoError(t, err)
	require.Equal(t, int64(8), got.ID)
	require.Equal(t, "quiz", got.SourceType)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerApplyDeltaRejectsConflictingIdempotencyKey(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "same-key").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(8), int64(42), 1.0, 1.0, 2.0, 0.0, 0.0, 0.0,
			"checkin", "2026-07-19", "same-key", "system", nil,
			"签到奖励", `{}`, false, "high", createdAt,
		))
	mock.ExpectRollback()

	_, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   1,
		SourceType:     "quiz",
		SourceID:       "attempt-88",
		IdempotencyKey: "same-key",
	})
	require.ErrorIs(t, err, ErrBalanceLedgerIdempotencyConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerApplyDeltaRejectsNegativeBalanceUnlessPolicyAllowsIt(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	svc := &BalanceLedgerService{db: db, now: func() time.Time { return time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC) }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "usage:42:req-1").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(0.25, 0.0))
	mock.ExpectRollback()

	_, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   -0.5,
		SourceType:     "usage_charge",
		SourceID:       "req-1",
		IdempotencyKey: "usage:42:req-1",
	})
	require.ErrorIs(t, err, ErrBalanceLedgerInsufficientBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerPolicyClampZeroUsesActualDelta(t *testing.T) {
	t.Parallel()

	after, delta, err := applyBalanceLedgerPolicy(0.25, -0.5, BalanceLedgerPolicyClampZero)
	require.NoError(t, err)
	require.Equal(t, 0.0, after)
	require.Equal(t, -0.25, delta)

	after, delta, err = applyBalanceLedgerPolicy(0.25, -0.5, BalanceLedgerPolicyAllowOverdraft)
	require.NoError(t, err)
	require.Equal(t, -0.25, after)
	require.Equal(t, -0.5, delta)
}

func newBalanceLedgerSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	return db, mock
}

func balanceLedgerSelectByKeyPattern() string {
	return regexp.QuoteMeta("FROM balance_transactions") + "(?s:.*)" + regexp.QuoteMeta("WHERE user_id = $1 AND idempotency_key = $2")
}

func balanceTransactionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id",
		"user_id",
		"balance_delta",
		"balance_before",
		"balance_after",
		"frozen_delta",
		"frozen_before",
		"frozen_after",
		"source_type",
		"source_id",
		"idempotency_key",
		"actor_type",
		"actor_user_id",
		"description",
		"metadata",
		"is_backfilled",
		"confidence",
		"created_at",
	})
}

func TestBalanceLedgerApplyDeltaKeepsCommitWhenCacheInvalidationFails(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	cache := &balanceLedgerCacheInvalidatorStub{err: errors.New("redis unavailable")}
	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{
		db:                      db,
		balanceCacheInvalidator: cache,
		now:                     func() time.Time { return createdAt },
	}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "admin:42:1").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.0, 0.0))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1, frozen_balance = \\$2").
		WithArgs(8.0, 0.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9002), int64(42), 1.0, 7.0, 8.0, 0.0, 0.0, 0.0,
			"admin_balance", "1", "admin:42:1", "admin", nil,
			"管理员增加余额", `{}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   1,
		SourceType:     "admin_balance",
		SourceID:       "1",
		IdempotencyKey: "admin:42:1",
		ActorType:      BalanceLedgerActorAdmin,
		Description:    "管理员增加余额",
	})
	require.NoError(t, err)
	require.Equal(t, int64(9002), got.ID)
	require.Equal(t, []int64{42}, cache.userIDs)
	require.NoError(t, mock.ExpectationsWereMet())
}
