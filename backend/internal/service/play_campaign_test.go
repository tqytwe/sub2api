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
	require.Equal(t, 3, rules.BlindboxExtraOpens)
	require.InDelta(t, 3.0, rules.ArenaScoreMultiplier, 1e-9)
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
