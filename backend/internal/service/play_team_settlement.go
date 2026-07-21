package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

const PlayRewardSourceTeamSharedReward = "team_shared_reward"

func (s *PlayService) GetTeamRewardSettings(ctx context.Context) PlayTeamRewardSettings {
	rt := s.GetRuntime(ctx)
	return PlayTeamRewardSettings{
		Enabled:    rt.TeamSharedRewardEnabled,
		Tiers:      append([]TeamRewardTier(nil), rt.TeamSharedRewardTiers...),
		Cap:        rt.TeamSharedRewardCap,
		StartMonth: rt.TeamSharedRewardStartMonth,
	}
}

func (s *PlayService) UpdateTeamRewardSettings(
	ctx context.Context,
	settings PlayTeamRewardSettings,
) (PlayTeamRewardSettings, error) {
	cfg := TeamRewardConfig{
		Enabled: settings.Enabled,
		Tiers:   append([]TeamRewardTier(nil), settings.Tiers...),
		Cap:     settings.Cap,
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		return PlayTeamRewardSettings{}, err
	}
	startMonth, diagnostic := parseTeamRewardStartMonth(settings.StartMonth)
	if diagnostic != nil || startMonth == "" {
		return PlayTeamRewardSettings{}, fmt.Errorf("team reward start month must use YYYY-MM")
	}
	if s.settingService == nil || s.settingService.settingRepo == nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("team reward settings repository missing")
	}
	tiersJSON, err := json.Marshal(cfg.Tiers)
	if err != nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("marshal team reward tiers: %w", err)
	}
	if err := s.settingService.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyPlayTeamSharedRewardEnabled:    strconv.FormatBool(cfg.Enabled),
		SettingKeyPlayTeamSharedRewardTiers:      string(tiersJSON),
		SettingKeyPlayTeamSharedRewardCap:        cfg.Cap.StringFixed(teamRewardAmountScale),
		SettingKeyPlayTeamSharedRewardStartMonth: startMonth,
	}); err != nil {
		return PlayTeamRewardSettings{}, fmt.Errorf("update team reward settings: %w", err)
	}
	if s.settingService.onUpdate != nil {
		s.settingService.onUpdate()
	}
	settings.StartMonth = startMonth
	settings.Tiers = cfg.Tiers
	settings.Cap = cfg.Cap
	return settings, nil
}

func (s *PlayService) SettleTeamRewardMonth(
	ctx context.Context,
	teamID int64,
	month time.Time,
) (*PlayTeamSettlement, error) {
	rt := s.GetRuntime(ctx)
	cfg := TeamRewardConfig{
		Enabled: rt.TeamSharedRewardEnabled,
		Cap:     rt.TeamSharedRewardCap,
		Tiers:   append([]TeamRewardTier(nil), rt.TeamSharedRewardTiers...),
	}
	return s.settleTeamRewardMonth(ctx, teamID, month, cfg)
}

func (s *PlayService) settleTeamRewardMonth(
	ctx context.Context,
	teamID int64,
	month time.Time,
	cfg TeamRewardConfig,
) (*PlayTeamSettlement, error) {
	if teamID <= 0 {
		return nil, fmt.Errorf("team reward settlement team ID must be positive")
	}
	if !cfg.Enabled {
		return nil, nil
	}
	if err := validateTeamRewardConfig(cfg); err != nil {
		return nil, fmt.Errorf("team reward settlement config: %w", err)
	}

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, fmt.Errorf("load team reward timezone: %w", err)
	}
	localMonth := month.In(location)
	windowStart := time.Date(localMonth.Year(), localMonth.Month(), 1, 0, 0, 0, 0, location)
	windowEnd := windowStart.AddDate(0, 1, 0)
	periodStart := time.Date(localMonth.Year(), localMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

	var snapshot *PlayTeamSettlement
	err = s.repo.WithTeamRewardSnapshotLock(ctx, teamID, func(lockCtx context.Context) error {
		existing, loadErr := s.repo.GetTeamRewardSettlementByTeamPeriod(lockCtx, teamID, periodStart)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			snapshot = existing
			return nil
		}

		contributions, loadErr := s.repo.ListTeamRewardContributions(lockCtx, teamID, windowStart, windowEnd)
		if loadErr != nil {
			return loadErr
		}
		contributions = normalizeTeamContributions(contributions)
		teamSpend := sumTeamContributions(contributions).Round(teamRewardAmountScale)
		pool := resolveTeamRewardPool(teamSpend, cfg)
		if !pool.IsPositive() {
			return nil
		}

		threshold, rate := reachedTeamRewardTier(teamSpend, cfg.Tiers)
		rewardByUser, allocationErr := allocateTeamReward(pool, contributions)
		if allocationErr != nil {
			return allocationErr
		}
		allocations := make([]PlayTeamRewardAllocation, 0, len(contributions))
		periodKey := periodStart.Format("2006-01")
		for _, contribution := range contributions {
			reward := rewardByUser[contribution.UserID].Round(teamRewardAmountScale)
			if !reward.IsPositive() {
				continue
			}
			allocations = append(allocations, PlayTeamRewardAllocation{
				UserID:         contribution.UserID,
				Contribution:   contribution.Amount.Round(teamRewardAmountScale),
				Ratio:          contribution.Amount.Div(teamSpend).Round(teamRewardAmountScale),
				RewardAmount:   reward,
				PayoutStatus:   PlayTeamRewardAllocationStatusPending,
				IdempotencyKey: fmt.Sprintf("team_reward:%d:%s:%d", teamID, periodKey, contribution.UserID),
			})
		}
		if len(allocations) == 0 {
			return nil
		}

		settlement := PlayTeamSettlement{
			TeamID:           teamID,
			PeriodStart:      periodStart,
			WindowStart:      windowStart,
			WindowEnd:        windowEnd,
			TeamSpend:        teamSpend,
			ReachedThreshold: threshold.Round(teamRewardAmountScale),
			RewardRate:       rate.Round(teamRewardAmountScale),
			PoolAmount:       pool.Round(teamRewardAmountScale),
			CapAmount:        cfg.Cap.Round(teamRewardAmountScale),
			Status:           PlayTeamSettlementStatusPending,
		}
		created, _, createErr := s.repo.CreateTeamRewardSnapshot(lockCtx, settlement, allocations)
		if createErr != nil {
			return createErr
		}
		snapshot = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *PlayService) PayoutTeamRewardSettlement(
	ctx context.Context,
	settlementID int64,
) (*PlayTeamSettlement, error) {
	settlement, err := s.repo.GetTeamRewardSettlement(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	if settlement == nil {
		return nil, fmt.Errorf("team reward settlement %d not found", settlementID)
	}
	if settlement.Status == PlayTeamSettlementStatusCompleted {
		return settlement, nil
	}
	if err := s.repo.MarkTeamRewardSettlementProcessing(ctx, settlementID); err != nil {
		return nil, err
	}

	allocations, err := s.repo.ListUnpaidTeamRewardAllocations(ctx, settlementID)
	if err != nil {
		return nil, err
	}
	var payoutErr error
	for _, allocation := range allocations {
		if allocation.UserID <= 0 || !allocation.RewardAmount.IsPositive() {
			continue
		}
		claimed, err := s.repo.ClaimTeamRewardAllocation(ctx, allocation.ID)
		if err != nil {
			payoutErr = errors.Join(payoutErr, err)
			continue
		}
		if !claimed {
			continue
		}
		amount, _ := allocation.RewardAmount.Float64()
		err = s.grantBalance(
			ctx,
			allocation.UserID,
			amount,
			PlayRewardSourceTeamSharedReward,
			allocation.IdempotencyKey,
			map[string]any{
				"team_id":       settlement.TeamID,
				"settlement_id": settlement.ID,
				"period":        settlement.PeriodStart.Format("2006-01"),
			},
			func(txCtx context.Context) error {
				return s.repo.MarkTeamRewardAllocationPaid(txCtx, allocation.ID)
			},
		)
		if err != nil {
			if markErr := s.repo.MarkTeamRewardAllocationFailed(ctx, allocation.ID, err.Error()); markErr != nil {
				err = errors.Join(err, markErr)
			}
			payoutErr = errors.Join(payoutErr, err)
		}
	}

	refreshed, refreshErr := s.repo.RefreshTeamRewardSettlementStatus(ctx, settlementID)
	if refreshErr != nil {
		return nil, errors.Join(payoutErr, refreshErr)
	}
	return refreshed, payoutErr
}

func (s *PlayService) SettleDueTeamRewardMonths(ctx context.Context, now time.Time) (int, error) {
	settings := s.GetTeamRewardSettings(ctx)
	if !settings.Enabled || settings.StartMonth == "" {
		return 0, nil
	}
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return 0, fmt.Errorf("load team reward timezone: %w", err)
	}
	currentMonth := time.Date(now.In(location).Year(), now.In(location).Month(), 1, 0, 0, 0, 0, location)
	period := currentMonth.AddDate(0, -1, 0)
	if period.Format("2006-01") < settings.StartMonth {
		return 0, nil
	}
	windowEnd := currentMonth
	teamIDs, err := s.repo.ListTeamIDsForRewardMonth(ctx, period, windowEnd)
	if err != nil {
		return 0, err
	}
	cfg := TeamRewardConfig{
		Enabled: settings.Enabled,
		Tiers:   settings.Tiers,
		Cap:     settings.Cap,
	}
	settled := 0
	var settleErr error
	for _, teamID := range teamIDs {
		snapshot, err := s.settleTeamRewardMonth(ctx, teamID, period, cfg)
		if err != nil {
			settleErr = errors.Join(settleErr, err)
			continue
		}
		if snapshot == nil {
			continue
		}
		if _, err := s.PayoutTeamRewardSettlement(ctx, snapshot.ID); err != nil {
			settleErr = errors.Join(settleErr, err)
			continue
		}
		settled++
	}
	return settled, settleErr
}

func (s *PlayService) ListUserTeamRewardSettlements(
	ctx context.Context,
	userID int64,
	limit int,
) ([]PlayUserTeamSettlementRecord, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	return s.repo.ListUserTeamRewardSettlements(ctx, userID, limit)
}

func (s *PlayService) ListAdminTeamRewardSettlements(
	ctx context.Context,
	limit int,
) ([]PlayTeamSettlementRecord, error) {
	settlements, err := s.repo.ListTeamRewardSettlements(ctx, limit)
	if err != nil {
		return nil, err
	}
	return s.attachTeamRewardAllocations(ctx, settlements)
}

func (s *PlayService) listTeamRewardSettlementRecords(
	ctx context.Context,
	teamID int64,
	limit int,
) ([]PlayTeamSettlementRecord, error) {
	settlements, err := s.repo.ListTeamRewardSettlementsByTeam(ctx, teamID, limit)
	if err != nil {
		return nil, err
	}
	return s.attachTeamRewardAllocations(ctx, settlements)
}

func (s *PlayService) attachTeamRewardAllocations(
	ctx context.Context,
	settlements []PlayTeamSettlement,
) ([]PlayTeamSettlementRecord, error) {
	records := make([]PlayTeamSettlementRecord, 0, len(settlements))
	for _, settlement := range settlements {
		allocations, err := s.repo.ListTeamRewardAllocations(ctx, settlement.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, PlayTeamSettlementRecord{
			Settlement:  settlement,
			Allocations: allocations,
		})
	}
	return records, nil
}

func normalizeTeamContributions(contributions []TeamContribution) []TeamContribution {
	amountByUser := make(map[int64]decimal.Decimal, len(contributions))
	for _, contribution := range contributions {
		if contribution.UserID <= 0 || !contribution.Amount.IsPositive() {
			continue
		}
		amountByUser[contribution.UserID] = amountByUser[contribution.UserID].Add(contribution.Amount)
	}
	out := make([]TeamContribution, 0, len(amountByUser))
	for userID, amount := range amountByUser {
		out = append(out, TeamContribution{UserID: userID, Amount: amount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
	return out
}

func sumTeamContributions(contributions []TeamContribution) decimal.Decimal {
	total := decimal.Zero
	for _, contribution := range contributions {
		total = total.Add(contribution.Amount)
	}
	return total
}

func reachedTeamRewardTier(
	teamSpend decimal.Decimal,
	tiers []TeamRewardTier,
) (decimal.Decimal, decimal.Decimal) {
	threshold := decimal.Zero
	rate := decimal.Zero
	for _, tier := range tiers {
		if teamSpend.LessThan(tier.Threshold) {
			break
		}
		threshold = tier.Threshold
		rate = tier.Rate
	}
	return threshold, rate
}
