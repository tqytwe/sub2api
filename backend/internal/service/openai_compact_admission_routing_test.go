package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type compactAdmissionRoutingRepo struct{ schedulerTestOpenAIAccountRepo }

func (r compactAdmissionRoutingRepo) GetOpenAITurnAdmission(ctx context.Context, id int64) (*Account, *Account, error) {
	a, err := r.GetByID(ctx, id)
	return a, nil, err
}

func TestCompactAdmissionRoutingCooldownUsesOutboundModel(t *testing.T) {
	for _, tc := range []struct {
		name, forwardModel, normalModel, compactModel, globalModel, blockedModel string
		compact, passthrough, rawChat, noContext, nativeV2, oauth, wantBlocked   bool
	}{
		{name: "normal mapping", normalModel: "gpt-5.4", blockedModel: "gpt-5.4", wantBlocked: true},
		{name: "native v2 ignores legacy mapping", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", nativeV2: true},
		{name: "OAuth normalizes outbound alias", normalModel: "gpt-5.4-high", blockedModel: "gpt-5.4", oauth: true, wantBlocked: true},
		{name: "channel compact mapping", forwardModel: "channel-model", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, wantBlocked: true},
		{name: "compact exact mapping", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, wantBlocked: true},
		{name: "compact flag without context", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, noContext: true, wantBlocked: true},
		{name: "global compact fallback", globalModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, wantBlocked: true},
		{name: "channel then account mapping", forwardModel: "channel-model", normalModel: "gpt-5.4", blockedModel: "gpt-5.4", wantBlocked: true},
		{name: "passthrough compact", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, passthrough: true, wantBlocked: true},
		{name: "passthrough skips normal mapping", normalModel: "gpt-5.4", blockedModel: "gpt-5.4", passthrough: true},
		{name: "normal ignores compact cooldown", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.5"},
		{name: "compact ignores normal cooldown", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.4", compact: true},
		{name: "raw chat ignores compact mapping", normalModel: "gpt-5.4", compactModel: "gpt-5.5", blockedModel: "gpt-5.5", compact: true, rawChat: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forwardModel := tc.forwardModel
			if forwardModel == "" {
				forwardModel = "public-model"
			}
			a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{}, Extra: map[string]any{"openai_compact_mode": "force_on", "openai_passthrough": tc.passthrough}}
			if tc.normalModel != "" {
				mapping := map[string]any{forwardModel: tc.normalModel, "public-model": tc.normalModel}
				if tc.forwardModel != "" {
					mapping["public-model"] = "gpt-5.3"
				}
				a.Credentials["model_mapping"] = mapping
			}
			if tc.compactModel != "" {
				a.Credentials["compact_model_mapping"] = map[string]any{forwardModel: tc.compactModel}
			}
			if tc.oauth {
				a.Type = AccountTypeOAuth
			}
			if tc.rawChat {
				a.Extra["openai_responses_mode"] = "force_chat_completions"
			}
			repo := compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}}
			svc := &OpenAIGatewayService{accountRepo: repo, cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: tc.globalModel}}}
			ctx := context.Background()
			if !tc.noContext {
				ctx = WithOpenAIForwardModel(ctx, forwardModel, tc.compact)
			}
			now := time.Now()
			svc.recordOpenAIAccountModelTransientFailure(&a, tc.blockedModel, now)
			svc.recordOpenAIAccountModelTransientFailure(&a, tc.blockedModel, now)
			selected, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{a}, "public-model", nil, tc.compact || tc.nativeV2, "", false)
			require.Equal(t, tc.wantBlocked, selected == nil, "legacy selection must check the outbound model")
			scheduler := &defaultOpenAIAccountScheduler{service: svc}
			compatible := scheduler.isAccountRequestCompatible(ctx, &a, OpenAIAccountScheduleRequest{RequestedModel: "public-model", RequireCompact: tc.compact || tc.nativeV2, Platform: PlatformOpenAI})
			require.Equal(t, !tc.wantBlocked, compatible, "advanced selection must match legacy selection")
		})
	}
}

func TestCompactCooldownExactExpiry(t *testing.T) {
	state := newOpenAIAccountModelTransientState(4)
	now := time.Date(2026, 10, 9, 23, 0, 0, 0, time.UTC)
	state.recordFailure(71, "gpt-5.5", now)
	decision := state.recordFailure(71, "gpt-5.5", now)
	require.True(t, state.isBlocked(71, "gpt-5.5", decision.BlockUntil.Add(-time.Nanosecond)))
	require.False(t, state.isBlocked(71, "gpt-5.5", decision.BlockUntil))
}

func TestCompactAdmissionRoutingPersistedCooldownUsesOutboundModel(t *testing.T) {
	for _, limitedModel := range []string{"gpt-5.4", "gpt-5.5"} {
		t.Run(limitedModel, func(t *testing.T) {
			a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "gpt-5.4"}, "compact_model_mapping": map[string]any{"public-model": "gpt-5.5"}},
				Extra:       map[string]any{"openai_compact_mode": "force_on", modelRateLimitsKey: map[string]any{limitedModel: map[string]any{"rate_limit_reset_at": time.Now().Add(time.Minute).Format(time.RFC3339)}}}}
			svc := &OpenAIGatewayService{accountRepo: compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}}}
			ctx := WithOpenAIForwardModel(context.Background(), "public-model", true)
			selected, _, _ := svc.selectBestAccount(ctx, nil, PlatformOpenAI, []Account{a}, "public-model", nil, true, "", false)
			require.Equal(t, limitedModel == "gpt-5.5", selected == nil)
		})
	}
}

func TestCompactAdmissionRoutingReselectsBeforeForward(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_load", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			primary := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.5"}}, Extra: map[string]any{"openai_compact_mode": "force_on"}}
			secondary := primary
			secondary.ID = 72
			secondary.Priority = 5
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_load"
			svc := &OpenAIGatewayService{cfg: cfg,
				accountRepo:        compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{primary, secondary}}},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			if mode == "advanced" {
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			}
			now := time.Now()
			svc.recordOpenAIAccountModelTransientFailure(&primary, "gpt-5.5", now)
			svc.recordOpenAIAccountModelTransientFailure(&primary, "gpt-5.5", now)
			ctx := WithOpenAIForwardModel(context.Background(), "gpt-5.4", true)
			selection, _, err := svc.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-5.4", nil, OpenAIUpstreamTransportAny, true)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, secondary.ID, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			admitted, err := svc.AdmitOpenAITurn(ctx, nil, selection.Account, "gpt-5.5")
			require.NoError(t, err)
			require.Equal(t, secondary.ID, admitted.ID)
			require.True(t, svc.getOpenAIAccountModelTransientState().isBlocked(primary.ID, "gpt-5.5", time.Now()), "selection must retain the first account's protection")
		})
	}
}

func TestCompactAdmissionAdvancedWithoutForwardContextIgnoresNormalCooldown(t *testing.T) {
	a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.5"}}, Extra: map[string]any{"openai_compact_mode": "force_on"}}
	svc := &OpenAIGatewayService{cfg: &config.Config{},
		accountRepo:        compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}},
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
		rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService("true")}
	now := time.Now()
	svc.recordOpenAIAccountModelTransientFailure(&a, "gpt-5.4", now)
	svc.recordOpenAIAccountModelTransientFailure(&a, "gpt-5.4", now)
	selection, _, err := svc.SelectAccountWithScheduler(context.Background(), nil, "", "", "gpt-5.4", nil, OpenAIUpstreamTransportAny, true)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, a.ID, selection.Account.ID)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestCompactAdmissionPreviousResponseCooldownUsesOutboundModel(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		for _, model := range []string{"gpt-5.4", "gpt-5.5"} {
			t.Run(map[bool]string{false: "transient/", true: "persisted/"}[persisted]+model, func(t *testing.T) {
				a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
					Credentials: map[string]any{"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.5"}}, Extra: map[string]any{"openai_compact_mode": "force_on"}}
				if persisted {
					a.Extra[modelRateLimitsKey] = map[string]any{model: map[string]any{"rate_limit_reset_at": time.Now().Add(time.Minute).Format(time.RFC3339)}}
				}
				cache := &stubGatewayCache{}
				store := NewOpenAIWSStateStore(cache)
				stale := a
				stale.Extra = map[string]any{"openai_compact_mode": "force_on"}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: cache, openaiWSStateStore: store,
					accountRepo:       compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}},
					schedulerSnapshot: &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{accountsByID: map[int64]*Account{a.ID: &stale}}}}
				if !persisted {
					now := time.Now()
					svc.recordOpenAIAccountModelTransientFailure(&a, model, now)
					svc.recordOpenAIAccountModelTransientFailure(&a, model, now)
				}
				ctx := WithOpenAIForwardModel(context.Background(), "gpt-5.4", true)
				require.NoError(t, store.BindResponseAccount(ctx, 0, "resp_compact", a.ID, time.Hour))
				id, selected, _, _ := svc.resolveAccountByPreviousResponseIDForCapability(ctx, nil, "resp_compact", "gpt-5.4", nil, "", true)
				boundID, err := store.GetResponseAccount(ctx, 0, "resp_compact")
				require.NoError(t, err)
				if model == "gpt-5.4" {
					require.Equal(t, a.ID, id)
					require.NotNil(t, selected)
					require.Equal(t, a.ID, boundID, "an unrelated normal-model cooldown must not delete continuation binding")
				} else {
					require.Zero(t, id)
					require.Nil(t, selected)
					require.Zero(t, boundID)
				}
			})
		}
	}
}

func TestCompactAdmissionStickyPreservesBindingAcrossNormalPersistedCooldown(t *testing.T) {
	for _, mode := range []string{"legacy", "legacy_load", "advanced"} {
		t.Run(mode, func(t *testing.T) {
			a := Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"compact_model_mapping": map[string]any{"gpt-5.4": "gpt-5.5"}},
				Extra:       map[string]any{"openai_compact_mode": "force_on", modelRateLimitsKey: map[string]any{"gpt-5.4": map[string]any{"rate_limit_reset_at": time.Now().Add(time.Minute).Format(time.RFC3339)}}}}
			cache := &stubGatewayCache{}
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.LoadBatchEnabled = mode == "legacy_load"
			svc := &OpenAIGatewayService{cfg: cfg, cache: cache,
				accountRepo:        compactAdmissionRoutingRepo{schedulerTestOpenAIAccountRepo{accounts: []Account{a}}},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{})}
			if mode == "advanced" {
				svc.rateLimitService = newOpenAIAdvancedSchedulerRateLimitService("true")
			}
			ctx := WithOpenAIForwardModel(context.Background(), "gpt-5.4", true)
			require.NoError(t, svc.setStickySessionAccountID(ctx, nil, "compact-session", a.ID, time.Hour))
			selection, decision, err := svc.SelectAccountWithScheduler(ctx, nil, "", "compact-session", "gpt-5.4", nil, OpenAIUpstreamTransportAny, true)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, a.ID, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			require.True(t, decision.StickySessionHit)
			require.Empty(t, cache.deletedSessions)
		})
	}
}
