package service

import (
	"context"
)

// PlayHubGrowth surfaces balance/recharge conversion signals for the dashboard and hub.
type PlayHubGrowth struct {
	Balance                  float64        `json:"balance"`
	TotalRecharged           float64        `json:"total_recharged"`
	FirstRechargeEligible    bool           `json:"first_recharge_eligible"`
	BalanceLowWarning        bool           `json:"balance_low_warning"`
	BalanceLowThreshold      float64        `json:"balance_low_threshold,omitempty"`
	RechargeMultiplier       float64        `json:"recharge_multiplier"`
	PaymentEnabled           bool           `json:"payment_enabled"`
	CampaignRechargeBonusPct float64        `json:"campaign_recharge_bonus_pct,omitempty"`
	VIP                      *PlayVIPStatus `json:"vip,omitempty"`
}

// PlayHubSummary aggregates all play module states for the logged-in user.
type PlayHubSummary struct {
	AnyEnabled     bool                  `json:"any_enabled"`
	PendingActions int                   `json:"pending_actions"`
	Growth         PlayHubGrowth         `json:"growth"`
	Campaigns      []PlayCampaignSummary `json:"campaigns,omitempty"`
	ImageStudio    *PlayHubImageStudio   `json:"image_studio,omitempty"`
	Quests         *PlayQuestToday       `json:"quests,omitempty"`
	Checkin        *PlayCheckinStatus    `json:"checkin,omitempty"`
	Arena          *PlayArenaCurrent     `json:"arena,omitempty"`
	DailyArena     *PlayArenaCurrent     `json:"daily_arena,omitempty"`
	Blindbox       *PlayBlindboxStatus   `json:"blindbox,omitempty"`
	Quiz           *PlayQuizToday        `json:"quiz,omitempty"`
	Team           *PlayTeamMe           `json:"team,omitempty"`
}

type PlayHubImageStudio struct {
	Enabled         bool `json:"enabled"`
	ImagesToday     int  `json:"images_today"`
	HasCompletedJob bool `json:"has_completed_job"`
}

// GetHub returns a single payload for the Play Hub dashboard.
func (s *PlayService) GetHub(ctx context.Context, userID int64, language string) (*PlayHubSummary, error) {
	rt := s.GetRuntime(ctx)
	hub := &PlayHubSummary{
		AnyEnabled: rt.CheckinEnabled || rt.ArenaEnabled || rt.BlindboxEnabled ||
			rt.QuizEnabled || rt.AgentTeamEnabled || rt.ImageStudioEnabled || rt.DailyQuestsEnabled,
	}

	if userID <= 0 {
		return hub, nil
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	hub.Growth = s.buildHubGrowth(ctx, user, s.GetRuntime(ctx))

	if rt.ImageStudioEnabled {
		dayStart := s.serverDate(s.serverNow())
		count, err := s.repo.CountImageStudioJobsToday(ctx, userID, dayStart)
		if err != nil {
			return nil, err
		}
		hasJob, err := s.repo.HasCompletedImageStudioJob(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.ImageStudio = &PlayHubImageStudio{
			Enabled:         true,
			ImagesToday:     count,
			HasCompletedJob: hasJob,
		}
		hub.AnyEnabled = true
		if !hasJob {
			hub.PendingActions++
		}
	}

	if rt.DailyQuestsEnabled {
		quests, err := s.GetQuestsToday(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.Quests = quests
		hub.AnyEnabled = true
		for _, task := range quests.Tasks {
			if !task.Completed {
				hub.PendingActions++
			}
		}
	}

	if rt.CheckinEnabled {
		status, err := s.GetCheckinStatus(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.Checkin = status
		if status != nil && !status.CheckedInToday {
			hub.PendingActions++
		}
	}

	if rt.ArenaEnabled {
		current, err := s.GetArenaCurrent(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.Arena = current
		if rt.DailyArenaEnabled {
			daily, err := s.GetDailyArenaCurrent(ctx, userID)
			if err != nil {
				return nil, err
			}
			hub.DailyArena = daily
		}
	}

	if rt.BlindboxEnabled {
		status, err := s.GetBlindboxStatus(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.Blindbox = status
		if status != nil && status.CanOpen {
			hub.PendingActions++
		}
	}

	if rt.QuizEnabled {
		today, err := s.GetQuizToday(ctx, userID, language)
		if err != nil {
			return nil, err
		}
		hub.Quiz = today
		if today != nil && !today.AlreadySubmitted && len(today.Questions) > 0 {
			hub.PendingActions++
		}
	}

	if rt.AgentTeamEnabled {
		team, err := s.GetTeamMe(ctx, userID)
		if err != nil {
			return nil, err
		}
		hub.Team = team
	}

	if rt.CampaignsEnabled {
		campaigns, err := s.ListActiveCampaigns(ctx)
		if err != nil {
			return nil, err
		}
		hub.Campaigns = campaigns
		if len(campaigns) > 0 {
			hub.AnyEnabled = true
		}
	}

	return hub, nil
}

func (s *PlayService) buildHubGrowth(ctx context.Context, user *User, rt PlayRuntime) PlayHubGrowth {
	out := PlayHubGrowth{
		Balance:        user.Balance,
		TotalRecharged: user.TotalRecharged,
	}
	vip := resolveVIPStatus(user.TotalRecharged, rt.VIPTiers)
	out.VIP = &vip
	if s.settingService == nil {
		return out
	}

	public, err := s.settingService.GetPublicSettings(ctx)
	if err != nil || public == nil {
		return out
	}

	out.PaymentEnabled = public.PaymentEnabled
	out.RechargeMultiplier = s.settingService.GetBalanceRechargeMultiplier(ctx)

	if rt.CampaignsEnabled {
		if campaigns, err := s.repo.ListActiveCampaigns(ctx, s.serverNow()); err == nil && len(campaigns) > 0 {
			rules := aggregateCampaignRules(campaigns)
			out.CampaignRechargeBonusPct = rules.RechargeBonusPct
		}
	}

	out.FirstRechargeEligible = public.PaymentEnabled && user.TotalRecharged <= 0

	if public.BalanceLowNotifyEnabled && public.BalanceLowNotifyThreshold > 0 {
		threshold := resolveBalanceThreshold(
			public.BalanceLowNotifyThreshold,
			user.BalanceNotifyThresholdType,
			user.TotalRecharged,
		)
		if user.BalanceNotifyThreshold != nil && *user.BalanceNotifyThreshold > 0 {
			threshold = resolveBalanceThreshold(
				*user.BalanceNotifyThreshold,
				user.BalanceNotifyThresholdType,
				user.TotalRecharged,
			)
		}
		if threshold > 0 && user.Balance < threshold {
			out.BalanceLowWarning = true
			out.BalanceLowThreshold = threshold
		}
	}

	return out
}
