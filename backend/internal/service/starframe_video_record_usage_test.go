//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStarframeActualRecordUsageCannotBecomeTokenOrMappedFreeUsage(t *testing.T) {
	for _, mapped := range []bool{false, true} {
		groupID := int64(128)
		price := 0.037
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		channelModel := "ch-custom-fast"
		fields := ChannelUsageFields{OriginalModel: channelModel}
		if mapped {
			channelModel = "token-only-model"
			fields.ChannelMappedModel = channelModel
			fields.BillingModelSource = BillingModelSourceChannelMapped
		}
		svc.resolver = newOpenAITokenImageChannelPricingResolverForTest(t, groupID, channelModel)
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{RequestID: "starframe-video:9:task-1", Model: "ch-custom-fast", BillingModel: "ch-custom-fast", UpstreamModel: "ch-custom-fast", VideoCount: 1, VideoResolution: "720p", VideoDurationSeconds: 10, Duration: time.Second, StarframeVideoBilling: &StarframeVideoBilling{Model: "ch-custom-fast", Duration: 10, Resolution: "720p", UnitPrice: price}},
			APIKey: &APIKey{ID: 10128, GroupID: i64p(groupID), Group: &Group{ID: groupID, Platform: PlatformOpenAI, RateMultiplier: 1, VideoRateIndependent: true, VideoRateMultiplier: 1,
				VideoModelPrices: map[string]map[string]float64{"ch-custom-fast": {"720p": price * 2}}}},
			User: &User{ID: 20128}, Account: &Account{ID: 9, Platform: PlatformOpenAI}, ChannelUsageFields: fields,
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		require.InDelta(t, price*10, usageRepo.lastLog.TotalCost, 1e-12)
		require.InDelta(t, price*10, usageRepo.lastLog.ActualCost, 1e-12)
		require.Equal(t, string(BillingModeVideo), *usageRepo.lastLog.BillingMode)
		require.Equal(t, "ch-custom-fast", usageRepo.lastLog.Model)
	}
}
