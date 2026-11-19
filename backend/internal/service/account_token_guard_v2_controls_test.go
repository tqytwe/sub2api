package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type reauthRuntimeTestSettings struct {
	values map[string]string
	fail   bool
}

func (s *reauthRuntimeTestSettings) GetValue(_ context.Context, key string) (string, error) {
	if s.fail {
		return "", errors.New("database unavailable")
	}
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}
func (s *reauthRuntimeTestSettings) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func TestAccountTokenGuardV2CredentialEditPreservesAutomation(t *testing.T) {
	reauth, reader, _, _, _, _ := newReauthTestService("acct-1")
	repo := &accountTokenGuardV2TestRepo{record: &AccountTokenGuardV2Record{AccountID: 42, Enabled: false, AutoReloginEnabled: false}}
	svc := NewAccountTokenGuardV2Service(repo, nil, reader, nil, reauth)
	_, err := svc.SaveAccount(context.Background(), 42, AccountTokenGuardV2AccountInput{
		LoginEmail: "user@example.com", CredentialMode: "password_totp", Password: "synthetic", Enabled: true, AutoReloginEnabled: true, PreserveEnabled: true, PreserveAutoRelogin: true,
	})
	require.NoError(t, err)
	require.False(t, repo.record.Enabled)
	require.False(t, repo.record.AutoReloginEnabled)
}

func (r *accountTokenGuardV2TestRepo) UpdateSwitches(_ context.Context, id int64, enabled, autoRelogin *bool) error {
	if r.record == nil || r.record.AccountID != id {
		return errors.New("account not found")
	}
	if enabled != nil {
		r.record.Enabled = *enabled
	}
	if autoRelogin != nil {
		r.record.AutoReloginEnabled = *autoRelogin
	}
	return nil
}

func TestAccountTokenGuardV2SwitchDoesNotReadOrSaveCredentials(t *testing.T) {
	repo := &accountTokenGuardV2TestRepo{record: &AccountTokenGuardV2Record{AccountID: 42, Enabled: true, AutoReloginEnabled: true}}
	// No credential reader/encryptor/reauth service: switching must still work.
	svc := NewAccountTokenGuardV2Service(repo, nil, nil, nil, nil)
	disabled := false
	saved, err := svc.UpdateSwitches(context.Background(), 42, &disabled, nil)
	require.NoError(t, err)
	require.False(t, saved.Enabled)
	require.True(t, saved.AutoReloginEnabled)
	saved, err = svc.UpdateSwitches(context.Background(), 42, nil, &disabled)
	require.NoError(t, err)
	require.False(t, saved.Enabled)
	require.False(t, saved.AutoReloginEnabled)
	_, err = svc.UpdateSwitches(context.Background(), 42, nil, nil)
	require.Error(t, err)
}

func TestOpenAIOAuthReauthRuntimeRejectsCorruptSettingsAndPreservesWorkerEnv(t *testing.T) {
	svc, _, _, _, _, _ := newReauthTestService("acct-1")
	settings := &reauthRuntimeTestSettings{values: map[string]string{}}
	svc.settings = settings
	t.Setenv("OPENAI_REAUTH_CONCURRENCY", "6")
	cfg, err := svc.GetRuntimeSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 6, cfg.WorkerConcurrency)
	require.False(t, cfg.ConcurrencyConfigured)
	for _, raw := range []string{"{bad", "null", "{\"worker_concurrency\":null}", "{\"worker_concurrency\":17}"} {
		settings.values[openAIOAuthReauthRuntimeSettingsKey] = raw
		_, err = svc.GetRuntimeSettings(context.Background())
		require.Error(t, err)
	}
	settings.values[openAIOAuthReauthRuntimeSettingsKey] = "{\"engine\":\"\",\"worker_concurrency\":2}"
	cfg, err = svc.GetRuntimeSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, cfg.WorkerConcurrency)
	require.True(t, cfg.ConcurrencyConfigured)
}
