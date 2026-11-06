package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDailyArenaBudgetIsSharedByAdminEstimateAndPayout(t *testing.T) {
	for _, test := range []struct {
		budget string
		third  float64
	}{{"0.3", 0.1}, {"0.25", 0}} {
		t.Run(test.budget, func(t *testing.T) {
			repo := &arenaSeasonRepo{daily: &PlayArenaPeriod{ID: 7, PeriodType: "daily", Status: "active"}, rows: []PlayArenaScoreRow{{Rank: 1}, {Rank: 2}, {Rank: 3}}}
			settings := NewSettingService(&dailyRewardSummarySettingRepo{values: map[string]string{
				SettingKeyPlayDailyArenaTopRewards: `[{"rank_max":3,"amount":0.1}]`, SettingKeyPlayDailyArenaDailyBudget: test.budget,
			}}, nil)
			svc := NewPlayService(repo, nil, nil, settings, nil, nil)
			rows, _, _, err := svc.ListAdminArenaLeaderboard(context.Background(), "daily", 0, 10)
			require.NoError(t, err)
			require.Equal(t, 0.1, rows[0].EstimatedReward)
			require.Equal(t, test.third, rows[2].EstimatedReward)
		})
	}
}
