package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestWithdrawableGrantPolicyClassifiesOnlyApprovedSources(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)

	affiliate := classifyWithdrawableGrant("affiliate_balance", createdAt)
	require.True(t, affiliate.Eligible)
	require.Equal(t, createdAt, affiliate.AvailableAt)

	for _, source := range []string{PlayRewardSourceArenaDaily, PlayRewardSourceArenaSettlement, PlayRewardSourceTeamSharedReward} {
		policy := classifyWithdrawableGrant(source, createdAt)
		require.True(t, policy.Eligible, source)
		require.Equal(t, createdAt.Add(72*time.Hour), policy.AvailableAt, source)
	}

	for _, source := range []string{PlayRewardSourceCheckin, PlayRewardSourceQuiz, PlayRewardSourceBlindbox, "payment_recharge", "promo_bonus", "unknown"} {
		policy := classifyWithdrawableGrant(source, createdAt)
		require.False(t, policy.Eligible, source)
		require.True(t, policy.AvailableAt.IsZero(), source)
	}
}

func TestWithdrawableConsumptionPlanUsesNonEntitlementThenFIFO(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	entitlements := []withdrawableEntitlementSnapshot{
		{ID: 10, Remaining: decimal.RequireFromString("5.00000000"), AvailableAt: now.Add(-time.Hour)},
		{ID: 11, Remaining: decimal.RequireFromString("7.00000000"), AvailableAt: now.Add(2 * time.Hour)},
	}

	plan := planWithdrawableConsumption(
		decimal.RequireFromString("30.00000000"),
		decimal.RequireFromString("25.00000000"),
		entitlements,
		now,
	)

	require.Equal(t, "18.00000000", plan.NonEntitlementAmount.StringFixed(8))
	require.Equal(t, "7.00000000", plan.EntitlementAmount.StringFixed(8))
	require.Equal(t, "5.00000000", plan.MatureWithdrawableAmount.StringFixed(8))
	require.Len(t, plan.Allocations, 2)
	require.Equal(t, int64(10), plan.Allocations[0].EntitlementID)
	require.Equal(t, "5.00000000", plan.Allocations[0].Amount.StringFixed(8))
	require.Equal(t, int64(11), plan.Allocations[1].EntitlementID)
	require.Equal(t, "2.00000000", plan.Allocations[1].Amount.StringFixed(8))
}

func TestWithdrawableRestorePlanRestoresOriginalBatchesAndMatureAmount(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	consumed := []withdrawableConsumedAllocationSnapshot{
		{EntitlementID: 10, Amount: decimal.RequireFromString("3.00000000"), AvailableAt: now.Add(-time.Hour)},
		{EntitlementID: 11, Amount: decimal.RequireFromString("4.00000000"), AvailableAt: now.Add(time.Hour)},
	}

	plan := planWithdrawableRestore(decimal.RequireFromString("5.00000000"), consumed, now)

	require.Equal(t, "5.00000000", plan.TotalAmount.StringFixed(8))
	require.Equal(t, "3.00000000", plan.MatureWithdrawableAmount.StringFixed(8))
	require.Len(t, plan.Allocations, 2)
	require.Equal(t, int64(10), plan.Allocations[0].EntitlementID)
	require.Equal(t, "3.00000000", plan.Allocations[0].Amount.StringFixed(8))
	require.Equal(t, int64(11), plan.Allocations[1].EntitlementID)
	require.Equal(t, "2.00000000", plan.Allocations[1].Amount.StringFixed(8))
}
