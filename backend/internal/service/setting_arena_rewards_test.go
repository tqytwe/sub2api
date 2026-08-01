package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type arenaRewardSettingRepo struct {
	SettingRepository
	values map[string]string
	last   map[string]string
}

func (r *arenaRewardSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *arenaRewardSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.last = make(map[string]string, len(settings))
	for key, value := range settings {
		r.values[key] = value
		r.last[key] = value
	}
	return nil
}

func TestSetPlayArenaRewardSettingsPersistsSchedulesAndBudget(t *testing.T) {
	repo := &arenaRewardSettingRepo{values: map[string]string{}}
	settings := NewSettingService(repo, nil)
	want := PlayArenaRewardSettings{
		Monthly:     []PlayArenaSettlementTier{{RankMax: 1, Amount: 50}, {RankMax: 3, Amount: 20}, {RankMax: 10, Amount: 5}},
		Daily:       []PlayArenaSettlementTier{{RankMax: 1, Amount: 0.5}, {RankMax: 3, Amount: 0.2}, {RankMax: 10, Amount: 0.1}},
		DailyBudget: 50,
	}

	got, err := settings.SetPlayArenaRewardSettings(context.Background(), want)

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.JSONEq(t, `[{"rank_max":1,"amount":50},{"rank_max":3,"amount":20},{"rank_max":10,"amount":5}]`, repo.last[SettingKeyPlayArenaSettlementRewards])
	require.JSONEq(t, `[{"rank_max":1,"amount":0.5},{"rank_max":3,"amount":0.2},{"rank_max":10,"amount":0.1}]`, repo.last[SettingKeyPlayDailyArenaTopRewards])
	require.Equal(t, "50", repo.last[SettingKeyPlayDailyArenaDailyBudget])
}

func TestSetPlayArenaRewardSettingsRejectsInvalidSchedules(t *testing.T) {
	settings := NewSettingService(&arenaRewardSettingRepo{values: map[string]string{}}, nil)
	base := PlayArenaRewardSettings{
		Monthly:     []PlayArenaSettlementTier{{RankMax: 1, Amount: 1}},
		Daily:       []PlayArenaSettlementTier{{RankMax: 1, Amount: 1}},
		DailyBudget: 10,
	}

	invalid := base
	invalid.Monthly = []PlayArenaSettlementTier{{RankMax: 2, Amount: 1}, {RankMax: 2, Amount: 1}}
	_, err := settings.SetPlayArenaRewardSettings(context.Background(), invalid)
	require.Error(t, err)

	invalid = base
	invalid.Daily[0].Amount = -0.1
	_, err = settings.SetPlayArenaRewardSettings(context.Background(), invalid)
	require.Error(t, err)

	invalid = base
	invalid.DailyBudget = 0
	_, err = settings.SetPlayArenaRewardSettings(context.Background(), invalid)
	require.Error(t, err)
}

func TestSumArenaRewardBudgetExpandsRankRanges(t *testing.T) {
	tiers := []PlayArenaSettlementTier{{RankMax: 1, Amount: 50}, {RankMax: 3, Amount: 20}, {RankMax: 10, Amount: 5}}
	require.InDelta(t, 125, sumArenaRewardBudget(tiers), 1e-9)
}
