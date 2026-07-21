package service

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestParseWithdrawalAmountRequiresWholeUnits(t *testing.T) {
	t.Parallel()

	amount, err := parseWithdrawalAmount("10")
	require.NoError(t, err)
	require.Equal(t, "10.00000000", amount.StringFixed(8))

	amount, err = parseWithdrawalAmount("10.00")
	require.NoError(t, err)
	require.Equal(t, "10.00000000", amount.StringFixed(8))

	for _, raw := range []string{"", "0", "10.50", "10.001", "10.005", "-10", "abc"} {
		_, err := parseWithdrawalAmount(raw)
		require.ErrorIs(t, err, ErrWithdrawalInvalidAmount, raw)
	}
}

func TestWithdrawalFreezePlanUsesOnlyMatureEntitlementsFIFO(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	entitlements := []withdrawableEntitlementSnapshot{
		{ID: 11, Remaining: decimal.RequireFromString("5.00000000"), AvailableAt: now.Add(time.Hour)},
		{ID: 12, Remaining: decimal.RequireFromString("3.00000000"), AvailableAt: now.Add(-2 * time.Hour)},
		{ID: 13, Remaining: decimal.RequireFromString("6.00000000"), AvailableAt: now.Add(-time.Hour)},
	}
	plan, err := planWithdrawalEntitlementFreeze(
		decimal.RequireFromString("7.00000000"),
		entitlements,
		now,
	)

	require.NoError(t, err)
	require.Len(t, plan.Allocations, 2)
	require.Equal(t, int64(12), plan.Allocations[0].EntitlementID)
	require.Equal(t, "3.00000000", plan.Allocations[0].Amount.StringFixed(8))
	require.Equal(t, int64(13), plan.Allocations[1].EntitlementID)
	require.Equal(t, "4.00000000", plan.Allocations[1].Amount.StringFixed(8))

	_, err = planWithdrawalEntitlementFreeze(decimal.RequireFromString("10.00000000"), entitlements, now)
	require.ErrorIs(t, err, ErrWithdrawalInsufficientWithdrawable)
}

func TestWithdrawalApprovalStatusRequiresSecondDistinctReviewerAtThreshold(t *testing.T) {
	t.Parallel()

	low := decimal.RequireFromString("99")
	threshold := decimal.RequireFromString("100")
	status, err := withdrawalStatusAfterApproval(WithdrawalStatusPendingReview, low, threshold, 10, nil)
	require.NoError(t, err)
	require.Equal(t, WithdrawalStatusPayoutPending, status)

	status, err = withdrawalStatusAfterApproval(WithdrawalStatusPendingReview, threshold, threshold, 10, nil)
	require.NoError(t, err)
	require.Equal(t, WithdrawalStatusSecondReview, status)

	first := int64(10)
	_, err = withdrawalStatusAfterApproval(WithdrawalStatusSecondReview, threshold, threshold, 10, &first)
	require.ErrorIs(t, err, ErrWithdrawalSelfReviewForbidden)

	status, err = withdrawalStatusAfterApproval(WithdrawalStatusSecondReview, threshold, threshold, 11, &first)
	require.NoError(t, err)
	require.Equal(t, WithdrawalStatusPayoutPending, status)
}

func TestWithdrawalAccountMaskDoesNotExposeFullDetails(t *testing.T) {
	t.Parallel()

	mask := maskWithdrawalPayoutAccount(WithdrawalPayoutMethodAlipay, map[string]string{
		"account":        "alice-withdrawal@example.com",
		"recipient_name": "Alice Zhang",
	})

	require.Contains(t, mask.AccountMask, "ali")
	require.NotContains(t, mask.AccountMask, "alice-withdrawal@example.com")
	require.NotContains(t, mask.RecipientNameMask, "Alice Zhang")
	require.NotEmpty(t, mask.AccountMask)
	require.NotEmpty(t, mask.RecipientNameMask)
}
