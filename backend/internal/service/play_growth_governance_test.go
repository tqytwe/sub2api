package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func completeGrowthCohort(now time.Time) PlayGrowthCohortMetrics {
	abnormal := 0.02
	falsePositive := 0.01
	return PlayGrowthCohortMetrics{
		WindowStart:              now.Add(-45 * 24 * time.Hour),
		WindowEnd:                now.Add(-31 * 24 * time.Hour),
		MetricsAvailable:         true,
		ParticipationUsers:       100,
		RealCall7dUsers:          40,
		RealCall7dRatio:          0.4,
		RealCall30dUsers:         60,
		RealCall30dRatio:         0.6,
		FirstRechargeUsers:       12,
		FirstRechargeRatio:       0.12,
		CouponsIssued:            100,
		CouponsRedeemed:          20,
		CouponRedemptionRatio:    0.2,
		ActualRewardCost:         25,
		D7RetainedUsers:          35,
		D7RetentionRatio:         0.35,
		AbnormalRedemptionUsers:  2,
		AbnormalRedemptionRatio:  &abnormal,
		AppealCount:              4,
		FalsePositiveAppeals:     1,
		AppealFalsePositiveRatio: &falsePositive,
	}
}

func validGrowthApproval(now time.Time) PlayGrowthGovernanceApprovalInput {
	return PlayGrowthGovernanceApprovalInput{
		BudgetAmount:   100,
		RolloutPercent: 10,
		Cohort:         completeGrowthCohort(now),
		RuleVersion:    PlayGrowthQualificationRuleVersion(),
		Reason:         "two week cohort passed operations review",
		ActorID:        42,
	}
}

func TestValidatePlayGrowthGovernanceApprovalRejectsUnsafeRolloutAndIncompleteCohort(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	base := validGrowthApproval(now)

	for _, rollout := range []int{9, 21} {
		input := base
		input.RolloutPercent = rollout
		err := ValidatePlayGrowthGovernanceApproval(input, now)
		require.Error(t, err)
	}

	input := base
	input.Cohort.WindowStart = now.Add(-20 * 24 * time.Hour)
	input.Cohort.WindowEnd = now.Add(-13 * 24 * time.Hour)
	require.Error(t, ValidatePlayGrowthGovernanceApproval(input, now))

	input = base
	input.Cohort.WindowEnd = now.Add(24 * time.Hour)
	require.Error(t, ValidatePlayGrowthGovernanceApproval(input, now))

	input = base
	input.Cohort.WindowStart = now.Add(-34 * 24 * time.Hour)
	input.Cohort.WindowEnd = now.Add(-20 * 24 * time.Hour)
	require.ErrorContains(t, ValidatePlayGrowthGovernanceApproval(input, now), "30-day metrics")

	input = base
	input.BudgetAmount = 0
	require.Error(t, ValidatePlayGrowthGovernanceApproval(input, now))

	input = base
	input.Cohort.AppealFalsePositiveRatio = nil
	require.Error(t, ValidatePlayGrowthGovernanceApproval(input, now))
}

func TestPlayGrowthGovernanceStateRequiresApprovalBudgetAndCurrentRule(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	input := validGrowthApproval(now)
	state := PlayGrowthGovernanceState{
		ID:              7,
		Decision:        PlayGrowthGovernanceDecisionApproved,
		Approved:        true,
		BudgetAmount:    input.BudgetAmount,
		BudgetRemaining: input.BudgetAmount,
		RolloutPercent:  input.RolloutPercent,
		Cohort:          input.Cohort,
		RuleVersion:     input.RuleVersion,
	}
	require.True(t, state.AllowsReward(now))
	allowedUserID := int64(0)
	for id := int64(1); id < 1000; id++ {
		if state.AllowsRewardForUser(id, now) {
			allowedUserID = id
			break
		}
	}
	require.Positive(t, allowedUserID)
	require.True(t, state.AllowsRewardForUser(allowedUserID, now))

	state.BudgetRemaining = 0
	require.False(t, state.AllowsReward(now))
	state.BudgetRemaining = input.BudgetAmount
	state.Decision = PlayGrowthGovernanceDecisionRevoked
	require.False(t, state.AllowsReward(now))
	state.Decision = PlayGrowthGovernanceDecisionApproved
	state.RuleVersion = "old"
	require.False(t, state.AllowsReward(now))
}

func TestStableGrowthRolloutBucketIsDeterministicAndBounded(t *testing.T) {
	for _, userID := range []int64{1, 2, 42, 999999} {
		bucket := StableGrowthRolloutBucket(userID)
		require.GreaterOrEqual(t, bucket, 1)
		require.LessOrEqual(t, bucket, 100)
		require.Equal(t, bucket, StableGrowthRolloutBucket(userID))
	}
	require.False(t, GrowthRolloutAllowsUser(0, 20))
	require.False(t, GrowthRolloutAllowsUser(42, 9))
}

type governanceOnlyRepo struct {
	PlayRepository
	state *PlayGrowthGovernanceState
}

func (r *governanceOnlyRepo) GetGrowthGovernance(context.Context, time.Time) (*PlayGrowthGovernanceState, error) {
	return r.state, nil
}

func (r *governanceOnlyRepo) GetGrowthCohort(context.Context, time.Time, time.Time) (PlayGrowthCohortMetrics, error) {
	return PlayGrowthCohortMetrics{}, nil
}

func (r *governanceOnlyRepo) CreateGrowthApproval(context.Context, PlayGrowthGovernanceApprovalInput) (*PlayGrowthGovernanceState, error) {
	return r.state, nil
}

func (r *governanceOnlyRepo) RevokeGrowthApproval(context.Context, int64, string) (*PlayGrowthGovernanceState, error) {
	return r.state, nil
}

func (r *governanceOnlyRepo) GetGrowthRewardSpend(context.Context, time.Time, time.Time) (float64, error) {
	return 0, nil
}

func TestGetGrowthGovernanceFailsClosedWhenRepositoryOrApprovalIsMissing(t *testing.T) {
	svc := NewPlayService(nil, nil, nil, nil, nil, nil)
	svc.RequireGrowthGovernance(true)
	_, err := svc.GetGrowthGovernance(context.Background())
	require.ErrorIs(t, err, ErrPlayGrowthGovernanceUnavailable)

	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	repo := &governanceOnlyRepo{}
	svc = NewPlayService(repo, nil, nil, nil, nil, nil)
	svc.RequireGrowthGovernance(true)
	_, err = svc.GetGrowthGovernance(context.Background())
	require.ErrorIs(t, err, ErrPlayGrowthGovernanceUnavailable)

	repo.state = &PlayGrowthGovernanceState{Decision: PlayGrowthGovernanceDecisionRevoked}
	state, err := svc.GetGrowthGovernance(context.Background())
	require.NoError(t, err)
	require.False(t, state.AllowsReward(now))
}
