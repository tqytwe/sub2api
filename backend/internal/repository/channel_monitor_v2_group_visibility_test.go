package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorV2NewConfiguredGroupAppearsWithoutTrafficWhenAuthorized(t *testing.T) {
	cfg := service.ChannelMonitorV2Config{
		Platforms: []service.ChannelMonitorV2PlatformConfig{{Platform: "openai", Enabled: true}},
		GroupIDs:  []int64{7},
	}
	filter := service.ChannelMonitorV2Filter{RestrictGroups: true, AllowedGroupIDs: []int64{7, 9}}
	info := map[int64]channelMonitorV2GroupInfo{
		7: {name: "Existing", platform: "openai"},
		9: {name: "New monitored group", platform: "openai"},
	}
	key := channelMonitorV2MatrixKey{platform: "openai", groupID: 9}
	before := seedChannelMonitorV2MatrixAccumulators(filter, cfg, service.ChannelMonitorV2GroupByPlatformGroup, info)
	require.NotContains(t, before, key)
	cfg.GroupIDs = append(cfg.GroupIDs, 9)
	after := seedChannelMonitorV2MatrixAccumulators(filter, cfg, service.ChannelMonitorV2GroupByPlatformGroup, info)
	require.Contains(t, after, key, "a saved authorized group is visible even before it has traffic")
	require.Equal(t, "New monitored group", after[key].groupName)
	filter.AllowedGroupIDs = []int64{7}
	denied := seedChannelMonitorV2MatrixAccumulators(filter, cfg, service.ChannelMonitorV2GroupByPlatformGroup, info)
	require.NotContains(t, denied, key, "monitor configuration must not grant subscription or exclusive-group access")
	where, _ := channelMonitorV2Where(filter, cfg, "m")
	require.Contains(t, where, "m.group_id = ANY")
}
