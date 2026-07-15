package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBlindboxPool_DefaultPool(t *testing.T) {
	pool := defaultBlindboxPool()
	require.NoError(t, ValidateBlindboxPool(pool))

	var expected float64
	for _, tier := range pool.Tiers {
		expected += tier.Amount * float64(tier.Weight) / float64(blindboxWeightTotal)
	}
	require.InDelta(t, 0.45, expected, 1e-9)
	require.InDelta(t, 0.9, expected/pool.Cost, 1e-9)
}

func TestValidateBlindboxPool_RejectsInvalidWeightAndRTP(t *testing.T) {
	pool := defaultBlindboxPool()
	pool.Tiers[0].Weight--
	require.ErrorContains(t, ValidateBlindboxPool(pool), "weights must total")

	pool = defaultBlindboxPool()
	pool.Tiers[len(pool.Tiers)-1].Amount = 200
	require.ErrorContains(t, ValidateBlindboxPool(pool), "exceeds RTP cap")
}

func TestParseBlindboxPool_FallsBackOnInvalidJSON(t *testing.T) {
	pool := ParseBlindboxPool(`{"version":"bad"}`)
	require.Equal(t, "season-1-v1", pool.Version)
	require.Len(t, pool.Tiers, 7)
}
