package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2MatrixRecentSamplesRespectViewerPrivacy(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "admin"}[admin], func(t *testing.T) {
			metric := ChannelMonitorV2Metric{
				HasSamples: true, RequestCount: 4, SuccessRequests: 4,
				InputTokens: 1000, TokenCount: 1000, CacheRateDenominator: 1000,
				CacheRateNumerator: 900, CacheRate: 0.9, RPM: 0.8, TPM: 200,
				TTFT: ChannelMonitorV2Latency{SampleCount: 4},
			}
			health := ChannelMonitorV2HealthFor(metric)
			require.Equal(t, "healthy", health.Overall, "cache samples can score health independently of request volume")
			samples := []ChannelMonitorV2TrendPoint{{Metrics: metric, Health: health}}
			repo := &channelMonitorV2RepoStub{
				config: ChannelMonitorV2Config{Enabled: true},
				matrix: &ChannelMonitorV2Matrix{Items: []ChannelMonitorV2MatrixRow{{
					Metrics: metric, Health: health,
					Buckets:       []ChannelMonitorV2TrendPoint{{Metrics: metric, Health: health}},
					SampleBuckets: &samples,
				}}},
			}
			result, err := NewChannelMonitorV2Service(repo).Matrix(context.Background(), ChannelMonitorV2Filter{IncludeRecentSamples: true}, ChannelMonitorV2GroupByPlatformGroup, admin)
			require.NoError(t, err)
			sample := (*result.Items[0].SampleBuckets)[0]
			require.True(t, sample.Metrics.HasSamples)
			require.Equal(t, health, sample.Health, "redaction must preserve the server verdict")
			require.Equal(t, 0.9, sample.Metrics.CacheRate)
			if admin {
				require.Equal(t, metric, sample.Metrics)
			} else {
				require.Zero(t, sample.Metrics.RequestCount)
				require.Zero(t, sample.Metrics.InputTokens)
				require.Zero(t, sample.Metrics.CacheRateDenominator)
				require.Zero(t, sample.Metrics.CacheRateNumerator)
				require.Zero(t, sample.Metrics.TTFT.SampleCount)
				require.Equal(t, result.Items[0].Buckets[0].Metrics, sample.Metrics)
			}
		})
	}
}
