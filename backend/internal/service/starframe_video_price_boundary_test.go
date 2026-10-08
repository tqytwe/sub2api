//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStarframeRawPricePersistenceAndLegacyProjection(t *testing.T) {
	reload := func(prices map[string]map[string]float64) *Group {
		t.Helper()
		body, err := json.Marshal(NormalizeVideoModelPrices(NormalizeVideoModelPrices(prices)))
		require.NoError(t, err)
		var stored map[string]map[string]float64
		require.NoError(t, json.Unmarshal(body, &stored))
		return &Group{VideoModelPrices: stored}
	}
	t.Run("legacy unknown mixed case survives exact persistence", func(t *testing.T) {
		group := reload(map[string]map[string]float64{"CustomVideoMixCase": {"720p": 0.074}})
		for _, model := range []string{"CustomVideoMixCase", "customvideomixcase"} {
			price := group.GetVideoPriceForModel(model, "720p")
			require.NotNil(t, price)
			require.Equal(t, 0.074, *price)
		}
		svc := &OpenAIGatewayService{}
		_, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, StarframeVideoRequest{Model: "customvideomixcase", Resolution: "720p", Duration: 10})
		require.Error(t, err, "legacy case folding must not create an SF configuration")
	})
	t.Run("legacy aliases retain disjoint tiers without SF borrowing", func(t *testing.T) {
		group := reload(map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo15: {"480p": 0.037},
			"grok-imagine-video-1.5-preview":   {"720p": 0.074},
		})
		for tier, want := range map[string]float64{"480p": 0.037, "720p": 0.074} {
			price := group.GetVideoPriceForModel(VideoPriceFamilyGrokImagineVideo15, tier)
			require.NotNil(t, price)
			require.Equal(t, want, *price)
		}
		svc := &OpenAIGatewayService{}
		_, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, StarframeVideoRequest{Model: VideoPriceFamilyGrokImagineVideo15, Resolution: "720p", Duration: 10})
		require.Error(t, err)
	})
	t.Run("only custom configuration cannot persist a canonical SF shadow", func(t *testing.T) {
		group := reload(map[string]map[string]float64{"ch-custom-video-1.5-fast": {"720p": 0.074}})
		require.NotContains(t, group.VideoModelPrices, VideoPriceFamilyGrokImagineVideo15)
		svc := &OpenAIGatewayService{}
		_, err := svc.SnapshotStarframeVideoBilling(context.Background(), &APIKey{Group: group}, StarframeVideoRequest{Model: VideoPriceFamilyGrokImagineVideo15, Resolution: "720p", Duration: 10})
		require.Error(t, err)
	})
	t.Run("legacy alias collisions keep sorted last wins", func(t *testing.T) {
		group := reload(map[string]map[string]float64{
			VideoPriceFamilyGrokImagineVideo15: {"480p": 0.037},
			"grok-imagine-video-1.5-preview":   {"480p": 0.074},
		})
		for i := 0; i < 30; i++ {
			price := group.GetVideoPriceForModel(VideoPriceFamilyGrokImagineVideo15, "480p")
			require.NotNil(t, price)
			require.Equal(t, 0.074, *price)
		}
	})
}
