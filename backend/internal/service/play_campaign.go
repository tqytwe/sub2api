package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type PlayCampaignRules struct {
	RechargeBonusPct     float64           `json:"recharge_bonus_pct,omitempty"`
	BlindboxExtraOpens   int               `json:"blindbox_extra_opens,omitempty"`
	ArenaScoreMultiplier float64           `json:"arena_score_multiplier,omitempty"`
	NameI18n             map[string]string `json:"name_i18n,omitempty"`
}

type PlayCampaign struct {
	ID        int64
	Name      string
	StartAt   time.Time
	EndAt     time.Time
	Rules     PlayCampaignRules
	Enabled   bool
	CreatedAt time.Time
}

type PlayCampaignSummary struct {
	ID      int64             `json:"id"`
	Name    string            `json:"name"`
	StartAt time.Time         `json:"start_at"`
	EndAt   time.Time         `json:"end_at"`
	Rules   PlayCampaignRules `json:"rules"`
}

type PlayEffectModifiers struct {
	BlindboxExtraOpens       int
	ArenaScoreMultiplier     float64
	CampaignRechargeBonusPct float64
	CampaignActive           bool
}

func (s *PlayService) ListActiveCampaigns(ctx context.Context) ([]PlayCampaignSummary, error) {
	rt := s.GetRuntime(ctx)
	if !rt.CampaignsEnabled || s.repo == nil {
		return nil, nil
	}
	rows, err := s.repo.ListActiveCampaigns(ctx, s.serverNow())
	if err != nil {
		return nil, err
	}
	out := make([]PlayCampaignSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, toPlayCampaignSummary(row))
	}
	return out, nil
}

func (s *PlayService) resolvePlayEffectModifiers(ctx context.Context, userID int64, rt PlayRuntime) (PlayEffectModifiers, error) {
	out := PlayEffectModifiers{ArenaScoreMultiplier: 1}

	boost, err := s.getRechargeBoostStatus(ctx, userID, rt)
	if err != nil {
		return out, err
	}
	if boost.Active {
		out.BlindboxExtraOpens += boost.BlindboxExtraOpens
		if boost.ArenaMultiplier > 1 {
			out.ArenaScoreMultiplier = boost.ArenaMultiplier
		}
	}

	if !rt.CampaignsEnabled || s.repo == nil {
		return out, nil
	}
	campaigns, err := s.repo.ListActiveCampaigns(ctx, s.serverNow())
	if err != nil {
		return out, err
	}
	if len(campaigns) == 0 {
		return out, nil
	}
	rules := aggregateCampaignRules(campaigns)
	out.CampaignActive = true
	out.BlindboxExtraOpens += rules.BlindboxExtraOpens
	if rules.ArenaScoreMultiplier > 1 {
		out.ArenaScoreMultiplier *= rules.ArenaScoreMultiplier
	}
	out.CampaignRechargeBonusPct = rules.RechargeBonusPct
	return out, nil
}

func aggregateCampaignRules(campaigns []PlayCampaign) PlayCampaignRules {
	var out PlayCampaignRules
	for _, c := range campaigns {
		if c.Rules.RechargeBonusPct > out.RechargeBonusPct {
			out.RechargeBonusPct = c.Rules.RechargeBonusPct
		}
		if c.Rules.BlindboxExtraOpens > 0 {
			out.BlindboxExtraOpens += c.Rules.BlindboxExtraOpens
		}
		if c.Rules.ArenaScoreMultiplier > 1 {
			if out.ArenaScoreMultiplier <= 1 {
				out.ArenaScoreMultiplier = c.Rules.ArenaScoreMultiplier
			} else {
				out.ArenaScoreMultiplier *= c.Rules.ArenaScoreMultiplier
			}
		}
	}
	return out
}

func toPlayCampaignSummary(c PlayCampaign) PlayCampaignSummary {
	return PlayCampaignSummary{
		ID:      c.ID,
		Name:    c.Name,
		StartAt: c.StartAt,
		EndAt:   c.EndAt,
		Rules:   c.Rules,
	}
}

func ParsePlayCampaignRules(raw string) PlayCampaignRules {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return PlayCampaignRules{}
	}
	var rules PlayCampaignRules
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return PlayCampaignRules{}
	}
	if rules.ArenaScoreMultiplier > 0 && rules.ArenaScoreMultiplier < 1 {
		rules.ArenaScoreMultiplier = 1
	}
	return rules
}
