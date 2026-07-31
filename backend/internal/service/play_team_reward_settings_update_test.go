package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUpdateTeamRewardSettingsRejectsInvalidConfigurationAsBadRequest(t *testing.T) {
	repo := &teamRewardUpdateSettingsRepo{}
	playService := &PlayService{settingService: &SettingService{settingRepo: repo}}

	_, err := playService.UpdateTeamRewardSettings(context.Background(), PlayTeamRewardSettings{
		Enabled:    true,
		Cap:        decimal.NewFromInt(100),
		StartMonth: "2026-08",
		Tiers: []TeamRewardTier{
			{Threshold: decimal.NewFromInt(20), Rate: decimal.RequireFromString("0.003")},
			{Threshold: decimal.NewFromInt(100), Rate: decimal.RequireFromString("0.004")},
			{Threshold: decimal.NewFromInt(500), Rate: decimal.RequireFromString("0.005")},
			{Threshold: decimal.NewFromInt(2000), Rate: decimal.RequireFromString("0.001")},
		},
	})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Empty(t, repo.values)
}

func TestValidateTeamRewardConfigRejectsMoreThanMaximumTiers(t *testing.T) {
	cfg := defaultTeamRewardConfig()
	cfg.Tiers = make([]TeamRewardTier, teamRewardMaxTiers+1)
	for index := range cfg.Tiers {
		cfg.Tiers[index] = TeamRewardTier{
			Threshold: decimal.NewFromInt(int64(index + 1)),
			Rate:      decimal.NewFromInt(int64(index + 1)).Div(decimal.NewFromInt(100)),
		}
	}

	require.ErrorContains(t, validateTeamRewardConfig(cfg), "at most")
}

type teamRewardUpdateSettingsRepo struct {
	values map[string]string
}

func (r *teamRewardUpdateSettingsRepo) Get(context.Context, string) (*Setting, error) {
	return nil, errors.New("unexpected Get call")
}

func (r *teamRewardUpdateSettingsRepo) GetValue(context.Context, string) (string, error) {
	return "", errors.New("unexpected GetValue call")
}

func (r *teamRewardUpdateSettingsRepo) Set(context.Context, string, string) error {
	return errors.New("unexpected Set call")
}

func (r *teamRewardUpdateSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *teamRewardUpdateSettingsRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.values = make(map[string]string, len(values))
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *teamRewardUpdateSettingsRepo) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *teamRewardUpdateSettingsRepo) Delete(context.Context, string) error {
	return errors.New("unexpected Delete call")
}
