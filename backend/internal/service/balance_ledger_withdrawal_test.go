package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestBalanceLedgerWithdrawalSubmitSkipsAutomaticEntitlementConsumption(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	createdAt := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	svc := &BalanceLedgerService{db: db, now: func() time.Time { return createdAt }}

	mock.ExpectBegin()
	mock.ExpectQuery(balanceLedgerSelectByKeyPattern()).
		WithArgs(int64(42), "withdrawal_submit:wd_1").
		WillReturnRows(balanceTransactionRows())
	mock.ExpectQuery("(?s)FROM users\\s+WHERE id = \\$1 AND deleted_at IS NULL\\s+FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(balanceLedgerUserStateRows().AddRow("100.00000000", "2.00000000", "100.00000000", "0.00000000"))
	expectWithdrawableSums(mock, 42, createdAt, "100.00000000", "0.00000000", "100.00000000")
	mock.ExpectExec("(?s)UPDATE users\\s+SET balance = \\$1,\\s+frozen_balance = \\$2,\\s+withdrawable_balance = \\$3,\\s+withdrawal_frozen_balance = \\$4").
		WithArgs("80.00000000", "2.00000000", "80.00000000", "20.00000000", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)INSERT INTO balance_transactions").
		WillReturnRows(balanceTransactionRows().AddRow(
			int64(9301), int64(42), -20.0, 100.0, 80.0, 0.0, 2.0, 2.0,
			-20.0, 100.0, 80.0, 20.0, 0.0, 20.0,
			WithdrawalLedgerSourceSubmit, "wd_1", "withdrawal_submit:wd_1", "user", int64(42),
			"withdrawal request submitted", `{}`, false, "high", createdAt,
		))
	mock.ExpectCommit()

	got, err := svc.ApplyDelta(context.Background(), BalanceLedgerApplyInput{
		UserID:                             42,
		BalanceDelta:                       -20,
		WithdrawableDelta:                  -20,
		WithdrawalFrozenDelta:              20,
		SourceType:                         WithdrawalLedgerSourceSubmit,
		SourceID:                           "wd_1",
		IdempotencyKey:                     "withdrawal_submit:wd_1",
		ActorType:                          BalanceLedgerActorUser,
		ActorUserID:                        int64Ptr(42),
		Description:                        "withdrawal request submitted",
		SkipWithdrawableEntitlementEffects: true,
		SkipFundBatchEffects:               true,
	})
	require.NoError(t, err)
	require.Equal(t, -20.0, got.WithdrawableDelta)
	require.Equal(t, 20.0, got.WithdrawalFrozenDelta)
	require.NoError(t, mock.ExpectationsWereMet())
}
