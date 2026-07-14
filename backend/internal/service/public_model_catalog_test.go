//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScalePricePtr(t *testing.T) {
	in := 3e-06
	out := scalePricePtr(&in, 0.8)
	require.NotNil(t, out)
	require.InDelta(t, 2.4e-06, *out, 1e-12)
	require.Nil(t, scalePricePtr(nil, 0.8))
}

func TestPublicCatalogModelsUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for _, m := range PublicCatalogModels {
		require.NotContains(t, seen, m.Name)
		seen[m.Name] = struct{}{}
	}
	require.Len(t, seen, len(PublicCatalogModels))
}

func TestListPublicModelPricing_AppliesMultiplier(t *testing.T) {
	billing := newTestBillingService()
	play := &PlayService{}
	rows := play.ListPublicModelPricing(t.Context(), billing)
	require.NotEmpty(t, rows)

	var sonnet *PublicModelPricingRow
	for i := range rows {
		if rows[i].Name == "claude-sonnet-4-6" {
			sonnet = &rows[i]
			break
		}
	}
	require.NotNil(t, sonnet)
	require.InDelta(t, 1, sonnet.RateMultiplier, 0.001)
	require.NotNil(t, sonnet.OfficialInputPrice)
	require.NotNil(t, sonnet.OurInputPrice)
	require.InDelta(t, *sonnet.OfficialInputPrice, *sonnet.OurInputPrice, 1e-12)
}
