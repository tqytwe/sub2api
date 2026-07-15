package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVIPTier(t *testing.T) {
	tiers := defaultPlayVIPTiers()

	tests := []struct {
		name     string
		recharge float64
		wantTier int
		wantNext int
		wantAmt  float64
		wantPerk string
	}{
		{name: "v0 baseline", recharge: 0, wantTier: 0, wantNext: 1, wantAmt: 50},
		{name: "v1 threshold", recharge: 50, wantTier: 1, wantNext: 2, wantAmt: 150, wantPerk: "models_vip_tag"},
		{name: "v2 threshold", recharge: 200, wantTier: 2, wantNext: 3, wantAmt: 300, wantPerk: "models_vip_tag"},
		{name: "v3 max", recharge: 500, wantTier: 3, wantNext: 0, wantAmt: 0, wantPerk: "affiliate_bonus_5pct"},
		{name: "between v1 and v2", recharge: 120, wantTier: 1, wantNext: 2, wantAmt: 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetVIPTier(tt.recharge, tiers)
			require.Equal(t, tt.wantTier, got.Tier)
			require.Equal(t, tt.wantNext, got.NextTier)
			if tt.wantNext == 0 {
				require.Equal(t, 0.0, got.AmountToNext)
			} else {
				require.InDelta(t, tt.wantAmt, got.AmountToNext, 1e-9)
			}
			if tt.wantPerk != "" {
				require.Contains(t, got.Perks, tt.wantPerk)
			}
		})
	}
}

func TestGetVIPTierEmptyUsesDefaults(t *testing.T) {
	got := GetVIPTier(50, nil)
	require.Equal(t, 1, got.Tier)
	require.Equal(t, "V1", got.Label)
}

func TestParsePlayVIPTiersRemovesUnsupportedBlindboxUpgrade(t *testing.T) {
	tiers := parsePlayVIPTiers(`[{"tier":2,"label":"V2","min_recharge":200,"perks":["models_vip_tag","blindbox_pool_upgrade"]}]`)
	require.Len(t, tiers, 1)
	require.Equal(t, []string{"models_vip_tag"}, tiers[0].Perks)
}
