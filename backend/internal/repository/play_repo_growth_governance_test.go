package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetGrowthGovernanceReadsLatestDecisionAndBudgetSpend(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	cohort := service.PlayGrowthCohortMetrics{
		WindowStart:             now.Add(-30 * 24 * time.Hour),
		WindowEnd:               now.Add(-16 * 24 * time.Hour),
		MetricsAvailable:        true,
		ParticipationUsers:      10,
		RealCall7dUsers:         2,
		RealCall7dRatio:         0.2,
		RealCall30dUsers:        4,
		RealCall30dRatio:        0.4,
		FirstRechargeUsers:      1,
		FirstRechargeRatio:      0.1,
		CouponsIssued:           2,
		CouponsRedeemed:         1,
		CouponRedemptionRatio:   0.5,
		ActualRewardCost:        4,
		D7RetainedUsers:         2,
		D7RetentionRatio:        0.2,
		AbnormalRedemptionUsers: 0,
		AppealCount:             1,
		FalsePositiveAppeals:    0,
	}
	abnormal, falsePositive := 0.0, 0.0
	cohort.AbnormalRedemptionRatio = &abnormal
	cohort.AppealFalsePositiveRatio = &falsePositive
	metricsJSON, err := json.Marshal(cohort)
	require.NoError(t, err)
	actorID := int64(17)
	mock.ExpectQuery(`(?s)SELECT a\.id.*FROM play_growth_governance_approvals a.*ORDER BY a\.id DESC`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "decision", "budget_amount", "rollout_percent", "cohort_start", "cohort_end", "cohort_metrics", "rule_version", "reason", "actor_id", "created_at", "budget_spent",
		}).AddRow(9, service.PlayGrowthGovernanceDecisionApproved, 100.0, 10, cohort.WindowStart, cohort.WindowEnd, metricsJSON, service.PlayGrowthQualificationRuleVersion(), "cohort approved by ops", actorID, now.Add(-time.Hour), 12.5))

	repo := &playRepository{sql: db}
	state, err := repo.GetGrowthGovernance(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, int64(9), state.ID)
	require.True(t, state.Approved)
	require.InDelta(t, 12.5, state.BudgetSpent, 0.000001)
	require.InDelta(t, 87.5, state.BudgetRemaining, 0.000001)
	require.Equal(t, actorID, *state.ActorID)
	require.True(t, state.Cohort.Complete())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetGrowthCohortPreservesUnavailableMetricsAsNull(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(14 * 24 * time.Hour)
	mock.ExpectQuery(`(?s)WITH participants AS.*FROM play_checkins.*FROM play_quiz_attempts.*usage_logs.*payment_orders.*user_coupons.*play_reward_ledger.*NULL::double precision`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{
			"window_start", "window_end", "participants", "usage_7d", "usage_7d_ratio", "usage_30d", "usage_30d_ratio", "first_recharge", "first_recharge_ratio", "coupons_issued", "coupons_redeemed", "coupon_redemption_ratio", "reward_cost", "d7_retained", "d7_retention_ratio", "abnormal_users", "abnormal_ratio", "appeal_count", "false_positive", "appeal_ratio",
		}).AddRow(start, end, int64(0), int64(0), 0.0, int64(0), 0.0, int64(0), 0.0, int64(0), int64(0), 0.0, 0.0, int64(0), 0.0, int64(0), nil, int64(0), int64(0), nil))

	repo := &playRepository{sql: db}
	metrics, err := repo.GetGrowthCohort(context.Background(), start, end)
	require.NoError(t, err)
	require.False(t, metrics.MetricsAvailable)
	require.Nil(t, metrics.AbnormalRedemptionRatio)
	require.Nil(t, metrics.AppealFalsePositiveRatio)
	require.Contains(t, metrics.UnavailableMetrics, "appeal_false_positive_ratio")
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectGrowthGovernanceLock(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtextextended\(\$1, 0\)\)`).
		WithArgs(growthGovernanceAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectNoGrowthBudgetReservation(mock sqlmock.Sqlmock, source, actionID string) {
	mock.ExpectQuery(`(?s)SELECT user_id, source, amount::double precision.*FROM play_growth_reward_budget_ledger.*WHERE source = \$1 AND action_id = \$2`).
		WithArgs(source, actionID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "source", "amount"}))
}

func TestReserveGrowthRewardBudgetValidatesAndReturnsAtomicDecision(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &playRepository{sql: db}
	reserved, err := repo.ReserveGrowthRewardBudget(context.Background(), 0, 42, "checkin", "a", 1)
	require.False(t, reserved)
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	expectNoGrowthBudgetReservation(mock, "checkin", "checkin:42:2026-08-31")
	mock.ExpectQuery(`(?s)WITH current_approval AS.*ORDER BY a\.id DESC.*a\.id = \$1.*a\.decision = 'approved'.*play_growth_reward_budget_ledger.*ON CONFLICT`).
		WithArgs(int64(7), int64(42), "checkin", "checkin:42:2026-08-31", 1.0).
		WillReturnRows(sqlmock.NewRows([]string{"reserved"}).AddRow(true))
	mock.ExpectCommit()
	reserved, err = repo.ReserveGrowthRewardBudget(context.Background(), 7, 42, "checkin", "checkin:42:2026-08-31", 1)
	require.NoError(t, err)
	require.True(t, reserved)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveGrowthRewardBudgetOnlyReplaysExactAction(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &playRepository{sql: db}

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	mock.ExpectQuery(`(?s)SELECT user_id, source, amount::double precision.*FROM play_growth_reward_budget_ledger.*WHERE source = \$1 AND action_id = \$2`).
		WithArgs("checkin", "checkin:42:2026-08-31").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "source", "amount"}).AddRow(int64(42), "checkin", 1.0))
	mock.ExpectCommit()

	reserved, err := repo.ReserveGrowthRewardBudget(context.Background(), 7, 42, "checkin", "checkin:42:2026-08-31", 1)
	require.NoError(t, err)
	require.True(t, reserved)

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	mock.ExpectQuery(`(?s)SELECT user_id, source, amount::double precision.*FROM play_growth_reward_budget_ledger.*WHERE source = \$1 AND action_id = \$2`).
		WithArgs("checkin", "checkin:42:2026-08-31").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "source", "amount"}).AddRow(int64(42), "checkin", 1.0))
	mock.ExpectRollback()

	reserved, err = repo.ReserveGrowthRewardBudget(context.Background(), 7, 42, "checkin", "checkin:42:2026-08-31", 2)
	require.False(t, reserved)
	require.Error(t, err)
	require.Contains(t, err.Error(), "budget action id conflicts")

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	mock.ExpectQuery(`(?s)SELECT user_id, source, amount::double precision.*FROM play_growth_reward_budget_ledger.*WHERE source = \$1 AND action_id = \$2`).
		WithArgs("checkin", "checkin:42:2026-08-31").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "source", "amount"}).AddRow(int64(42), "checkin", 1.0))
	mock.ExpectRollback()

	reserved, err = repo.ReserveGrowthRewardBudget(context.Background(), 7, 99, "checkin", "checkin:42:2026-08-31", 1)
	require.False(t, reserved)
	require.Error(t, err)
	require.Contains(t, err.Error(), "budget action id conflicts")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveGrowthRewardBudgetRejectsSupersededApprovalAfterLock(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &playRepository{sql: db}

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	expectNoGrowthBudgetReservation(mock, "quiz", "quiz:42:2026-08-31")
	mock.ExpectQuery(`(?s)WITH current_approval AS.*ORDER BY a\.id DESC.*a\.id = \$1.*a\.decision = 'approved'.*play_growth_reward_budget_ledger.*ON CONFLICT`).
		WithArgs(int64(7), int64(42), "quiz", "quiz:42:2026-08-31", 1.0).
		WillReturnRows(sqlmock.NewRows([]string{"reserved"}).AddRow(false))
	mock.ExpectCommit()

	reserved, err := repo.ReserveGrowthRewardBudget(context.Background(), 7, 42, "quiz", "quiz:42:2026-08-31", 1)
	require.NoError(t, err)
	require.False(t, reserved)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrowthGovernanceMutationsReturnDatabaseActorAndTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &playRepository{sql: db}
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	cohort := service.PlayGrowthCohortMetrics{
		WindowStart:      now.Add(-30 * 24 * time.Hour),
		WindowEnd:        now.Add(-16 * 24 * time.Hour),
		MetricsAvailable: true,
	}

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	mock.ExpectQuery(`(?s)INSERT INTO play_growth_governance_approvals.*VALUES \('approved'.*RETURNING id, actor_id, created_at`).
		WithArgs(20.0, 10, cohort.WindowStart, cohort.WindowEnd, sqlmock.AnyArg(), service.PlayGrowthQualificationRuleVersion(), "approved after reviewed cohort", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "created_at"}).AddRow(int64(11), int64(9), now))
	mock.ExpectCommit()

	approved, err := repo.CreateGrowthApproval(context.Background(), service.PlayGrowthGovernanceApprovalInput{
		BudgetAmount: 20, RolloutPercent: 10, Cohort: cohort,
		RuleVersion: service.PlayGrowthQualificationRuleVersion(), Reason: "approved after reviewed cohort", ActorID: 9,
	})
	require.NoError(t, err)
	require.Equal(t, int64(11), approved.ID)
	require.NotNil(t, approved.ActorID)
	require.Equal(t, int64(9), *approved.ActorID)
	require.Equal(t, now, approved.CreatedAt)

	mock.ExpectBegin()
	expectGrowthGovernanceLock(mock)
	mock.ExpectQuery(`(?s)INSERT INTO play_growth_governance_approvals.*VALUES \('revoked'.*RETURNING id, actor_id, created_at`).
		WithArgs(service.PlayGrowthQualificationRuleVersion(), "revoked because anomaly evidence is unavailable", int64(13)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "created_at"}).AddRow(int64(12), int64(13), now.Add(time.Minute)))
	mock.ExpectCommit()

	revoked, err := repo.RevokeGrowthApproval(context.Background(), 13, "revoked because anomaly evidence is unavailable")
	require.NoError(t, err)
	require.Equal(t, int64(12), revoked.ID)
	require.NotNil(t, revoked.ActorID)
	require.Equal(t, int64(13), *revoked.ActorID)
	require.Equal(t, now.Add(time.Minute), revoked.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}
