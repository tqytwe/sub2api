//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSubscriptionFrozenPricingSerialization(t *testing.T) {
	for _, mode := range []BillingMode{BillingModeToken, BillingModePerRequest} {
		t.Run(string(mode), func(t *testing.T) {
			price := ChannelModelPricing{Platform: PlatformOpenAI, Models: []string{"gpt-5.1"}, BillingMode: mode, PerRequestPrice: testPtrFloat64(3), InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6), ReasoningEffortMultipliers: map[string]float64{"high": 2}, Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: testPtrInt(1000), InputPrice: testPtrFloat64(4e-6), OutputPrice: testPtrFloat64(16e-6)}}}
			repo := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
				return []Channel{{ID: 1, Status: StatusActive, GroupIDs: []int64{100}, ModelPricing: []ChannelModelPricing{price}}}, nil
			}, getGroupPlatformsFn: func(context.Context, []int64) (map[int64]string, error) {
				return map[int64]string{100: PlatformOpenAI}, nil
			}}
			resolver := NewModelPricingResolver(NewChannelService(repo, nil, nil, nil, nil), newTestBillingServiceForResolver())
			resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gpt-5.1", GroupID: groupIDPtr(), Group: &Group{LongContextPricingEnabled: false}})
			require.NotNil(t, resolved.channelPricing)
			s := NewBillingService(&config.Config{}, nil)
			tokens := UsageTokens{InputTokens: 82, OutputTokens: 20, CacheCreationTokens: 7, CacheReadTokens: 11}
			at := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
			snap := subscriptionFrozenPricing{Version: 1, Models: map[string]subscriptionResolvedPricing{"gpt-5.1": freezeResolvedPricing(resolved)}, Multiplier: 0.5, PricingAt: at}
			want, err := snap.calculate(s, context.Background(), "gpt-5.1", tokens, "priority", "high")
			require.NoError(t, err)
			require.Greater(t, want.TotalCost, 0.0)
			data, err := json.Marshal(snap)
			require.NoError(t, err)
			var restored subscriptionFrozenPricing
			require.NoError(t, json.Unmarshal(data, &restored))
			resolved.channelPricing.ReasoningEffortMultipliers["high"] = 100
			resolved.BasePricing = &ModelPricing{InputPricePerToken: 100, OutputPricePerToken: 100}
			got, err := restored.calculate(s, context.Background(), "gpt-5.1", tokens, "priority", "high")
			require.NoError(t, err)
			require.Equal(t, want, got)
			require.Equal(t, false, restored.Models["gpt-5.1"].resolved().longContextPricingEnabled)
			require.Equal(t, 2.0, restored.Models["gpt-5.1"].resolved().channelPricing.ReasoningEffortMultipliers["high"])
		})
	}
}

func TestSubscriptionTextAdmissionScope(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"string", `"hello"`, true},
		{"message", `[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]`, true},
		{"opaque_text_compaction", `[{"type":"compaction","encrypted_content":"fixture"}]`, true},
		{"text_reasoning", `[{"type":"reasoning","summary":[{"type":"summary_text","text":"hi"}]}]`, true},
		{"image", `[{"role":"user","content":[{"type":"input_image","image_url":"fixture"}]}]`, false},
		{"file", `[{"role":"user","content":[{"type":"input_file","file_id":"fixture"}]}]`, false},
		{"audio", `[{"role":"user","content":[{"type":"input_audio","data":"fixture"}]}]`, false},
		{"tool", `[{"type":"function_call_output","output":"fixture"}]`, false},
		{"unknown", `[{"role":"user","content":[{"type":"future_media","text":"fixture"}]}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.ok, subscriptionTextInput(gjson.Parse(tc.body))) })
	}
}

func TestSubscriptionFrozenResponseEligibility(t *testing.T) {
	snap := &subscriptionFrozenPricing{Models: map[string]subscriptionResolvedPricing{"gpt-5.1": {Identified: true, ChannelPriced: true}, "unpriced": {}}}
	ctx := context.WithValue(context.Background(), subscriptionFrozenKey{}, snap)
	s := &OpenAIGatewayService{}
	identified, channel := s.hasIdentifiedOpenAIResponsePricing(ctx, "gpt-5.1", nil)
	require.True(t, identified)
	require.True(t, channel)
	identified, channel = s.hasIdentifiedOpenAIResponsePricing(ctx, "unpriced", nil)
	require.False(t, identified)
	require.False(t, channel)
	identified, channel = s.hasIdentifiedOpenAIResponsePricing(ctx, "new-after-send", nil)
	require.False(t, identified)
	require.False(t, channel)
}
