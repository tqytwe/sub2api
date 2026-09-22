package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playHubGrowthDTO struct {
	Balance                  float64               `json:"balance"`
	TotalRecharged           float64               `json:"total_recharged"`
	FirstRechargeEligible    bool                  `json:"first_recharge_eligible"`
	BalanceLowWarning        bool                  `json:"balance_low_warning"`
	BalanceLowThreshold      float64               `json:"balance_low_threshold,omitempty"`
	RechargeMultiplier       float64               `json:"recharge_multiplier"`
	PaymentEnabled           bool                  `json:"payment_enabled"`
	CampaignRechargeBonusPct float64               `json:"campaign_recharge_bonus_pct,omitempty"`
	VIP                      *playVIPStatusDTO     `json:"vip,omitempty"`
	VIPTiers                 []service.PlayVIPTier `json:"vip_tiers,omitempty"`
	MembershipPaidAmount     float64               `json:"membership_paid_amount,omitempty"`
	IsMember                 bool                  `json:"is_member"`
}

type playCampaignRulesDTO struct {
	RechargeBonusPct     float64                          `json:"recharge_bonus_pct,omitempty"`
	BlindboxExtraOpens   int                              `json:"blindbox_extra_opens,omitempty"`
	ArenaScoreMultiplier float64                          `json:"arena_score_multiplier,omitempty"`
	NameI18n             map[string]string                `json:"name_i18n,omitempty"`
	CampaignType         string                           `json:"campaign_type,omitempty"`
	ReferralCampaignID   int64                            `json:"referral_campaign_id,omitempty"`
	QualificationMetric  string                           `json:"qualification_metric,omitempty"`
	RewardTiers          []service.PlayCampaignRewardTier `json:"reward_tiers,omitempty"`
	RequireInvite        bool                             `json:"require_invite,omitempty"`
	LegacyRebatePolicy   string                           `json:"legacy_rebate_policy,omitempty"`
	DisplayTitleI18n     map[string]string                `json:"display_title_i18n,omitempty"`
	DisplayBodyI18n      map[string]string                `json:"display_body_i18n,omitempty"`
	DisplayCTA           string                           `json:"display_cta,omitempty"`
	DisplayPriority      int                              `json:"display_priority,omitempty"`
}

type playCampaignSummaryDTO struct {
	ID            int64                              `json:"id"`
	Name          string                             `json:"name"`
	StartAt       string                             `json:"start_at"`
	EndAt         string                             `json:"end_at"`
	Rules         playCampaignRulesDTO               `json:"rules"`
	NewUserGrowth *service.PlayNewUserGrowthProgress `json:"new_user_growth,omitempty"`
}

type playVIPStatusDTO struct {
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

type playHubSummaryDTO struct {
	AnyEnabled     bool                     `json:"any_enabled"`
	PendingActions int                      `json:"pending_actions"`
	Growth         playHubGrowthDTO         `json:"growth"`
	Campaigns      []playCampaignSummaryDTO `json:"campaigns,omitempty"`
	ImageStudio    *playHubImageStudioDTO   `json:"image_studio,omitempty"`
	Quests         *playQuestTodayDTO       `json:"quests,omitempty"`
	Checkin        *playCheckinStatusDTO    `json:"checkin,omitempty"`
	Arena          *playArenaCurrentDTO     `json:"arena,omitempty"`
	DailyArena     *playArenaCurrentDTO     `json:"daily_arena,omitempty"`
	Blindbox       *playBlindboxStatusDTO   `json:"blindbox,omitempty"`
	Quiz           *playQuizTodayDTO        `json:"quiz,omitempty"`
	Team           *playTeamMeDTO           `json:"team,omitempty"`
}

// Hub returns aggregated play state for the logged-in user.
// GET /api/v1/play/hub
func (h *PlayHandler) Hub(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	language := c.GetHeader("Accept-Language")
	summary, err := h.playService.GetHub(c.Request.Context(), subject.UserID, language)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayHubSummaryDTOForActor(summary, subject.UserID))
}

func toPlayHubSummaryDTOForActor(s *service.PlayHubSummary, actorUserID int64) playHubSummaryDTO {
	if s == nil {
		return playHubSummaryDTO{}
	}
	out := playHubSummaryDTO{
		AnyEnabled:     s.AnyEnabled,
		PendingActions: s.PendingActions,
		Growth: playHubGrowthDTO{
			Balance:                  s.Growth.Balance,
			TotalRecharged:           s.Growth.TotalRecharged,
			FirstRechargeEligible:    s.Growth.FirstRechargeEligible,
			BalanceLowWarning:        s.Growth.BalanceLowWarning,
			BalanceLowThreshold:      s.Growth.BalanceLowThreshold,
			RechargeMultiplier:       s.Growth.RechargeMultiplier,
			PaymentEnabled:           s.Growth.PaymentEnabled,
			CampaignRechargeBonusPct: s.Growth.CampaignRechargeBonusPct,
			VIP:                      toPlayVIPStatusDTO(s.Growth.VIP),
			VIPTiers:                 s.Growth.VIPTiers,
			MembershipPaidAmount:     s.Growth.MembershipPaidAmount,
			IsMember:                 s.Growth.IsMember,
		},
		Campaigns: toPlayCampaignSummaryDTOs(s.Campaigns),
	}
	out.ImageStudio = toPlayHubImageStudioDTO(s.ImageStudio)
	if s.Quests != nil {
		q := toPlayQuestTodayDTO(s.Quests)
		out.Quests = &q
	}
	if s.Checkin != nil {
		out.Checkin = &playCheckinStatusDTO{
			Enabled:                  s.Checkin.Enabled,
			Eligible:                 s.Checkin.Eligible,
			IneligibleReason:         s.Checkin.IneligibleReason,
			CheckedInToday:           s.Checkin.CheckedInToday,
			RewardAmount:             s.Checkin.RewardAmount,
			CouponPoolReady:          s.Checkin.CouponPoolReady,
			CouponWeightBP:           s.Checkin.CouponWeightBP,
			RedeemCodeWeightBP:       s.Checkin.RedeemCodeWeightBP,
			BalanceWeightBP:          s.Checkin.BalanceWeightBP,
			ServerDate:               s.Checkin.ServerDate,
			StreakCount:              s.Checkin.StreakCount,
			NextMilestoneDays:        s.Checkin.NextMilestoneDays,
			NextMilestoneBonus:       s.Checkin.NextMilestoneBonus,
			CanMakeup:                s.Checkin.CanMakeup,
			MakeupDate:               s.Checkin.MakeupDate,
			RechargeBoostActive:      s.Checkin.RechargeBoostActive,
			BoostCheckinMultiplier:   s.Checkin.BoostCheckinMultiplier,
			GrowthEligibility:        s.Checkin.GrowthEligibility,
			GrowthEnergyEnabled:      s.Checkin.GrowthEnergyEnabled,
			RedeemableRewardEligible: s.Checkin.RedeemableRewardEligible,
		}
	}
	if s.Arena != nil {
		dto := playArenaCurrentDTO{
			Enabled:              s.Arena.Enabled,
			TokenSum:             s.Arena.TokenSum,
			DisplayTokenSum:      s.Arena.DisplayTokenSum,
			Rank:                 s.Arena.Rank,
			TokensToPrevRank:     s.Arena.TokensToPrevRank,
			EstimatedReward:      s.Arena.EstimatedReward,
			RechargeBoostActive:  s.Arena.RechargeBoostActive,
			ArenaScoreMultiplier: s.Arena.ArenaScoreMultiplier,
			CampaignActive:       s.Arena.CampaignActive,
		}
		if s.Arena.Period != nil {
			dto.Period = toPlayArenaPeriodDTO(s.Arena.Period)
		}
		out.Arena = &dto
	}
	if s.DailyArena != nil {
		dto := playArenaCurrentDTO{
			Enabled:          s.DailyArena.Enabled,
			TokenSum:         s.DailyArena.TokenSum,
			DisplayTokenSum:  s.DailyArena.DisplayTokenSum,
			Rank:             s.DailyArena.Rank,
			TokensToPrevRank: s.DailyArena.TokensToPrevRank,
			EstimatedReward:  s.DailyArena.EstimatedReward,
		}
		if s.DailyArena.Period != nil {
			dto.Period = toPlayArenaPeriodDTO(s.DailyArena.Period)
		}
		out.DailyArena = &dto
	}
	if s.Blindbox != nil {
		blindbox := playBlindboxStatusDTO{
			Enabled:             s.Blindbox.Enabled,
			CouponPoolReady:     s.Blindbox.CouponPoolReady,
			DailyLimit:          s.Blindbox.DailyLimit,
			EffectiveLimit:      s.Blindbox.EffectiveLimit,
			OpensToday:          s.Blindbox.OpensToday,
			CanOpen:             s.Blindbox.CanOpen,
			ServerDate:          s.Blindbox.ServerDate,
			GrowthEligibility:   s.Blindbox.GrowthEligibility,
			RechargeBoostActive: s.Blindbox.RechargeBoostActive,
			CampaignActive:      s.Blindbox.CampaignActive,
		}
		// Reward-pool internals are only safe for an explicitly redeemable,
		// authenticated actor. A missing actor must fail closed as well: this
		// helper is used by compatibility tests and may be reused by a route
		// whose authentication context is absent or malformed.
		if actorUserID > 0 && s.Blindbox.GrowthEligibility.RewardMode == service.PlayGrowthRewardRedeemable {
			blindbox.CostAmount = s.Blindbox.CostAmount
			blindbox.Pool = toPlayBlindboxPoolDTOPtr(s.Blindbox.BlindboxPool)
			blindbox.CurrentPool = toPlayBlindboxPoolDTOPtr(s.Blindbox.CurrentPool)
			blindbox.NextPool = toOptionalPlayBlindboxPoolDTO(s.Blindbox.NextPool)
			blindbox.VIPTier = s.Blindbox.VIPTier
			blindbox.ExpectedReward = s.Blindbox.ExpectedReward
			blindbox.NextExpectedReward = s.Blindbox.NextExpectedReward
			blindbox.PoolVersion = s.Blindbox.PoolVersion
			blindbox.RTPCap = s.Blindbox.RTPCap
		}
		out.Blindbox = &blindbox
	}
	if s.Quiz != nil {
		qdto := playQuizTodayDTO{
			Enabled:                   s.Quiz.Enabled,
			CouponPoolReady:           s.Quiz.CouponPoolReady,
			Questions:                 make([]playQuizQuestionDTO, 0, len(s.Quiz.Questions)),
			AlreadySubmitted:          s.Quiz.AlreadySubmitted,
			PreviousScore:             s.Quiz.PreviousScore,
			PreviousTotal:             s.Quiz.PreviousTotal,
			PreviousReward:            s.Quiz.PreviousReward,
			PreviousRewardType:        s.Quiz.PreviousRewardType,
			PreviousCoupon:            toPlayCouponRewardDTO(s.Quiz.PreviousCoupon),
			PreviousRedeemCode:        toPlayRedeemCodeRewardDTO(s.Quiz.PreviousRedeemCode),
			PreviousCouponPoolVersion: s.Quiz.PreviousCouponPoolVersion,
			RewardPerCorrect:          s.Quiz.RewardPerCorrect,
			ServerDate:                s.Quiz.ServerDate,
			GrowthEligibility:         s.Quiz.GrowthEligibility,
		}
		for _, q := range s.Quiz.Questions {
			qdto.Questions = append(qdto.Questions, playQuizQuestionDTO{
				ID:      q.ID,
				Prompt:  q.Prompt,
				Options: q.Options,
			})
		}
		out.Quiz = &qdto
	}
	if s.Team != nil {
		tdto := playTeamMeDTO{Enabled: s.Team.Enabled}
		if s.Team.Team != nil {
			tdto.Team = toPlayTeamSummaryDTOForActor(s.Team.Team, actorUserID)
		}
		out.Team = &tdto
	}
	return out
}

// toPlayHubSummaryDTO is retained for focused DTO tests that do not have an
// authenticated actor. A missing actor never receives team management data.
func toPlayHubSummaryDTO(s *service.PlayHubSummary) playHubSummaryDTO {
	return toPlayHubSummaryDTOForActor(s, 0)
}

func toPlayVIPStatusDTO(v *service.PlayVIPStatus) *playVIPStatusDTO {
	if v == nil {
		return nil
	}
	return &playVIPStatusDTO{
		Tier:             v.Tier,
		Label:            v.Label,
		RechargeBonusPct: v.RechargeBonusPct,
		ColorKey:         v.ColorKey,
		Perks:            v.Perks,
		NextTier:         v.NextTier,
		NextLabel:        v.NextLabel,
		NextMinRecharge:  v.NextMinRecharge,
		AmountToNext:     v.AmountToNext,
	}
}
