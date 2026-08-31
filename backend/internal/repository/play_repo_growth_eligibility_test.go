package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetGrowthEligibilitySignalsUsesCompletedCNYBalanceNetRecharge(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	usageSince := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	rechargeSince := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`(?is)SELECT.*ai\.provider_type IN \('github', 'google', 'oidc'\).*ai\.metadata->>'email_verified'.*ul\.created_at >= \$2.*ul\.created_at < \$7.*SUM\(GREATEST\(\s*CASE WHEN po\.list_amount > 0 THEN po\.qualifying_recharge_amount ELSE po\.amount END\s*\* \(1 - LEAST\(GREATEST\(COALESCE\(po\.refund_amount / NULLIF\(po\.amount, 0\), 0\), 0\), 1\)\),\s*0\s*\)\).*po\.order_type = \$4.*po\.status = \$5.*UPPER\(COALESCE\(po\.payment_currency, 'CNY'\)\) = 'CNY'.*po\.completed_at IS NOT NULL.*po\.completed_at >= \$3.*po\.completed_at < \$7`).
		WithArgs(int64(42), usageSince, rechargeSince, payment.OrderTypeBalance, payment.OrderStatusCompleted, service.SubscriptionStatusActive, now).
		WillReturnRows(sqlmock.NewRows([]string{
			"email_verified", "created_at", "has_recent_usage", "net_balance_recharge_30d", "has_active_subscription",
		}).AddRow(true, createdAt, true, 9.5, false))

	repo := &playRepository{sql: db}
	signals, err := repo.GetGrowthEligibilitySignals(context.Background(), 42, usageSince, rechargeSince, now)

	require.NoError(t, err)
	require.True(t, signals.EmailVerified)
	require.Equal(t, createdAt, signals.CreatedAt)
	require.True(t, signals.HasRecentUsage)
	require.InDelta(t, 9.5, signals.NetBalanceRecharge30d, 0.000001)
	require.False(t, signals.HasActiveSubscription)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGrowthEligibilitySignalsReturnsUserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{
		"email_verified", "created_at", "has_recent_usage", "net_balance_recharge_30d", "has_active_subscription",
	}))

	repo := &playRepository{sql: db}
	_, err = repo.GetGrowthEligibilitySignals(context.Background(), 404, time.Time{}, time.Time{}, time.Time{})

	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateGrowthEligibilitySnapshotRequiresAnActionIDAndPersistsIt(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	_, err = repo.CreateGrowthEligibilitySnapshot(context.Background(), service.PlayGrowthEligibilitySnapshot{
		UserID:   42,
		Source:   service.PlayRewardSourceCheckin,
		ActionID: " \t ",
	})
	require.ErrorContains(t, err, "action id is required")
	require.NoError(t, mock.ExpectationsWereMet())

	activityDate := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	snapshot := service.PlayGrowthEligibilitySnapshot{
		UserID:       42,
		Source:       service.PlayRewardSourceQuiz,
		ActionID:     " quiz:42:2026-08-29 ",
		ActivityDate: activityDate,
		Eligibility: service.PlayGrowthEligibility{
			Tier:                  service.PlayGrowthTierExplorer,
			RewardMode:            service.PlayGrowthRewardEnergy,
			PrimaryReason:         service.PlayGrowthEligibilityReasonNoRecentActivity,
			EmailVerified:         true,
			AccountAgeDays:        8,
			HasRecentUsage:        false,
			NetBalanceRecharge30d: 0,
			HasActiveSubscription: false,
		},
	}
	mock.ExpectQuery(`(?s)INSERT INTO play_growth_eligibility_snapshots`).
		WithArgs(
			int64(42),
			service.PlayRewardSourceQuiz,
			"quiz:42:2026-08-29",
			"2026-08-29",
			service.PlayGrowthTierExplorer,
			service.PlayGrowthRewardEnergy,
			service.PlayGrowthEligibilityReasonNoRecentActivity,
			true,
			8,
			false,
			float64(0),
			false,
			service.PlayGrowthQualificationRuleVersion(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(901)))

	id, err := repo.CreateGrowthEligibilitySnapshot(context.Background(), snapshot)
	require.NoError(t, err)
	require.Equal(t, int64(901), id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateGrowthEligibilitySnapshotPersistsCurrentRuleVersion(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	activityDate := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`(?s)INSERT INTO play_growth_eligibility_snapshots`).
		WithArgs(
			int64(42), service.PlayRewardSourceCheckin, "checkin:42:2026-08-29", "2026-08-29",
			service.PlayGrowthTierActive, service.PlayGrowthRewardRedeemable, service.PlayGrowthEligibilityReasonEligible,
			true, 8, true, float64(0), false, service.PlayGrowthQualificationRuleVersion(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(902)))

	_, err = repo.CreateGrowthEligibilitySnapshot(context.Background(), service.PlayGrowthEligibilitySnapshot{
		UserID: 42, Source: service.PlayRewardSourceCheckin, ActionID: "checkin:42:2026-08-29", ActivityDate: activityDate,
		Eligibility: service.PlayGrowthEligibility{
			Tier: service.PlayGrowthTierActive, RewardMode: service.PlayGrowthRewardRedeemable,
			PrimaryReason: service.PlayGrowthEligibilityReasonEligible, EmailVerified: true, AccountAgeDays: 8, HasRecentUsage: true,
		},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLinkGrowthEligibilitySnapshotRequiresMatchingUnlinkedActivityEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	activityDate := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE play_checkins AS activity.*FROM play_growth_eligibility_snapshots AS snapshot.*growth_eligibility_snapshot_id IS NULL.*snapshot\.id = \$3.*snapshot\.user_id = \$1.*snapshot\.source = \$4.*snapshot\.activity_date = \$2`).
		WithArgs(int64(42), "2026-08-29", int64(901), service.PlayRewardSourceCheckin).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.LinkGrowthEligibilitySnapshot(context.Background(), service.PlayRewardSourceCheckin, 42, activityDate, 901))

	mock.ExpectExec(`(?s)UPDATE play_quiz_attempts AS activity.*FROM play_growth_eligibility_snapshots AS snapshot.*growth_eligibility_snapshot_id IS NULL.*snapshot\.id = \$3.*snapshot\.user_id = \$1.*snapshot\.source = \$4.*snapshot\.activity_date = \$2`).
		WithArgs(int64(42), "2026-08-29", int64(902), service.PlayRewardSourceQuiz).
		WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.LinkGrowthEligibilitySnapshot(context.Background(), service.PlayRewardSourceQuiz, 42, activityDate, 902), service.ErrPlayRewardDuplicate)

	mock.ExpectExec(`(?s)UPDATE play_blindbox_opens AS activity.*FROM play_growth_eligibility_snapshots AS snapshot.*growth_eligibility_snapshot_id IS NULL.*snapshot\.id = \$3.*snapshot\.user_id = \$1.*snapshot\.source = \$4.*snapshot\.action_id = \$2`).
		WithArgs(int64(42), "blindbox:42:request-1", int64(903), service.PlayRewardSourceBlindbox).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.LinkBlindboxGrowthEligibilitySnapshot(context.Background(), 42, "blindbox:42:request-1", 903))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestInsertGrowthEnergyLedgerRequiresActionAndSnapshotEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &playRepository{sql: db}
	for _, entry := range []service.PlayGrowthEnergyLedgerEntry{
		{UserID: 42, Source: service.PlayRewardSourceCheckin, Amount: 1, EligibilitySnapshotID: 901},
		{UserID: 42, Source: service.PlayRewardSourceCheckin, ActionID: "checkin:42:2026-08-29", Amount: 0, EligibilitySnapshotID: 901},
		{UserID: 42, Source: service.PlayRewardSourceCheckin, ActionID: "checkin:42:2026-08-29", Amount: 1},
	} {
		err := repo.InsertGrowthEnergyLedger(context.Background(), entry)
		require.Error(t, err)
	}
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectExec(`(?s)INSERT INTO play_growth_energy_ledger`).
		WithArgs(int64(42), service.PlayRewardSourceCheckin, "checkin:42:2026-08-29", int64(1), int64(901)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	err = repo.InsertGrowthEnergyLedger(context.Background(), service.PlayGrowthEnergyLedgerEntry{
		UserID: 42, Source: service.PlayRewardSourceCheckin, ActionID: " checkin:42:2026-08-29 ", Amount: 1, EligibilitySnapshotID: 901,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
