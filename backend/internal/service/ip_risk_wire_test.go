package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestProvideIPRiskHasherUsesDedicatedOrDomainSeparatedJWTKey(t *testing.T) {
	t.Parallel()

	jwtOnly := &config.Config{
		JWT: config.JWTConfig{Secret: strings.Repeat("j", 32)},
	}
	first := ProvideIPRiskHasher(jwtOnly)
	second := ProvideIPRiskHasher(jwtOnly)
	require.NotNil(t, first)
	require.Equal(t, first.OpaqueCode("same"), second.OpaqueCode("same"))

	dedicated := &config.Config{
		JWT: config.JWTConfig{Secret: strings.Repeat("j", 32)},
		IPRisk: config.IPRiskConfig{
			HMACKey: strings.Repeat("ab", 32),
		},
	}
	require.NotEqual(
		t,
		first.OpaqueCode("same"),
		ProvideIPRiskHasher(dedicated).OpaqueCode("same"),
	)
}

func TestProvideIPRiskRuntimeConfigMapsSafeDefaults(t *testing.T) {
	t.Parallel()

	runtime := ProvideIPRiskRuntimeConfig(&config.Config{
		IPRisk: config.IPRiskConfig{
			Enabled:                   true,
			IncrementalDelaySeconds:   10,
			ReconcileIntervalMinutes:  5,
			DailyScanIntervalHours:    24,
			EventRetentionDays:        90,
			CaseRetentionDays:         365,
			HistoricalBackfillMaxDays: 90,
			ManualScanMaxDays:         90,
			RetentionBatchSize:        5000,
			EvaluationQueueCapacity:   4096,
		},
	})
	require.True(t, runtime.Enabled)
	require.True(t, runtime.ShadowMode)
	require.Equal(t, 10*time.Second, runtime.IncrementalDelay)
	require.Equal(t, 5*time.Minute, runtime.ReconcileInterval)
	require.Equal(t, 90*24*time.Hour, runtime.EventRetention)
	require.Equal(t, 365*24*time.Hour, runtime.CaseRetention)
}
