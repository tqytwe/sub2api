package service

import "context"

// PublicGrowthTeaser exposes marketing-safe signup/perks signals for public landing pages.
type PublicGrowthTeaser struct {
	RegistrationEnabled bool     `json:"registration_enabled"`
	SignupBalanceUSD    float64  `json:"signup_balance_usd"`
	SignupGrantEnabled  bool     `json:"signup_grant_enabled"`
	PaymentEnabled      bool     `json:"payment_enabled"`
	CheckinEnabled      bool     `json:"checkin_enabled"`
	CheckinDailyReward  float64  `json:"checkin_daily_reward,omitempty"`
	AffiliateEnabled    bool     `json:"affiliate_enabled"`
	AffiliateRebatePct  float64  `json:"affiliate_rebate_pct,omitempty"`
	PublicModelsEnabled bool     `json:"public_models_enabled"`
	PublicModelCount    int      `json:"public_model_count"`
	PlayAnyEnabled      bool     `json:"play_any_enabled"`
	PlayFeatures        []string `json:"play_features,omitempty"`
	VIPTiersEnabled     bool     `json:"vip_tiers_enabled"`
	TotalRequests       int64    `json:"total_requests,omitempty"`
	HasLiveStats        bool     `json:"has_live_stats"`
}

// BuildPublicGrowthTeaser aggregates public settings and optional live counters for marketing UI.
func (s *SettingService) BuildPublicGrowthTeaser(ctx context.Context, publicModelCount int, totalRequests int64, hasLiveStats bool) (*PublicGrowthTeaser, error) {
	public, err := s.GetPublicSettings(ctx)
	if err != nil {
		return nil, err
	}

	play := s.GetPlayRuntime(ctx)
	signupBalance := s.GetDefaultBalance(ctx)
	signupGrantEnabled := signupBalance > 0

	if resolved, enabled, err := s.ResolveAuthSourceGrantSettings(ctx, "email", false); err == nil && enabled {
		signupBalance = resolved.Balance
		signupGrantEnabled = signupBalance > 0
	}

	affiliatePct := 0.0
	if public.AffiliateEnabled {
		affiliatePct = s.GetAffiliateRebateRatePercent(ctx)
	}

	features := make([]string, 0, 5)
	if play.CheckinEnabled {
		features = append(features, "checkin")
	}
	if play.ArenaEnabled {
		features = append(features, "arena")
	}
	if play.BlindboxEnabled {
		features = append(features, "blindbox")
	}
	if play.QuizEnabled {
		features = append(features, "quiz")
	}
	if play.AgentTeamEnabled {
		features = append(features, "agent_team")
	}

	playAny := len(features) > 0
	vipEnabled := len(play.VIPTiers) > 0

	out := &PublicGrowthTeaser{
		RegistrationEnabled: public.RegistrationEnabled,
		SignupBalanceUSD:    signupBalance,
		SignupGrantEnabled:  signupGrantEnabled,
		PaymentEnabled:      public.PaymentEnabled,
		CheckinEnabled:      play.CheckinEnabled,
		AffiliateEnabled:    public.AffiliateEnabled,
		PublicModelsEnabled: play.PublicModelsEnabled,
		PublicModelCount:    publicModelCount,
		PlayAnyEnabled:      playAny,
		PlayFeatures:        features,
		VIPTiersEnabled:     vipEnabled,
		TotalRequests:       totalRequests,
		HasLiveStats:        hasLiveStats,
	}

	if play.CheckinEnabled && play.CheckinReward > 0 {
		out.CheckinDailyReward = play.CheckinReward
	}
	if public.AffiliateEnabled && affiliatePct > 0 {
		out.AffiliateRebatePct = affiliatePct
	}

	return out, nil
}
