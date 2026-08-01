package service

import (
	"context"

	"golang.org/x/sync/errgroup"
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
	VIPTiers                 []PlayVIPTier  `json:"vip_tiers,omitempty"`
	MembershipPaidAmount     float64        `json:"membership_paid_amount,omitempty"`
	IsMember                 bool           `json:"is_member"`
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
	ctx = withPlayRequestCache(ctx)
	rt := s.GetRuntime(ctx)
	hub := &PlayHubSummary{
		AnyEnabled: rt.CheckinEnabled || rt.ArenaEnabled || rt.BlindboxEnabled ||
			rt.QuizEnabled || rt.AgentTeamEnabled || rt.ImageStudioEnabled || rt.DailyQuestsEnabled,
	}

	if userID <= 0 {
		return hub, nil
	}

	user, err := s.playRequestUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var (
		growth     PlayHubGrowth
		campaigns  []PlayCampaignSummary
		image      *PlayHubImageStudio
		quests     *PlayQuestToday
		checkin    *PlayCheckinStatus
		arena      *PlayArenaCurrent
		dailyArena *PlayArenaCurrent
		blindbox   *PlayBlindboxStatus
		quiz       *PlayQuizToday
		team       *PlayTeamMe
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var growthErr error
		growth, growthErr = s.buildHubGrowth(groupCtx, user, rt)
		return growthErr
	})
	if rt.ImageStudioEnabled {
		group.Go(func() error {
			dayStart := s.serverDate(s.serverNow())
			count, imageErr := s.repo.CountImageStudioJobsToday(groupCtx, userID, dayStart)
			if imageErr != nil {
				return imageErr
			}
			hasJob, imageErr := s.repo.HasCompletedImageStudioJob(groupCtx, userID)
			if imageErr != nil {
				return imageErr
			}
			image = &PlayHubImageStudio{Enabled: true, ImagesToday: count, HasCompletedJob: hasJob}
			return nil
		})
	}

	if rt.DailyQuestsEnabled {
		group.Go(func() error {
			var questsErr error
			quests, questsErr = s.GetQuestsToday(groupCtx, userID)
			return questsErr
		})
	}

	if rt.CheckinEnabled {
		group.Go(func() error {
			var checkinErr error
			checkin, checkinErr = s.GetCheckinStatus(groupCtx, userID)
			return checkinErr
		})
	}

	if rt.ArenaEnabled {
		group.Go(func() error {
			var arenaErr error
			arena, arenaErr = s.GetArenaCurrent(groupCtx, userID)
			return arenaErr
		})
		if rt.DailyArenaEnabled {
			group.Go(func() error {
				var dailyArenaErr error
				dailyArena, dailyArenaErr = s.GetDailyArenaCurrent(groupCtx, userID)
				return dailyArenaErr
			})
		}
	}

	if rt.BlindboxEnabled {
		group.Go(func() error {
			var blindboxErr error
			blindbox, blindboxErr = s.GetBlindboxStatus(groupCtx, userID)
			return blindboxErr
		})
	}

	if rt.QuizEnabled {
		group.Go(func() error {
			var quizErr error
			quiz, quizErr = s.GetQuizToday(groupCtx, userID, language)
			return quizErr
		})
	}

	if rt.AgentTeamEnabled {
		group.Go(func() error {
			var teamErr error
			team, teamErr = s.GetTeamMe(groupCtx, userID)
			return teamErr
		})
	}

	if rt.CampaignsEnabled {
		group.Go(func() error {
			var campaignsErr error
			campaigns, campaignsErr = s.ListActiveCampaignsForUser(groupCtx, userID)
			return campaignsErr
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}

	hub.Growth = growth
	hub.ImageStudio = image
	hub.Quests = quests
	hub.Checkin = checkin
	hub.Arena = arena
	hub.DailyArena = dailyArena
	hub.Blindbox = blindbox
	hub.Quiz = quiz
	hub.Team = team
	hub.Campaigns = campaigns
	// The campaign list is already loaded above for the response. Reuse that
	// result for the summary instead of issuing another identical audience query.
	if len(campaigns) > 0 {
		hub.Growth.CampaignRechargeBonusPct = aggregateCampaignRulesFromSummaries(campaigns).RechargeBonusPct
	}

	if image != nil {
		hub.AnyEnabled = true
		if !image.HasCompletedJob {
			hub.PendingActions++
		}
	}
	if quests != nil {
		hub.AnyEnabled = true
		for _, task := range quests.Tasks {
			if !task.Completed {
				hub.PendingActions++
			}
		}
	}
	if checkin != nil && !checkin.CheckedInToday {
		hub.PendingActions++
	}
	if blindbox != nil && blindbox.CanOpen {
		hub.PendingActions++
	}
	if quiz != nil && !quiz.AlreadySubmitted && len(quiz.Questions) > 0 {
		hub.PendingActions++
	}
	if len(campaigns) > 0 {
		hub.AnyEnabled = true
	}

	return hub, nil
}

func (s *PlayService) buildHubGrowth(ctx context.Context, user *User, rt PlayRuntime) (PlayHubGrowth, error) {
	out := PlayHubGrowth{
		Balance:        user.Balance,
		TotalRecharged: user.TotalRecharged,
	}
	paidTotal, err := s.MembershipPaidTotal(ctx, user.ID)
	if err != nil {
		return PlayHubGrowth{}, err
	}
	vip := resolveVIPStatus(paidTotal, rt.VIPTiers)
	out.VIP = &vip
	out.VIPTiers = append([]PlayVIPTier(nil), rt.VIPTiers...)
	out.MembershipPaidAmount = paidTotal
	out.IsMember = paidTotal+1e-9 >= firstMemberThreshold(rt.VIPTiers)
	if s.settingService == nil {
		return out, nil
	}

	public, err := s.settingService.GetPublicSettings(ctx)
	if err != nil || public == nil {
		return out, nil
	}

	out.PaymentEnabled = public.PaymentEnabled
	out.RechargeMultiplier = s.settingService.GetBalanceRechargeMultiplier(ctx)

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

	return out, nil
}

func aggregateCampaignRulesFromSummaries(campaigns []PlayCampaignSummary) PlayCampaignRules {
	rows := make([]PlayCampaign, 0, len(campaigns))
	for _, campaign := range campaigns {
		rows = append(rows, PlayCampaign{Rules: campaign.Rules})
	}
	return aggregateCampaignRules(rows)
}
