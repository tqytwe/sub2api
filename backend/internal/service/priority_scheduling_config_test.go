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
