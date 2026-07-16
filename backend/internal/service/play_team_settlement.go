package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

const PlayRewardSourceTeamSharedReward = "team_shared_reward"

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

	existing, err := s.repo.GetTeamRewardSettlementByTeamPeriod(ctx, teamID, periodStart)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	contributions, err := s.repo.ListTeamRewardContributions(ctx, teamID, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}
	contributions = normalizeTeamContributions(contributions)
	teamSpend := sumTeamContributions(contributions).Round(teamRewardAmountScale)
	pool := resolveTeamRewardPool(teamSpend, cfg)
	if !pool.IsPositive() {
		return nil, nil
	}

	threshold, rate := reachedTeamRewardTier(teamSpend, cfg.Tiers)
	rewardByUser, err := allocateTeamReward(pool, contributions)
	if err != nil {
		return nil, err
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
		return nil, nil
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
	snapshot, _, err := s.repo.CreateTeamRewardSnapshot(ctx, settlement, allocations)
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
