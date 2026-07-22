//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceRuntime_FailsClosed(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{})

		require.False(t, svc.GetMarketplaceRuntime(context.Background()).Enabled)
	})

	t.Run("invalid", func(t *testing.T) {
		for _, raw := range []string{"", "TRUE", "1", "yes", " true "} {
			svc := NewSettingService(&settingPublicRepoStub{
				values: map[string]string{SettingKeyMarketplaceEnabled: raw},
			}, &config.Config{})

			require.False(t, svc.GetMarketplaceRuntime(context.Background()).Enabled, raw)
		}
	})

	t.Run("repository_error", func(t *testing.T) {
		svc := NewSettingService(&settingPublicRepoStub{
			values:         map[string]string{},
			getMultipleErr: errors.New("database unavailable"),
		}, &config.Config{})

		require.False(t, svc.GetMarketplaceRuntime(context.Background()).Enabled)
	})

	t.Run("unwired", func(t *testing.T) {
		var svc *SettingService

		require.False(t, svc.GetMarketplaceRuntime(context.Background()).Enabled)
	})
}

func TestMarketplaceRuntime_ExplicitBoolean(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{raw: "false", want: false},
		{raw: "true", want: true},
	}

	for _, tt := range tests {
		svc := NewSettingService(&settingPublicRepoStub{
			values: map[string]string{SettingKeyMarketplaceEnabled: tt.raw},
		}, &config.Config{})

		require.Equal(t, tt.want, svc.GetMarketplaceRuntime(context.Background()).Enabled)
	}
}

func TestSettingService_InitializeDefaultSettings_DefaultsMarketplaceDisabled(t *testing.T) {
	repo := &forwardedIPMigrationRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
	require.Equal(t, "false", repo.values[SettingKeyMarketplaceEnabled])
}
