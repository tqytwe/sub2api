package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregateCampaignRules(t *testing.T) {
	rules := aggregateCampaignRules([]PlayCampaign{
		{Rules: PlayCampaignRules{RechargeBonusPct: 10, BlindboxExtraOpens: 1, ArenaScoreMultiplier: 2}},
		{Rules: PlayCampaignRules{RechargeBonusPct: 5, BlindboxExtraOpens: 2, ArenaScoreMultiplier: 1.5}},
	})
	require.Equal(t, 10.0, rules.RechargeBonusPct)
	require.Equal(t, 3, rules.BlindboxExtraOpens)
	require.InDelta(t, 3.0, rules.ArenaScoreMultiplier, 1e-9)
}

func TestParsePlayCampaignRules(t *testing.T) {
	got := ParsePlayCampaignRules(`{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2}`)
	require.Equal(t, 10.0, got.RechargeBonusPct)
	require.Equal(t, 2, got.BlindboxExtraOpens)
	require.Equal(t, 2.0, got.ArenaScoreMultiplier)
}
