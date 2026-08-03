package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrPlayCampaignRewardBudgetExceeded = infraerrors.Conflict("PLAY_CAMPAIGN_REWARD_BUDGET_EXCEEDED", "campaign reward budget is exhausted")

type PlayCampaignRules struct {
	RechargeBonusPct     float64                  `json:"recharge_bonus_pct,omitempty"`
	BlindboxExtraOpens   int                      `json:"blindbox_extra_opens,omitempty"`
	ArenaScoreMultiplier float64                  `json:"arena_score_multiplier,omitempty"`
	NameI18n             map[string]string        `json:"name_i18n,omitempty"`
	CampaignType         string                   `json:"campaign_type,omitempty"`
	ReferralCampaignID   int64                    `json:"referral_campaign_id,omitempty"`
	QualificationMetric  string                   `json:"qualification_metric,omitempty"`
	RewardTiers          []PlayCampaignRewardTier `json:"reward_tiers,omitempty"`
	RequireInvite        bool                     `json:"require_invite,omitempty"`
	LegacyRebatePolicy   string                   `json:"legacy_rebate_policy,omitempty"`
}

const (
	PlayCampaignTypeBenefitOverlay  = "benefit_overlay"
	PlayCampaignTypeNewUserGrowth   = "new_user_growth"
	PlayCampaignTypeHybrid          = "hybrid"
	PlayCampaignMetricNetRecharge   = "net_recharge"
	PlayCampaignMetricConsumption   = "actual_consumption"
	PlayCampaignLegacyRebateExclude = "exclude"
	PlayCampaignLegacyRebateStack   = "stack"
)

type PlayCampaignRewardTier struct {
	Tier           int     `json:"tier"`
	RequiredAmount float64 `json:"required_amount"`
	RewardAmount   float64 `json:"reward_amount"`
	Currency       string  `json:"currency"`
}

func (r PlayCampaignRules) MaxReward() float64 {
	total := 0.0
	for _, tier := range r.RewardTiers {
		total += tier.RewardAmount
	}
	return total
}

const (
	playCampaignMaxRechargeBonusPct = 100.0
	playCampaignMaxBlindboxOpens    = 10
	playCampaignMaxArenaMultiplier  = 5.0
)

// PlayCampaignAudience is deliberately separate from rewards. Dimensions are
// ANDed; values within VIP tiers are ORed. An empty struct means all users.
type PlayCampaignAudience struct {
	All                  bool  `json:"all,omitempty"`
	Ordinary             bool  `json:"ordinary,omitempty"`
	Member               bool  `json:"member,omitempty"`
	VIPTiers             []int `json:"vip_tiers,omitempty"`
	RegisteredWithinDays int   `json:"registered_within_days,omitempty"`
}

type PlayCampaign struct {
	ID        int64
	Name      string
	StartAt   time.Time
	EndAt     time.Time
	Rules     PlayCampaignRules
	Audience  PlayCampaignAudience
	Enabled   bool
	CreatedAt time.Time
}

type PlayCampaignSummary struct {
	ID            int64                      `json:"id"`
	Name          string                     `json:"name"`
	StartAt       time.Time                  `json:"start_at"`
	EndAt         time.Time                  `json:"end_at"`
	Rules         PlayCampaignRules          `json:"rules"`
	NewUserGrowth *PlayNewUserGrowthProgress `json:"new_user_growth,omitempty"`
}

type PlayNewUserGrowthRewardProgress struct {
	RewardID       int64   `json:"reward_id,omitempty"`
	Tier           int     `json:"tier"`
	RequiredAmount float64 `json:"required_amount"`
	RewardAmount   float64 `json:"reward_amount"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status,omitempty"`
}

type PlayNewUserGrowthProgress struct {
	Eligible            bool                              `json:"eligible"`
	FundingConflict     bool                              `json:"funding_conflict,omitempty"`
	ReferralCampaignID  int64                             `json:"referral_campaign_id"`
	ReferralVersion     int64                             `json:"referral_version"`
	QualificationMetric string                            `json:"qualification_metric"`
	QualifiedAmount     float64                           `json:"qualified_amount"`
	Rewards             []PlayNewUserGrowthRewardProgress `json:"rewards"`
}

type PlayEffectModifiers struct {
	BlindboxExtraOpens       int
	ArenaScoreMultiplier     float64
	CampaignRechargeBonusPct float64
	CampaignActive           bool
}

func (s *PlayService) ListActiveCampaigns(ctx context.Context) ([]PlayCampaignSummary, error) {
	return s.ListActiveCampaignsForUser(ctx, 0)
}

func (s *PlayService) ListActiveCampaignsForUser(ctx context.Context, userID int64) ([]PlayCampaignSummary, error) {
	rt := s.GetRuntime(ctx)
	if !rt.CampaignsEnabled || s.repo == nil {
		return nil, nil
	}
	rows, err := s.activeCampaignsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]PlayCampaignSummary, 0, len(rows))
	for _, row := range rows {
		if userID > 0 && (row.Rules.CampaignType == PlayCampaignTypeNewUserGrowth || row.Rules.CampaignType == PlayCampaignTypeHybrid) {
			progressRepo, ok := s.repo.(PlayNewUserGrowthProgressRepository)
			if !ok {
				continue
			}
			if row.Rules.QualificationMetric == PlayCampaignMetricConsumption {
				if reconcileRepo, supported := s.repo.(PlayNewUserGrowthRepository); supported {
					if err := reconcileRepo.ReconcileNewUserGrowthCampaign(ctx, row, userID, s.serverNow()); err != nil {
						return nil, err
					}
				}
			}
			progress, progressErr := progressRepo.GetNewUserGrowthProgress(ctx, row, userID, s.serverNow())
			if progressErr != nil {
				return nil, progressErr
			}
			if !progress.Eligible {
				continue
			}
			summary := toPlayCampaignSummary(row)
			summary.NewUserGrowth = &progress
			out = append(out, summary)
			continue
		}
		out = append(out, toPlayCampaignSummary(row))
	}
	if userID > 0 {
		if progressRepo, ok := s.repo.(PlayNewUserGrowthProgressRepository); ok {
			seen := make(map[int64]struct{}, len(out))
			for _, item := range out {
				seen[item.ID] = struct{}{}
			}
			linked, linkedErr := s.newUserGrowthCampaignsForUser(ctx, userID, s.serverNow())
			if linkedErr != nil {
				return nil, linkedErr
			}
			for _, row := range linked {
				if _, exists := seen[row.ID]; exists {
					continue
				}
				if row.Rules.QualificationMetric == PlayCampaignMetricConsumption {
					if reconcileRepo, supported := s.repo.(PlayNewUserGrowthRepository); supported {
						if err := reconcileRepo.ReconcileNewUserGrowthCampaign(ctx, row, userID, s.serverNow()); err != nil {
							return nil, err
						}
					}
				}
				progress, progressErr := progressRepo.GetNewUserGrowthProgress(ctx, row, userID, s.serverNow())
				if progressErr != nil {
					return nil, progressErr
				}
				if !progress.Eligible {
					continue
				}
				summary := toPlayCampaignSummary(row)
				summary.NewUserGrowth = &progress
				out = append(out, summary)
			}
		}
	}
	return out, nil
}

// ReconcileNewUserGrowth refreshes milestone rewards after a financial state
// change. Consumption campaigns also reconcile when the user opens progress so
// a completed API charge can unlock its next tier without a separate payout job.
func (s *PlayService) ReconcileNewUserGrowth(ctx context.Context, userID int64, now time.Time) error {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil
	}
	repo, ok := s.repo.(PlayNewUserGrowthRepository)
	if !ok {
		return nil
	}
	campaigns, err := s.newUserGrowthCampaignsForUser(ctx, userID, now)
	if err != nil {
		return err
	}
	for _, campaign := range campaigns {
		if campaign.Rules.CampaignType != PlayCampaignTypeNewUserGrowth && campaign.Rules.CampaignType != PlayCampaignTypeHybrid {
			continue
		}
		if err := repo.ReconcileNewUserGrowthCampaign(ctx, campaign, userID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *PlayService) newUserGrowthCampaignsForUser(ctx context.Context, userID int64, now time.Time) ([]PlayCampaign, error) {
	if repo, ok := s.repo.(PlayNewUserGrowthCampaignRepository); ok {
		campaigns, err := repo.ListNewUserGrowthCampaignsForUser(ctx, userID, now)
		if err != nil {
			return nil, err
		}
		out := make([]PlayCampaign, 0, len(campaigns))
		for _, campaign := range campaigns {
			matched, matchErr := s.campaignAudienceMatches(ctx, userID, campaign.Audience)
			if matchErr != nil {
				return nil, matchErr
			}
			if matched {
				out = append(out, campaign)
			}
		}
		return out, nil
	}
	return s.activeCampaignsForUser(ctx, userID)
}

func (s *PlayService) validateNewUserGrowthLink(ctx context.Context, campaign PlayCampaign) error {
	if campaign.Rules.CampaignType != PlayCampaignTypeNewUserGrowth && campaign.Rules.CampaignType != PlayCampaignTypeHybrid {
		return nil
	}
	repo, ok := s.repo.(PlayNewUserGrowthLinkRepository)
	if !ok {
		return nil
	}
	policy, err := repo.GetReferralCampaignLegacyRebatePolicy(ctx, campaign.Rules.ReferralCampaignID)
	if err != nil {
		return err
	}
	if policy != campaign.Rules.LegacyRebatePolicy {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_REFERRAL_POLICY_MISMATCH", "new user growth rebate policy must match its referral campaign")
	}
	return nil
}

func (s *PlayService) ListAdminCampaigns(ctx context.Context) ([]PlayCampaign, error) {
	if s.repo == nil {
		return nil, nil
	}
	return s.repo.ListAdminCampaigns(ctx)
}

func (s *PlayService) ResolveRechargeCampaignBonus(ctx context.Context, userID int64) (float64, []int64, error) {
	if s == nil || s.repo == nil {
		return 0, nil, nil
	}
	rt := s.GetRuntime(ctx)
	if !rt.CampaignsEnabled {
		return 0, nil, nil
	}
	campaigns, err := s.activeCampaignsForUser(ctx, userID)
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
	if err := s.validateNewUserGrowthLink(ctx, campaign); err != nil {
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
	if err := s.validateNewUserGrowthLink(ctx, campaign); err != nil {
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
	campaigns, err := s.activeCampaignsForUser(ctx, userID)
	if err != nil {
		return out, err
	}
	if len(campaigns) == 0 {
		return out, nil
	}
	rules := aggregateCampaignRules(campaigns)
	out.CampaignActive = true
	if rules.BlindboxExtraOpens > out.BlindboxExtraOpens {
		out.BlindboxExtraOpens = rules.BlindboxExtraOpens
	}
	if rules.ArenaScoreMultiplier > 1 {
		if rules.ArenaScoreMultiplier > out.ArenaScoreMultiplier {
			out.ArenaScoreMultiplier = rules.ArenaScoreMultiplier
		}
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
		if c.Rules.BlindboxExtraOpens > out.BlindboxExtraOpens {
			out.BlindboxExtraOpens = c.Rules.BlindboxExtraOpens
		}
		if c.Rules.ArenaScoreMultiplier > out.ArenaScoreMultiplier {
			out.ArenaScoreMultiplier = c.Rules.ArenaScoreMultiplier
		}
	}
	if out.RechargeBonusPct > playCampaignMaxRechargeBonusPct {
		out.RechargeBonusPct = playCampaignMaxRechargeBonusPct
	}
	if out.BlindboxExtraOpens > playCampaignMaxBlindboxOpens {
		out.BlindboxExtraOpens = playCampaignMaxBlindboxOpens
	}
	if out.ArenaScoreMultiplier > playCampaignMaxArenaMultiplier {
		out.ArenaScoreMultiplier = playCampaignMaxArenaMultiplier
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

func (s *PlayService) activeCampaignsForUser(ctx context.Context, userID int64) ([]PlayCampaign, error) {
	if cache := playRequestCacheFromContext(ctx); cache != nil {
		return cache.getCampaigns(ctx, userID, s.activeCampaignsForUserUncached)
	}
	return s.activeCampaignsForUserUncached(ctx, userID)
}

func (s *PlayService) activeCampaignsForUserUncached(ctx context.Context, userID int64) ([]PlayCampaign, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	rows, err := s.repo.ListActiveCampaigns(ctx, s.serverNow())
	if err != nil || userID <= 0 {
		return rows, err
	}
	out := make([]PlayCampaign, 0, len(rows))
	for _, row := range rows {
		matched, matchErr := s.campaignAudienceMatches(ctx, userID, row.Audience)
		if matchErr != nil {
			return nil, matchErr
		}
		if matched {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *PlayService) campaignAudienceMatches(ctx context.Context, userID int64, audience PlayCampaignAudience) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	if !audience.Ordinary && !audience.Member && len(audience.VIPTiers) == 0 && audience.RegisteredWithinDays <= 0 {
		return true, nil
	}
	user, err := s.playRequestUser(ctx, userID)
	if err != nil {
		return false, err
	}
	paid, err := s.MembershipPaidTotal(ctx, userID)
	if err != nil {
		return false, err
	}
	vip := resolveVIPStatus(paid, s.GetRuntime(ctx).VIPTiers)
	memberMin := firstMemberThreshold(s.GetRuntime(ctx).VIPTiers)
	isMember := paid+1e-9 >= memberMin
	if audience.Ordinary && isMember {
		return false, nil
	}
	if audience.Member && !isMember {
		return false, nil
	}
	if len(audience.VIPTiers) > 0 {
		found := false
		for _, tier := range audience.VIPTiers {
			if tier == vip.Tier {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	if audience.RegisteredWithinDays > 0 && user.CreatedAt.Before(s.serverNow().AddDate(0, 0, -audience.RegisteredWithinDays)) {
		return false, nil
	}
	return true, nil
}

func (s *PlayService) playRequestUser(ctx context.Context, userID int64) (*User, error) {
	if s == nil || s.userRepo == nil {
		return nil, ErrUserNotFound
	}
	if cache := playRequestCacheFromContext(ctx); cache != nil {
		return cache.getUser(ctx, userID, s.userRepo.GetByID)
	}
	return s.userRepo.GetByID(ctx, userID)
}

func firstMemberThreshold(tiers []PlayVIPTier) float64 {
	if len(tiers) == 0 {
		tiers = defaultPlayVIPTiers()
	}
	threshold := -1.0
	for _, tier := range tiers {
		if tier.Tier > 0 && (threshold < 0 || tier.MinRecharge < threshold) {
			threshold = tier.MinRecharge
		}
	}
	if threshold < 0 {
		return 0
	}
	return threshold
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

func ParsePlayCampaignAudience(raw string) PlayCampaignAudience {
	var audience PlayCampaignAudience
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &audience) != nil {
		return PlayCampaignAudience{}
	}
	if audience.RegisteredWithinDays < 0 {
		audience.RegisteredWithinDays = 0
	}
	seen := map[int]bool{}
	tiers := make([]int, 0, len(audience.VIPTiers))
	for _, tier := range audience.VIPTiers {
		if tier >= 0 && !seen[tier] {
			seen[tier] = true
			tiers = append(tiers, tier)
		}
	}
	audience.VIPTiers = tiers
	return audience
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

	if c.Rules.RechargeBonusPct < 0 || c.Rules.RechargeBonusPct > playCampaignMaxRechargeBonusPct {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_RECHARGE_BONUS_INVALID", "recharge bonus must be between 0 and 100")
	}
	if c.Rules.BlindboxExtraOpens < 0 || c.Rules.BlindboxExtraOpens > playCampaignMaxBlindboxOpens {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_BLINDBOX_EXTRA_INVALID", "blindbox extra opens must be between 0 and 10")
	}
	if c.Rules.ArenaScoreMultiplier < 0 || c.Rules.ArenaScoreMultiplier > playCampaignMaxArenaMultiplier {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_ARENA_MULTIPLIER_INVALID", "arena score multiplier must be between 0 and 5")
	}
	if c.Rules.CampaignType == "" {
		c.Rules.CampaignType = PlayCampaignTypeBenefitOverlay
	}
	if c.Rules.CampaignType != PlayCampaignTypeBenefitOverlay && c.Rules.CampaignType != PlayCampaignTypeNewUserGrowth && c.Rules.CampaignType != PlayCampaignTypeHybrid {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_TYPE_INVALID", "campaign type is invalid")
	}
	if c.Rules.CampaignType == PlayCampaignTypeNewUserGrowth || c.Rules.CampaignType == PlayCampaignTypeHybrid {
		if c.Rules.ReferralCampaignID <= 0 {
			return infraerrors.BadRequest("PLAY_CAMPAIGN_REFERRAL_REQUIRED", "new user growth campaign requires a referral campaign")
		}
		if c.Rules.QualificationMetric != PlayCampaignMetricNetRecharge && c.Rules.QualificationMetric != PlayCampaignMetricConsumption {
			return infraerrors.BadRequest("PLAY_CAMPAIGN_METRIC_INVALID", "new user growth campaign metric is invalid")
		}
		if c.Rules.LegacyRebatePolicy == "" {
			c.Rules.LegacyRebatePolicy = PlayCampaignLegacyRebateExclude
		}
		if c.Rules.LegacyRebatePolicy != PlayCampaignLegacyRebateExclude && c.Rules.LegacyRebatePolicy != PlayCampaignLegacyRebateStack {
			return infraerrors.BadRequest("PLAY_CAMPAIGN_REBATE_POLICY_INVALID", "legacy rebate policy is invalid")
		}
		c.Rules.RequireInvite = true
		if len(c.Rules.RewardTiers) == 0 {
			return infraerrors.BadRequest("PLAY_CAMPAIGN_REWARD_TIERS_REQUIRED", "new user growth campaign requires reward tiers")
		}
		seenTier := map[int]bool{}
		previousThreshold := 0.0
		for _, tier := range c.Rules.RewardTiers {
			if tier.Tier <= 0 || seenTier[tier.Tier] || tier.RequiredAmount <= previousThreshold || tier.RewardAmount <= 0 || strings.ToUpper(strings.TrimSpace(tier.Currency)) != "CNY" {
				return infraerrors.BadRequest("PLAY_CAMPAIGN_REWARD_TIER_INVALID", "new user growth reward tiers are invalid")
			}
			seenTier[tier.Tier] = true
			previousThreshold = tier.RequiredAmount
		}
	}
	if c.Rules.ArenaScoreMultiplier > 0 && c.Rules.ArenaScoreMultiplier < 1 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_ARENA_MULTIPLIER_INVALID", "arena score multiplier must be 0 or at least 1")
	}
	if c.Audience.RegisteredWithinDays < 0 || c.Audience.RegisteredWithinDays > 3650 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_AUDIENCE_INVALID", "registered days must be between 0 and 3650")
	}
	if c.Audience.Ordinary && c.Audience.Member {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_AUDIENCE_INVALID", "ordinary and member segments cannot be selected together")
	}
	if c.Audience.Ordinary && len(c.Audience.VIPTiers) > 0 {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_AUDIENCE_INVALID", "ordinary users cannot be combined with VIP tiers")
	}
	if c.Audience.All && (c.Audience.Ordinary || c.Audience.Member || len(c.Audience.VIPTiers) > 0 || c.Audience.RegisteredWithinDays > 0) {
		return infraerrors.BadRequest("PLAY_CAMPAIGN_AUDIENCE_INVALID", "all users cannot be combined with other audience conditions")
	}
	for _, tier := range c.Audience.VIPTiers {
		if tier <= 0 {
			return infraerrors.BadRequest("PLAY_CAMPAIGN_AUDIENCE_INVALID", "VIP tier values must be greater than zero")
		}
	}
	c.Audience = ParsePlayCampaignAudience(mustMarshalCampaignAudience(c.Audience))

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

func mustMarshalCampaignAudience(audience PlayCampaignAudience) string {
	raw, _ := json.Marshal(audience)
	return string(raw)
}
