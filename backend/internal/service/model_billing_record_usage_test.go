//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func runtimeModelBillingSettings(t *testing.T, config ModelBillingConfig) *SettingService {
	t.Helper()
	raw, err := json.Marshal(config)
	require.NoError(t, err)
	return NewSettingService(&modelBillingCASRepo{value: string(raw), revision: "r1"}, nil)
}

func TestModelBillingRecordUsageBothGateways(t *testing.T) {
	for _, openAI := range []bool{false, true} {
		for _, subscription := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, groupID := range []int64{5, 6} {
					t.Run(fmt.Sprintf("openai=%v/subscription=%v/enabled=%v/group=%d", openAI, subscription, enabled, groupID), func(t *testing.T) {
						logs := &openAIRecordUsageLogRepoStub{inserted: true}
						billing := &simpleModeUsageBillingRepoStub{}
						group := &Group{ID: groupID, Platform: PlatformOpenAI, RateMultiplier: .2}
						key := &APIKey{ID: 1, GroupID: &group.ID, Group: group, Quota: 100, RateLimit5h: 100}
						user := &User{ID: 2}
						accountRate := .3
						account := &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: &accountRate, Extra: map[string]any{"quota_limit": 100.0}}
						var sub *UserSubscription
						if subscription {
							group.SubscriptionType = SubscriptionTypeSubscription
							sub = &UserSubscription{ID: 8}
						}
						settings := runtimeModelBillingSettings(t, ModelBillingConfig{Enabled: enabled, Rules: []ModelBillingRule{{Model: "gpt-*", Multiplier: 3, GroupIDs: []int64{5}}, {Model: "claude-*", Multiplier: 3, GroupIDs: []int64{5}}}})
						var base *CostBreakdown
						var search *CostBreakdown
						if openAI {
							svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil, nil)
							result := &OpenAIForwardResult{RequestID: "model-billing-openai", Model: "gpt-5.1", Usage: OpenAIUsage{InputTokens: 1000, OutputTokens: 100}, SearchCount: 2}
							base = expectedOpenAICost(t, svc, result.Model, result.Usage, .2)
							search = svc.billingService.CalculateSearchCost(2, nil, .2)
							svc.settingService = settings
							input := &OpenAIRecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: sub, APIKeyService: &apiKeyQuotaUpdaterStub{}}
							for range 2 {
								require.NoError(t, svc.RecordUsage(context.Background(), input))
							}
						} else {
							group.Platform, account.Platform = PlatformAnthropic, PlatformAnthropic
							svc := newGatewayRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil)
							result := &ForwardResult{RequestID: "model-billing-generic", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 100}, SearchCount: 2}
							var err error
							base, err = svc.billingService.CalculateCost(result.Model, UsageTokens{InputTokens: 1000, OutputTokens: 100}, .2)
							require.NoError(t, err)
							search = svc.billingService.CalculateSearchCost(2, nil, .2)
							svc.settingService = settings
							input := &RecordUsageInput{Result: result, APIKey: key, User: user, Account: account, Subscription: sub, APIKeyService: &apiKeyQuotaUpdaterStub{}}
							for range 2 {
								require.NoError(t, svc.RecordUsage(context.Background(), input))
							}
						}
						factor := 1.0
						if enabled && groupID == 5 {
							factor = 3
						}
						want := QuantizeUsageBillingAmount(base.ActualCost*factor + search.ActualCost)
						require.Len(t, billing.seen, 1, "replay preserves billing identity")
						require.Len(t, billing.cmds, 2)
						for _, cmd := range billing.cmds {
							require.InDelta(t, want, cmd.ActualCost, 1e-12)
							require.InDelta(t, want, cmd.APIKeyQuotaCost, 1e-12)
							require.InDelta(t, want, cmd.APIKeyRateLimitCost, 1e-12)
							require.InDelta(t, (base.TotalCost+search.TotalCost)*accountRate, cmd.AccountQuotaCost, 1e-8)
							if subscription {
								require.InDelta(t, want, cmd.SubscriptionCost, 1e-12)
								require.Zero(t, cmd.BalanceCost)
							} else {
								require.InDelta(t, want, cmd.BalanceCost, 1e-12)
								require.Zero(t, cmd.SubscriptionCost)
							}
						}
						require.InDelta(t, want, logs.lastLog.ActualCost, 1e-12)
						require.InDelta(t, base.TotalCost+search.TotalCost, logs.lastLog.TotalCost, 1e-12)
						require.InDelta(t, .2*factor, logs.lastLog.RateMultiplier, 1e-12)
						require.Equal(t, accountRate, *logs.lastLog.AccountRateMultiplier)
					})
				}
			}
		}
	}
}

func TestModelBillingFreeFastRepricesOnce(t *testing.T) {
	logs := &openAIRecordUsageLogRepoStub{inserted: true}
	billing := &openAIRecordUsageBillingRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, billing, nil, nil, nil)
	svc.settingService = runtimeModelBillingSettings(t, ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: "gpt-5.1", Multiplier: 3}}})
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	inputPrice, outputPrice, fast := .001, .002, 4.0
	groupID := int64(7)
	key := &APIKey{ID: 1, Quota: 100, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformOpenAI, Hydrated: true, RateMultiplier: .2, FreeOpenAIFast: true,
		ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-5.1"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice, FastMultiplier: &fast}}}}
	tier := "priority"
	require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "free-fast-model-billing", Model: "gpt-5.1", ServiceTier: &tier, Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}}, APIKey: key, User: &User{ID: 2}, Account: &Account{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth}, APIKeyService: &apiKeyQuotaUpdaterStub{}}))
	standard := inputPrice*100 + outputPrice*50
	require.InDelta(t, standard*fast, logs.lastLog.TotalCost, 1e-12)
	require.InDelta(t, standard*.2*3, logs.lastLog.ActualCost, 1e-12)
	require.InDelta(t, logs.lastLog.ActualCost, billing.lastCmd.BalanceCost, 1e-12)
}

func TestModelBillingResponseRepricingBothGateways(t *testing.T) {
	for _, openAI := range []bool{false, true} {
		t.Run(fmt.Sprint(openAI), func(t *testing.T) {
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			users := &openAIRecordUsageUserRepoStub{}
			fields := ChannelUsageFields{BillingModelSource: BillingModelSourceResponse}
			var want, total float64
			if openAI {
				svc := newOpenAIRecordUsageServiceForTest(logs, users, nil, nil)
				cheap, expensive, cheapCost, _ := orderedResponseBillingModels(t, svc.billingService, UsageTokens{InputTokens: 100, OutputTokens: 50}, openAICheapFixtureModel, openAIPriceyFixtureModel)
				fields.OriginalModel, fields.ChannelMappedModel = expensive, expensive
				svc.settingService = runtimeModelBillingSettings(t, ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: expensive, Multiplier: 2}, {Model: cheap, Multiplier: 3}}})
				require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{RequestID: "response-reprice", Model: expensive, UpstreamResponseModel: cheap, Usage: OpenAIUsage{InputTokens: 100, OutputTokens: 50}}, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3}, ChannelUsageFields: fields}))
				want, total = cheapCost.ActualCost*3, cheapCost.TotalCost
			} else {
				svc := newGatewayRecordUsageServiceForTest(logs, users, nil)
				cheap, expensive, cheapCost, _ := orderedResponseBillingModels(t, svc.billingService, UsageTokens{InputTokens: 100, OutputTokens: 50}, anthropicCheapFixtureModel, anthropicPriceyFixtureModel)
				fields.OriginalModel, fields.ChannelMappedModel = expensive, expensive
				svc.settingService = runtimeModelBillingSettings(t, ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: expensive, Multiplier: 2}, {Model: cheap, Multiplier: 3}}})
				require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{Result: &ForwardResult{RequestID: "response-reprice", Model: expensive, UpstreamResponseModel: cheap, Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 50}}, APIKey: &APIKey{ID: 1}, User: &User{ID: 2}, Account: &Account{ID: 3}, ChannelUsageFields: fields}))
				want, total = cheapCost.ActualCost*3, cheapCost.TotalCost
			}
			require.InDelta(t, total, logs.lastLog.TotalCost, 1e-12)
			require.InDelta(t, want, logs.lastLog.ActualCost, 1e-12)
			require.InDelta(t, want, users.lastAmount, 1e-12)
		})
	}
}

func TestModelBillingFrozenSubscriptionKeepsAdmissionPolicy(t *testing.T) {
	billing := NewBillingService(nil, nil)
	model := "gpt-5.1"
	resolved := NewModelPricingResolver(nil, billing).Resolve(context.Background(), PricingInput{Model: model})
	require.NotNil(t, resolved)
	snap := subscriptionFrozenPricing{Models: map[string]subscriptionResolvedPricing{subscriptionPricingModelKey(model): freezeResolvedPricing(resolved)}, Multiplier: .2, PricingAt: time.Now(), BillingGroupID: 7,
		ModelBilling: ModelBillingConfig{Enabled: true, Rules: []ModelBillingRule{{Model: model, Multiplier: 3, GroupIDs: []int64{7}}}}}
	tokens := UsageTokens{InputTokens: 100, OutputTokens: 50}
	cost, err := snap.calculate(billing, context.Background(), model, tokens, "", "")
	require.NoError(t, err)
	legacy := snap
	legacy.ModelBilling = ModelBillingConfig{}
	base, err := legacy.calculate(billing, context.Background(), model, tokens, "", "")
	require.NoError(t, err)
	require.Equal(t, base.TotalCost, cost.TotalCost)
	require.InDelta(t, base.ActualCost*3, cost.ActualCost, 1e-12)
}
