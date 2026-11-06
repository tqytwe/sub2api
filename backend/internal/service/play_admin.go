package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

func (s *PlayService) GetAdminOpsSummary(ctx context.Context) (*PlayAdminOpsSummary, error) {
	start, end, err := currentTeamRewardWindow(s.serverNow())
	if err != nil {
		return nil, err
	}
	totalTeams, activeTeams, err := s.repo.CountAdminTeams(ctx)
	if err != nil {
		return nil, err
	}
	spends, err := s.repo.ListAdminTeamMonthlySpends(ctx, start, end)
	if err != nil {
		return nil, err
	}
	pendingFailed, err := s.repo.CountTeamRewardSettlementsNeedingAttention(ctx)
	if err != nil {
		return nil, err
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	monthSpend := decimal.Zero
	estimatedPool := decimal.Zero
	for _, spend := range spends {
		monthSpend = monthSpend.Add(spend)
		estimatedPool = estimatedPool.Add(resolveTeamRewardPool(spend, cfg))
	}
	rt := s.GetRuntime(ctx)
	return &PlayAdminOpsSummary{
		TotalTeams:               totalTeams,
		ActiveTeams:              activeTeams,
		MonthSpend:               monthSpend.Round(teamRewardAmountScale),
		EstimatedSharedPool:      estimatedPool.Round(teamRewardAmountScale),
		PendingFailedSettlements: pendingFailed,
		MonthlyArenaRewardBudget: sumArenaRewardBudget(rt.ArenaSettlementRewards),
		DailyArenaRewardBudget:   sumArenaRewardBudget(rt.DailyArenaTopRewards),
	}, nil
}

func (s *PlayService) ListAdminTeams(
	ctx context.Context,
	status string,
	query string,
	page int,
	pageSize int,
) (*PlayAdminTeamList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	start, end, err := currentTeamRewardWindow(s.serverNow())
	if err != nil {
		return nil, err
	}
	items, total, err := s.repo.ListAdminTeams(ctx, status, query, start, end, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	for i := range items {
		items[i].EstimatedPool = resolveTeamRewardPool(items[i].TeamSpend, cfg)
	}
	return &PlayAdminTeamList{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *PlayService) GetAdminTeamDetail(ctx context.Context, teamID int64) (*PlayAdminTeamDetail, error) {
	meta, err := s.repo.GetAdminTeamMeta(ctx, teamID)
	if err != nil || meta == nil {
		return nil, err
	}
	cfg := s.currentCompetitionRewardConfig(ctx)
	meta.EstimatedPool = resolveTeamRewardPool(meta.TeamSpend, cfg)

	summary, err := s.buildTeamSummaryByID(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = &PlayTeamSummary{
			ID:            meta.ID,
			Name:          meta.Name,
			InviteCode:    meta.InviteCode,
			CaptainID:     meta.CaptainID,
			MemberCount:   meta.MemberCount,
			TokenSum:      meta.TokenSum,
			TeamSpend:     meta.TeamSpend,
			EstimatedPool: meta.EstimatedPool,
			RewardCap:     cfg.Cap,
			RewardTiers:   cfg.Tiers,
		}
	}
	settlements, err := s.listTeamRewardSettlementRecords(ctx, teamID, 24)
	if err != nil {
		return nil, err
	}
	return &PlayAdminTeamDetail{
		Team:        summary,
		CreatedAt:   meta.CreatedAt,
		ArchivedAt:  meta.ArchivedAt,
		Settlements: settlements,
	}, nil
}

func (s *PlayService) ListAdminArenaLeaderboard(
	ctx context.Context,
	periodType string,
	periodID int64,
	limit int,
) ([]PlayArenaScoreRow, *PlayArenaPeriod, []PlayArenaSettlementTier, error) {
	rt := s.GetRuntime(ctx)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var period *PlayArenaPeriod
	var err error
	if periodID > 0 {
		period, err = s.repo.GetArenaPeriodByID(ctx, periodID)
	} else if periodType == "daily" {
		period, err = s.ensureDailyArenaPeriod(ctx, s.serverNow(), rt)
	} else {
		period, err = s.getExistingMonthlyArenaPeriod(ctx, s.serverNow())
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if period == nil {
		return []PlayArenaScoreRow{}, nil, nil, nil
	}
	if period.Status == "settled" {
		history, ok := s.repo.(PlayArenaPaidLeaderboardRepository)
		if !ok {
			return nil, nil, nil, fmt.Errorf("arena payout evidence repository unavailable")
		}
		rows, err := history.ListArenaPaidLeaderboard(ctx, period.ID, limit)
		return rows, period, []PlayArenaSettlementTier{}, err
	}
	rows, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	var rewards []PlayArenaSettlementTier
	if period.PeriodType == "daily" {
		policy, policyErr := s.dailyArenaPolicyForPeriod(ctx, period)
		if policyErr != nil {
			return nil, nil, nil, policyErr
		}
		rewards = policy.Rewards
		for i := range rows {
			rows[i].EstimatedReward = dailyArenaRewardForRank(rows[i].Rank, policy)
		}
	} else {
		rewards, err = s.arenaRewardTiersForPeriod(ctx, period)
		if err != nil {
			return nil, nil, nil, err
		}
		for i := range rows {
			rows[i].EstimatedReward = arenaRewardForRank(rows[i].Rank, rewards)
		}
	}
	return rows, period, rewards, nil
}

func (s *PlayService) currentTeamRewardConfig(ctx context.Context) TeamRewardConfig {
	rt := s.GetRuntime(ctx)
	return TeamRewardConfig{
		Enabled: rt.TeamSharedRewardEnabled,
		Cap:     rt.TeamSharedRewardCap,
		Tiers:   append([]TeamRewardTier(nil), rt.TeamSharedRewardTiers...),
	}
}

func currentTeamRewardWindow(now time.Time) (time.Time, time.Time, error) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	localNow := now.In(shanghai)
	start := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, shanghai)
	return start, start.AddDate(0, 1, 0), nil
}

func sumArenaRewardBudget(tiers []PlayArenaSettlementTier) float64 {
	total := 0.0
	previousRankMax := 0
	for _, tier := range tiers {
		if tier.RankMax <= previousRankMax {
			continue
		}
		total += float64(tier.RankMax-previousRankMax) * tier.Amount
		previousRankMax = tier.RankMax
	}
	return total
}

// Settled periods are explained exclusively by payment evidence. Live usage
// and the current settings cannot reconstruct a historical winner list.
type PlayArenaPaidLeaderboardRepository interface {
	ListArenaPaidLeaderboard(context.Context, int64, int) ([]PlayArenaScoreRow, error)
}
