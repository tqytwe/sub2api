package service

import "testing"

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
