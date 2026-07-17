package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

func (s *PlayService) MarkQuestCompleted(ctx context.Context, userID int64, questKey string) error {
	if !s.GetRuntime(ctx).DailyQuestsEnabled || userID <= 0 {
		return nil
	}
	now := s.serverNow()
	return s.repo.UpsertQuestProgress(ctx, userID, s.serverDate(now), questKey, true)
}

func (s *PlayService) syncQuestAutoProgress(ctx context.Context, userID int64, defs []PlayDailyQuestDef, progress map[string]PlayQuestProgressRow) error {
	now := s.serverNow()
	day := s.serverDate(now)
	dayEnd := day.AddDate(0, 0, 1)

	for _, def := range defs {
		if progress[def.Key].Completed {
			continue
		}
		switch def.Key {
		case PlayQuestKeyCheckin:
			if s.GetRuntime(ctx).CheckinEnabled {
				done, err := s.repo.HasCheckin(ctx, userID, day)
				if err != nil {
					return err
				}
				if done {
					if err := s.repo.UpsertQuestProgress(ctx, userID, day, def.Key, true); err != nil {
						return err
					}
				}
			}
		case PlayQuestKeyAPICall:
			minTokens := def.MinTokens
			if minTokens <= 0 {
				minTokens = 100
			}
			tokens, err := s.repo.GetUserDailyTokenSum(ctx, userID, day, dayEnd)
			if err != nil {
				return err
			}
			if tokens >= minTokens {
				if err := s.repo.UpsertQuestProgress(ctx, userID, day, def.Key, true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *PlayService) GetQuestsToday(ctx context.Context, userID int64) (*PlayQuestToday, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayQuestToday{Enabled: rt.DailyQuestsEnabled}
	now := s.serverNow()
	out.ServerDate = s.serverDate(now).Format("2006-01-02")
	if !out.Enabled || userID <= 0 {
		return out, nil
	}
	defs := rt.DailyQuests
	if len(defs) == 0 {
		defs = defaultPlayDailyQuests()
	}
	rows, err := s.repo.ListQuestProgress(ctx, userID, s.serverDate(now))
	if err != nil {
		return nil, err
	}
	progress := make(map[string]PlayQuestProgressRow, len(rows))
	for _, row := range rows {
		progress[row.QuestKey] = row
	}
	if err := s.syncQuestAutoProgress(ctx, userID, defs, progress); err != nil {
		return nil, err
	}
	rows, err = s.repo.ListQuestProgress(ctx, userID, s.serverDate(now))
	if err != nil {
		return nil, err
	}
	progress = make(map[string]PlayQuestProgressRow, len(rows))
	for _, row := range rows {
		progress[row.QuestKey] = row
	}
	energy := 0
	out.Tasks = make([]PlayQuestTask, 0, len(defs))
	for _, def := range defs {
		completed := progress[def.Key].Completed
		if !completed && def.Key == PlayQuestKeyImageGenerate {
			count, err := s.repo.CountImageStudioJobsToday(ctx, userID, s.serverDate(now))
			if err != nil {
				return nil, err
			}
			minCount := def.MinCount
			if minCount <= 0 {
				minCount = 1
			}
			if count >= minCount {
				completed = true
				_ = s.repo.UpsertQuestProgress(ctx, userID, s.serverDate(now), def.Key, true)
			}
		}
		if completed {
			energy += def.Energy
		}
		route := def.CTARoute
		if route == "" {
			route = questCTARoute(def.Key)
		}
		out.Tasks = append(out.Tasks, PlayQuestTask{
			Key:       def.Key,
			Completed: completed,
			Energy:    def.Energy,
			CTARoute:  route,
		})
	}
	monthlyTokens := int64(0)
	if cur, err := s.GetArenaCurrent(ctx, userID); err == nil && cur != nil {
		monthlyTokens = cur.TokenSum
	}
	energy += int(monthlyTokens / 10000)
	out.Energy = energy
	out.Level, out.EnergyToNextLevel = computeQuestLevel(energy)
	return out, nil
}

func (s *PlayService) GetDailyArenaCurrent(ctx context.Context, userID int64) (*PlayArenaCurrent, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayArenaCurrent{Enabled: rt.ArenaEnabled && rt.DailyArenaEnabled}
	if !out.Enabled {
		return out, nil
	}
	now := s.serverNow()
	period, err := s.repo.EnsureDailyArenaPeriod(ctx, now)
	if err != nil {
		return nil, err
	}
	if period == nil {
		return out, nil
	}
	out.Period = period
	if userID <= 0 {
		return out, nil
	}
	tokenSum, rank, err := s.repo.GetUserArenaScore(ctx, userID, period.StartAt, period.EndAt)
	if err != nil {
		return nil, err
	}
	out.TokenSum = tokenSum
	out.Rank = rank
	out.EstimatedReward = arenaRewardForRank(rank, rt.DailyArenaTopRewards)
	out.DisplayTokenSum = tokenSum
	if rank > 1 {
		gap, err := s.repo.GetArenaTokensToPrevRank(ctx, userID, period.StartAt, period.EndAt, rank, tokenSum)
		if err != nil {
			return nil, err
		}
		out.TokensToPrevRank = gap
	}
	return out, nil
}

func (s *PlayService) ListDailyArenaLeaderboard(ctx context.Context, limit int) ([]PlayArenaScoreRow, *PlayArenaPeriod, error) {
	rt := s.GetRuntime(ctx)
	if !rt.DailyArenaEnabled || !rt.ArenaEnabled {
		return nil, nil, ErrPlayFeatureDisabled
	}
	now := s.serverNow()
	period, err := s.repo.EnsureDailyArenaPeriod(ctx, now)
	if err != nil {
		return nil, nil, err
	}
	if period == nil {
		return []PlayArenaScoreRow{}, nil, nil
	}
	rows, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, limit)
	if err != nil {
		return nil, nil, err
	}
	return rows, period, nil
}

func (s *PlayService) SettleDailyArenaPeriod(ctx context.Context, periodID int64) (*PlayArenaSettlementResult, error) {
	rt := s.GetRuntime(ctx)
	rewards := rt.DailyArenaTopRewards
	if len(rewards) == 0 {
		rewards = parsePlayDailyArenaRewards("")
	}
	if len(rewards) == 0 {
		return nil, fmt.Errorf("daily arena settlement rewards not configured")
	}
	period, err := s.repo.GetArenaPeriodByID(ctx, periodID)
	if err != nil || period == nil {
		return nil, ErrPlayArenaNoPeriod
	}
	if period.Status != "active" {
		return nil, ErrPlayArenaPeriodNotSettleable
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
	for _, row := range rows {
		amount := arenaRewardForRank(row.Rank, rewards)
		if amount <= 0 {
			continue
		}
		if result.TotalAwarded+amount > playDailyArenaDailyBudgetUSD {
			break
		}
		idempotencyKey := "arena_daily_settlement:" + strconv.FormatInt(period.ID, 10) + ":" + strconv.FormatInt(row.UserID, 10)
		if err := s.grantBalance(ctx, row.UserID, amount, PlayRewardSourceArenaDaily, idempotencyKey, map[string]any{
			"period_id": period.ID,
			"rank":      row.Rank,
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
	_ = rt
	return result, nil
}

func (s *PlayService) SettleExpiredDailyArenaPeriods(ctx context.Context, now time.Time) (int, error) {
	periods, err := s.repo.ListExpiredActiveDailyArenaPeriods(ctx, now)
	if err != nil {
		return 0, err
	}
	settled := 0
	for _, period := range periods {
		if _, err := s.SettleDailyArenaPeriod(ctx, period.ID); err != nil {
			return settled, err
		}
		settled++
	}
	return settled, nil
}
