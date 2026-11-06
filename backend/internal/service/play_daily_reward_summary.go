package service

import (
	"context"
	"sort"
	"time"
)

const maxPublicDailyRewardRows = 10

func (s *PlayService) GetDailyArenaRewardSummary(ctx context.Context) (*PlayArenaDailyRewardSummary, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayArenaDailyRewardSummary{Enabled: rt.ArenaEnabled && rt.DailyArenaEnabled}
	if !out.Enabled || s.repo == nil {
		return out, nil
	}

	latest, err := s.repo.GetLatestSettledDailyArenaPeriod(ctx)
	if err != nil {
		return nil, err
	}
	if latest != nil {
		recent, err := s.buildDailyArenaRecentRewardSummary(ctx, latest)
		if err != nil {
			return nil, err
		}
		out.Recent = recent
	}

	current, err := s.ensureDailyArenaPeriod(ctx, s.serverNow(), rt)
	if err != nil {
		return nil, err
	}
	if current != nil {
		policy, err := s.dailyArenaPolicyForPeriod(ctx, current)
		if err != nil {
			return nil, err
		}
		rewards := policy.Rewards
		limit := rewardTierMaxRank(rewards)
		if limit <= 0 || limit > maxPublicDailyRewardRows {
			limit = maxPublicDailyRewardRows
		}
		rows, err := s.repo.ListArenaLeaderboard(ctx, current.StartAt, current.EndAt, limit)
		if err != nil {
			return nil, err
		}
		out.Current = &PlayArenaDailyCurrentRewardEstimate{
			Period: current,
			Rows:   make([]PlayArenaDailyRewardEstimateRow, 0, len(rows)),
		}
		for _, row := range rows {
			out.Current.Rows = append(out.Current.Rows, PlayArenaDailyRewardEstimateRow{
				Rank:            row.Rank,
				DisplayName:     row.DisplayName,
				Anonymous:       row.Anonymous,
				AvatarURL:       row.AvatarURL,
				TokenSum:        row.TokenSum,
				EstimatedReward: dailyArenaRewardForRank(row.Rank, policy),
			})
		}
	}

	return out, nil
}

func (s *PlayService) buildDailyArenaRecentRewardSummary(
	ctx context.Context,
	period *PlayArenaPeriod,
) (*PlayArenaDailyRecentRewardSummary, error) {
	rows, err := s.repo.ListArenaDailyRewardLedger(ctx, period.ID)
	if err != nil {
		return nil, err
	}
	// Missing historical rank/token evidence stays unknown. Current usage may
	// have been cleaned or backfilled after payout and cannot prove that rank.

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Rank > 0 && rows[j].Rank > 0 && rows[i].Rank != rows[j].Rank {
			return rows[i].Rank < rows[j].Rank
		}
		if rows[i].Rank > 0 && rows[j].Rank <= 0 {
			return true
		}
		if rows[i].Rank <= 0 && rows[j].Rank > 0 {
			return false
		}
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.Before(rows[j].CreatedAt)
		}
		return rows[i].UserID < rows[j].UserID
	})

	recent := &PlayArenaDailyRecentRewardSummary{
		Period:       period,
		SettledAt:    period.SettledAt,
		PaidToday:    sameShanghaiDay(period.SettledAt, s.serverNow()),
		WinnersCount: len(rows),
		Winners:      make([]PlayArenaDailyRewardWinner, 0, min(len(rows), maxPublicDailyRewardRows)),
	}
	for idx, row := range rows {
		recent.TotalAmount += row.Amount
		if idx >= maxPublicDailyRewardRows {
			continue
		}
		recent.Winners = append(recent.Winners, PlayArenaDailyRewardWinner{
			Rank:        row.Rank,
			DisplayName: row.DisplayName,
			Anonymous:   row.Anonymous,
			AvatarURL:   row.AvatarURL,
			TokenSum:    row.TokenSum,
			Amount:      row.Amount,
		})
	}
	return recent, nil
}

func rewardTierMaxRank(tiers []PlayArenaSettlementTier) int {
	maxRank := 0
	for _, tier := range tiers {
		if tier.RankMax > maxRank {
			maxRank = tier.RankMax
		}
	}
	return maxRank
}

func sameShanghaiDay(t *time.Time, now time.Time) bool {
	if t == nil {
		return false
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	settled := t.In(loc)
	current := now.In(loc)
	return settled.Year() == current.Year() && settled.Month() == current.Month() && settled.Day() == current.Day()
}
