package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVIPTier(t *testing.T) {
	tiers := defaultPlayVIPTiers()

	tests := []struct {
		name      string
		recharge  float64
		wantTier  int
		wantNext  int
		wantAmt   float64
		wantBonus float64
		wantColor string
		wantPerk  string
	}{
		{name: "v0 baseline", recharge: 0, wantTier: 0, wantNext: 1, wantAmt: 50, wantBonus: 0, wantColor: "neutral"},
		{name: "v1 threshold", recharge: 50, wantTier: 1, wantNext: 2, wantAmt: 450, wantBonus: 1, wantColor: "emerald", wantPerk: "models_vip_tag"},
		{name: "v2 threshold", recharge: 500, wantTier: 2, wantNext: 3, wantAmt: 500, wantBonus: 2, wantColor: "sky", wantPerk: "blindbox_pool_upgrade"},
		{name: "v3 threshold", recharge: 1000, wantTier: 3, wantNext: 4, wantAmt: 1000, wantBonus: 3, wantColor: "indigo", wantPerk: "arena_settlement_bonus"},
		{name: "v5 threshold", recharge: 5000, wantTier: 5, wantNext: 6, wantAmt: 5000, wantBonus: 5, wantColor: "gold", wantPerk: "affiliate_bonus_5pct"},
		{name: "v6 max", recharge: 10000, wantTier: 6, wantNext: 0, wantAmt: 0, wantBonus: 6, wantColor: "neutral", wantPerk: "affiliate_bonus_5pct"},
		{name: "between v1 and v2", recharge: 80, wantTier: 1, wantNext: 2, wantAmt: 420, wantBonus: 1, wantColor: "emerald"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetVIPTier(tt.recharge, tiers)
			require.Equal(t, tt.wantTier, got.Tier)
			require.Equal(t, tt.wantNext, got.NextTier)
			require.Equal(t, tt.wantBonus, got.RechargeBonusPct)
			require.Equal(t, tt.wantColor, got.ColorKey)
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
	require.Equal(t, 1.0, got.RechargeBonusPct)
	require.Equal(t, "emerald", got.ColorKey)
}

func TestParsePlayVIPTiersNormalizesBonusAndColor(t *testing.T) {
	got := parsePlayVIPTiers(`[
		{"tier":5,"label":"V5","min_recharge":1000,"recharge_bonus_pct":99,"color_key":"rose"},
		{"tier":1,"min_recharge":50,"recharge_bonus_pct":-3,"color_key":"unknown"}
	]`)

	require.Len(t, got, 2)
	require.Equal(t, 1, got[0].Tier)
	require.Equal(t, "V1", got[0].Label)
	require.Equal(t, 0.0, got[0].RechargeBonusPct)
	require.Equal(t, "emerald", got[0].ColorKey)
	require.Contains(t, got[0].Perks, "models_vip_tag")
	require.Equal(t, 5, got[1].Tier)
	require.Equal(t, 10.0, got[1].RechargeBonusPct)
	require.Equal(t, "gold", got[1].ColorKey)
}

func TestValidateAdminVIPTiers(t *testing.T) {
	valid := []PlayVIPTier{
		{Tier: 0, Label: "V0", MinRecharge: 0, RechargeBonusPct: 0},
		{Tier: 1, Label: "V1", MinRecharge: 50, RechargeBonusPct: 1, Perks: []string{"models_vip_tag"}},
		{Tier: 2, Label: "V2", MinRecharge: 500, RechargeBonusPct: 2, Perks: []string{"models_vip_tag", "blindbox_pool_upgrade"}},
		{Tier: 9, Label: "V9", MinRecharge: 5400, RechargeBonusPct: 10, Perks: []string{"models_vip_tag"}},
	}
	_, err := validateAdminVIPTiers(valid)
	require.NoError(t, err)

	_, err = validateAdminVIPTiers([]PlayVIPTier{
		{Tier: 0, Label: "V0", MinRecharge: 0},
		{Tier: 1, Label: "V1", MinRecharge: 50},
		{Tier: 2, Label: "V2", MinRecharge: 50},
	})
	var validationErr *vipConfigValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Equal(t, "PLAY_VIP_CONFIG_THRESHOLD_INVALID", validationErr.reason)
}
