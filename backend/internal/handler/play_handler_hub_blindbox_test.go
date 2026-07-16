package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPlayHubBlindboxStatusIncludesConfiguredPool(t *testing.T) {
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
	require.NotNil(t, dto.Blindbox.Pool)
	require.Equal(t, pool.Version, dto.Blindbox.Pool.Version)
	require.Equal(t, 20.0, dto.Blindbox.Pool.Tiers[0].Amount)
}
