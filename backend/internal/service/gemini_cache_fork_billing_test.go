package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiCacheQuoteUsesGroupMultiplier(t *testing.T) {
	s := &GatewayService{billingService: NewBillingService(nil, nil)}
	groupID := int64(1)
	group := &Group{ID: groupID, RateMultiplier: 1}
	input := &RecordUsageInput{
		APIKey: &APIKey{User: &User{ID: 1}, GroupID: &groupID, Group: group},
		Result: &ForwardResult{Model: "gemini-3.6-flash", CacheStorageTokenHours: 1000},
	}
	base := s.QuoteUsageCost(t.Context(), input)
	group.RateMultiplier = 3
	require.Positive(t, base)
	require.InDelta(t, base*3, s.QuoteUsageCost(t.Context(), input), 1e-12)
}
