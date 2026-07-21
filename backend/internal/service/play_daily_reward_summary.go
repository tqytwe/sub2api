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

	rewards := rt.DailyArenaTopRewards
	if len(rewards) == 0 {
		rewards = parsePlayDailyArenaRewards("")
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

	current, err := s.repo.EnsureDailyArenaPeriod(ctx, s.serverNow())
	if err != nil {
		return nil, err
	}
	if current != nil {
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
				UserID:          row.UserID,
				DisplayName:     row.DisplayName,
				AvatarURL:       row.AvatarURL,
				TokenSum:        row.TokenSum,
				EstimatedReward: arenaRewardForRank(row.Rank, rewards),
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
	if needsDailyRewardLeaderboardFallback(rows) {
		leaderboard, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, max(len(rows), maxPublicDailyRewardRows))
		if err != nil {
			return nil, err
		}
		fillDailyRewardLedgerFallback(rows, leaderboard)
	}

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
			UserID:      row.UserID,
			DisplayName: row.DisplayName,
			AvatarURL:   row.AvatarURL,
			TokenSum:    row.TokenSum,
			Amount:      row.Amount,
		})
	}
	return recent, nil
}

func needsDailyRewardLeaderboardFallback(rows []PlayArenaDailyRewardLedgerRow) bool {
	for _, row := range rows {
		if row.Rank <= 0 || row.TokenSum <= 0 {
			return true
		}
	}
	return false
}

func fillDailyRewardLedgerFallback(rows []PlayArenaDailyRewardLedgerRow, leaderboard []PlayArenaScoreRow) {
	byUser := make(map[int64]PlayArenaScoreRow, len(leaderboard))
	for _, row := range leaderboard {
		byUser[row.UserID] = row
	}
	for i := range rows {
		score, ok := byUser[rows[i].UserID]
		if !ok {
			continue
		}
		if rows[i].Rank <= 0 {
			rows[i].Rank = score.Rank
		}
		if rows[i].TokenSum <= 0 {
			rows[i].TokenSum = score.TokenSum
		}
		if rows[i].DisplayName == "" {
			rows[i].DisplayName = score.DisplayName
		}
		if rows[i].AvatarURL == "" {
			rows[i].AvatarURL = score.AvatarURL
		}
	}
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
