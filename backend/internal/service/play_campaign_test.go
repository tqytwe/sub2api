package service

import (
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAggregateCampaignRules(t *testing.T) {
	rules := aggregateCampaignRules([]PlayCampaign{
		{Rules: PlayCampaignRules{RechargeBonusPct: 10, BlindboxExtraOpens: 1, ArenaScoreMultiplier: 2}},
		{Rules: PlayCampaignRules{RechargeBonusPct: 5, BlindboxExtraOpens: 2, ArenaScoreMultiplier: 1.5}},
	})
	require.Equal(t, 10.0, rules.RechargeBonusPct)
	require.Equal(t, 2, rules.BlindboxExtraOpens)
	require.InDelta(t, 2.0, rules.ArenaScoreMultiplier, 1e-9)
}

func TestAggregateCampaignRulesUsesMaximumAndAppliesHardCaps(t *testing.T) {
	rules := aggregateCampaignRules([]PlayCampaign{
		{Rules: PlayCampaignRules{RechargeBonusPct: 150, BlindboxExtraOpens: 12, ArenaScoreMultiplier: 7}},
		{Rules: PlayCampaignRules{RechargeBonusPct: 80, BlindboxExtraOpens: 8, ArenaScoreMultiplier: 4}},
	})

	require.Equal(t, playCampaignMaxRechargeBonusPct, rules.RechargeBonusPct)
	require.Equal(t, playCampaignMaxBlindboxOpens, rules.BlindboxExtraOpens)
	require.Equal(t, playCampaignMaxArenaMultiplier, rules.ArenaScoreMultiplier)
}

func TestParsePlayCampaignRules(t *testing.T) {
	got := ParsePlayCampaignRules(`{"recharge_bonus_pct":10,"blindbox_extra_opens":2,"arena_score_multiplier":2}`)
	require.Equal(t, 10.0, got.RechargeBonusPct)
	require.Equal(t, 2, got.BlindboxExtraOpens)
	require.Equal(t, 2.0, got.ArenaScoreMultiplier)

	withI18n := ParsePlayCampaignRules(`{"name_i18n":{"en":"Launch week","zh":"开服福利周"}}`)
	require.Equal(t, "Launch week", withI18n.NameI18n["en"])
	require.Equal(t, "开服福利周", withI18n.NameI18n["zh"])
}

func TestValidateAdminPlayCampaignNormalizesNewUserGrowthDefaults(t *testing.T) {
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	campaign := PlayCampaign{
		Name:    "新用户成长奖励",
		StartAt: start,
		EndAt:   start.Add(7 * 24 * time.Hour),
		Rules: PlayCampaignRules{
			CampaignType:        PlayCampaignTypeNewUserGrowth,
			ReferralCampaignID:  7,
			QualificationMetric: PlayCampaignMetricNetRecharge,
			RewardTiers: []PlayCampaignRewardTier{
				{Tier: 1, RequiredAmount: 50, RewardAmount: 50, Currency: "CNY"},
				{Tier: 2, RequiredAmount: 200, RewardAmount: 100, Currency: "CNY"},
				{Tier: 3, RequiredAmount: 500, RewardAmount: 150, Currency: "CNY"},
				{Tier: 4, RequiredAmount: 1000, RewardAmount: 200, Currency: "CNY"},
			},
		},
	}

	require.NoError(t, validateAdminPlayCampaign(&campaign))
	require.Equal(t, PlayCampaignLegacyRebateExclude, campaign.Rules.LegacyRebatePolicy)
	require.True(t, campaign.Rules.RequireInvite)
	require.InDelta(t, 500, campaign.Rules.MaxReward(), 1e-9)
}

func TestValidateAdminPlayCampaignValidatesNewUserGrowthRules(t *testing.T) {
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	base := PlayCampaign{
		Name:    "新用户成长奖励",
		StartAt: start,
		EndAt:   start.Add(7 * 24 * time.Hour),
		Rules: PlayCampaignRules{
			CampaignType:        PlayCampaignTypeNewUserGrowth,
			QualificationMetric: PlayCampaignMetricNetRecharge,
			RewardTiers:         []PlayCampaignRewardTier{{Tier: 1, RequiredAmount: 50, RewardAmount: 501, Currency: "CNY"}},
		},
	}

	err := validateAdminPlayCampaign(&base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "referral campaign")

	base.Rules.ReferralCampaignID = 7
	err = validateAdminPlayCampaign(&base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")

	base.Rules.RewardTiers[0].RewardAmount = 50
	base.Rules.LegacyRebatePolicy = PlayCampaignLegacyRebateStack
	err = validateAdminPlayCampaign(&base)
	require.NoError(t, err)
	require.Equal(t, PlayCampaignLegacyRebateStack, base.Rules.LegacyRebatePolicy)
}

func TestParsePlayCampaignAudienceNormalizesTiers(t *testing.T) {
	got := ParsePlayCampaignAudience(`{"ordinary":true,"vip_tiers":[6,6,-1,2],"registered_within_days":-2}`)
	require.True(t, got.Ordinary)
	require.Equal(t, []int{6, 2}, got.VIPTiers)
	require.Zero(t, got.RegisteredWithinDays)
	require.Equal(t, PlayCampaignAudience{}, ParsePlayCampaignAudience("not-json"))
}

func TestFirstMemberThresholdAndDynamicTier(t *testing.T) {
	tiers := []PlayVIPTier{{Tier: 0, Label: "V0", MinRecharge: 0}, {Tier: 1, Label: "V1", MinRecharge: 20}, {Tier: 7, Label: "V7", MinRecharge: 700}}
	require.Equal(t, 20.0, firstMemberThreshold(tiers))
	require.Equal(t, 7, GetVIPTier(700, tiers).Tier)
}

func TestValidateAdminPlayCampaignCleansI18n(t *testing.T) {
	start := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	campaign := PlayCampaign{
		Name:    "  开服福利周  ",
		StartAt: start,
		EndAt:   start.Add(24 * time.Hour),
		Rules: PlayCampaignRules{
			RechargeBonusPct:     15,
			BlindboxExtraOpens:   2,
			ArenaScoreMultiplier: 2,
			NameI18n: map[string]string{
				"ZH": "  开服福利周  ",
				"en": " Launch week ",
				"":   "ignored",
			},
		},
	}

	require.NoError(t, validateAdminPlayCampaign(&campaign))
	require.Equal(t, "开服福利周", campaign.Name)
	require.Equal(t, map[string]string{"zh": "开服福利周", "en": "Launch week"}, campaign.Rules.NameI18n)
}

func TestValidateAdminPlayCampaignRejectsInvalidWindow(t *testing.T) {
	start := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	err := validateAdminPlayCampaign(&PlayCampaign{
		Name:    "bad window",
		StartAt: start,
		EndAt:   start,
	})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "PLAY_CAMPAIGN_TIME_INVALID", infraerrors.Reason(err))
}

func TestValidateAdminPlayCampaignRejectsInvalidRules(t *testing.T) {
	start := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	err := validateAdminPlayCampaign(&PlayCampaign{
		Name:    "bad multiplier",
		StartAt: start,
		EndAt:   start.Add(time.Hour),
		Rules:   PlayCampaignRules{ArenaScoreMultiplier: 0.5},
	})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "PLAY_CAMPAIGN_ARENA_MULTIPLIER_INVALID", infraerrors.Reason(err))
}

func TestValidateAdminPlayCampaignRejectsContradictoryAudienceCombinations(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		audience PlayCampaignAudience
	}{
		{name: "ordinary and member", audience: PlayCampaignAudience{Ordinary: true, Member: true}},
		{name: "ordinary and VIP tier", audience: PlayCampaignAudience{Ordinary: true, VIPTiers: []int{2}}},
		{name: "all and new user window", audience: PlayCampaignAudience{All: true, RegisteredWithinDays: 7}},
		{name: "invalid VIP tier", audience: PlayCampaignAudience{VIPTiers: []int{0}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			campaign := &PlayCampaign{Name: "audience", StartAt: start, EndAt: start.Add(24 * time.Hour), Audience: tt.audience}
			err := validateAdminPlayCampaign(campaign)
			require.Error(t, err)
			require.True(t, infraerrors.IsBadRequest(err))
			require.Equal(t, "PLAY_CAMPAIGN_AUDIENCE_INVALID", infraerrors.Reason(err))
		})
	}
}
