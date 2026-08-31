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

func TestListMembershipContributionsAllowsNullQualificationReason(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	updatedAt := time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?is)SELECT order_id,order_type,paid_amount::text,refund_amount::text,net_amount::text,paid_at,status,updated_at,qualification_state,qualification_source,qualification_reason FROM \(.*play_membership_order_contributions.*play_membership_manual_contributions.*WHERE user_id=\$1.*LIMIT \$2`).
		WithArgs(int64(264), 50).
		WillReturnRows(sqlmock.NewRows([]string{
			"order_id", "order_type", "paid_amount", "refund_amount", "net_amount", "paid_at", "status", "updated_at", "qualification_state", "qualification_source", "qualification_reason",
		}).AddRow(
			int64(188), "balance", "50.00000000", "0.00000000", "50.00000000", nil, "COMPLETED", updatedAt, "verified", "historical_snapshot", nil,
		))

	repo := &playRepository{sql: db}
	contributions, err := repo.ListMembershipContributions(context.Background(), 264, 50)

	require.NoError(t, err)
	require.Len(t, contributions, 1)
	require.Nil(t, contributions[0].PaidAt)
	require.Empty(t, contributions[0].QualificationReason)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMembershipPaidTotalExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)FROM \( SELECT net_amount FROM play_membership_order_contributions.*SELECT net_amount FROM play_membership_manual_contributions.*\) contributions`).
		WithArgs(int64(264)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow("50.00000000"))

	repo := &playRepository{sql: db}
	total, err := repo.GetMembershipPaidTotal(context.Background(), 264)

	require.NoError(t, err)
	require.Equal(t, 50.0, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMembershipAdminOverviewExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)FROM \(SELECT c\.user_id.*play_membership_order_contributions.*play_membership_manual_contributions.*u\.deleted_at IS NULL`).
		WithArgs(100.0).
		WillReturnRows(sqlmock.NewRows([]string{"total_members", "net_paid"}).AddRow(1, "50.00000000"))

	repo := &playRepository{sql: db}
	total, amount, err := repo.MembershipAdminOverview(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "50", amount.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListMembershipAdminRowsExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)SELECT COUNT\(\*\).*FROM users u.*u\.deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?is)SELECT u\.id.*FROM users u.*u\.deleted_at IS NULL.*LIMIT \$1 OFFSET \$2`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "total_paid", "created_at", "first_paid_at", "last_paid_at"}).
			AddRow(int64(264), "user@example.com", "user", "50.00000000", time.Now(), nil, nil))

	repo := &playRepository{sql: db}
	rows, total, err := repo.ListMembershipAdminRows(context.Background(), "", nil, 100, 1, 20)

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, rows, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListMembershipPaidTotalsExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)SELECT u\.id.*FROM users u LEFT JOIN \(.*play_membership_order_contributions.*play_membership_manual_contributions.*WHERE u\.deleted_at IS NULL.*GROUP BY u\.id`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "total"}).AddRow(int64(264), "50.00000000"))

	repo := &playRepository{sql: db}
	totals, err := repo.ListMembershipPaidTotals(context.Background())

	require.NoError(t, err)
	require.Equal(t, "50", totals[264].String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMembershipAdminRowExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)FROM users u LEFT JOIN \(.*play_membership_order_contributions.*play_membership_manual_contributions.*WHERE u\.id=\$1 AND u\.deleted_at IS NULL`).
		WithArgs(int64(264)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "total_paid", "created_at", "first_paid_at", "last_paid_at"}).
			AddRow(int64(264), "user@example.com", "user", "50.00000000", time.Now(), nil, nil))

	repo := &playRepository{sql: db}
	row, err := repo.GetMembershipAdminRow(context.Background(), 264)

	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, int64(264), row.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetMembershipAdminRowReturnsNilForMissingOrSoftDeletedUser(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)FROM users u LEFT JOIN \(.*play_membership_order_contributions.*play_membership_manual_contributions.*WHERE u\.id=\$1 AND u\.deleted_at IS NULL`).
		WithArgs(int64(264)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "total_paid", "created_at", "first_paid_at", "last_paid_at"}))

	repo := &playRepository{sql: db}
	row, err := repo.GetMembershipAdminRow(context.Background(), 264)

	require.NoError(t, err)
	require.Nil(t, row, "missing and soft-deleted members must not bubble sql.ErrNoRows into a 500")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListMembershipTierHistoryExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?is)SELECT h\.user_id.*FROM play_membership_tier_history h JOIN users u ON u\.id = h\.user_id AND u\.deleted_at IS NULL.*WHERE h\.user_id=\$1.*LIMIT \$2`).
		WithArgs(int64(264), 50).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "order_id", "from_tier", "to_tier", "net_paid_before", "net_paid_after", "reason", "created_at"}).
			AddRow(int64(264), int64(188), 0, 1, "0.00000000", "50.00000000", "recharge", time.Now()))

	repo := &playRepository{sql: db}
	history, err := repo.ListMembershipTierHistory(context.Background(), 264, 50)

	require.NoError(t, err)
	require.Len(t, history, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCountRecentMembershipTierChangesExcludesSoftDeletedUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	since := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?is)SELECT COUNT\(\*\).*FROM play_membership_tier_history h JOIN users u ON u\.id = h\.user_id AND u\.deleted_at IS NULL.*WHERE h\.created_at >= \$1`).
		WithArgs(since).
		WillReturnRows(sqlmock.NewRows([]string{"upgrades", "downgrades"}).AddRow(2, 1))

	repo := &playRepository{sql: db}
	upgrades, downgrades, err := repo.CountRecentMembershipTierChanges(context.Background(), since)

	require.NoError(t, err)
	require.Equal(t, 2, upgrades)
	require.Equal(t, 1, downgrades)
	require.NoError(t, mock.ExpectationsWereMet())
}
