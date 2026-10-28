package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"time"
)

const prioritySchedulingSettingKey = "priority_scheduling_v1"

const (
	PriorityLatencyDeprioritize = "deprioritize"
	PriorityLatencyStop         = "stop"
)

// PrioritySchedulingConfig is the persisted admin contract for the priority
// scheduler. The fields are intentionally scoped to scheduling signals and do
// not include credentials, account bodies, or request content.
type PrioritySchedulingConfig struct {
	ErrorRateWeight            float64  `json:"error_rate_weight"`
	ErrorRateWindowMinutes     int      `json:"error_rate_window_minutes"`
	ErrorRatePriorSamples      int      `json:"error_rate_prior_samples"`
	OAuthQuotaPriority         bool     `json:"oauth_quota_priority"`
	OAuthQuotaThreshold        int      `json:"oauth_quota_threshold"`
	BalanceProtocols           bool     `json:"balance_protocols"`
	Enabled                    bool     `json:"enabled"`
	Mode                       string   `json:"mode"`
	GroupIDs                   []int64  `json:"group_ids"`
	Models                     []string `json:"models"`
	WindowMinutes              int      `json:"window_minutes"`
	MinSamples                 int      `json:"min_samples"`
	LatencyPercentile          int      `json:"latency_percentile"`
	LatencyMeasurement         string   `json:"latency_measurement"`
	RecoveryExplorationPercent int      `json:"recovery_exploration_percent"`
	TargetTTFTMs               int      `json:"target_ttft_ms"`
	LatencyPenaltyMode         string   `json:"latency_penalty_mode"`
	LatencyPenaltyMinutes      int      `json:"latency_penalty_minutes"`
	MaxLoadPercent             int      `json:"max_load_percent"`
	MinQualityPercent          int      `json:"min_quality_percent"`
	QualityMaxAgeHours         int      `json:"quality_max_age_hours"`
	QualityWeight              float64  `json:"quality_weight"`
	LatencyWeight              float64  `json:"latency_weight"`
	LoadWeight                 float64  `json:"load_weight"`
	CostWeight                 float64  `json:"cost_weight"`
}

func DefaultPrioritySchedulingConfig() PrioritySchedulingConfig {
	return PrioritySchedulingConfig{
		ErrorRateWeight: 25, ErrorRateWindowMinutes: 10, ErrorRatePriorSamples: 5,
		OAuthQuotaThreshold: 90, BalanceProtocols: true, Mode: "balanced",
		GroupIDs: []int64{}, Models: []string{}, WindowMinutes: 60, MinSamples: 5,
		LatencyPercentile: 90, LatencyMeasurement: "legacy_forward",
		RecoveryExplorationPercent: 10, TargetTTFTMs: 15000,
		LatencyPenaltyMode: PriorityLatencyDeprioritize, LatencyPenaltyMinutes: 10,
		MaxLoadPercent: 80, MinQualityPercent: 90, QualityMaxAgeHours: 24,
		QualityWeight: 30, LatencyWeight: 25, LoadWeight: 25, CostWeight: 20,
	}
}

func ValidatePrioritySchedulingConfig(c PrioritySchedulingConfig) error {
	if math.IsNaN(c.ErrorRateWeight) || math.IsInf(c.ErrorRateWeight, 0) || c.ErrorRateWeight < 0 || c.ErrorRateWeight > 100 || c.ErrorRateWindowMinutes < 1 || c.ErrorRateWindowMinutes > 60 || c.ErrorRatePriorSamples < 1 || c.ErrorRatePriorSamples > 1000 {
		return errors.New("error rate parameters outside allowed range")
	}
	if c.LatencyPercentile != 50 && c.LatencyPercentile != 70 && c.LatencyPercentile != 90 && c.LatencyPercentile != 95 {
		return errors.New("latency percentile must be 50, 70, 90 or 95")
	}
	if c.LatencyMeasurement != "legacy_forward" && c.LatencyMeasurement != "attributed_v1" {
		return errors.New("invalid latency measurement")
	}
	if c.RecoveryExplorationPercent < 0 || c.RecoveryExplorationPercent > 10 || c.LatencyPenaltyMinutes < 0 || c.LatencyPenaltyMinutes > 1440 {
		return errors.New("latency recovery parameters outside allowed range")
	}
	if c.LatencyPenaltyMode != PriorityLatencyDeprioritize && c.LatencyPenaltyMode != PriorityLatencyStop {
		return errors.New("invalid latency penalty mode")
	}
	if c.OAuthQuotaThreshold < 1 || c.OAuthQuotaThreshold > 100 {
		return errors.New("OAuth quota threshold must be between 1 and 100")
	}
	if c.Mode != "experience" && c.Mode != "balanced" && c.Mode != "profit" && c.Mode != "custom" {
		return errors.New("invalid scheduling mode")
	}
	if c.WindowMinutes < 5 || c.WindowMinutes > 1440 || c.MinSamples < 1 || c.MinSamples > 1000 || c.TargetTTFTMs < 100 || c.TargetTTFTMs > 120000 || c.MaxLoadPercent < 10 || c.MaxLoadPercent > 100 || c.MinQualityPercent < 0 || c.MinQualityPercent > 100 || c.QualityMaxAgeHours < 1 || c.QualityMaxAgeHours > 168 {
		return errors.New("scheduling thresholds outside allowed range")
	}
	if len(c.GroupIDs) > 100 || len(c.Models) > 100 {
		return errors.New("at most 100 groups or models allowed")
	}
	for _, id := range c.GroupIDs {
		if id <= 0 {
			return errors.New("group IDs must be positive")
		}
	}
	for _, model := range c.Models {
		if len(model) == 0 || len(model) > 200 {
			return errors.New("model names must contain 1-200 bytes")
		}
	}
	sum := 0.0
	for _, weight := range []float64{c.QualityWeight, c.LatencyWeight, c.LoadWeight, c.CostWeight} {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 100 {
			return errors.New("weights must be between 0 and 100")
		}
		sum += weight
	}
	if sum <= 0 {
		return errors.New("at least one weight must be positive")
	}
	return nil
}

func (c PrioritySchedulingConfig) applies(group *int64, model string) bool {
	if !c.Enabled {
		return false
	}
	if len(c.GroupIDs) > 0 {
		if group == nil || !slices.Contains(c.GroupIDs, *group) {
			return false
		}
	}
	return len(c.Models) == 0 || slices.Contains(c.Models, model)
}

func (c PrioritySchedulingConfig) weights() [4]float64 {
	switch c.Mode {
	case "experience":
		return [4]float64{40, 35, 20, 5}
	case "profit":
		return [4]float64{20, 15, 20, 45}
	case "balanced":
		return [4]float64{30, 25, 25, 20}
	default:
		return [4]float64{c.QualityWeight, c.LatencyWeight, c.LoadWeight, c.CostWeight}
	}
}

func (s *SettingService) GetPrioritySchedulingConfig(ctx context.Context) (PrioritySchedulingConfig, error) {
	cfg := DefaultPrioritySchedulingConfig()
	raw, err := s.settingRepo.GetValue(ctx, prioritySchedulingSettingKey)
	if errors.Is(err, ErrSettingNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, err
	}
	if cfg.LatencyPenaltyMode == "" {
		cfg.LatencyPenaltyMode = PriorityLatencyDeprioritize
	}
	return cfg, ValidatePrioritySchedulingConfig(cfg)
}

func (s *SettingService) SavePrioritySchedulingConfig(ctx context.Context, cfg PrioritySchedulingConfig) error {
	if cfg.LatencyPenaltyMode == "" {
		cfg.LatencyPenaltyMode = PriorityLatencyDeprioritize
	}
	if err := ValidatePrioritySchedulingConfig(cfg); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := s.settingRepo.Set(ctx, prioritySchedulingSettingKey, string(raw)); err != nil {
		return err
	}
	s.prioritySchedulingRuntimeMu.Lock()
	s.prioritySchedulingRuntimeConfig = cfg
	s.prioritySchedulingRuntimeExpires = time.Now().Add(5 * time.Second)
	s.prioritySchedulingRuntimeMu.Unlock()
	InvalidateOpenAIAdvancedSchedulerSettingCache()
	return nil
}
