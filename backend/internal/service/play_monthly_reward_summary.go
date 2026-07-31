package service

import "context"

const maxPublicMonthlyRewardRows = 20

func (s *PlayService) GetMonthlyArenaRewardSummary(ctx context.Context) (*PlayArenaMonthlyRewardSummary, error) {
	rt := s.GetRuntime(ctx)
	out := &PlayArenaMonthlyRewardSummary{Enabled: rt.ArenaEnabled}
	if !out.Enabled || s.repo == nil {
		return out, nil
	}
	repo, ok := s.repo.(PlayMonthlyArenaRewardsRepository)
	if !ok {
		return out, nil
	}
	period, err := repo.GetLatestSettledMonthlyArenaPeriod(ctx)
	if err != nil {
		return nil, err
	}
	if period == nil {
		return out, nil
	}
	rows, err := repo.ListArenaMonthlyRewardLedger(ctx, period.ID)
	if err != nil {
		return nil, err
	}
	out.Period = period
	out.SettledAt = period.SettledAt
	out.WinnersCount = len(rows)
	out.Winners = make([]PlayArenaRewardPublicWinner, 0, min(len(rows), maxPublicMonthlyRewardRows))
	for _, row := range rows {
		out.TotalAmount += row.Amount
		if len(out.Winners) >= maxPublicMonthlyRewardRows {
			continue
		}
		paidAt := row.CreatedAt
		out.Winners = append(out.Winners, PlayArenaRewardPublicWinner{
			Rank: row.Rank, Period: period, DisplayName: row.DisplayName,
			AvatarURL: row.AvatarURL, Amount: row.Amount, PaidAt: &paidAt,
		})
	}
	return out, nil
}
