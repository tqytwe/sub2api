package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type automationRuntimeAccounts struct {
	AccountRepository
	oauth    []Account
	selected []*Account
	opened   []int64
	reason   string
	previous *time.Time
}

func (r *automationRuntimeAccounts) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]Account, error) {
	return r.oauth, nil
}
func (r *automationRuntimeAccounts) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return r.selected, nil
}
func (r *automationRuntimeAccounts) EnableOpenAIAPIKeyFallback(_ context.Context, id int64, _ string, reason string) (bool, error) {
	r.opened = append(r.opened, id)
	r.reason = reason
	return true, nil
}
func (r *automationRuntimeAccounts) LoadOpenAIAPIKeyFallbackState(context.Context) (*time.Time, string, error) {
	return r.previous, r.reason, nil
}

type automationRuntimeMetric struct {
	value *OpenAIAPIKeyFallbackObservation
	err   error
}

func (m automationRuntimeMetric) GetOpenAIAPIKeyFallbackMetric(context.Context, string, time.Time, time.Time) (*OpenAIAPIKeyFallbackObservation, error) {
	return m.value, m.err
}
func TestOAuthFallbackRuntimeSelectedAccountsAndIndependentTriggers(t *testing.T) {
	for _, scenario := range []string{"unavailable pool", "metric low", "healthy pool", "no samples", "monitor unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := DefaultOAuthAutoConfig()
			cfg.Revision = "runtime"
			cfg.OpenAIAPIKeyFallback.Enabled = true
			cfg.OpenAIAPIKeyFallback.AccountIDs = []int64{7}
			raw, err := json.Marshal(cfg)
			require.NoError(t, err)
			accounts := &automationRuntimeAccounts{oauth: []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: scenario != "unavailable pool", Credentials: map[string]any{"model_mapping": map[string]any{"a": "a"}}, GroupIDs: []int64{10}}, {ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: scenario != "unavailable pool", Credentials: map[string]any{"model_mapping": map[string]any{"b": "b"}}, GroupIDs: []int64{10}}}, selected: []*Account{{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false}, {ID: 8, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false}}}
			svc := NewOAuthAutomationService(&oauthAutomationSettingsStub{raw: string(raw)}, accounts, autoConfigGroups{})
			metric := automationRuntimeMetric{value: &OpenAIAPIKeyFallbackObservation{Value: 100, Samples: 10}}
			if scenario == "metric low" {
				metric.value.Value = 90
			}
			if scenario == "no samples" {
				metric.value.Samples = 0
				metric.value.Value = 0
			}
			if scenario == "monitor unavailable" {
				metric.err = errors.New("unavailable")
			}
			svc.setOpenAIAPIKeyFallbackSLAReader(metric, nil)
			svc.fallbackSharedJobs.Store(true)
			svc.evaluateOpenAIAPIKeyFallback(context.Background())
			if scenario == "unavailable pool" || scenario == "metric low" {
				require.Equal(t, []int64{7}, accounts.opened)
			} else {
				require.Empty(t, accounts.opened)
			}
			state := svc.OpenAIAPIKeyFallbackStatus()
			require.Equal(t, 2, state.OAuthAccountCount)
			if scenario == "unavailable pool" {
				require.Equal(t, "oauth_unavailable", accounts.reason)
			}
			if scenario == "metric low" {
				require.Equal(t, "metric_below_threshold", accounts.reason)
			}
		})
	}
}
func TestOAuthAutomationTokenRefreshIsNotManualModelRecovery(t *testing.T) {
	require.False(t, OAuthAutomationModelsEdited(map[string]any{"access_token": "fixture", "refresh_token": "fixture"}))
	require.True(t, OAuthAutomationModelsEdited(map[string]any{"model_mapping": map[string]any{"a": "a", "b": "b"}}))
	ctx := WithOAuthAutomationManualEdit(context.Background(), true, false, false)
	require.Equal(t, OAuthAutomationManualEdit{Concurrency: true}, OAuthAutomationManualEditFrom(ctx))
}
