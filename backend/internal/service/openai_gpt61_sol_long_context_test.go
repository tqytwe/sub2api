//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGPT61SolLongContextExempt(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	setupToken := &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken}
	apiKey := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	tests := []struct {
		name    string
		account *Account
		result  *OpenAIForwardResult
		tokens  UsageTokens
		want    bool
	}{
		{name: "oauth at threshold uses cached input", account: account, result: &OpenAIForwardResult{UpstreamModel: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 272000, CacheReadTokens: 228000}, want: true},
		{name: "setup token at threshold uses cache creation", account: setupToken, result: &OpenAIForwardResult{Model: "openai/gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 300000, CacheCreationTokens: 200000}, want: true},
		{name: "subscription over threshold is charged", account: account, result: &OpenAIForwardResult{UpstreamModel: "gpt-6.1-sol", Model: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 500001}},
		{name: "api key keeps catalog threshold", account: apiKey, result: &OpenAIForwardResult{UpstreamModel: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 300000}},
		{name: "final sent model controls exemption", account: account, result: &OpenAIForwardResult{UpstreamModel: "gpt-6-sol", Model: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 300000}},
		{name: "fallback model used when sent model missing", account: account, result: &OpenAIForwardResult{Model: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 300000}, want: true},
		{name: "nil result", account: account, tokens: UsageTokens{InputTokens: 300000}},
		{name: "nil account", result: &OpenAIForwardResult{Model: "gpt-6.1-sol"}, tokens: UsageTokens{InputTokens: 300000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, openAIGPT61SolLongContextExempt(tt.account, tt.result, tt.tokens))
		})
	}
}

func TestDisableModelLongContextPricingKeepsCustomIntervals(t *testing.T) {
	svc := newTestBillingService()
	resolver := NewModelPricingResolver(nil, svc)
	longContextEnabled := true
	resolved := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{InputPricePerToken: 1e-6, OutputPricePerToken: 2e-6, LongContextInputThreshold: 272000, LongContextInputMultiplier: 2, LongContextOutputMultiplier: 1.5},
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: testPtrInt(272000), InputPrice: testPtrFloat64(1e-6), OutputPrice: testPtrFloat64(2e-6)},
			{MinTokens: 272000, InputPrice: testPtrFloat64(3e-6), OutputPrice: testPtrFloat64(4e-6)},
		},
		longContextPricingEnabled: true,
	}

	cost, err := svc.CalculateCostUnified(CostInput{
		Model: "gpt-6.1-sol", Tokens: UsageTokens{InputTokens: 300000, OutputTokens: 1000},
		RateMultiplier: 1, Resolver: resolver, Resolved: resolved,
		LongContextBillingEnabled: &longContextEnabled, DisableModelLongContextPricing: true,
	})
	require.NoError(t, err)
	require.InDelta(t, 300000*3e-6, cost.InputCost, 1e-10)
	require.InDelta(t, 1000*4e-6, cost.OutputCost, 1e-10)
}

func TestDisableModelLongContextPricingLeavesBasePrice(t *testing.T) {
	svc := newTestBillingServiceWithOpenAILadderCatalog(t)
	tokens := UsageTokens{InputTokens: 300000, OutputTokens: 1000}
	longContextEnabled := true

	charged, err := svc.CalculateCostUnified(CostInput{
		Model: "gpt-5.4", Tokens: tokens, RateMultiplier: 1,
		LongContextBillingEnabled: &longContextEnabled,
	})
	require.NoError(t, err)
	require.True(t, charged.LongContextBillingApplied)

	exempt, err := svc.CalculateCostUnified(CostInput{
		Model: "gpt-5.4", Tokens: tokens, RateMultiplier: 1,
		LongContextBillingEnabled: &longContextEnabled, DisableModelLongContextPricing: true,
	})
	require.NoError(t, err)
	require.False(t, exempt.LongContextBillingApplied)
	require.InDelta(t, charged.InputCost/2, exempt.InputCost, 1e-10)
	require.InDelta(t, charged.OutputCost/1.5, exempt.OutputCost, 1e-10)
}
