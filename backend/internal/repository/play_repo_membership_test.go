package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSyncMembershipOrderContributionUsesExplicitNumericAmounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	paidAt := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
	mock.ExpectExec(`(?is)GREATEST\(\$4::numeric - \$5::numeric, 0::numeric\).*GREATEST\(EXCLUDED\.paid_amount - EXCLUDED\.refund_amount, 0::numeric\)`).
		WithArgs(int64(212), int64(698), "balance", 10.0, 0.0, &paidAt, "COMPLETED").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &playRepository{sql: db}
	require.NoError(t, repo.SyncMembershipOrderContribution(
		context.Background(), 212, 698, "balance", 10, 0, &paidAt, "COMPLETED",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSyncMembershipOrderContributionClampsInvalidAmounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`(?is)INSERT INTO play_membership_order_contributions.*GREATEST\(\$4::numeric - \$5::numeric, 0::numeric\)`).
		WithArgs(int64(215), int64(701), "subscription", 0.0, 0.0, nil, "COMPLETED").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &playRepository{sql: db}
	require.NoError(t, repo.SyncMembershipOrderContribution(
		context.Background(), 215, 701, "subscription", -10, 20, nil, "COMPLETED",
	))
	require.NoError(t, mock.ExpectationsWereMet())
}
