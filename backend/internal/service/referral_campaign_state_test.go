package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validReferralCampaignForTest() ReferralCampaign {
	start := time.Now().UTC().Add(24 * time.Hour)
	return ReferralCampaign{
		Key:              "august-growth",
		Name:             "August growth",
		Status:           ReferralCampaignStatusDraft,
		Version:          1,
		RegistrationFrom: start,
		RegistrationTo:   start.Add(7 * 24 * time.Hour),
		StartsAt:         start,
		EndsAt:           start.Add(14 * 24 * time.Hour),
		QualificationTo:  start.Add(21 * 24 * time.Hour),
		ClaimDeadline:    start.Add(28 * 24 * time.Hour),
		RiskHoldHours:    24 * 7,
		PayThreshold:     10,
		UsageThreshold:   1,
		MaxEnrollments:   100,
		BudgetTotal:      1000,
		RewardMode:       "replace",
	}
}

func TestValidateReferralCampaignDraftRejectsNonIncreasingReplaceRewards(t *testing.T) {
	campaign := validReferralCampaignForTest()
	err := validateReferralCampaignDraft(&campaign, []ReferralCampaignTier{
		{Tier: 1, RequiredInvites: 1, RewardAmount: 100, Currency: "CNY"},
		{Tier: 2, RequiredInvites: 3, RewardAmount: 90, Currency: "CNY"},
	})
	require.ErrorContains(t, err, "must increase")
}

func TestReferralCampaignTransitionRequiresReviewApproval(t *testing.T) {
	require.True(t, CanTransitionReferralCampaign(ReferralCampaignStatusDraft, ReferralCampaignStatusReview))
	require.False(t, CanTransitionReferralCampaign(ReferralCampaignStatusDraft, ReferralCampaignStatusApproved))
	require.False(t, CanTransitionReferralCampaign(ReferralCampaignStatusReview, ReferralCampaignStatusApproved))
	require.True(t, CanTransitionReferralCampaign(ReferralCampaignStatusRunning, ReferralCampaignStatusSettling))
}

func TestValidateReferralCampaignDraftRejectsNonCNYWalletReward(t *testing.T) {
	campaign := validReferralCampaignForTest()
	err := validateReferralCampaignDraft(&campaign, []ReferralCampaignTier{{Tier: 1, RequiredInvites: 1, RewardAmount: 100, Currency: "USD"}})
	require.ErrorContains(t, err, "must be CNY")
}

func TestValidateReferralCampaignDraftRequiresWorstCaseLiability(t *testing.T) {
	campaign := validReferralCampaignForTest()
	campaign.MaxEnrollments = 3
	campaign.BudgetTotal = 749.99
	tiers := []ReferralCampaignTier{
		{Tier: 1, RequiredInvites: 1, RewardAmount: 100, Currency: "CNY"},
		{Tier: 2, RequiredInvites: 3, RewardAmount: 250, Currency: "CNY"},
	}

	err := validateReferralCampaignDraft(&campaign, tiers)
	require.ErrorContains(t, err, "maximum liability 750.00")

	campaign.BudgetTotal = 750
	require.NoError(t, validateReferralCampaignDraft(&campaign, tiers))
}
