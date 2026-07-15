package service

import (
	"encoding/json"
	"sort"
	"strings"
)

// PlayVIPTier defines a cumulative recharge tier and its perk keys.
type PlayVIPTier struct {
	Tier        int      `json:"tier"`
	Label       string   `json:"label"`
	MinRecharge float64  `json:"min_recharge"`
	Perks       []string `json:"perks,omitempty"`
}

// PlayVIPStatus is the resolved VIP state for a user.
type PlayVIPStatus struct {
	Tier            int      `json:"tier"`
	Label           string   `json:"label"`
	Perks           []string `json:"perks,omitempty"`
	NextTier        int      `json:"next_tier,omitempty"`
	NextLabel       string   `json:"next_label,omitempty"`
	NextMinRecharge float64  `json:"next_min_recharge,omitempty"`
	AmountToNext    float64  `json:"amount_to_next,omitempty"`
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
			Tier:            current.Tier,
			Label:           current.Label,
			Perks:           append([]string(nil), current.Perks...),
			NextTier:        tier.Tier,
			NextLabel:       tier.Label,
			NextMinRecharge: tier.MinRecharge,
			AmountToNext:    amountToNext,
		}
	}

	return PlayVIPStatus{
		Tier:  current.Tier,
		Label: current.Label,
		Perks: append([]string(nil), current.Perks...),
	}
}

func resolveVIPStatus(totalRecharged float64, tiers []PlayVIPTier) PlayVIPStatus {
	return GetVIPTier(totalRecharged, tiers)
}

func defaultPlayVIPTiers() []PlayVIPTier {
	return []PlayVIPTier{
		{Tier: 0, Label: "V0", MinRecharge: 0},
		{Tier: 1, Label: "V1", MinRecharge: 50, Perks: []string{"models_vip_tag"}},
		{Tier: 2, Label: "V2", MinRecharge: 200, Perks: []string{"models_vip_tag", "blindbox_pool_upgrade"}},
		{Tier: 3, Label: "V3", MinRecharge: 500, Perks: []string{"models_vip_tag", "blindbox_pool_upgrade", "arena_settlement_bonus", "affiliate_bonus_5pct"}},
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
	sort.Slice(items, func(i, j int) bool {
		if items[i].MinRecharge == items[j].MinRecharge {
			return items[i].Tier < items[j].Tier
		}
		return items[i].MinRecharge < items[j].MinRecharge
	})
	return items
}
