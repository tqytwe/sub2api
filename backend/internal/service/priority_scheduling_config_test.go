package service

import (
	"context"
	"testing"
	"time"
)

func TestPrioritySchedulingDefaultsUseDeprioritizeMode(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	if cfg.LatencyPenaltyMode != PriorityLatencyDeprioritize || cfg.LatencyPenaltyMinutes != 10 {
		t.Fatalf("unexpected legacy-compatible defaults: %+v", cfg)
	}
	if err := ValidatePrioritySchedulingConfig(cfg); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
}

func TestPrioritySchedulingStopModeAndBounds(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	if err := ValidatePrioritySchedulingConfig(cfg); err != nil {
		t.Fatalf("stop mode must validate: %v", err)
	}
	cfg.LatencyPenaltyMode = "invalid"
	if err := ValidatePrioritySchedulingConfig(cfg); err == nil {
		t.Fatal("invalid penalty mode must be rejected")
	}
}

func TestPrioritySchedulingScopeAndScoreKeepAdmissionSeparate(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Enabled = true
	cfg.GroupIDs = []int64{7}
	cfg.Models = []string{"gpt-test"}
	group := int64(7)
	if !cfg.applies(&group, "gpt-test") || cfg.applies(&group, "other") || cfg.applies(nil, "gpt-test") {
		t.Fatal("priority scope was not enforced")
	}
	rate := 0.2
	item := openAIAccountCandidateScore{
		account:   &Account{ID: 11, Name: "eligible", RateMultiplier: &rate, Concurrency: 10},
		loadInfo:  &AccountLoadInfo{LoadRate: 10},
		loadKnown: true,
		priority:  1,
	}
	score := scorePriorityCandidate(cfg, item, PrioritySchedulingSignal{Samples: 20, P90TTFTMs: 100})
	if score.Tier != "insufficient" || score.Score <= 0 {
		t.Fatalf("missing quality history must remain explicitly insufficient: %+v", score)
	}
	if score.QualityStatus != "unavailable" || score.QualitySamples != 0 {
		t.Fatalf("quality schema absence must be explicit: %+v", score)
	}
}

func TestPrioritySchedulingErrorRateWeightAffectsScore(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Mode = "custom"
	cfg.QualityWeight, cfg.LatencyWeight, cfg.LoadWeight, cfg.CostWeight = 0, 0, 0, 1
	cfg.ErrorRateWeight = 0
	rate := 0.2
	item := openAIAccountCandidateScore{account: &Account{ID: 1, RateMultiplier: &rate}, loadInfo: &AccountLoadInfo{}, loadKnown: true, errorRate: 0.9}
	without := scorePriorityCandidate(cfg, item, PrioritySchedulingSignal{Samples: 5, P90TTFTMs: 100})
	cfg.ErrorRateWeight = 100
	with := scorePriorityCandidate(cfg, item, PrioritySchedulingSignal{Samples: 5, P90TTFTMs: 100})
	if with.Score >= without.Score || !hasPriorityReason(with.Reasons, "error_rate_high") {
		t.Fatalf("error-rate weight must affect score and expose high-rate evidence: without=%+v with=%+v", without, with)
	}
}

func TestPrioritySchedulingSnapshotStartsUnavailable(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	if got := gateway.PrioritySchedulingSnapshot(); got != nil {
		t.Fatalf("cold scheduler must not claim an empty candidate pool: %+v", got)
	}
	gateway.publishPrioritySnapshot(&PrioritySchedulingSnapshot{HistoryStatus: "warming"})
	got := gateway.PrioritySchedulingSnapshot()
	if got == nil || got.HistoryStatus != "warming" || got.HistoryReady {
		t.Fatalf("unexpected warming snapshot: %+v", got)
	}
}

func TestPrioritySchedulingHistoryFailureFailsOpen(t *testing.T) {
	gateway := &OpenAIGatewayService{}
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Enabled = true
	group := int64(1)
	items := []openAIAccountCandidateScore{{account: &Account{ID: 1}, loadInfo: &AccountLoadInfo{}}}
	signals, ready := gateway.prioritySignals(OpenAIAccountScheduleRequest{GroupID: &group, RequestedModel: "m"}, cfg, items)
	if ready || signals != nil {
		t.Fatalf("missing signal reader must fail open: ready=%v signals=%v", ready, signals)
	}
}

func TestPrioritySignalCacheKeySeparatesCandidateSetsAndQuerySettings(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	group := int64(9)
	req := OpenAIAccountScheduleRequest{GroupID: &group, RequestedModel: "m", InboundEndpoint: "responses"}
	ids := []int64{1, 2, 3}
	base := prioritySignalCacheKey(req, cfg, ids)
	if base == prioritySignalCacheKey(req, cfg, []int64{1}) {
		t.Fatal("single-account sticky signals must not share a cache entry with the full candidate pool")
	}
	if base == prioritySignalCacheKey(req, cfg, []int64{1, 2, 4}) {
		t.Fatal("different candidate sets must not share signal cache entries")
	}
	changedWindow := cfg
	changedWindow.WindowMinutes++
	if base == prioritySignalCacheKey(req, changedWindow, ids) {
		t.Fatal("different observation windows must not share signal cache entries")
	}
	changedQualityWindow := cfg
	changedQualityWindow.QualityMaxAgeHours++
	if base == prioritySignalCacheKey(req, changedQualityWindow, ids) {
		t.Fatal("different quality evidence windows must not share signal cache entries")
	}
	changedPercentile := cfg
	changedPercentile.LatencyPercentile++
	if base == prioritySignalCacheKey(req, changedPercentile, ids) {
		t.Fatal("different latency percentiles must not share signal cache entries")
	}
	changedTarget := cfg
	changedTarget.TargetTTFTMs++
	if base == prioritySignalCacheKey(req, changedTarget, ids) {
		t.Fatal("different latency targets must not share bad-sample timestamps")
	}
}

func TestPriorityLatencyStopEvaluatesOverflowCandidates(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	plan := &openAIAccountLoadPlan{
		candidates:    []openAIAccountCandidateScore{{account: &Account{ID: 1}}},
		allCandidates: []openAIAccountCandidateScore{{account: &Account{ID: 1}}, {account: &Account{ID: 2}}},
	}
	got := prioritySchedulingScoringCandidates(cfg, plan)
	if len(got) != 2 || got[0].account.ID != 1 || got[1].account.ID != 2 {
		t.Fatalf("stop mode must measure candidates available to overflow fallback: %+v", got)
	}

	cfg.LatencyPenaltyMode = PriorityLatencyDeprioritize
	got = prioritySchedulingScoringCandidates(cfg, plan)
	if len(got) != 1 || got[0].account.ID != 1 {
		t.Fatalf("deprioritize mode must preserve ranked-only history reads: %+v", got)
	}
}

func TestPriorityLatencyStopFiltersOnlyMeasuredAboveTargetCandidates(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Enabled = true
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	cfg.MinSamples = 5
	cfg.TargetTTFTMs = 1000
	rate := 0.2
	stopped := openAIAccountCandidateScore{
		account:   &Account{ID: 1, Name: "slow", RateMultiplier: &rate, Concurrency: 10},
		loadInfo:  &AccountLoadInfo{LoadRate: 10},
		loadKnown: true,
	}
	unknown := openAIAccountCandidateScore{
		account:   &Account{ID: 2, Name: "cold", RateMultiplier: &rate, Concurrency: 10},
		loadInfo:  &AccountLoadInfo{LoadRate: 10},
		loadKnown: true,
	}
	stoppedScore := scorePriorityCandidate(cfg, stopped, PrioritySchedulingSignal{Samples: 5, P90TTFTMs: 2000})
	unknownScore := scorePriorityCandidate(cfg, unknown, PrioritySchedulingSignal{Samples: 4, P90TTFTMs: 2000})
	if !priorityLatencyStopped(cfg, stoppedScore) {
		t.Fatalf("measured above-target latency must stop new selections: %+v", stoppedScore)
	}
	if priorityLatencyStopped(cfg, unknownScore) {
		t.Fatalf("insufficient latency history must not be treated as a stop: %+v", unknownScore)
	}

	overflowStopped := stopped
	overflowStopped.account = &Account{ID: 3, Name: "overflow-slow", RateMultiplier: &rate, Concurrency: 10}
	candidates := []openAIAccountCandidateScore{unknown}
	allCandidates := []openAIAccountCandidateScore{stopped, unknown, overflowStopped}
	plan := openAIAccountLoadPlan{
		allCandidates:  allCandidates,
		candidates:     candidates,
		candidateCount: 1,
		topK:           1,
	}
	stoppedScore.LatencyStopped = true
	unknownScore.LatencyStopped = priorityLatencyStopped(cfg, unknownScore)
	overflowScore := scorePriorityCandidate(cfg, overflowStopped, PrioritySchedulingSignal{Samples: 5, P90TTFTMs: 2500})
	overflowScore.LatencyStopped = true
	filterPriorityLatencyStopped(&plan, []PrioritySchedulingScore{stoppedScore, unknownScore, overflowScore})
	if len(plan.candidates) != 1 || plan.candidates[0].account.ID != 2 || len(plan.allCandidates) != 1 {
		t.Fatalf("stop mode must remove only measured slow account from every candidate pool: %+v", plan)
	}
	if !priorityLatencyStopBlocksAccount(plan.priorityLatencyStopped, 1) ||
		!priorityLatencyStopBlocksAccount(plan.priorityLatencyStopped, 3) ||
		priorityLatencyStopBlocksAccount(plan.priorityLatencyStopped, 2) {
		t.Fatalf("sticky fallback must receive only measured stop decisions: %+v", plan.priorityLatencyStopped)
	}
	if len(plan.staleSnapshotCompactRetry) != 0 {
		t.Fatalf("stale compact retry pool must also exclude measured slow accounts: %+v", plan.staleSnapshotCompactRetry)
	}
	if plan.candidateCount != 1 || plan.topK != 1 {
		t.Fatalf("candidate counts must follow hard filtering: count=%d topK=%d", plan.candidateCount, plan.topK)
	}
}

func TestPriorityLatencyPenaltyExpiresFromLatestBadSample(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	cfg.TargetTTFTMs = 1000
	cfg.MinSamples = 5
	cfg.LatencyPenaltyMinutes = 10
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	item := openAIAccountCandidateScore{account: &Account{ID: 7, RateMultiplier: ptrFloat64(0.2)}}

	badAt := now.Add(-11 * time.Minute)
	recovered := scorePriorityCandidateAt(cfg, item, PrioritySchedulingSignal{
		Samples: 10, P90TTFTMs: 2000, LatestBadLatencyAt: &badAt,
	}, now)
	if priorityLatencyStopped(cfg, recovered) || !recovered.LatencyRecovery {
		t.Fatalf("expired penalty must enter recovery instead of stopping traffic: %+v", recovered)
	}
	if recovered.LatencyPenaltyUntil == nil || !recovered.LatencyPenaltyUntil.Equal(badAt.Add(10*time.Minute)) {
		t.Fatalf("penalty expiry must be anchored to the latest bad sample: %+v", recovered.LatencyPenaltyUntil)
	}

	missingTimestamp := scorePriorityCandidateAt(cfg, item, PrioritySchedulingSignal{Samples: 10, P90TTFTMs: 2000}, now)
	if !priorityLatencyStopped(cfg, missingTimestamp) || missingTimestamp.LatencyRecovery {
		t.Fatalf("missing bad-sample time must retain the penalty rather than invent recovery: %+v", missingTimestamp)
	}

	freshBadAt := now.Add(-time.Minute)
	stillPenalized := scorePriorityCandidateAt(cfg, item, PrioritySchedulingSignal{
		Samples: 10, P90TTFTMs: 2000, LatestBadLatencyAt: &freshBadAt,
	}, now)
	if !priorityLatencyStopped(cfg, stillPenalized) || stillPenalized.LatencyRecovery {
		t.Fatalf("unexpired penalty must continue to stop traffic: %+v", stillPenalized)
	}
}

func TestPriorityLatencyStopBlocksWeightedStickyFallback(t *testing.T) {
	scheduler := &defaultOpenAIAccountScheduler{}
	_, err := scheduler.tryFallbackToWeightedSticky(
		context.Background(),
		OpenAIAccountScheduleRequest{
			StickyWeighted:          true,
			StickyPreviousAccountID: 7,
			StickyAccountID:         7,
		},
		map[int64]struct{}{7: {}},
	)
	if err != nil {
		t.Fatalf("stopped sticky fallback must be vetoed without reading/acquiring the account: %v", err)
	}
}

func TestPriorityLatencyStopAdmissionScope(t *testing.T) {
	cfg := DefaultPrioritySchedulingConfig()
	cfg.Enabled = true
	cfg.LatencyPenaltyMode = PriorityLatencyStop
	cfg.GroupIDs = []int64{7}
	cfg.Models = []string{"gpt-test"}
	group := int64(7)
	req := OpenAIAccountScheduleRequest{
		Platform:             PlatformOpenAI,
		GroupID:              &group,
		RequestedModel:       "gpt-test",
		UseUpstreamTokenCost: true,
	}
	if !shouldApplyPriorityLatencyStop(req, cfg) {
		t.Fatal("enabled OpenAI stop policy should gate this scoped request")
	}
	req.Platform = PlatformAnthropic
	if shouldApplyPriorityLatencyStop(req, cfg) {
		t.Fatal("priority stop policy must not gate a different platform")
	}
	req.Platform = PlatformOpenAI
	req.UseUpstreamTokenCost = false
	if shouldApplyPriorityLatencyStop(req, cfg) {
		t.Fatal("priority stop policy must follow the same upstream-token-cost scope as scoring")
	}
}

func TestPrioritySchedulingUsesIndependentQualityFreshnessWindow(t *testing.T) {
	queries := make(chan PrioritySchedulingQuery, 1)
	gateway := &OpenAIGatewayService{usageLogRepo: priorityHistoryReaderFunc{read: func(_ context.Context, q PrioritySchedulingQuery) (map[int64]PrioritySchedulingSignal, error) {
		queries <- q
		return map[int64]PrioritySchedulingSignal{}, nil
	}}}
	cfg := DefaultPrioritySchedulingConfig()
	cfg.WindowMinutes, cfg.QualityMaxAgeHours = 5, 24
	gateway.prioritySignals(OpenAIAccountScheduleRequest{RequestedModel: "m"}, cfg, []openAIAccountCandidateScore{{account: &Account{ID: 1}}})
	select {
	case q := <-queries:
		if got := q.Since.Sub(q.QualitySince); got != 24*time.Hour-5*time.Minute {
			t.Fatalf("quality window must be independent of the latency window: %s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("priority signal refresh did not run")
	}
}
