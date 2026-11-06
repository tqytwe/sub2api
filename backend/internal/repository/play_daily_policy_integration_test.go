//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDailyArenaPolicyFreezesRewardsAndBudgetAcrossConfigurationChanges(t *testing.T) {
	ctx := context.Background()
	repo := &playRepository{client: integrationEntClient, sql: integrationDB}
	newService := func(amount, budget string) *service.PlayService {
		settings := service.NewSettingService(&blindboxIntegrationSettingRepo{values: map[string]string{
			service.SettingKeyPlayArenaEnabled:          "true",
			service.SettingKeyPlayDailyArenaEnabled:     "true",
			service.SettingKeyPlayDailyArenaTopRewards:  `[{"rank_max":2,"amount":` + amount + `}]`,
			service.SettingKeyPlayDailyArenaDailyBudget: budget,
		}}, nil)
		return service.NewPlayService(repo, nil, nil, settings, nil, integrationEntClient)
	}
	day := time.Date(2039, 4, 15, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	first := newService("1", "1.5")
	period, err := first.PrepareCurrentArenaDailySeason(ctx, day)
	require.NoError(t, err)
	require.NotNil(t, period)
	second := newService("5", "20")
	again, err := second.PrepareCurrentArenaDailySeason(ctx, day)
	require.NoError(t, err)
	require.Equal(t, period.ID, again.ID)
	policy, err := repo.GetDailyArenaPolicy(ctx, period.ID)
	require.NoError(t, err)
	require.Equal(t, 1.5, policy.Budget)
	require.Equal(t, []service.PlayArenaSettlementTier{{RankMax: 2, Amount: 1}}, policy.Rewards)
	_, _, tiers, err := second.ListAdminArenaLeaderboard(ctx, "daily", period.ID, 10)
	require.NoError(t, err)
	require.Equal(t, policy.Rewards, tiers)
	next, err := second.PrepareCurrentArenaDailySeason(ctx, day.AddDate(0, 0, 1))
	require.NoError(t, err)
	policy, err = repo.GetDailyArenaPolicy(ctx, next.ID)
	require.NoError(t, err)
	require.Equal(t, 20.0, policy.Budget)
	require.Equal(t, 5.0, policy.Rewards[0].Amount)
}
