package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func sharedPriorityFixture() PrioritySchedulingSignal {
	tps, ttft, cache, degraded := 30.0, 100.0, 50.0, 0.0
	return PrioritySchedulingSignal{Performance: &AccountPerformance{AvgTPS: &tps, TPSSamples: 10, AvgTTFTMs: &ttft, TTFTSamples: 10, AvgCachePercent: &cache, CacheSamples: 10, DegradationPercent: &degraded, QualitySamples: 10}}
}
func sharedPriorityCandidate() openAIAccountCandidateScore {
	rate := .2
	return openAIAccountCandidateScore{account: &Account{ID: 7, Name: "fixture", Concurrency: 10, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{LoadRate: 10}, loadKnown: true}
}
func TestPrioritySharedPerformanceUsesSameMetricsAndCacheWeight(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.CacheWeight = 100
	low, high := sharedPriorityFixture(), sharedPriorityFixture()
	lowCache, highCache := 10.0, 90.0
	low.Performance.AvgCachePercent = &lowCache
	high.Performance.AvgCachePercent = &highCache
	a := scorePriorityCandidate(cfg, sharedPriorityCandidate(), low)
	b := scorePriorityCandidate(cfg, sharedPriorityCandidate(), high)
	require.InDelta(t, 40, b.BaseScore-a.BaseScore, 1e-8)
	require.Equal(t, "insufficient", a.Tier)
	require.Contains(t, a.Reasons, "upstream_error_unknown")
	require.Equal(t, low.Performance.AvgTPS, a.AvgTPS)
	// A separate scoped latency observation can govern admission without
	// replacing shared throughput/quality/cache ranking evidence.
	high.Samples = 0
	high.P90TTFTMs = 900000
	high.QualitySamples = 50
	high.QualityPassed = 0
	require.Equal(t, b.Score, scorePriorityCandidate(cfg, sharedPriorityCandidate(), high).Score)
	high.Performance = nil
	require.Contains(t, scorePriorityCandidate(cfg, sharedPriorityCandidate(), high).Reasons, "cache_insufficient")
}
func TestPrioritySharedPerformanceUnknownRemainsNeutral(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.CacheWeight = 100
	for _, value := range []*float64{nil, ptrFloat64(-1), ptrFloat64(101), ptrFloat64(math.NaN())} {
		signal := sharedPriorityFixture()
		signal.Performance.AvgCachePercent = value
		score := scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal)
		require.Contains(t, score.Reasons, "cache_insufficient")
		require.Equal(t, "insufficient", score.Tier)
	}
	signal := sharedPriorityFixture()
	signal.Performance.AvgTTFTMs = nil
	score := scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal)
	require.Contains(t, score.Reasons, "latency_insufficient")
	require.False(t, priorityLatencyStopped(cfg, score))
	signal.Performance.DegradationPercent = nil
	require.Contains(t, scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal).Reasons, "quality_unknown")
	signal.Performance = &AccountPerformance{}
	unknownError := scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal)
	require.Contains(t, unknownError.Reasons, "tps_insufficient")
	require.Contains(t, unknownError.Reasons, "upstream_error_unknown")
	require.False(t, unknownError.ErrorRateKnown)
	require.Equal(t, 1.0, unknownError.ErrorPenalty)
}
func TestPriorityCacheWeightValidation(t *testing.T) {
	for _, value := range []float64{-1, 101, math.Inf(1), math.NaN()} {
		cfg := DefaultPrioritySchedulingConfig()
		cfg.CacheWeight = value
		require.Error(t, ValidatePrioritySchedulingConfig(cfg))
	}
	cfg := DefaultPrioritySchedulingConfig()
	cfg.QualityWeight = 0
	cfg.LatencyWeight = 0
	cfg.LoadWeight = 0
	cfg.CostWeight = 0
	cfg.CacheWeight = 100
	require.NoError(t, ValidatePrioritySchedulingConfig(cfg))
}
func TestPrioritySnapshotOwnsReturnedMetrics(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	signal := sharedPriorityFixture()
	score := scorePriorityCandidate(DefaultPrioritySchedulingConfig(), sharedPriorityCandidate(), signal)
	gateway.publishPrioritySnapshot(&PrioritySchedulingSnapshot{Candidates: []PrioritySchedulingScore{score}})
	first := gateway.PrioritySchedulingSnapshot()
	*first.Candidates[0].Performance.AvgTPS = 999
	require.Equal(t, 30.0, *gateway.PrioritySchedulingSnapshot().Candidates[0].Performance.AvgTPS)
}
func TestPriorityRecomputeIsReadOnlyAndSortsDiff(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.CacheWeight = 100
	cfg.LatencyMeasurement = "attributed_v1"
	cfg.LatencyPercentile = 95
	cfg.TargetTTFTMs = 700
	signal := sharedPriorityFixture()
	gateway := &OpenAIGatewayService{usageLogRepo: priorityHistoryReaderFunc{read: func(_ context.Context, q PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
		require.True(t, q.UseAccountPerformance)
		require.Equal(t, cfg.LatencyMeasurement, q.LatencyMeasurement)
		require.Equal(t, cfg.LatencyPercentile, q.LatencyPercentile)
		require.Equal(t, cfg.TargetTTFTMs, q.TargetTTFTMs)
		return map[int64]PrioritySchedulingSignal{7: signal}, nil
	}}}
	before := &PrioritySchedulingSnapshot{At: time.Now(), Candidates: []PrioritySchedulingScore{{AccountID: 7, Score: 1}}}
	gateway.priorityScheduling.latest = before
	gateway.priorityScheduling.latestSet = true
	gateway.priorityScheduling.latestConfig = cfg
	gateway.priorityScheduling.latestCandidates = []openAIAccountCandidateScore{sharedPriorityCandidate()}
	gateway.priorityScheduling.latestRequest = OpenAIAccountScheduleRequest{RequestedModel: "test"}
	got, err := gateway.RecomputePriorityScheduling(context.Background())
	require.NoError(t, err)
	require.True(t, got.Diff[0].Changed)
	require.Greater(t, got.After.Candidates[0].Score, 1.0)
	require.Same(t, before, gateway.priorityScheduling.latest)
	require.Empty(t, gateway.priorityScheduling.entries)
}

func TestPrioritySharedRankingKeepsScopedPenaltyExpiry(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	cfg.TargetTTFTMs = 1000
	now := time.Now()
	badAt := now.Add(-time.Minute)
	signal := sharedPriorityFixture()
	signal.Samples, signal.P90TTFTMs, signal.LatestBadLatencyAt = 10, 2000, &badAt
	score := scorePriorityCandidateAt(cfg, sharedPriorityCandidate(), signal, now)
	require.True(t, priorityLatencyStopped(cfg, score))
	require.InDelta(t, 100, *score.AvgTTFTMs, 1e-9)
	require.NotNil(t, score.LatencyPenaltyUntil)
	expired := scorePriorityCandidateAt(cfg, sharedPriorityCandidate(), signal, now.Add(20*time.Minute))
	require.False(t, priorityLatencyStopped(cfg, expired))
	require.Contains(t, expired.Reasons, "latency_recovery")
}

// Percentile admission evidence must retain its population and value even
// when account-wide average latency provides separate ranking evidence.
func TestPrioritySharedSnapshotPreservesScopedPercentile(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	signal := sharedPriorityFixture()
	signal.Samples, signal.P90TTFTMs = 8, 2750
	score := scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal)
	require.Equal(t, 8, score.Samples)
	require.Equal(t, 2750.0, score.P90TTFTMs)
	require.InDelta(t, 100, *score.AvgTTFTMs, 1e-9)
	require.Equal(t, 10, score.Performance.TTFTSamples)
}

func TestPrioritySnapshotSerializesConfiguredPercentile(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.LatencyPercentile = 95
	signal := sharedPriorityFixture()
	signal.Samples, signal.P90TTFTMs = 8, 2750
	score := scorePriorityCandidate(cfg, sharedPriorityCandidate(), signal)
	encoded, err := json.Marshal(score)
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal(encoded, &row))
	require.Equal(t, 95.0, row["latency_percentile"])
	require.Equal(t, 2750.0, row["selected_ttft_ms"])
	require.Equal(t, 100.0, row["avg_ttft_ms"])
}
