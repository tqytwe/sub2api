package service

import (
	"context"
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
	cfg := s.currentTeamRewardConfig(ctx)
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
	cfg := s.currentTeamRewardConfig(ctx)
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
	cfg := s.currentTeamRewardConfig(ctx)
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
		period, err = s.repo.EnsureDailyArenaPeriod(ctx, s.serverNow())
	} else {
		period, err = s.repo.EnsureMonthlyArenaPeriod(ctx, s.serverNow())
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if period == nil {
		return []PlayArenaScoreRow{}, nil, nil, nil
	}
	rows, err := s.repo.ListArenaLeaderboard(ctx, period.StartAt, period.EndAt, limit)
	if err != nil {
		return nil, nil, nil, err
	}
	rewards := rt.ArenaSettlementRewards
	if periodType == "daily" {
		rewards = rt.DailyArenaTopRewards
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
	for _, tier := range tiers {
		total += tier.Amount
	}
	return total
}
