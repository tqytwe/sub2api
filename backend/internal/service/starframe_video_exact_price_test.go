//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStarframeExactModelPricingAndGroupSaveRoundtrip(t *testing.T) {
	model := "ch-custom-video-1.5-fast"
	info := StarframeVideoRequest{Model: model, Duration: 10, Resolution: "720p"}
	svc := &OpenAIGatewayService{}
	t.Run("unconfigured full ID cannot borrow Grok family", func(t *testing.T) {
		group := &Group{VideoModelPrices: map[string]map[string]float64{VideoPriceFamilyGrokImagineVideo15: {"720p": 0.037}}}
		quote, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, info)
		require.Error(t, err)
		require.Nil(t, quote)
	})
	t.Run("full ID own price wins independently", func(t *testing.T) {
		group := &Group{VideoModelPrices: map[string]map[string]float64{VideoPriceFamilyGrokImagineVideo15: {"720p": 0.037}, model: {"720p": 0.074}}}
		quote, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, info)
		require.NoError(t, err)
		require.Equal(t, 0.074, quote.UnitPrice)
	})
	t.Run("explicit old flat price remains supported", func(t *testing.T) {
		price := 0.061
		group := &Group{VideoPrice720P: &price}
		quote, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, info)
		require.NoError(t, err)
		require.Equal(t, price, quote.UnitPrice)
	})
	t.Run("save reload preserves case distinct full IDs and canonical price", func(t *testing.T) {
		prices := map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo15: {"720p": 0.037},
			model:                              {"720p": 0.074},
			"zz-custom-video-1.5-fast":         {"720p": 0.089},
			"CH-Case-Fast":                     {"720p": 0.052},
			"ch-case-fast":                     {"720p": 0.063},
		}
		// Both admin service and repository normalize before JSONB persistence.
		stored, err := json.Marshal(NormalizeVideoModelPrices(NormalizeVideoModelPrices(prices)))
		require.NoError(t, err)
		var reloaded map[string]map[string]float64
		require.NoError(t, json.Unmarshal(stored, &reloaded))
		group := &Group{VideoModelPrices: reloaded}
		for _, name := range []string{model, "zz-custom-video-1.5-fast", "CH-Case-Fast", "ch-case-fast"} {
			request := info
			request.Model = name
			quote, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, request)
			require.NoError(t, err)
			require.Equal(t, prices[name]["720p"], quote.UnitPrice, name)
		}
		require.Equal(t, 0.089, *group.GetVideoPriceForModel(VideoPriceFamilyGrokImagineVideo15, "720p"))
	})
}
