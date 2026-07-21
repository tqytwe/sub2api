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
		WillReturnRows(balanceLedgerUserStateRows().AddRow("7.00000000", "1.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("7.50000000", "1.25000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WithArgs(
			int64(42),
			"0.50000000",
			"7.00000000",
			"7.50000000",
			"0.25000000",
			"1.00000000",
			"1.25000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
			"0.00000000",
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
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
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

func TestBalanceLedgerApplyDeltaInvalidatesWhenOnlyWithdrawableChanges(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	auth := &balanceLedgerAuthInvalidatorStub{}
	cache := &balanceLedgerCacheInvalidatorStub{}
	createdAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{
		db:                      db,
		authCacheInvalidator:    auth,
		balanceCacheInvalidator: cache,
		now:                     func() time.Time { return createdAt },
	}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "withdrawable:42:manual-sync").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("10.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("10.00000000", "0.00000000", "1.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9003), int64(42), 0.0, 10.0, 10.0, 0.0, 0.0, 0.0,
			1.0, 0.0, 1.0, 0.0, 0.0, 0.0,
			"withdrawable_adjustment", "manual-sync", "withdrawable:42:manual-sync", "system", nil,
			"可提现权益同步", `{}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:             42,
		WithdrawableDelta:  1,
		SourceType:         "withdrawable_adjustment",
		SourceID:           "manual-sync",
		IdempotencyKey:     "withdrawable:42:manual-sync",
		ActorType:          BalanceLedgerActorSystem,
		Description:        "可提现权益同步",
		WithdrawablePolicy: BalanceLedgerPolicyRejectNegative,
	})
	require.NoError(t, err)
	require.Equal(t, int64(9003), got.ID)
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
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
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
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
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
		WillReturnRows(balanceLedgerUserStateRows().AddRow("0.25000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC), "0.00000000", "0.00000000", "0.00000000")
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

func TestBalanceLedgerAllowOverdraftDoesNotFailWithdrawableInvariant(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "usage:42:req-overdraft").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("0.25000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "available_at"}))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("-0.25000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9003), int64(42), -0.5, 0.25, -0.25, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			"usage_charge", "req-overdraft", "usage:42:req-overdraft", "system", nil,
			"API 消耗扣费", `{}`, false, "high", createdAt,
		))
	expectEmptyFundBatchConsumption(mock, 42)
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   -0.5,
		SourceType:     "usage_charge",
		SourceID:       "req-overdraft",
		IdempotencyKey: "usage:42:req-overdraft",
		BalancePolicy:  BalanceLedgerPolicyAllowOverdraft,
		Description:    "API 消耗扣费",
	})
	require.NoError(t, err)
	require.Equal(t, -0.25, *got.BalanceAfter)
	require.Equal(t, 0.0, *got.WithdrawableAfter)
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
		"withdrawable_delta",
		"withdrawable_before",
		"withdrawable_after",
		"withdrawal_frozen_delta",
		"withdrawal_frozen_before",
		"withdrawal_frozen_after",
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

func balanceLedgerUserStateRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"balance",
		"frozen_balance",
		"withdrawable_balance",
		"withdrawal_frozen_balance",
	})
}

func expectWithdrawableSums(mock sqlmock.Sqlmock, userID int64, asOf time.Time, mature string, pending string, total string) {
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1").
		WithArgs(userID, asOf).
		WillReturnRows(sqlmock.NewRows([]string{"mature_amount", "pending_amount", "total_amount"}).AddRow(mature, pending, total))
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
		WillReturnRows(balanceLedgerUserStateRows().AddRow("7.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("8.00000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9002), int64(42), 1.0, 7.0, 8.0, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			"admin_balance", "1", "admin:42:1", "admin", nil,
			"管理员增加余额", `{}`, false, "high", createdAt,
		))
	expectFundBatchGrant(mock, 42, 9002, "ops_gift", "admin_balance", "1", "1.00000000", false, createdAt)
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

func TestBalanceLedgerGrantArenaDailyCreatesPendingWithdrawableEntitlement(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	availableAt := createdAt.Add(72 * time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "arena_daily_settlement:77:42").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("10.00000000", "0.00000000", "1.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "1.00000000", "0.00000000", "1.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("12.00000000", "0.00000000", "1.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9201), int64(42), 2.0, 10.0, 12.0, 0.0, 0.0, 0.0,
			0.0, 1.0, 1.0, 0.0, 0.0, 0.0,
			PlayRewardSourceArenaDaily, "arena_daily_settlement:77:42", "arena_daily_settlement:77:42", "system", nil,
			"日榜竞技场结算", `{}`, false, "high", createdAt,
		))
	mock.ExpectQuery("(?s)INSERT INTO withdrawable_entitlements").
		WithArgs(
			int64(42),
			int64(9201),
			PlayRewardSourceArenaDaily,
			"arena_daily_settlement:77:42",
			"2.00000000",
			availableAt,
			createdAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(3001)))
	mock.ExpectExec("(?s)INSERT INTO withdrawable_entitlement_allocations").
		WithArgs(int64(42), int64(3001), int64(9201), "grant", "2.00000000", availableAt, PlayRewardSourceArenaDaily, "arena_daily_settlement:77:42", sqlmock.AnyArg(), createdAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   2,
		SourceType:     PlayRewardSourceArenaDaily,
		SourceID:       "arena_daily_settlement:77:42",
		IdempotencyKey: "arena_daily_settlement:77:42",
		Description:    "日榜竞技场结算",
	})
	require.NoError(t, err)
	require.NotNil(t, got.WithdrawableAfter)
	require.Equal(t, 1.0, *got.WithdrawableAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerImageHoldConsumesEntitlementsFIFOWithoutWithdrawalFrozen(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	matureAt := createdAt.Add(-time.Hour)
	pendingAt := createdAt.Add(time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "image_balance_hold:7:req-hold").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("10.00000000", "0.00000000", "5.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "5.00000000", "4.00000000", "9.00000000")
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "available_at"}).
			AddRow(int64(5001), "4.00000000", matureAt).
			AddRow(int64(5002), "5.00000000", pendingAt))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("2.00000000", "8.00000000", "1.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9202), int64(42), -8.0, 10.0, 2.0, 8.0, 0.0, 8.0,
			-4.0, 5.0, 1.0, 0.0, 0.0, 0.0,
			"image_balance_hold", "batch-1", "image_balance_hold:7:req-hold", "system", nil,
			"图片余额预留", `{}`, false, "high", createdAt,
		))
	expectConsumeAllocation(mock, 42, 9202, 5001, "4.00000000", matureAt, "image_balance_hold", "batch-1", createdAt)
	expectConsumeAllocation(mock, 42, 9202, 5002, "3.00000000", pendingAt, "image_balance_hold", "batch-1", createdAt)
	expectEmptyFundBatchConsumption(mock, 42)
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   -8,
		FrozenDelta:    8,
		SourceType:     "image_balance_hold",
		SourceID:       "batch-1",
		IdempotencyKey: "image_balance_hold:7:req-hold",
		Description:    "图片余额预留",
	})
	require.NoError(t, err)
	require.Equal(t, 8.0, *got.FrozenAfter)
	require.Equal(t, 0.0, *got.WithdrawalFrozenAfter)
	require.Equal(t, -4.0, got.WithdrawableDelta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerReleaseRestoresOriginalConsumedEntitlementBatches(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}
	matureAt := createdAt.Add(-2 * time.Hour)
	pendingAt := createdAt.Add(time.Hour)
	restoreKey := "image_balance_hold:7:req-hold"

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "image_balance_release:7:req-release").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("2.00000000", "8.00000000", "1.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "1.00000000", "2.00000000", "3.00000000")
	mock.ExpectQuery("(?s)WITH consumed AS").
		WithArgs(int64(42), restoreKey).
		WillReturnRows(sqlmock.NewRows([]string{"entitlement_id", "restorable_amount", "available_at"}).
			AddRow(int64(5001), "4.00000000", matureAt).
			AddRow(int64(5002), "3.00000000", pendingAt))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("7.00000000", "3.00000000", "5.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9203), int64(42), 5.0, 2.0, 7.0, -5.0, 8.0, 3.0,
			4.0, 1.0, 5.0, 0.0, 0.0, 0.0,
			"image_balance_release", "batch-1", "image_balance_release:7:req-release", "system", nil,
			"图片预留释放", `{}`, false, "high", createdAt,
		))
	expectRestoreAllocation(mock, 42, 9203, 5001, "4.00000000", matureAt, "image_balance_release", "batch-1", restoreKey, createdAt)
	expectRestoreAllocation(mock, 42, 9203, 5002, "1.00000000", pendingAt, "image_balance_release", "batch-1", restoreKey, createdAt)
	expectEmptyFundBatchRestore(mock, 42, restoreKey)
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   5,
		FrozenDelta:    -5,
		SourceType:     "image_balance_release",
		SourceID:       "batch-1",
		IdempotencyKey: "image_balance_release:7:req-release",
		Description:    "图片预留释放",
		Metadata:       map[string]any{"restore_ledger_key": restoreKey},
	})
	require.NoError(t, err)
	require.Equal(t, 4.0, got.WithdrawableDelta)
	require.Equal(t, 0.0, *got.WithdrawalFrozenAfter)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBalanceLedgerPaymentRefundConsumesRechargeBeforeGiftFunds(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 22, 9, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "payment_refund:77:deduct").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("20.00000000", "0.00000000", "0.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "0.00000000", "0.00000000", "0.00000000")
	mock.ExpectQuery("(?s)FROM withdrawable_entitlements\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_amount", "available_at"}))
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("8.00000000", "0.00000000", "0.00000000", "0.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9401), int64(42), -12.0, 20.0, 8.0, 0.0, 0.0, 0.0,
			0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
			BalanceFlowTypeRefund, "77", "payment_refund:77:deduct", "admin", nil,
			"退款扣回", `{"order_id":77}`, false, "high", createdAt,
		))
	mock.ExpectQuery("(?s)FROM balance_fund_batches\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(int64(42), int64(77), true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source_kind", "remaining_amount"}).
			AddRow(int64(7002), FundSourceKindOnlineRecharge, "10.00000000").
			AddRow(int64(7001), FundSourceKindOpsGift, "30.00000000"))
	expectFundBatchConsumption(mock, 42, 9401, 7002, "10.00000000", BalanceFlowTypeRefund, "77", createdAt)
	expectFundBatchConsumption(mock, 42, 9401, 7001, "2.00000000", BalanceFlowTypeRefund, "77", createdAt)
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:         42,
		BalanceDelta:   -12,
		SourceType:     BalanceFlowTypeRefund,
		SourceID:       "77",
		IdempotencyKey: "payment_refund:77:deduct",
		ActorType:      BalanceLedgerActorAdmin,
		Description:    "退款扣回",
		Metadata:       map[string]any{"order_id": int64(77)},
	})
	require.NoError(t, err)
	require.Equal(t, -12.0, got.BalanceDelta)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectConsumeAllocation(mock sqlmock.Sqlmock, userID int64, transactionID int64, entitlementID int64, amount string, availableAt time.Time, sourceType string, sourceID string, createdAt time.Time) {
	mock.ExpectExec("(?s)UPDATE withdrawable_entitlements\\s+SET remaining_amount = remaining_amount -").
		WithArgs(amount, entitlementID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO withdrawable_entitlement_allocations").
		WithArgs(userID, entitlementID, transactionID, "consume", amount, availableAt, sourceType, sourceID, sqlmock.AnyArg(), createdAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectRestoreAllocation(mock sqlmock.Sqlmock, userID int64, transactionID int64, entitlementID int64, amount string, availableAt time.Time, sourceType string, sourceID string, restoreKey string, createdAt time.Time) {
	mock.ExpectExec("(?s)UPDATE withdrawable_entitlements\\s+SET remaining_amount = remaining_amount \\+").
		WithArgs(amount, entitlementID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO withdrawable_entitlement_allocations").
		WithArgs(userID, entitlementID, transactionID, "restore", amount, availableAt, sourceType, sourceID, sqlmock.AnyArg(), createdAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectEmptyFundBatchConsumption(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery("(?s)FROM balance_fund_batches\\s+WHERE user_id = \\$1\\s+AND status = 'active'").
		WithArgs(userID, int64(0), false).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source_kind", "remaining_amount"}))
}

func expectEmptyFundBatchRestore(mock sqlmock.Sqlmock, userID int64, restoreKey string) {
	mock.ExpectQuery("(?s)WITH consumed AS \\(\\s+SELECT bfa.batch_id").
		WithArgs(userID, restoreKey).
		WillReturnRows(sqlmock.NewRows([]string{"batch_id", "restorable_amount"}))
}

func expectFundBatchGrant(mock sqlmock.Sqlmock, userID int64, transactionID int64, kind string, sourceType string, sourceID string, amount string, refundable bool, createdAt time.Time) {
	mock.ExpectQuery("(?s)INSERT INTO balance_fund_batches").
		WithArgs(userID, transactionID, nil, kind, sourceType, sourceID, amount, refundable, createdAt, sqlmock.AnyArg(), createdAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7001)))
	mock.ExpectExec("(?s)INSERT INTO balance_fund_allocations").
		WithArgs(userID, int64(7001), transactionID, "grant", amount, sourceType, sourceID, sqlmock.AnyArg(), createdAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func expectFundBatchConsumption(mock sqlmock.Sqlmock, userID int64, transactionID int64, batchID int64, amount string, sourceType string, sourceID string, createdAt time.Time) {
	mock.ExpectExec("(?s)UPDATE balance_fund_batches\\s+SET remaining_amount = remaining_amount -").
		WithArgs(amount, batchID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO balance_fund_allocations").
		WithArgs(userID, batchID, transactionID, "consume", amount, sourceType, sourceID, sqlmock.AnyArg(), createdAt).
		WillReturnResult(sqlmock.NewResult(1, 1))
}
