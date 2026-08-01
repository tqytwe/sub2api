package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (s *PlayService) SettleArenaPeriod(ctx context.Context, periodID int64) (*PlayArenaSettlementResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.ArenaEnabled {
		return nil, ErrPlayFeatureDisabled
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
	if period.PeriodType == "monthly" {
		// Monthly results are intentionally held until 00:10 Shanghai time so
		// in-flight usage writes at the calendar boundary cannot alter the
		// published snapshot. The admin endpoint is a retry path, not a way to
		// publish a running season early.
		if !isArenaPeriodSettlementDue(period, now) {
			return nil, ErrPlayArenaPeriodNotSettleable
		}
	} else if now.Before(period.EndAt) {
		return nil, ErrPlayArenaPeriodNotSettleable
	}
	rewards, err := s.arenaRewardTiersForPeriod(ctx, period)
	if err != nil {
		return nil, err
	}
	if len(rewards) == 0 {
		return nil, fmt.Errorf("arena settlement rewards not configured")
	}

	maxRank := rewards[len(rewards)-1].RankMax
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
	seasonRepo, canSnapshot := s.repo.(PlayArenaSeasonSettlementRepository)
	for _, row := range rows {
		amount := arenaRewardForRank(row.Rank, rewards)
		if amount <= 0 {
			continue
		}
		idempotencyKey := fmt.Sprintf("arena_settlement:%d:%d", period.ID, row.UserID)
		paidAt := s.serverNow()
		if err := s.grantBalance(ctx, row.UserID, amount, PlayRewardSourceArenaSettlement, idempotencyKey, map[string]any{
			"period_id":    period.ID,
			"period_name":  period.Name,
			"period_type":  "monthly",
			"period_start": period.StartAt.Format(time.RFC3339),
			"period_end":   period.EndAt.Format(time.RFC3339),
			"rank":         row.Rank,
			"token_sum":    row.TokenSum,
		}, func(txCtx context.Context) error {
			if !canSnapshot {
				return nil
			}
			return seasonRepo.CreateArenaSeasonSnapshot(txCtx, PlayArenaSeasonSnapshot{
				PeriodID:     period.ID,
				Rank:         row.Rank,
				UserID:       row.UserID,
				TokenSum:     row.TokenSum,
				RewardAmount: amount,
				PayoutStatus: "paid",
				PaidAt:       &paidAt,
			})
		}); err != nil {
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
