package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadMobilePushConfigFromEnv(t *testing.T) {
	t.Setenv("MOBILE_PUSH_ENABLED", "true")
	t.Setenv("FCM_PROJECT_ID", "project-1")
	t.Setenv("FCM_SERVICE_ACCOUNT_JSON", `{"private_key":"secret"}`)
	t.Setenv("MOBILE_PUSH_POLL_INTERVAL", "3s")
	t.Setenv("MOBILE_PUSH_BATCH_SIZE", "25")

	cfg := LoadMobilePushConfigFromEnv()
	require.True(t, cfg.HasCredentials())
	require.Equal(t, 3*time.Second, cfg.PollInterval)
	require.Equal(t, 25, cfg.BatchSize)

	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
}

func TestMobilePushConfigDisabledWithoutCredentials(t *testing.T) {
	cfg := MobilePushConfig{Enabled: true}.Normalized()
	require.False(t, cfg.HasCredentials())
	require.Positive(t, cfg.PollInterval)
	require.Positive(t, cfg.ClaimLease)
	require.Positive(t, cfg.BatchSize)
	require.Positive(t, cfg.MaxAttempts)
}

func TestMobilePushConfigDetectsAnyCredentialAndValidatesSource(t *testing.T) {
	cfg := MobilePushConfig{Enabled: true, ProjectID: "project-1"}.Normalized()
	require.True(t, cfg.HasAnyCredentialSetting())
	require.False(t, cfg.HasCredentials())
	require.ErrorContains(t, cfg.ValidateCredentials(), "FCM_SERVICE_ACCOUNT_FILE or FCM_SERVICE_ACCOUNT_JSON")

	cfg = MobilePushConfig{Enabled: true, ServiceAccountJSON: `{}`}.Normalized()
	require.True(t, cfg.HasAnyCredentialSetting())
	require.True(t, cfg.HasCredentials())
	require.NoError(t, cfg.ValidateCredentials())
}
