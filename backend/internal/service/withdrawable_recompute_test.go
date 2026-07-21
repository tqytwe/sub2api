package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestWithdrawableRecomputeDryRunMarksReadyFromHighConfidenceLedger(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	svc := &WithdrawableRecomputeService{db: db, now: func() time.Time { return now }}

	mock.ExpectQuery("(?s)SELECT id\\s+FROM users").
		WithArgs(int64(7), 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery("(?s)SELECT\\s+COALESCE\\(u.balance").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "entitlement_count"}).AddRow("8.00000000", int64(0)))
	mock.ExpectQuery("(?s)FROM balance_transactions\\s+WHERE user_id = \\$1").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"balance_delta",
			"balance_before",
			"balance_after",
			"source_type",
			"source_id",
			"idempotency_key",
			"metadata",
			"confidence",
			"created_at",
		}).
			AddRow(int64(101), "10.00000000", "0.00000000", "10.00000000", PlayRewardSourceTeamSharedReward, "team:2026-07", "team:7:2026-07", `{}`, BalanceLedgerConfidenceHigh, now.Add(-96*time.Hour)).
			AddRow(int64(102), "-2.00000000", "10.00000000", "8.00000000", "usage_charge", "req-1", "usage:7:req-1", `{}`, BalanceLedgerConfidenceHigh, now.Add(-2*time.Hour)))

	report, err := svc.Recompute(context.Background(), WithdrawableRecomputeOptions{UserID: 7})
	require.NoError(t, err)
	require.Equal(t, WithdrawableRecomputeModeDryRun, report.Mode)
	require.Equal(t, 1, report.ReadyUsers)
	require.Equal(t, 0, report.NeedsReviewUsers)
	require.Len(t, report.Users, 1)
	user := report.Users[0]
	require.Equal(t, WithdrawableRecomputeStatusReady, user.Status)
	require.Equal(t, "8.00000000", user.ComputedWithdrawableBalance.StringFixed(8))
	require.Equal(t, "0.00000000", user.ComputedPendingBalance.StringFixed(8))
	require.Equal(t, "8.00000000", user.ComputedEntitlementBalance.StringFixed(8))
	require.Len(t, user.Batches, 1)
	require.Equal(t, "8.00000000", user.Batches[0].RemainingAmount.StringFixed(8))
	require.Equal(t, "2.00000000", user.Batches[0].ConsumedAmount.StringFixed(8))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithdrawableRecomputeMarksNeedsReviewWhenSnapshotsAreMissing(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	svc := &WithdrawableRecomputeService{db: db, now: func() time.Time { return now }}

	mock.ExpectQuery("(?s)SELECT id\\s+FROM users").
		WithArgs(int64(7), 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery("(?s)SELECT\\s+COALESCE\\(u.balance").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "entitlement_count"}).AddRow("3.00000000", int64(0)))
	mock.ExpectQuery("(?s)FROM balance_transactions\\s+WHERE user_id = \\$1").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"balance_delta",
			"balance_before",
			"balance_after",
			"source_type",
			"source_id",
			"idempotency_key",
			"metadata",
			"confidence",
			"created_at",
		}).AddRow(int64(201), "-1.00000000", nil, nil, "usage_charge", "req-2", "usage:7:req-2", `{}`, BalanceLedgerConfidenceHigh, now.Add(-time.Hour)))

	report, err := svc.Recompute(context.Background(), WithdrawableRecomputeOptions{UserID: 7})
	require.NoError(t, err)
	require.Equal(t, 0, report.ReadyUsers)
	require.Equal(t, 1, report.NeedsReviewUsers)
	require.Contains(t, report.Users[0].Anomalies[0], "missing reliable balance_before")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithdrawableRecomputeClampsNegativeUserBalanceToZeroWithdrawable(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	svc := &WithdrawableRecomputeService{db: db, now: func() time.Time { return now }}

	mock.ExpectQuery("(?s)SELECT id\\s+FROM users").
		WithArgs(int64(7), 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery("(?s)SELECT\\s+COALESCE\\(u.balance").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "entitlement_count"}).AddRow("-1.00000000", int64(0)))
	mock.ExpectQuery("(?s)FROM balance_transactions\\s+WHERE user_id = \\$1").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"balance_delta",
			"balance_before",
			"balance_after",
			"source_type",
			"source_id",
			"idempotency_key",
			"metadata",
			"confidence",
			"created_at",
		}).AddRow(int64(301), "-1.00000000", "0.00000000", "-1.00000000", "usage_charge", "req-3", "usage:7:req-3", `{}`, BalanceLedgerConfidenceHigh, now.Add(-time.Hour)))

	report, err := svc.Recompute(context.Background(), WithdrawableRecomputeOptions{UserID: 7})
	require.NoError(t, err)
	require.Equal(t, 0, report.ReadyUsers)
	require.Equal(t, 1, report.NeedsReviewUsers)
	require.Equal(t, "0.00000000", report.Users[0].ComputedWithdrawableBalance.StringFixed(8))
	require.Contains(t, report.Users[0].Anomalies[0], "computed entitlement balance exceeds current balance")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWithdrawableInvariantCheckReportsImageFrozenIsolation(t *testing.T) {
	t.Parallel()

	db, mock := newBalanceLedgerSQLMock(t)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	svc := &WithdrawableRecomputeService{db: db, now: func() time.Time { return now }}

	for _, value := range []int64{0, 0, 0, 0} {
		mock.ExpectQuery("(?s)SELECT COUNT\\(\\*\\)::bigint").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(value))
	}

	report, err := svc.CheckInvariants(context.Background())
	require.NoError(t, err)
	require.True(t, report.Passed)
	require.Equal(t, int64(0), report.ImageTouchedWithdrawalFrozenCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
