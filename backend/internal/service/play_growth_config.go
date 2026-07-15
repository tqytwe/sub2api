package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const blindboxWeightTotal int64 = 10_000

func defaultBlindboxPool() PlayBlindboxPool {
	return PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []PlayBlindboxTier{
			{Amount: 0.05, Weight: 4000},
			{Amount: 0.2, Weight: 3000},
			{Amount: 0.5, Weight: 1800},
			{Amount: 1, Weight: 800},
			{Amount: 3, Weight: 300},
			{Amount: 10, Weight: 90},
			{Amount: 20, Weight: 10},
		},
	}
}

func ParseBlindboxPool(raw string) PlayBlindboxPool {
	pool := defaultBlindboxPool()
	if strings.TrimSpace(raw) == "" {
		return pool
	}
	var parsed PlayBlindboxPool
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return pool
	}
	if err := ValidateBlindboxPool(parsed); err != nil {
		return pool
	}
	return parsed
}

func ValidateBlindboxPool(pool PlayBlindboxPool) error {
	if strings.TrimSpace(pool.Version) == "" {
		return errors.New("blindbox pool version is required")
	}
	if pool.Cost <= 0 || !isFinite(pool.Cost) {
		return errors.New("blindbox pool cost must be positive")
	}
	if pool.RTPCap <= 0 || pool.RTPCap > 1 || !isFinite(pool.RTPCap) {
		return errors.New("blindbox RTP cap must be in (0, 1]")
	}
	if len(pool.Tiers) == 0 || len(pool.Tiers) > 32 {
		return errors.New("blindbox pool must contain 1-32 tiers")
	}
	var totalWeight int64
	var expected float64
	for _, tier := range pool.Tiers {
		if tier.Amount < 0 || !isFinite(tier.Amount) {
			return errors.New("blindbox reward amount must be non-negative")
		}
		if tier.Weight <= 0 {
			return errors.New("blindbox tier weight must be positive")
		}
		totalWeight += tier.Weight
		expected += tier.Amount * float64(tier.Weight) / float64(blindboxWeightTotal)
	}
	if totalWeight != blindboxWeightTotal {
		return fmt.Errorf("blindbox tier weights must total %d", blindboxWeightTotal)
	}
	if expected > pool.Cost*pool.RTPCap+1e-9 {
		return fmt.Errorf("blindbox expected reward %.4f exceeds RTP cap %.4f", expected, pool.Cost*pool.RTPCap)
	}
	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

type PlayGrowthConfig struct {
	BlindboxPool            PlayBlindboxPool `json:"blindbox_pool"`
	BlindboxPaidEnabled     bool             `json:"blindbox_paid_enabled"`
	BlindboxRegionEnabled   bool             `json:"blindbox_region_enabled"`
	TeamMaxMembers          int              `json:"team_max_members"`
	TeamWeeklyTokenTarget   int64            `json:"team_weekly_token_target"`
	TeamWeeklyRequestTarget int64            `json:"team_weekly_request_target"`
	PublicActivityMinCount  int              `json:"public_activity_min_count"`
	FounderSeasonJSON       string           `json:"founder_season_json"`
	GrowthExperimentJSON    string           `json:"growth_experiment_json"`
}

func (s *SettingService) GetPlayGrowthConfig(ctx context.Context) (PlayGrowthConfig, error) {
	if s == nil || s.settingRepo == nil {
		return PlayGrowthConfig{}, errors.New("setting service is unavailable")
	}
	keys := []string{
		SettingKeyPlayBlindboxPoolJSON,
		SettingKeyPlayBlindboxPaidEnabled,
		SettingKeyPlayBlindboxRegionEnabled,
		SettingKeyPlayTeamMaxMembers,
		SettingKeyPlayTeamWeeklyTokenTarget,
		SettingKeyPlayTeamWeeklyRequestTarget,
		SettingKeyPlayPublicActivityMinCount,
		SettingKeyPlayFounderSeasonJSON,
		SettingKeyPlayGrowthExperimentJSON,
	}
	values, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return PlayGrowthConfig{}, err
	}
	return parsePlayGrowthConfig(values), nil
}

func parsePlayGrowthConfig(values map[string]string) PlayGrowthConfig {
	teamMax, _ := strconv.Atoi(values[SettingKeyPlayTeamMaxMembers])
	if teamMax < 2 || teamMax > 100 {
		teamMax = 8
	}
	tokenTarget, _ := strconv.ParseInt(values[SettingKeyPlayTeamWeeklyTokenTarget], 10, 64)
	if tokenTarget <= 0 {
		tokenTarget = 100_000
	}
	requestTarget, _ := strconv.ParseInt(values[SettingKeyPlayTeamWeeklyRequestTarget], 10, 64)
	if requestTarget <= 0 {
		requestTarget = 20
	}
	activityMin, _ := strconv.Atoi(values[SettingKeyPlayPublicActivityMinCount])
	if activityMin <= 0 {
		activityMin = 1
	}
	season := strings.TrimSpace(values[SettingKeyPlayFounderSeasonJSON])
	if season == "" {
		season = `{"name":"Founding Season","duration_weeks":6,"enabled":true}`
	}
	experiment := strings.TrimSpace(values[SettingKeyPlayGrowthExperimentJSON])
	if experiment == "" {
		experiment = `{"holdout_pct":5,"enabled":false}`
	}
	return PlayGrowthConfig{
		BlindboxPool:            ParseBlindboxPool(values[SettingKeyPlayBlindboxPoolJSON]),
		BlindboxPaidEnabled:     values[SettingKeyPlayBlindboxPaidEnabled] == "true",
		BlindboxRegionEnabled:   values[SettingKeyPlayBlindboxRegionEnabled] == "true",
		TeamMaxMembers:          teamMax,
		TeamWeeklyTokenTarget:   tokenTarget,
		TeamWeeklyRequestTarget: requestTarget,
		PublicActivityMinCount:  activityMin,
		FounderSeasonJSON:       season,
		GrowthExperimentJSON:    experiment,
	}
}

func (s *SettingService) UpdatePlayGrowthConfig(ctx context.Context, cfg PlayGrowthConfig) error {
	if s == nil || s.settingRepo == nil {
		return errors.New("setting service is unavailable")
	}
	if err := ValidateBlindboxPool(cfg.BlindboxPool); err != nil {
		return err
	}
	if cfg.TeamMaxMembers < 2 || cfg.TeamMaxMembers > 100 {
		return errors.New("team max members must be between 2 and 100")
	}
	if cfg.TeamWeeklyTokenTarget <= 0 || cfg.TeamWeeklyRequestTarget <= 0 {
		return errors.New("team weekly targets must be positive")
	}
	if cfg.PublicActivityMinCount <= 0 || cfg.PublicActivityMinCount > 1000 {
		return errors.New("public activity minimum count must be between 1 and 1000")
	}
	for name, raw := range map[string]string{
		"founder_season_json":    cfg.FounderSeasonJSON,
		"growth_experiment_json": cfg.GrowthExperimentJSON,
	} {
		var v any
		if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &v) != nil {
			return fmt.Errorf("%s must be valid JSON", name)
		}
	}
	poolJSON, _ := json.Marshal(cfg.BlindboxPool)
	updates := map[string]string{
		SettingKeyPlayBlindboxPoolJSON:        string(poolJSON),
		SettingKeyPlayBlindboxPaidEnabled:     strconv.FormatBool(cfg.BlindboxPaidEnabled),
		SettingKeyPlayBlindboxRegionEnabled:   strconv.FormatBool(cfg.BlindboxRegionEnabled),
		SettingKeyPlayTeamMaxMembers:          strconv.Itoa(cfg.TeamMaxMembers),
		SettingKeyPlayTeamWeeklyTokenTarget:   strconv.FormatInt(cfg.TeamWeeklyTokenTarget, 10),
		SettingKeyPlayTeamWeeklyRequestTarget: strconv.FormatInt(cfg.TeamWeeklyRequestTarget, 10),
		SettingKeyPlayPublicActivityMinCount:  strconv.Itoa(cfg.PublicActivityMinCount),
		SettingKeyPlayFounderSeasonJSON:       strings.TrimSpace(cfg.FounderSeasonJSON),
		SettingKeyPlayGrowthExperimentJSON:    strings.TrimSpace(cfg.GrowthExperimentJSON),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return err
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return nil
}
