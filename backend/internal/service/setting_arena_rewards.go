package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const maxPlayArenaRewardTiers = 32

// GetPlayArenaRewardSettings returns the effective reward configuration used by
// the arena runtime. The existing reward setting keys remain the source of truth.
func (s *SettingService) GetPlayArenaRewardSettings(ctx context.Context) (PlayArenaRewardSettings, error) {
	if s == nil || s.settingRepo == nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("arena reward settings repository is not configured")
	}

	runtime := s.GetPlayRuntime(ctx)
	return PlayArenaRewardSettings{
		Monthly:     append([]PlayArenaSettlementTier(nil), runtime.ArenaSettlementRewards...),
		Daily:       append([]PlayArenaSettlementTier(nil), runtime.DailyArenaTopRewards...),
		DailyBudget: runtime.DailyArenaDailyBudget,
	}, nil
}

// SetPlayArenaRewardSettings validates and atomically replaces both schedules
// and the daily payout cap. It deliberately does not touch period or ledger
// data, so settled reward history remains immutable.
func (s *SettingService) SetPlayArenaRewardSettings(ctx context.Context, settings PlayArenaRewardSettings) (PlayArenaRewardSettings, error) {
	if s == nil || s.settingRepo == nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("arena reward settings repository is not configured")
	}
	if err := validatePlayArenaRewardTiers("monthly", settings.Monthly); err != nil {
		return PlayArenaRewardSettings{}, infraerrors.BadRequest("PLAY_ARENA_REWARDS_INVALID", err.Error())
	}
	if err := validatePlayArenaRewardTiers("daily", settings.Daily); err != nil {
		return PlayArenaRewardSettings{}, infraerrors.BadRequest("PLAY_ARENA_REWARDS_INVALID", err.Error())
	}
	if math.IsNaN(settings.DailyBudget) || math.IsInf(settings.DailyBudget, 0) || settings.DailyBudget <= 0 {
		return PlayArenaRewardSettings{}, infraerrors.BadRequest("PLAY_ARENA_REWARDS_INVALID", "daily budget must be positive")
	}

	monthly, err := json.Marshal(settings.Monthly)
	if err != nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("marshal monthly arena rewards: %w", err)
	}
	daily, err := json.Marshal(settings.Daily)
	if err != nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("marshal daily arena rewards: %w", err)
	}
	budget, err := json.Marshal(settings.DailyBudget)
	if err != nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("marshal daily arena budget: %w", err)
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyPlayArenaSettlementRewards: string(monthly),
		SettingKeyPlayDailyArenaTopRewards:   string(daily),
		SettingKeyPlayDailyArenaDailyBudget:  string(budget),
	}); err != nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("persist arena reward settings: %w", err)
	}
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return settings, nil
}

func validatePlayArenaRewardTiers(name string, tiers []PlayArenaSettlementTier) error {
	if len(tiers) == 0 {
		return fmt.Errorf("%s arena reward tiers are required", name)
	}
	if len(tiers) > maxPlayArenaRewardTiers {
		return fmt.Errorf("%s arena reward tiers must contain at most %d tiers", name, maxPlayArenaRewardTiers)
	}

	previousRank := 0
	for i, tier := range tiers {
		if tier.RankMax <= previousRank {
			return fmt.Errorf("%s arena reward tier %d rank max must be strictly increasing", name, i+1)
		}
		if math.IsNaN(tier.Amount) || math.IsInf(tier.Amount, 0) || tier.Amount <= 0 {
			return fmt.Errorf("%s arena reward tier %d amount must be positive", name, i+1)
		}
		previousRank = tier.RankMax
	}
	return nil
}

func (s *PlayService) GetArenaRewardSettings(ctx context.Context) (PlayArenaRewardSettings, error) {
	if s == nil || s.settingService == nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.GetPlayArenaRewardSettings(ctx)
}

func (s *PlayService) UpdateArenaRewardSettings(ctx context.Context, settings PlayArenaRewardSettings) (PlayArenaRewardSettings, error) {
	if s == nil || s.settingService == nil {
		return PlayArenaRewardSettings{}, fmt.Errorf("play settings service is not configured")
	}
	return s.settingService.SetPlayArenaRewardSettings(ctx, settings)
}
