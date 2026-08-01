package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type growthRunnerTeamCompetitionRepo struct {
	PlayRepository

	ensureCalls int
	seasons     map[string]*PlayTeamSeason
}

func (*growthRunnerTeamCompetitionRepo) ListExpiredActiveDailyArenaPeriods(context.Context, time.Time) ([]PlayArenaPeriod, error) {
	return nil, nil
}

func (r *growthRunnerTeamCompetitionRepo) GetTeamCompetitionSeason(_ context.Context, periodStart time.Time) (*PlayTeamSeason, error) {
	season := r.seasons[periodStart.Format("2006-01")]
	if season == nil {
		return nil, nil
	}
	copy := *season
	return &copy, nil
}

func (r *growthRunnerTeamCompetitionRepo) EnsureTeamCompetitionSeason(
	_ context.Context,
	periodStart, windowStart, windowEnd time.Time,
	rules map[string]any,
) (*PlayTeamSeason, error) {
	r.ensureCalls++
	key := periodStart.Format("2006-01")
	if season := r.seasons[key]; season != nil {
		copy := *season
		return &copy, nil
	}
	season := &PlayTeamSeason{
		ID:          int64(len(r.seasons) + 1),
		Month:       key,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
		Rules:       rules,
		Status:      "active",
	}
	r.seasons[key] = season
	copy := *season
	return &copy, nil
}

func (*growthRunnerTeamCompetitionRepo) CreateTeamCompetitionSeasonSnapshot(
	context.Context,
	time.Time,
	time.Time,
	time.Time,
	map[string]any,
) (bool, error) {
	return false, nil
}

func TestPlayGrowthRunnerPreparesAndPreservesCurrentDisabledTeamSeason(t *testing.T) {
	settingsRepo := &teamRewardSettingRepoStub{values: map[string]string{
		SettingKeyPlayTeamSharedRewardEnabled: "false",
	}}
	repo := &growthRunnerTeamCompetitionRepo{seasons: map[string]*PlayTeamSeason{}}
	svc := NewPlayService(repo, nil, nil, NewSettingService(settingsRepo, nil), nil, nil)
	runner := NewPlayGrowthRunner(svc, nil, nil, nil, nil)

	runner.runOnce(context.Background())
	require.Len(t, repo.seasons, 1)
	require.Equal(t, 1, repo.ensureCalls)
	for _, season := range repo.seasons {
		frozen, found, err := teamCompetitionFrozenRewardConfig(season)
		require.NoError(t, err)
		require.True(t, found)
		require.False(t, frozen.Enabled)
	}

	settingsRepo.values[SettingKeyPlayTeamSharedRewardEnabled] = "true"
	settingsRepo.values[SettingKeyPlayTeamSharedRewardCap] = "100"
	settingsRepo.values[SettingKeyPlayTeamSharedRewardTiers] = `[{"threshold":"20","rate":"0.50"}]`
	runner.runOnce(context.Background())

	require.Equal(t, 2, repo.ensureCalls)
	require.Len(t, repo.seasons, 1)
	for _, season := range repo.seasons {
		frozen, found, err := teamCompetitionFrozenRewardConfig(season)
		require.NoError(t, err)
		require.True(t, found)
		require.False(t, frozen.Enabled)
		require.Equal(t, "250.00000000", frozen.Cap.StringFixed(8))
	}
}
