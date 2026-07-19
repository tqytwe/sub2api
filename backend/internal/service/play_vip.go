package service

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

const maxVIPRechargeBonusPct = 10

// PlayVIPTier defines a cumulative recharge tier and its perk keys.
type PlayVIPTier struct {
	Tier             int      `json:"tier"`
	Label            string   `json:"label"`
	MinRecharge      float64  `json:"min_recharge"`
	RechargeBonusPct float64  `json:"recharge_bonus_pct"`
	ColorKey         string   `json:"color_key"`
	Perks            []string `json:"perks,omitempty"`
}

// PlayVIPStatus is the resolved VIP state for a user.
type PlayVIPStatus struct {
	Tier             int      `json:"tier"`
	Label            string   `json:"label"`
	RechargeBonusPct float64  `json:"recharge_bonus_pct"`
	ColorKey         string   `json:"color_key"`
	Perks            []string `json:"perks,omitempty"`
	NextTier         int      `json:"next_tier,omitempty"`
	NextLabel        string   `json:"next_label,omitempty"`
	NextMinRecharge  float64  `json:"next_min_recharge,omitempty"`
	AmountToNext     float64  `json:"amount_to_next,omitempty"`
}

// GetVIPTier resolves the highest tier reached by total lifetime recharge.
func GetVIPTier(totalRecharged float64, tiers []PlayVIPTier) PlayVIPStatus {
	if len(tiers) == 0 {
		tiers = defaultPlayVIPTiers()
	}
	sorted := append([]PlayVIPTier(nil), tiers...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].MinRecharge == sorted[j].MinRecharge {
			return sorted[i].Tier < sorted[j].Tier
		}
		return sorted[i].MinRecharge < sorted[j].MinRecharge
	})

	current := sorted[0]
	for _, tier := range sorted {
		if totalRecharged+1e-9 >= tier.MinRecharge {
			current = tier
			continue
		}
		amountToNext := tier.MinRecharge - totalRecharged
		if amountToNext < 0 {
			amountToNext = 0
		}
		return PlayVIPStatus{
			Tier:             current.Tier,
			Label:            current.Label,
			RechargeBonusPct: current.RechargeBonusPct,
			ColorKey:         current.ColorKey,
			Perks:            append([]string(nil), current.Perks...),
			NextTier:         tier.Tier,
			NextLabel:        tier.Label,
			NextMinRecharge:  tier.MinRecharge,
			AmountToNext:     amountToNext,
		}
	}

	return PlayVIPStatus{
		Tier:             current.Tier,
		Label:            current.Label,
		RechargeBonusPct: current.RechargeBonusPct,
		ColorKey:         current.ColorKey,
		Perks:            append([]string(nil), current.Perks...),
	}
}

func resolveVIPStatus(totalRecharged float64, tiers []PlayVIPTier) PlayVIPStatus {
	return GetVIPTier(totalRecharged, tiers)
}

func defaultPlayVIPTiers() []PlayVIPTier {
	return []PlayVIPTier{
		{Tier: 0, Label: "V0", MinRecharge: 0, RechargeBonusPct: 0, ColorKey: "neutral"},
		{Tier: 1, Label: "V1", MinRecharge: 50, RechargeBonusPct: 2, ColorKey: "emerald", Perks: []string{"models_vip_tag"}},
		{Tier: 2, Label: "V2", MinRecharge: 100, RechargeBonusPct: 4, ColorKey: "sky", Perks: []string{"models_vip_tag", "blindbox_pool_upgrade"}},
		{Tier: 3, Label: "V3", MinRecharge: 200, RechargeBonusPct: 6, ColorKey: "indigo", Perks: []string{"models_vip_tag", "blindbox_pool_upgrade", "arena_settlement_bonus"}},
		{Tier: 4, Label: "V4", MinRecharge: 500, RechargeBonusPct: 8, ColorKey: "amber", Perks: []string{"models_vip_tag", "blindbox_pool_upgrade", "arena_settlement_bonus", "affiliate_bonus_5pct"}},
		{Tier: 5, Label: "V5", MinRecharge: 1000, RechargeBonusPct: 10, ColorKey: "gold", Perks: []string{"models_vip_tag", "blindbox_pool_upgrade", "arena_settlement_bonus", "affiliate_bonus_5pct"}},
	}
}

func parsePlayVIPTiers(raw string) []PlayVIPTier {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultPlayVIPTiers()
	}
	var items []PlayVIPTier
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		return defaultPlayVIPTiers()
	}
	items = normalizePlayVIPTiers(items)
	sort.Slice(items, func(i, j int) bool {
		if items[i].MinRecharge == items[j].MinRecharge {
			return items[i].Tier < items[j].Tier
		}
		return items[i].MinRecharge < items[j].MinRecharge
	})
	return items
}

func normalizePlayVIPTiers(items []PlayVIPTier) []PlayVIPTier {
	defaults := make(map[int]PlayVIPTier, len(defaultPlayVIPTiers()))
	for _, tier := range defaultPlayVIPTiers() {
		defaults[tier.Tier] = tier
	}
	out := make([]PlayVIPTier, 0, len(items))
	for _, item := range items {
		if item.Tier < 0 {
			continue
		}
		if item.Label = strings.TrimSpace(item.Label); item.Label == "" {
			item.Label = "V" + strconv.Itoa(item.Tier)
		}
		if item.MinRecharge < 0 {
			item.MinRecharge = 0
		}
		if item.RechargeBonusPct < 0 {
			item.RechargeBonusPct = 0
		}
		if item.RechargeBonusPct > maxVIPRechargeBonusPct {
			item.RechargeBonusPct = maxVIPRechargeBonusPct
		}
		item.ColorKey = normalizeVIPColorKey(item.ColorKey, item.Tier)
		if len(item.Perks) == 0 {
			if def, ok := defaults[item.Tier]; ok && len(def.Perks) > 0 {
				item.Perks = append([]string(nil), def.Perks...)
			}
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return defaultPlayVIPTiers()
	}
	return out
}

func defaultVIPTierByNumber(tier int) (PlayVIPTier, bool) {
	for _, def := range defaultPlayVIPTiers() {
		if def.Tier == tier {
			return def, true
		}
	}
	return PlayVIPTier{}, false
}

func normalizeVIPColorKey(colorKey string, tier int) string {
	switch strings.TrimSpace(strings.ToLower(colorKey)) {
	case "neutral", "emerald", "sky", "indigo", "amber", "gold":
		return strings.TrimSpace(strings.ToLower(colorKey))
	case "rose":
		return "gold"
	default:
		if def, ok := defaultVIPTierByNumber(tier); ok {
			return def.ColorKey
		}
		return "neutral"
	}
}
