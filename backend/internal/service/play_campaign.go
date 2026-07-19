package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
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

func (s *PlayService) ListAdminCampaigns(ctx context.Context) ([]PlayCampaign, error) {
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.ListAdminCampaigns(ctx)
}

func (s *PlayService) ResolveRechargeCampaignBonus(ctx context.Context) (float64, []int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil, nil
	}
	rt := s.GetRuntime(ctx)
	if !rt.CampaignsEnabled {
		return 0, nil, nil
	}
	campaigns, err := s.repo.ListActiveCampaigns(ctx, s.serverNow())
	if err != nil {
		return 0, nil, err
	}
	var bonus float64
	var ids []int64
	for _, campaign := range campaigns {
		if campaign.Rules.RechargeBonusPct <= 0 {
			continue
		}
		if campaign.Rules.RechargeBonusPct > bonus {
			bonus = campaign.Rules.RechargeBonusPct
			ids = []int64{campaign.ID}
			continue
		}
		if campaign.Rules.RechargeBonusPct == bonus {
			ids = append(ids, campaign.ID)
		}
	}
	return bonus, ids, nil
}

func (s *PlayService) CreateAdminCampaign(ctx context.Context, campaign PlayCampaign) (*PlayCampaign, error) {
	if err := validateAdminPlayCampaign(&campaign); err != nil {
		return nil, err
	}
	return s.repo.CreateAdminCampaign(ctx, campaign)
}

func (s *PlayService) UpdateAdminCampaign(ctx context.Context, campaign PlayCampaign) (*PlayCampaign, error) {
	if campaign.ID <= 0 {
		return nil, infraerrors.BadRequest("PLAY_CAMPAIGN_INVALID_ID", "campaign id is invalid")
	}
	if err := validateAdminPlayCampaign(&campaign); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateAdminCampaign(ctx, campaign)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, infraerrors.NotFound("PLAY_CAMPAIGN_NOT_FOUND", "campaign not found")
	}
	return updated, nil
}

func (s *PlayService) DeleteAdminCampaign(ctx context.Context, id int64) error {
	if id <= 0 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_INVALID_ID", "campaign id is invalid")
	}
	if err := s.repo.DeleteAdminCampaign(ctx, id); err != nil {
		return err
	}
	return nil
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

func validateAdminPlayCampaign(c *PlayCampaign) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_NAME_REQUIRED", "campaign name is required")
	}
	if len([]rune(c.Name)) > 128 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_NAME_TOO_LONG", "campaign name must be at most 128 characters")
	}
	if c.StartAt.IsZero() || c.EndAt.IsZero() {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_TIME_REQUIRED", "campaign start and end time are required")
	}
	if !c.EndAt.After(c.StartAt) {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_TIME_INVALID", "campaign end time must be after start time")
	}

	if c.Rules.RechargeBonusPct < 0 || c.Rules.RechargeBonusPct > 1000 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_RECHARGE_BONUS_INVALID", "recharge bonus must be between 0 and 1000")
	}
	if c.Rules.BlindboxExtraOpens < 0 || c.Rules.BlindboxExtraOpens > 100 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_BLINDBOX_EXTRA_INVALID", "blindbox extra opens must be between 0 and 100")
	}
	if c.Rules.ArenaScoreMultiplier < 0 || c.Rules.ArenaScoreMultiplier > 100 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_ARENA_MULTIPLIER_INVALID", "arena score multiplier must be between 0 and 100")
	}
	if c.Rules.ArenaScoreMultiplier > 0 && c.Rules.ArenaScoreMultiplier < 1 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_ARENA_MULTIPLIER_INVALID", "arena score multiplier must be 0 or at least 1")
	}

	if len(c.Rules.NameI18n) > 0 {
		clean := make(map[string]string, len(c.Rules.NameI18n))
		for key, value := range c.Rules.NameI18n {
			locale := strings.TrimSpace(strings.ToLower(key))
			name := strings.TrimSpace(value)
			if locale == "" || name == "" {
				continue
			}
			if locale != "zh" && locale != "en" {
				return infraerrors.BadRequest("PLAY_CAMPAIGN_NAME_I18N_INVALID", "campaign localized names only support zh and en")
			}
			if len([]rune(name)) > 128 {
				return infraerrors.BadRequest("PLAY_CAMPAIGN_NAME_I18N_TOO_LONG", "campaign localized names must be at most 128 characters")
			}
			clean[locale] = name
		}
		if len(clean) == 0 {
			c.Rules.NameI18n = nil
		} else {
			c.Rules.NameI18n = clean
		}
	}
	return nil
}
