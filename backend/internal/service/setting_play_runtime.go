package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// GetPlayRuntime reads play feature toggles and reward config directly from the
// settings store. Fail-closed: on error returns zero values (opt-in defaults).
func (s *SettingService) GetPlayRuntime(ctx context.Context) PlayRuntime {
	keys := []string{
		SettingKeyPlayCheckinEnabled,
		SettingKeyPlayCheckinDailyReward,
		SettingKeyPlayCheckinMakeupEnabled,
		SettingKeyPlayCheckinStreakMilestones,
		SettingKeyPlayArenaEnabled,
		SettingKeyPlayArenaSettlementRewards,
		SettingKeyPlayBlindboxEnabled,
		SettingKeyPlayBlindboxCost,
		SettingKeyPlayBlindboxPoolJSON,
		SettingKeyPlayBlindboxDailyLimit,
		SettingKeyPlayQuizEnabled,
		SettingKeyPlayQuizRewardPerCorrect,
		SettingKeyPlayQuizQuestionsPerDay,
		SettingKeyPlayAgentTeamEnabled,
		SettingKeyPublicModelsEnabled,
		SettingKeyPlayRechargeBoostEnabled,
		SettingKeyPlayRechargeBoostDurationHours,
		SettingKeyPlayRechargeBoostCheckinMult,
		SettingKeyPlayRechargeBoostBlindboxExtra,
		SettingKeyPlayRechargeBoostArenaMult,
		SettingKeyPlayVIPTiers,
		SettingKeyPlayTeamAffiliateEnabled,
		SettingKeyPlayTeamAffiliateTokenThreshold,
		SettingKeyPlayTeamAffiliateCaptainBonus,
		SettingKeyPlayTeamSharedRewardEnabled,
		SettingKeyPlayTeamSharedRewardTiers,
		SettingKeyPlayTeamSharedRewardCap,
		SettingKeyPlayTeamSharedRewardStartMonth,
		SettingKeyPlayCampaignsEnabled,
		SettingKeyImageStudioEnabled,
		SettingKeyPlayDailyQuestsEnabled,
		SettingKeyPlayDailyArenaEnabled,
		SettingKeyPlayDailyQuests,
		SettingKeyPlayDailyArenaTopRewards,
	}
	vals, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return PlayRuntime{}
	}
	reward, _ := strconv.ParseFloat(vals[SettingKeyPlayCheckinDailyReward], 64)
	if reward <= 0 {
		reward = 0.5
	}
	blindboxCost, _ := strconv.ParseFloat(vals[SettingKeyPlayBlindboxCost], 64)
	if blindboxCost <= 0 {
		blindboxCost = 0.5
	}
	blindboxLimit, _ := strconv.Atoi(vals[SettingKeyPlayBlindboxDailyLimit])
	if blindboxLimit <= 0 {
		blindboxLimit = 10
	}
	quizReward, _ := strconv.ParseFloat(vals[SettingKeyPlayQuizRewardPerCorrect], 64)
	if quizReward <= 0 {
		quizReward = 0.1
	}
	quizCount, _ := strconv.Atoi(vals[SettingKeyPlayQuizQuestionsPerDay])
	if quizCount <= 0 {
		quizCount = 5
	}
	blindboxPool, blindboxPoolDiagnostic := parseBlindboxPool(vals[SettingKeyPlayBlindboxPoolJSON])
	if blindboxPoolDiagnostic != nil {
		logger.FromContext(ctx).Warn(
			"invalid play blindbox pool configuration; using approved default",
			zap.String("setting_key", SettingKeyPlayBlindboxPoolJSON),
			zap.String("reason", blindboxPoolDiagnostic.Reason),
		)
	}
	teamRewardConfig, teamRewardDiagnostic := parseTeamRewardConfig(
		vals[SettingKeyPlayTeamSharedRewardEnabled],
		vals[SettingKeyPlayTeamSharedRewardTiers],
		vals[SettingKeyPlayTeamSharedRewardCap],
	)
	if teamRewardDiagnostic != nil {
		logger.FromContext(ctx).Warn(
			"invalid play team shared reward configuration; using approved default",
			zap.String("setting_key", teamRewardDiagnostic.SettingKey),
			zap.String("reason", teamRewardDiagnostic.Reason),
		)
	}
	teamRewardStartMonth, teamRewardStartMonthDiagnostic := parseTeamRewardStartMonth(
		vals[SettingKeyPlayTeamSharedRewardStartMonth],
	)
	if teamRewardStartMonthDiagnostic != nil {
		logger.FromContext(ctx).Warn(
			"invalid play team shared reward start month; using empty value",
			zap.String("setting_key", teamRewardStartMonthDiagnostic.SettingKey),
			zap.String("reason", teamRewardStartMonthDiagnostic.Reason),
		)
	}
	return PlayRuntime{
		CheckinEnabled:              vals[SettingKeyPlayCheckinEnabled] == "true",
		CheckinReward:               reward,
		CheckinMakeupEnabled:        vals[SettingKeyPlayCheckinMakeupEnabled] != "false",
		StreakMilestones:            parsePlayStreakMilestones(vals[SettingKeyPlayCheckinStreakMilestones]),
		ArenaEnabled:                vals[SettingKeyPlayArenaEnabled] == "true",
		ArenaSettlementRewards:      parseArenaSettlementRewards(vals[SettingKeyPlayArenaSettlementRewards]),
		BlindboxEnabled:             vals[SettingKeyPlayBlindboxEnabled] == "true",
		BlindboxCost:                blindboxCost,
		BlindboxPool:                blindboxPool,
		BlindboxDailyLimit:          blindboxLimit,
		QuizEnabled:                 vals[SettingKeyPlayQuizEnabled] == "true",
		QuizRewardPerCorrect:        quizReward,
		QuizQuestionsPerDay:         quizCount,
		AgentTeamEnabled:            vals[SettingKeyPlayAgentTeamEnabled] == "true",
		PublicModelsEnabled:         vals[SettingKeyPublicModelsEnabled] == "true",
		RechargeBoostEnabled:        vals[SettingKeyPlayRechargeBoostEnabled] == "true",
		RechargeBoostDurationHours:  parsePositiveIntSetting(vals[SettingKeyPlayRechargeBoostDurationHours], 24),
		RechargeBoostCheckinMult:    parsePositiveFloatSetting(vals[SettingKeyPlayRechargeBoostCheckinMult], 2),
		RechargeBoostBlindboxExtra:  parsePositiveIntSetting(vals[SettingKeyPlayRechargeBoostBlindboxExtra], 1),
		RechargeBoostArenaMult:      parsePositiveFloatSetting(vals[SettingKeyPlayRechargeBoostArenaMult], 1.5),
		VIPTiers:                    parsePlayVIPTiers(vals[SettingKeyPlayVIPTiers]),
		TeamAffiliateEnabled:        vals[SettingKeyPlayTeamAffiliateEnabled] == "true",
		TeamAffiliateTokenThreshold: parsePositiveInt64Setting(vals[SettingKeyPlayTeamAffiliateTokenThreshold], 1_000_000),
		TeamAffiliateCaptainBonus:   parsePositiveFloatSetting(vals[SettingKeyPlayTeamAffiliateCaptainBonus], 5),
		TeamSharedRewardEnabled:     teamRewardConfig.Enabled,
		TeamSharedRewardTiers:       teamRewardConfig.Tiers,
		TeamSharedRewardCap:         teamRewardConfig.Cap,
		TeamSharedRewardStartMonth:  teamRewardStartMonth,
		CampaignsEnabled:            vals[SettingKeyPlayCampaignsEnabled] == "true",
		ImageStudioEnabled:          vals[SettingKeyImageStudioEnabled] == "true",
		DailyQuestsEnabled:          vals[SettingKeyPlayDailyQuestsEnabled] == "true",
		DailyArenaEnabled:           vals[SettingKeyPlayDailyArenaEnabled] == "true",
		DailyQuests:                 parsePlayDailyQuests(vals[SettingKeyPlayDailyQuests]),
		DailyArenaTopRewards:        parsePlayDailyArenaRewards(vals[SettingKeyPlayDailyArenaTopRewards]),
	}
}

// GetPublicModelRateMultiplier returns the public /models reference multiplier (default 1).
func (s *SettingService) GetPublicModelRateMultiplier(ctx context.Context) float64 {
	if s == nil || s.settingRepo == nil {
		return 1
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyPublicModelRateMultiplier})
	if err != nil {
		return 1
	}
	mult, err := strconv.ParseFloat(strings.TrimSpace(vals[SettingKeyPublicModelRateMultiplier]), 64)
	if err != nil || mult <= 0 {
		return 1
	}
	return mult
}

// GetBalanceRechargeMultiplier returns the configured payment balance credit multiplier.
func (s *SettingService) GetBalanceRechargeMultiplier(ctx context.Context) float64 {
	if s == nil || s.settingRepo == nil {
		return defaultBalanceRechargeMultiplier
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{SettingBalanceRechargeMult})
	if err != nil {
		return defaultBalanceRechargeMultiplier
	}
	multiplier, _ := strconv.ParseFloat(vals[SettingBalanceRechargeMult], 64)
	return normalizeBalanceRechargeMultiplier(multiplier)
}
