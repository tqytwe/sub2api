package handler

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPlayHubBlindboxStatusWithoutActorOmitsConfiguredPool(t *testing.T) {
	pool := service.PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 20, Weight: 10_000},
		},
	}

	dto := toPlayHubSummaryDTO(&service.PlayHubSummary{
		Blindbox: &service.PlayBlindboxStatus{
			Enabled:      true,
			CostAmount:   pool.Cost,
			BlindboxPool: pool,
		},
	})

	require.NotNil(t, dto.Blindbox)
	require.Nil(t, dto.Blindbox.Pool)
	require.Zero(t, dto.Blindbox.CostAmount)
}

func TestPlayHubBlindboxStatusIncludesConfiguredPoolForRedeemableActor(t *testing.T) {
	pool := service.PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 20, Weight: 10_000},
		},
	}

	dto := toPlayHubSummaryDTOForActor(&service.PlayHubSummary{
		Blindbox: &service.PlayBlindboxStatus{
			Enabled:           true,
			CostAmount:        pool.Cost,
			BlindboxPool:      pool,
			GrowthEligibility: service.PlayGrowthEligibility{RewardMode: service.PlayGrowthRewardRedeemable},
		},
	}, 42)

	require.NotNil(t, dto.Blindbox)
	require.NotNil(t, dto.Blindbox.Pool)
	require.Equal(t, pool.Version, dto.Blindbox.Pool.Version)
	require.Equal(t, 20.0, dto.Blindbox.Pool.Tiers[0].Amount)
}

func TestPlayHubBlindboxExplorerOmitsRewardPoolFields(t *testing.T) {
	dto := toPlayHubSummaryDTOForActor(&service.PlayHubSummary{
		Blindbox: &service.PlayBlindboxStatus{
			Enabled:         true,
			CouponPoolReady: false,
			CostAmount:      0.5,
			BlindboxPool:    service.PlayBlindboxPool{Version: "secret", Cost: 0.5, RTPCap: 0.9},
			CurrentPool:     service.PlayBlindboxPool{Version: "secret-current"},
			ExpectedReward:  0.45,
			PoolVersion:     "secret",
			RTPCap:          0.9,
			GrowthEligibility: service.PlayGrowthEligibility{
				Tier:       service.PlayGrowthTierExplorer,
				RewardMode: service.PlayGrowthRewardEnergy,
			},
		},
	}, 42)

	payload, err := json.Marshal(dto.Blindbox)
	require.NoError(t, err)
	body := string(payload)
	for _, forbidden := range []string{"\"pool\"", "\"current_pool\"", "\"next_pool\"", "\"cost_amount\"", "\"expected_reward\"", "\"rtp_cap\"", "\"pool_version\"", "\"coupon_prizes\"", "\"coupon_weight_bp\"", "\"balance_weight_bp\""} {
		require.NotContains(t, body, forbidden)
	}
}

func TestPlayHubBlindboxAuthenticatedUnknownEligibilityOmitsRewardPoolFields(t *testing.T) {
	pool := service.PlayBlindboxPool{
		Version: "secret",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers:   []service.PlayBlindboxTier{{Amount: 20, Weight: 10_000}},
	}
	dto := toPlayHubSummaryDTOForActor(&service.PlayHubSummary{
		Blindbox: &service.PlayBlindboxStatus{
			Enabled:        false,
			CostAmount:     pool.Cost,
			BlindboxPool:   pool,
			CurrentPool:    pool,
			ExpectedReward: 0.45,
			PoolVersion:    pool.Version,
			RTPCap:         pool.RTPCap,
		},
	}, 42)

	payload, err := json.Marshal(dto.Blindbox)
	require.NoError(t, err)
	body := string(payload)
	for _, forbidden := range []string{"\"pool\"", "\"current_pool\"", "\"cost_amount\"", "\"expected_reward\"", "\"rtp_cap\"", "\"pool_version\""} {
		require.NotContains(t, body, forbidden)
	}
}
