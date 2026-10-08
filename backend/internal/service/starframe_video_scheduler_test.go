//go:build unit

package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStarframeSchedulerVideoDoesNotUseTextProfitRate(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	for _, scenario := range []struct {
		name       string
		capability OpenAIEndpointCapability
		protocol   string
		available  bool
	}{
		{"starframe-video", OpenAIEndpointCapabilityVideos, "starframe", true},
		{"starframe-text-unchanged", OpenAIEndpointCapabilityResponses, "starframe", false},
		{"agnes-video-unchanged", OpenAIEndpointCapabilityVideos, "agnes", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			account := starframeTestAccount()
			account.Credentials["model_mapping"] = map[string]any{"ch-custom": "ch-custom"}
			account.Status, account.Schedulable, account.Concurrency = StatusActive, true, 5
			account.Credentials["video_protocol"] = scenario.protocol
			account.Credentials["openai_capabilities"] = []string{"starframe", "responses"}
			if scenario.protocol == "agnes" {
				delete(account.Credentials, "openai_capabilities")
			}
			profitControlTestAccountWithRate(account, 0.8)
			cache := &upstreamCostTrackingConcurrencyCache{loadMap: map[int64]*AccountLoadInfo{account.ID: {AccountID: account.ID}}}
			svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{*account}}, cfg: &config.Config{},
				rateLimitService: newOpenAIAdvancedSchedulerRateLimitService("true"), concurrencyService: NewConcurrencyService(cache)}
			groupID := int64(7)
			group := profitControlTestGroup(groupID, 0.5, 0)
			group.VideoModelPrices = map[string]map[string]float64{"ch-custom": {"720p": 0.037}}
			selection, _, err := svc.SelectAccountWithSchedulerForCapability(profitControlTestCtx(group), &groupID, "", "", "ch-custom", nil, OpenAIUpstreamTransportHTTPSSE, scenario.capability, false, false, false)
			if !scenario.available {
				require.Error(t, err)
				require.Nil(t, selection)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, account.ID, selection.Account.ID)
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}
