package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccountCostMultiplierDefaultsAndOverrides(t *testing.T) {
	require.Equal(t, 0.1, (*Account)(nil).CostMultiplier())
	require.Equal(t, 0.1, (&Account{}).CostMultiplier())
	require.Equal(t, 0.1, (&Account{Extra: map[string]any{"cost_multiplier": "0.4"}}).CostMultiplier())
	require.Equal(t, 0.0, (&Account{Extra: map[string]any{"cost_multiplier": 0.0}}).CostMultiplier())
	require.Equal(t, 0.4, (&Account{Extra: map[string]any{"cost_multiplier": 0.4}}).CostMultiplier())
	require.Equal(t, 2.0, (&Account{Extra: map[string]any{"cost_multiplier": int64(2)}}).CostMultiplier())
}

func TestAccountCostMultiplierRejectsNonFiniteAndOutOfRange(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), -0.1, 1000001} {
		require.Equal(t, 0.1, (&Account{Extra: map[string]any{"cost_multiplier": value}}).CostMultiplier())
	}
}
