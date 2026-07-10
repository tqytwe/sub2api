package service

import (
	"context"
	"errors"
	"fmt"
)

func (s *PlayService) SettleArenaPeriod(ctx context.Context, periodID int64) (*PlayArenaSettlementResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.ArenaEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if len(rt.ArenaSettlementRewards) == 0 {
		return nil, fmt.Errorf("arena settlement rewards not configured")
	}

	now := s.serverNow()
	var period *PlayArenaPeriod
	var err error
	if periodID > 0 {
		period, err = s.repo.GetArenaPeriodByID(ctx, periodID)
	} else {
		period, err = s.repo.GetActiveArenaPeriod(ctx, now)
	}
	if err != nil {
		return nil, err
	}
	if period == nil {
		return nil, ErrPlayArenaNoPeriod
	}
	if period.Status != "active" {
		return nil, ErrPlayArenaPeriodNotSettleable
	}
	if periodID <= 0 && period.EndAt.After(now) {
		return nil, ErrPlayArenaPeriodNotSettleable
	}

	maxRank := rt.ArenaSettlementRewards[len(rt.ArenaSettlementRewards)-1].RankMax
	if maxRank <= 0 {
		maxRank = 10
	}
	rows, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, maxRank)
	if err != nil {
		return nil, err
	}

	result := &PlayArenaSettlementResult{
		PeriodID:   period.ID,
		PeriodName: period.Name,
	}
	for _, row := range rows {
		amount := arenaRewardForRank(row.Rank, rt.ArenaSettlementRewards)
		if amount <= 0 {
			continue
		}
		idempotencyKey := fmt.Sprintf("arena_settlement:%d:%d", period.ID, row.UserID)
		if err := s.grantBalance(ctx, row.UserID, amount, PlayRewardSourceArenaSettlement, idempotencyKey, map[string]any{
			"period_id":   period.ID,
			"period_name": period.Name,
			"rank":        row.Rank,
			"token_sum":   row.TokenSum,
		}, nil); err != nil {
			if errors.Is(err, ErrPlayRewardDuplicate) {
				continue
			}
			return nil, err
		}
		result.WinnersCount++
		result.TotalAwarded += amount
	}

	if err := s.repo.MarkArenaPeriodSettled(ctx, period.ID); err != nil {
		return nil, err
	}
	return result, nil
}

func arenaRewardForRank(rank int, tiers []PlayArenaSettlementTier) float64 {
	if rank <= 0 {
		return 0
	}
	for _, tier := range tiers {
		if rank <= tier.RankMax {
			return tier.Amount
		}
	}
	return 0
}
