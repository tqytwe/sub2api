package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPerformanceConfigValidationAndUnknownMetrics(t *testing.T) {
	minTPS, maxTTFT, maxDegradation := 8.0, 2500.0, 15.0
	cfg := PerformanceConfig{
		Enabled:     true,
		TPS:         PerformanceBounds{Min: &minTPS},
		TTFT:        PerformanceBounds{Max: &maxTTFT},
		Degradation: PerformanceBounds{Max: &maxDegradation},
	}
	require.NoError(t, cfg.Validate())

	unknown := AccountPerformance{Config: cfg}
	require.Empty(t, unknown.Breaches(), "missing samples must stay unknown, not become zero")

	lowTPS := 7.5
	metrics := AccountPerformance{Config: cfg, AvgTPS: &lowTPS, TPSSamples: 3}
	require.Equal(t, []string{"tps"}, metrics.Breaches())

	badBounds := []PerformanceBounds{
		{Min: float64Pointer(-0.1)},
		{Max: float64Pointer(math.NaN())},
		{Max: float64Pointer(math.Inf(1))},
		{Min: float64Pointer(10), Max: float64Pointer(9)},
		{Max: float64Pointer(100.1)},
	}
	for _, bounds := range badBounds {
		require.Error(t, bounds.Validate(true))
	}
}

func TestPerformanceSnapshotClonesMutableEvidence(t *testing.T) {
	tps := 12.0
	min := 10.0
	metrics := &AccountPerformance{
		AvgTPS: &tps,
		Config: PerformanceConfig{TPS: PerformanceBounds{Min: &min}},
	}

	copy := cloneAccountPerformance(metrics)
	*copy.AvgTPS = 1
	*copy.Config.TPS.Min = 2

	require.Equal(t, 12.0, *metrics.AvgTPS)
	require.Equal(t, 10.0, *metrics.Config.TPS.Min)
}

func float64Pointer(value float64) *float64 { return &value }
