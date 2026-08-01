package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const arenaMonthlySettlementDelay = 10 * time.Minute

func isArenaPeriodSettlementDue(period *PlayArenaPeriod, now time.Time) bool {
	if period == nil || period.PeriodType != "monthly" || period.EndAt.IsZero() {
		return false
	}
	return !now.Before(period.EndAt.Add(arenaMonthlySettlementDelay))
}

// PrepareCurrentArenaMonthlySeason creates the current Shanghai-month period
// and freezes its reward rules before users can be ranked against it. The
// runner calls this independently of page traffic, so a late first visit
// cannot let an operator rewrite an already-running season.
func (s *PlayService) PrepareCurrentArenaMonthlySeason(ctx context.Context, now time.Time) (*PlayArenaPeriod, error) {
	rt := s.GetRuntime(ctx)
	if !rt.ArenaEnabled {
		return nil, nil
	}
	return s.ensureMonthlyArenaPeriod(ctx, now, rt)
}

func (s *PlayService) ensureMonthlyArenaPeriod(ctx context.Context, now time.Time, rt PlayRuntime) (*PlayArenaPeriod, error) {
	period, err := s.repo.EnsureMonthlyArenaPeriod(ctx, now)
	if err != nil || period == nil {
		return period, err
	}
	repo, ok := s.repo.(PlayArenaSeasonRulesWriter)
	if !ok {
		return period, nil
	}
	raw, err := json.Marshal(rt.ArenaSettlementRewards)
	if err != nil {
		return nil, fmt.Errorf("marshal arena reward rules: %w", err)
	}
	if err := repo.FreezeArenaRewardRules(ctx, period.ID, string(raw)); err != nil {
		return nil, err
	}
	return period, nil
}

// getExistingMonthlyArenaPeriod is read-only. Monthly period creation and
// reward-rule freezing belong exclusively to PrepareCurrentArenaMonthlySeason,
// which is invoked by startup/scheduler paths.
func (s *PlayService) getExistingMonthlyArenaPeriod(ctx context.Context, now time.Time) (*PlayArenaPeriod, error) {
	period, err := s.repo.GetActiveArenaPeriod(ctx, now)
	if err != nil {
		return nil, err
	}
	if period != nil && period.PeriodType != "monthly" {
		return nil, nil
	}
	return period, nil
}

func (s *PlayService) arenaRewardTiersForPeriod(ctx context.Context, period *PlayArenaPeriod) ([]PlayArenaSettlementTier, error) {
	if period != nil && period.PeriodType == "monthly" {
		repo, ok := s.repo.(PlayArenaSeasonRulesReader)
		if !ok {
			return []PlayArenaSettlementTier{}, nil
		}
		raw, err := repo.GetArenaRewardRulesSnapshot(ctx, period.ID)
		if err != nil {
			return nil, err
		}
		if raw == "" || raw == "[]" || raw == "null" {
			return []PlayArenaSettlementTier{}, nil
		}
		var tiers []PlayArenaSettlementTier
		if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
			return nil, fmt.Errorf("decode frozen arena reward rules: %w", err)
		}
		if len(tiers) == 0 {
			return []PlayArenaSettlementTier{}, nil
		}
		return tiers, nil
	}
	if period == nil {
		return []PlayArenaSettlementTier{}, nil
	}
	return append([]PlayArenaSettlementTier(nil), s.GetRuntime(ctx).ArenaSettlementRewards...), nil
}

// GetArenaSeasonOverview loads one selected arena tab. Public ranking rows are
// deliberately stripped of user IDs; an authenticated viewer only receives an
// is_mine marker and their own private progress in Current. Callers rendering
// the first screen can defer immutable history with includeHistory=false.
// Keeping this optional preserves the original call contract for native and
// older web clients while allowing the interactive view to avoid its second
// aggregate query until the user asks for season proof.
func (s *PlayService) GetArenaSeasonOverview(ctx context.Context, userID int64, periodType string, includeHistoryOption ...bool) (*PlayArenaSeasonOverview, error) {
	includeHistory := true
	if len(includeHistoryOption) > 0 {
		includeHistory = includeHistoryOption[0]
	}
	if periodType != "daily" {
		periodType = "monthly"
	}
	rt := s.GetRuntime(ctx)
	out := &PlayArenaSeasonOverview{Rows: []PlayArenaScoreRow{}, History: []PlayArenaSeasonHistory{}}

	var (
		current *PlayArenaCurrent
		rows    []PlayArenaScoreRow
		period  *PlayArenaPeriod
		err     error
	)
	if periodType == "daily" {
		out.Enabled = rt.ArenaEnabled && rt.DailyArenaEnabled
		if !out.Enabled {
			return out, nil
		}
		current, err = s.GetDailyArenaCurrent(ctx, userID)
		if err != nil {
			return nil, err
		}
		rows, period, err = s.ListDailyArenaLeaderboard(ctx, 50)
		out.RewardTiers = append([]PlayArenaSettlementTier(nil), rt.DailyArenaTopRewards...)
	} else {
		out.Enabled = rt.ArenaEnabled
		if !out.Enabled {
			return out, nil
		}
		current, err = s.GetArenaCurrent(ctx, userID)
		if err != nil {
			return nil, err
		}
		rows, period, err = s.ListArenaLeaderboard(ctx, 50)
		if err == nil {
			out.RewardTiers, err = s.arenaRewardTiersForPeriod(ctx, period)
		}
	}
	if err != nil {
		return nil, err
	}
	out.Period = period
	out.Current = current
	out.Rows = make([]PlayArenaScoreRow, 0, len(rows))
	for _, row := range rows {
		row.IsMine = userID > 0 && row.UserID == userID
		row.UserID = 0
		row.Email = ""
		out.Rows = append(out.Rows, row)
	}
	if includeHistory {
		if periodType == "daily" {
			history, err := s.dailyArenaSeasonHistory(ctx)
			if err != nil {
				return nil, err
			}
			out.History = history
		} else if repo, ok := s.repo.(PlayArenaSeasonHistoryRepository); ok {
			history, err := repo.ListArenaSeasonHistory(ctx, periodType, 6)
			if err != nil {
				return nil, err
			}
			out.History = history
		}
	}
	return out, nil
}

// dailyArenaSeasonHistory preserves the existing daily payout proof in the
// new aggregate response. Daily results already live in the immutable reward
// ledger, whereas monthly seasons additionally receive dedicated snapshots.
func (s *PlayService) dailyArenaSeasonHistory(ctx context.Context) ([]PlayArenaSeasonHistory, error) {
	latest, err := s.repo.GetLatestSettledDailyArenaPeriod(ctx)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return []PlayArenaSeasonHistory{}, nil
	}
	recent, err := s.buildDailyArenaRecentRewardSummary(ctx, latest)
	if err != nil {
		return nil, err
	}
	if recent == nil {
		return []PlayArenaSeasonHistory{}, nil
	}
	item := PlayArenaSeasonHistory{
		Period:       *latest,
		WinnersCount: recent.WinnersCount,
		TotalAmount:  recent.TotalAmount,
		Winners:      make([]PlayArenaSeasonSnapshot, 0, len(recent.Winners)),
	}
	for _, winner := range recent.Winners {
		item.Winners = append(item.Winners, PlayArenaSeasonSnapshot{
			Rank:         winner.Rank,
			DisplayName:  winner.DisplayName,
			Anonymous:    winner.Anonymous,
			AvatarURL:    winner.AvatarURL,
			TokenSum:     winner.TokenSum,
			RewardAmount: winner.Amount,
			PayoutStatus: "paid",
			PaidAt:       recent.SettledAt,
		})
	}
	return []PlayArenaSeasonHistory{item}, nil
}

// SettleExpiredMonthlyArenaPeriods is restart-safe because each individual
// reward remains keyed by a deterministic ledger idempotency key.
func (s *PlayService) SettleExpiredMonthlyArenaPeriods(ctx context.Context, now time.Time) (int, error) {
	repo, ok := s.repo.(PlayArenaSeasonSettlementRepository)
	if !ok {
		return 0, nil
	}
	periods, err := repo.ListExpiredActiveMonthlyArenaPeriods(ctx, now)
	if err != nil {
		return 0, err
	}
	settled := 0
	var settleErr error
	for i := range periods {
		period := periods[i]
		if !isArenaPeriodSettlementDue(&period, now) {
			continue
		}
		if _, err := s.SettleArenaPeriod(ctx, period.ID); err != nil {
			settleErr = errors.Join(settleErr, fmt.Errorf("settle arena period %d: %w", period.ID, err))
			continue
		}
		settled++
	}
	return settled, settleErr
}
