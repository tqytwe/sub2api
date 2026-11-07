package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQualityIsolationBlocksOnlyTargetModelAcrossPlatforms(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformDeepseek} {
		t.Run(platform, func(t *testing.T) {
			account := &Account{Platform: platform, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"alias": "isolated", "healthy": "healthy"}},
				Extra: map[string]any{"quality_model_isolation": map[string]any{
					"isolated": map[string]any{"reason": "quality_rule:7", "isolated_at": "2026-11-07T00:00:00Z"},
				}}}
			require.False(t, account.IsSchedulableForModelWithContext(context.Background(), "alias"), "persistent isolation must block mapped aliases")
			require.False(t, account.IsSchedulableForModel("isolated"))
			require.True(t, account.IsSchedulableForModel("healthy"), "a failed peer must not disable the whole account")
			require.True(t, account.Schedulable)
		})
	}
}

func TestQualityIsolationUsesOpenAIActualOutboundIdentity(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{"quality_model_isolation": map[string]any{
			"compact-model": map[string]any{"reason": "quality_rule:7", "isolated_at": "2026-11-07T00:00:00Z"},
		}}}
	require.True(t, openAIAccountOutboundModelRateLimited(context.Background(), account, "public-model", "compact-model"), "compact/sticky/WS selection must honor the actual outbound model")
	require.False(t, openAIAccountOutboundModelRateLimited(context.Background(), account, "public-model", "healthy"))
}

func TestQualityIsolationPolicyIsExplicitAndValidatesGroupScope(t *testing.T) {
	var config PelicanTestConfig
	require.NoError(t, json.Unmarshal([]byte(`{"model_ids":["first","second"],"prompt":"probe","quality":{"action":"isolate_models","remove_group_ids":[10],"expected_answer":"ok","judge":{"kind":"expected"}}}`), &config))
	require.NoError(t, validateQualityConfig(&config), "isolation must be available through the existing quality plan API")
	var missingGroups PelicanTestConfig
	require.NoError(t, json.Unmarshal([]byte(`{"model_ids":["first"],"prompt":"probe","quality":{"action":"isolate_models","expected_answer":"ok","judge":{"kind":"expected"}}}`), &missingGroups))
	require.Error(t, validateQualityConfig(&missingGroups), "upstream isolation requires explicit quarantine groups")
}

func TestQualityIsolationProbeBypassKeepsNativeCooldownAndAliases(t *testing.T) {
	ctx := context.Background()
	account := &Account{Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"alias": "actual", "actual": "other"}}, Extra: map[string]any{QualityModelIsolationKey: map[string]any{"actual": map[string]any{"reason": "quality_rule:7"}, "other": map[string]any{"reason": "quality_rule:8"}}}}
	require.False(t, account.IsSchedulableForModelWithContext(ctx, "alias"))
	probe := context.WithValue(ctx, qualityIsolationProbeKey{}, true)
	require.True(t, account.IsSchedulableForModelWithContext(probe, "alias"))
	account.Extra[modelRateLimitsKey] = map[string]any{"actual": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}}
	require.False(t, account.IsSchedulableForModelWithContext(probe, "alias"), "probe bypass must keep independent upstream cooldowns")
	delete(account.Extra, modelRateLimitsKey)
	isolation, ok := account.Extra[QualityModelIsolationKey].(map[string]any)
	require.True(t, ok)
	delete(isolation, "actual")
	require.False(t, openAIAccountOutboundModelRateLimited(ctx, account, "alias", "actual"), "actual outbound identity must not be mapped a second time")
}
func TestQualityIsolationCannotBeBypassedByAntigravityCredits(t *testing.T) {
	account := &Account{Platform: PlatformAntigravity, Status: StatusActive, Schedulable: true, Extra: map[string]any{"allow_overages": true, QualityModelIsolationKey: map[string]any{"gemini-test": map[string]any{"reason": "quality_rule:7"}}}}
	require.True(t, account.IsOveragesEnabled())
	require.False(t, account.IsSchedulableForModel("gemini-test"))
	require.True(t, account.IsSchedulableForModel("healthy"))
}

// An authenticated admin probe reaches this entry; ordinary requests never do.
func TestQualityIsolationManualProbeGetsTrustedContext(t *testing.T) {
	svc, _, requests, c, _ := newTypeSafeAccountTestFixture(t, http.StatusOK, `{"answers":{"connection_test":{"type":"noul"}}}`)
	require.NoError(t, svc.TestAccountConnection(c, 31, "", "", AccountTestModeDefault))
	require.Len(t, *requests, 1)
	require.Equal(t, true, (*requests)[0].Context().Value(qualityIsolationProbeKey{}))
}
