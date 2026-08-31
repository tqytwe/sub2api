package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPlayHubCheckinPreservesGrowthQualificationContract(t *testing.T) {
	hub := &service.PlayHubSummary{Checkin: &service.PlayCheckinStatus{
		Enabled:                  true,
		Eligible:                 true,
		IneligibleReason:         service.PlayGrowthEligibilityReasonNoRecentActivity,
		CouponPoolReady:          false,
		CouponWeightBP:           8000,
		RedeemCodeWeightBP:       2000,
		BalanceWeightBP:          0,
		GrowthEligibility:        service.PlayGrowthEligibility{Tier: service.PlayGrowthTierExplorer, RewardMode: service.PlayGrowthRewardEnergy, PrimaryReason: service.PlayGrowthEligibilityReasonNoRecentActivity},
		GrowthEnergyEnabled:      true,
		RedeemableRewardEligible: false,
	}}

	dto := toPlayHubSummaryDTO(hub)
	require.NotNil(t, dto.Checkin)
	require.Equal(t, service.PlayGrowthTierExplorer, dto.Checkin.GrowthEligibility.Tier)
	require.Equal(t, service.PlayGrowthRewardEnergy, dto.Checkin.GrowthEligibility.RewardMode)
	require.True(t, dto.Checkin.GrowthEnergyEnabled)
	require.False(t, dto.Checkin.RedeemableRewardEligible)
	require.False(t, dto.Checkin.CouponPoolReady)
	require.Equal(t, 8000, dto.Checkin.CouponWeightBP)
	require.Equal(t, 2000, dto.Checkin.RedeemCodeWeightBP)
}

func TestPlayHubExplorerBlindboxDoesNotExposeRewardPool(t *testing.T) {
	hub := &service.PlayHubSummary{Blindbox: &service.PlayBlindboxStatus{
		Enabled:           true,
		CouponPoolReady:   false,
		CostAmount:        0,
		PoolVersion:       "",
		ExpectedReward:    0,
		RTPCap:            0,
		OpensToday:        4,
		DailyLimit:        10,
		GrowthEligibility: service.PlayGrowthEligibility{Tier: service.PlayGrowthTierExplorer, RewardMode: service.PlayGrowthRewardEnergy, PrimaryReason: service.PlayGrowthEligibilityReasonNoRecentActivity},
	}}

	dto := toPlayHubSummaryDTOForActor(hub, 42)
	require.NotNil(t, dto.Blindbox)
	require.Nil(t, dto.Blindbox.Pool)
	require.Nil(t, dto.Blindbox.CurrentPool)
	require.Nil(t, dto.Blindbox.NextPool)
	require.Zero(t, dto.Blindbox.CostAmount)
	require.Zero(t, dto.Blindbox.ExpectedReward)
	require.Zero(t, dto.Blindbox.RTPCap)
	require.Equal(t, 4, dto.Blindbox.OpensToday)
	require.False(t, dto.Blindbox.CanOpen)
}
