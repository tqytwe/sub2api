//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelCapabilityCooldownDiagnosis(t *testing.T) {
	for _, tc := range []struct {
		name, reason               string
		expired, peer, wantSupport bool
	}{
		{"model missing", upstreamModelNotFoundReason, false, false, false},
		{"provider 401 model missing", upstreamModelNotFound401Reason, false, false, false},
		{"plan gated", upstreamCodexPlanGatedModelReason, false, false, false},
		{"ordinary rate limit", "upstream_429", false, false, true},
		{"unknown reason", "", false, false, true},
		{"expired", upstreamModelNotFoundReason, true, false, true},
		{"another provider supports it", upstreamModelNotFoundReason, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.2-chat-latest": "gpt-5.2-chat-latest"}}}
			until := time.Now().Add(time.Hour)
			if tc.expired {
				until = time.Now().Add(-time.Hour)
			}
			setAccountModelRateLimitSnapshot(&account, "gpt-5.2", until, tc.reason, time.Now())
			repo := &mockAccountRepoForPlatform{accounts: []Account{account}, accountsByID: map[int64]*Account{}}
			if tc.peer {
				repo.accounts = append(repo.accounts, Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true})
			}
			for i := range repo.accounts {
				repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
			}
			generic := (&GatewayService{accountRepo: repo, cfg: testConfig()}).DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "gpt-5.2-chat-latest", PlatformOpenAI)
			openai := (&OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}).DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "gpt-5.2-chat-latest", PlatformOpenAI)
			for _, diag := range []ModelAvailabilityDiagnosis{generic, openai} {
				require.True(t, diag.HasAccountsInPool)
				require.Equal(t, tc.wantSupport, diag.HasModelSupport)
			}
		})
	}
}

func TestAnthropicModelCapabilityCooldownDiagnosis(t *testing.T) {
	for _, reason := range []string{upstreamModelNotFoundReason, "upstream_429", ""} {
		t.Run(reason, func(t *testing.T) {
			account := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"claude-example": "claude-provider-example"}}}
			setAccountModelRateLimitSnapshot(&account, "claude-provider-example", time.Now().Add(time.Hour), reason, time.Now())
			repo := &mockAccountRepoForPlatform{accounts: []Account{account}, accountsByID: map[int64]*Account{1: &account}}
			svc := &GatewayService{accountRepo: repo, cfg: testConfig()}
			diag := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "claude-example", PlatformAnthropic)
			require.True(t, diag.HasAccountsInPool)
			require.Equal(t, reason != upstreamModelNotFoundReason, diag.HasModelSupport)
			other := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "another-model", PlatformAnthropic)
			require.False(t, other.HasModelSupport, "unmapped model remains unsupported")
		})
	}
}

func TestModelCapabilityDiagnosisScopeAndInvalidRecords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{name: "missing"},
		{name: "wrong limits type", extra: map[string]any{modelRateLimitsKey: "invalid"}},
		{name: "wrong record type", extra: map[string]any{modelRateLimitsKey: map[string]any{"provider-model": 12}}},
		{name: "invalid timestamp", extra: map[string]any{modelRateLimitsKey: map[string]any{"provider-model": map[string]any{"rate_limit_reset_at": "bad", "reason": upstreamModelNotFoundReason}}}},
		{name: "wrong reason type", extra: map[string]any{modelRateLimitsKey: map[string]any{"provider-model": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Hour).Format(time.RFC3339), "reason": 123}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &Account{Extra: tc.extra}
			require.Equal(t, tc.want, a.modelCapabilityRejected("provider-model"))
		})
	}
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"alias-a": "provider-model", "alias-b": "provider-model", "other": "other-provider"}}}
	setAccountModelRateLimitSnapshot(a, "provider-model", time.Now().Add(time.Hour), upstreamModelNotFoundReason, time.Now())
	for _, alias := range []string{"alias-a", "alias-b"} {
		require.True(t, a.modelCapabilityRejectedForRequest(context.Background(), alias, nil))
	}
	require.False(t, a.modelCapabilityRejectedForRequest(context.Background(), "other", nil))
	require.True(t, a.isRateLimitActiveForKey("provider-model"), "diagnosis must not clear the cooldown")
}

func TestModelCapabilityDiagnosisMixedBoundary(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		a := Account{ID: 1, Platform: PlatformAntigravity, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Extra: map[string]any{"mixed_scheduling": enabled}, Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet-4-5": "claude-sonnet-4-5"}}}
		setAccountModelRateLimitSnapshot(&a, "claude-sonnet-4-5", time.Now().Add(time.Hour), upstreamModelNotFoundReason, time.Now())
		repo := &mockAccountRepoForPlatform{accounts: []Account{a}, accountsByID: map[int64]*Account{1: &a}}
		svc := &GatewayService{accountRepo: repo, cfg: testConfig()}
		got := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "claude-sonnet-4-5", PlatformAnthropic)
		require.Equal(t, enabled, got.HasAccountsInPool)
		require.False(t, got.HasModelSupport)
		isolated := (&OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}).DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "claude-sonnet-4-5", PlatformOpenAI)
		require.False(t, isolated.HasAccountsInPool)
	}
}

func TestModelCapabilityDiagnosisForwardContext(t *testing.T) {
	for _, tc := range []struct {
		name, forward, compactMap, global, blocked string
		compact                                    bool
		wantSupport                                bool
	}{
		{"channel ignores public cooldown", "provider-model", "", "", "public-model", false, true},
		{"channel sees provider cooldown", "provider-model", "", "", "provider-model", false, false},
		{"legacy ignores public cooldown", "public-model", "compact-model", "", "public-model", true, true},
		{"legacy sees compact cooldown", "public-model", "compact-model", "", "compact-model", true, false},
		{"global fallback ignores public cooldown", "public-model", "", "fallback-model", "public-model", true, true},
		{"global fallback sees target cooldown", "public-model", "", "fallback-model", "fallback-model", true, false},
		{"native false ignores compact cooldown", "public-model", "compact-model", "", "compact-model", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			until := time.Now().Add(time.Hour)
			a := Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, OverloadUntil: &until, Credentials: map[string]any{}, Extra: map[string]any{"openai_compact_mode": "force_on"}}
			if tc.compactMap != "" {
				a.Credentials["compact_model_mapping"] = map[string]any{tc.forward: tc.compactMap}
			}
			setAccountModelRateLimitSnapshot(&a, tc.blocked, until, upstreamModelNotFoundReason, time.Now())
			repo := &mockAccountRepoForPlatform{accounts: []Account{a}, accountsByID: map[int64]*Account{1: &a}}
			cfg := testConfig()
			cfg.Gateway.OpenAICompactModel = tc.global
			svc := &OpenAIGatewayService{accountRepo: repo, cfg: cfg}
			ctx := WithOpenAIForwardModel(context.Background(), tc.forward, tc.compact)
			selected, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{a}, tc.forward, nil, tc.compact, "", false)
			require.Nil(t, selected, "actual selection fails on temporary account capacity")
			for _, diag := range []ModelAvailabilityDiagnosis{svc.DiagnoseModelAvailabilityForPlatform(ctx, nil, "public-model", PlatformOpenAI), (&GatewayService{accountRepo: repo, cfg: cfg}).DiagnoseModelAvailabilityForPlatform(ctx, nil, "public-model", PlatformOpenAI)} {
				require.True(t, diag.HasAccountsInPool)
				require.Equal(t, tc.wantSupport, diag.HasModelSupport)
			}
		})
	}
}

func TestModelCapabilityDiagnosisAnthropicProviderIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, kind, blocked, mapped string
		want                        bool
	}{
		{"OAuth provider rejected", AccountTypeOAuth, "claude-sonnet-4-5-20250929", "", false},
		{"OAuth short stale", AccountTypeOAuth, "claude-sonnet-4-5", "", true},
		{"setup provider rejected", AccountTypeSetupToken, "claude-sonnet-4-5-20250929", "", false},
		{"setup short stale", AccountTypeSetupToken, "claude-sonnet-4-5", "", true},
		{"Vertex provider rejected", AccountTypeServiceAccount, "claude-sonnet-4-5@20250929", "", false},
		{"Vertex short stale", AccountTypeServiceAccount, "claude-sonnet-4-5", "", true},
		{"Vertex explicit mapping", AccountTypeServiceAccount, "vertex-provider", "vertex-provider", false},
		{"API key keeps short", AccountTypeAPIKey, "claude-sonnet-4-5", "", false},
		{"API key ignores long", AccountTypeAPIKey, "claude-sonnet-4-5-20250929", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := Account{ID: 1, Platform: PlatformAnthropic, Type: tc.kind, Status: StatusActive, Schedulable: true, Credentials: map[string]any{}}
			if tc.mapped != "" {
				a.Credentials["model_mapping"] = map[string]any{"*": tc.mapped}
			}
			setAccountModelRateLimitSnapshot(&a, tc.blocked, time.Now().Add(time.Hour), upstreamModelNotFoundReason, time.Now())
			repo := &mockAccountRepoForPlatform{accounts: []Account{a}, accountsByID: map[int64]*Account{1: &a}}
			got := (&GatewayService{accountRepo: repo, cfg: testConfig()}).DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "claude-sonnet-4-5", PlatformAnthropic)
			require.True(t, got.HasAccountsInPool)
			require.Equal(t, tc.want, got.HasModelSupport)
		})
	}
}
