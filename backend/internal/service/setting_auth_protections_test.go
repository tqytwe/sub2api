package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type authProtectionSettingsRepo struct {
	SettingRepository
	values map[string]string
	err    error
}

func (r *authProtectionSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], r.err
}

func TestAuthProtectionSettingsApplyLiveAndIndependently(t *testing.T) {
	ctx := context.Background()
	repo := &authProtectionSettingsRepo{values: map[string]string{}}
	settings := &SettingService{settingRepo: repo}
	require.True(t, settings.IsRegistrationIPLimitEnabled(ctx))
	require.True(t, settings.IsRegistrationEmailAliasBlockEnabled(ctx))
	require.True(t, settings.IsLoginRateLimitEnabled(ctx))
	repo.values[SettingKeyLoginRateLimitEnabled] = "false"
	require.False(t, settings.IsLoginRateLimitEnabled(ctx))
	require.True(t, settings.IsRegistrationIPLimitEnabled(ctx))
	repo.values[SettingKeyRegistrationIPLimitEnabled] = "false"
	require.False(t, settings.IsRegistrationIPLimitEnabled(ctx))
	require.True(t, settings.IsRegistrationEmailAliasBlockEnabled(ctx))
	repo.err = errors.New("lookup failed")
	require.True(t, settings.IsLoginRateLimitEnabled(ctx))
	require.True(t, settings.IsRegistrationIPLimitEnabled(ctx))
}

func TestRegistrationEmailAliasPolicyPreservesPrimaryAddresses(t *testing.T) {
	ctx := context.Background()
	repo := &authProtectionSettingsRepo{values: map[string]string{}}
	s := &AuthService{settingService: &SettingService{settingRepo: repo}}
	for _, email := range []string{"first.last@example.test", "primary@gmail.com"} {
		require.NoError(t, s.validateRegistrationEmailAlias(ctx, email))
	}
	for _, email := range []string{"firstname+newsletter@example.test", "first.last@gmail.com", "primary@example.test."} {
		require.ErrorIs(t, s.validateRegistrationEmailAlias(ctx, email), ErrEmailAliasNotAllowed)
	}
	repo.values[SettingKeyRegistrationEmailAliasBlockEnabled] = "false"
	require.NoError(t, s.validateRegistrationEmailAlias(ctx, "first.last@gmail.com"))
}
