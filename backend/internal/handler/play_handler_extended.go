package handler

import (
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playBlindboxStatusDTO struct {
	Enabled             bool                  `json:"enabled"`
	CouponPoolReady     bool                  `json:"coupon_pool_ready"`
	CostAmount          float64               `json:"cost_amount"`
	Pool                *playBlindboxPoolDTO  `json:"pool,omitempty"`
	CurrentPool         *playBlindboxPoolDTO  `json:"current_pool,omitempty"`
	NextPool            *playBlindboxPoolDTO  `json:"next_pool,omitempty"`
	VIPTier             service.PlayVIPStatus `json:"vip_tier"`
	ExpectedReward      float64               `json:"expected_reward,omitempty"`
	NextExpectedReward  float64               `json:"next_expected_reward,omitempty"`
	PoolVersion         string                `json:"pool_version,omitempty"`
	RTPCap              float64               `json:"rtp_cap,omitempty"`
	DailyLimit          int                   `json:"daily_limit"`
	EffectiveLimit      int                   `json:"effective_limit,omitempty"`
	OpensToday          int                   `json:"opens_today"`
	CanOpen             bool                  `json:"can_open"`
	ServerDate          string                `json:"server_date"`
	RechargeBoostActive bool                  `json:"recharge_boost_active,omitempty"`
	CampaignActive      bool                  `json:"campaign_active,omitempty"`
}

type playUserTeamSettlementDTO struct {
	SettlementID         int64   `json:"settlement_id"`
	TeamID               int64   `json:"team_id"`
	TeamName             string  `json:"team_name"`
	SettlementMonth      string  `json:"settlement_month"`
	TeamSpend            string  `json:"team_spend"`
	PoolAmount           string  `json:"pool_amount"`
	SettlementStatus     string  `json:"settlement_status"`
	PersonalContribution string  `json:"personal_contribution"`
	PersonalRatio        string  `json:"personal_ratio"`
	PersonalReward       string  `json:"personal_reward"`
	PayoutStatus         string  `json:"payout_status"`
	PaidAt               *string `json:"paid_at,omitempty"`
}

type playBlindboxPoolResponseDTO struct {
	Enabled            bool                  `json:"enabled"`
	CouponPoolReady    bool                  `json:"coupon_pool_ready"`
	Pool               playBlindboxPoolDTO   `json:"pool"`
	CurrentPool        playBlindboxPoolDTO   `json:"current_pool"`
	NextPool           *playBlindboxPoolDTO  `json:"next_pool,omitempty"`
	VIPTier            service.PlayVIPStatus `json:"vip_tier"`
	ExpectedReward     float64               `json:"expected_reward,omitempty"`
	NextExpectedReward float64               `json:"next_expected_reward,omitempty"`
	PoolVersion        string                `json:"pool_version,omitempty"`
	RTPCap             float64               `json:"rtp_cap,omitempty"`
}

type playBlindboxPoolDTO struct {
	Version string                    `json:"version"`
	Cost    float64                   `json:"cost"`
	RTPCap  float64                   `json:"rtp_cap"`
	Tiers   []playBlindboxPoolTierDTO `json:"tiers"`
}

type playBlindboxPoolTierDTO struct {
	Amount float64 `json:"amount"`
	Weight int64   `json:"weight"`
}

type playBlindboxOpenResultDTO struct {
	CostAmount        float64                `json:"cost_amount"`
	RewardAmount      float64                `json:"reward_amount"`
	NetAmount         float64                `json:"net_amount"`
	RewardType        service.PlayRewardType `json:"reward_type"`
	Coupon            *playCouponRewardDTO   `json:"coupon,omitempty"`
	CouponPoolVersion string                 `json:"coupon_pool_version,omitempty"`
	OpensToday        int                    `json:"opens_today"`
	ServerDate        string                 `json:"server_date"`
	PoolVersion       string                 `json:"pool_version"`
	OpenSource        string                 `json:"open_source"`
	VIPTier           service.PlayVIPStatus  `json:"vip_tier"`
	ExpectedReward    float64                `json:"expected_reward,omitempty"`
	RTPCap            float64                `json:"rtp_cap,omitempty"`
}

type playCouponRewardDTO struct {
	UserCouponID       int64                     `json:"user_coupon_id"`
	TemplateID         int64                     `json:"template_id"`
	Name               string                    `json:"name"`
	BenefitType        service.CouponBenefitType `json:"benefit_type"`
	BenefitValue       float64                   `json:"benefit_value"`
	MaxDiscountAmount  *float64                  `json:"max_discount_amount,omitempty"`
	Currency           string                    `json:"currency"`
	ApplicableScopes   []service.CouponScope     `json:"applicable_scopes"`
	MinimumOrderAmount float64                   `json:"minimum_order_amount"`
	ValidFrom          string                    `json:"valid_from"`
	ExpiresAt          string                    `json:"expires_at"`
}

type playBlindboxRecentWinDTO struct {
	User   string  `json:"user"`
	Reward float64 `json:"reward"`
	When   string  `json:"when"`
}

type playQuizQuestionDTO struct {
	ID      int64    `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
}

type playQuizTodayDTO struct {
	Enabled                   bool                   `json:"enabled"`
	CouponPoolReady           bool                   `json:"coupon_pool_ready"`
	Questions                 []playQuizQuestionDTO  `json:"questions"`
	AlreadySubmitted          bool                   `json:"already_submitted"`
	PreviousScore             int                    `json:"previous_score,omitempty"`
	PreviousTotal             int                    `json:"previous_total,omitempty"`
	PreviousReward            float64                `json:"previous_reward,omitempty"`
	PreviousRewardType        service.PlayRewardType `json:"previous_reward_type,omitempty"`
	PreviousCoupon            *playCouponRewardDTO   `json:"previous_coupon,omitempty"`
	PreviousCouponPoolVersion string                 `json:"previous_coupon_pool_version,omitempty"`
	RewardPerCorrect          float64                `json:"reward_per_correct"`
	ServerDate                string                 `json:"server_date"`
}

type playQuizSubmitRequest struct {
	Answers []playQuizAnswerDTO `json:"answers"`
}

type playQuizAnswerDTO struct {
	QuestionID  int64 `json:"question_id"`
	ChoiceIndex int   `json:"choice_index"`
}

type playQuizSubmitResultDTO struct {
	Score             int                    `json:"score"`
	Total             int                    `json:"total"`
	RewardAmount      float64                `json:"reward_amount"`
	RewardType        service.PlayRewardType `json:"reward_type"`
	Coupon            *playCouponRewardDTO   `json:"coupon,omitempty"`
	CouponPoolVersion string                 `json:"coupon_pool_version,omitempty"`
	ServerDate        string                 `json:"server_date"`
}

type playTeamMemberDTO struct {
	UserID          int64  `json:"user_id"`
	DisplayName     string `json:"display_name"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	JoinedAt        string `json:"joined_at"`
	TokenSum        int64  `json:"token_sum"`
	TokenPct        int    `json:"token_pct"`
	Spend           string `json:"spend"`
	SpendPct        int    `json:"spend_pct"`
	EstimatedReward string `json:"estimated_reward"`
}

type playTeamAffiliateDTO struct {
	Enabled             bool    `json:"enabled"`
	TokenThreshold      int64   `json:"token_threshold"`
	MilestoneReached    bool    `json:"milestone_reached"`
	TokensToMilestone   int64   `json:"tokens_to_milestone,omitempty"`
	CaptainBonus        float64 `json:"captain_bonus,omitempty"`
	CaptainBonusGranted bool    `json:"captain_bonus_granted,omitempty"`
}

type playTeamSummaryDTO struct {
	ID               int64                    `json:"id"`
	Name             string                   `json:"name"`
	InviteCode       string                   `json:"invite_code"`
	CaptainID        int64                    `json:"captain_id"`
	MemberCount      int                      `json:"member_count"`
	TokenSum         int64                    `json:"token_sum"`
	Members          []playTeamMemberDTO      `json:"members"`
	Affiliate        *playTeamAffiliateDTO    `json:"affiliate,omitempty"`
	CurrentMonth     string                   `json:"current_month"`
	TeamSpend        string                   `json:"team_spend"`
	ReachedThreshold string                   `json:"reached_threshold"`
	RewardRate       string                   `json:"reward_rate"`
	NextThreshold    string                   `json:"next_threshold"`
	EstimatedPool    string                   `json:"estimated_pool"`
	RewardCap        string                   `json:"reward_cap"`
	RewardTiers      []service.TeamRewardTier `json:"reward_tiers"`
}

type playTeamMeDTO struct {
	Enabled bool                `json:"enabled"`
	Team    *playTeamSummaryDTO `json:"team,omitempty"`
}

type playTeamCreateRequest struct {
	Name string `json:"name"`
}

type playTeamJoinRequest struct {
	InviteCode string `json:"invite_code"`
}

type playTeamMemberActionRequest struct {
	TargetUserID int64 `json:"target_user_id" binding:"required,gt=0"`
}

func (h *PlayHandler) BlindboxStatus(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.playService.GetBlindboxStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playBlindboxStatusDTO{
		Enabled:             status.Enabled,
		CouponPoolReady:     status.CouponPoolReady,
		CostAmount:          status.CostAmount,
		Pool:                toPlayBlindboxPoolDTOPtr(status.BlindboxPool),
		CurrentPool:         toPlayBlindboxPoolDTOPtr(status.CurrentPool),
		NextPool:            toOptionalPlayBlindboxPoolDTO(status.NextPool),
		VIPTier:             status.VIPTier,
		ExpectedReward:      status.ExpectedReward,
		NextExpectedReward:  status.NextExpectedReward,
		PoolVersion:         status.PoolVersion,
		RTPCap:              status.RTPCap,
		DailyLimit:          status.DailyLimit,
		EffectiveLimit:      status.EffectiveLimit,
		OpensToday:          status.OpensToday,
		CanOpen:             status.CanOpen,
		ServerDate:          status.ServerDate,
		RechargeBoostActive: status.RechargeBoostActive,
		CampaignActive:      status.CampaignActive,
	})
}

func (h *PlayHandler) BlindboxPool(c *gin.Context) {
	status, err := h.playService.GetBlindboxStatus(c.Request.Context(), 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playBlindboxPoolResponseDTO{
		Enabled:            status.Enabled,
		CouponPoolReady:    status.CouponPoolReady,
		Pool:               toPlayBlindboxPoolDTO(status.BlindboxPool),
		CurrentPool:        toPlayBlindboxPoolDTO(status.CurrentPool),
		NextPool:           toOptionalPlayBlindboxPoolDTO(status.NextPool),
		VIPTier:            status.VIPTier,
		ExpectedReward:     status.ExpectedReward,
		NextExpectedReward: status.NextExpectedReward,
		PoolVersion:        status.PoolVersion,
		RTPCap:             status.RTPCap,
	})
}

func (h *PlayHandler) BlindboxOpen(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.playService.OpenBlindbox(c.Request.Context(), subject.UserID, c.GetHeader("Idempotency-Key"))
	if err != nil {
		response.ErrorFrom(c, playCouponPoolHTTPError(err))
		return
	}
	response.Success(c, playBlindboxOpenResultDTO{
		CostAmount:        result.CostAmount,
		RewardAmount:      result.RewardAmount,
		NetAmount:         result.NetAmount,
		RewardType:        result.RewardType,
		Coupon:            toPlayCouponRewardDTO(result.Coupon),
		CouponPoolVersion: result.CouponPoolVersion,
		OpensToday:        result.OpensToday,
		ServerDate:        result.ServerDate,
		PoolVersion:       result.PoolVersion,
		OpenSource:        result.OpenSource,
		VIPTier:           result.VIPTier,
		ExpectedReward:    result.ExpectedReward,
		RTPCap:            result.RTPCap,
	})
}

func toPlayCouponRewardDTO(coupon *service.PlayCouponRewardSummary) *playCouponRewardDTO {
	if coupon == nil {
		return nil
	}
	return &playCouponRewardDTO{
		UserCouponID:       coupon.UserCouponID,
		TemplateID:         coupon.TemplateID,
		Name:               coupon.Name,
		BenefitType:        coupon.BenefitType,
		BenefitValue:       coupon.BenefitValue,
		MaxDiscountAmount:  coupon.MaxDiscountAmount,
		Currency:           coupon.Currency,
		ApplicableScopes:   append([]service.CouponScope(nil), coupon.ApplicableScopes...),
		MinimumOrderAmount: coupon.MinimumOrderAmount,
		ValidFrom:          coupon.ValidFrom.Format(time.RFC3339),
		ExpiresAt:          coupon.ExpiresAt.Format(time.RFC3339),
	}
}

func toPlayBlindboxPoolDTO(pool service.PlayBlindboxPool) playBlindboxPoolDTO {
	out := playBlindboxPoolDTO{
		Version: pool.Version,
		Cost:    pool.Cost,
		RTPCap:  pool.RTPCap,
		Tiers:   make([]playBlindboxPoolTierDTO, 0, len(pool.Tiers)),
	}
	for _, tier := range pool.Tiers {
		out.Tiers = append(out.Tiers, playBlindboxPoolTierDTO{
			Amount: tier.Amount,
			Weight: tier.Weight,
		})
	}
	return out
}

func toPlayBlindboxPoolDTOPtr(pool service.PlayBlindboxPool) *playBlindboxPoolDTO {
	out := toPlayBlindboxPoolDTO(pool)
	return &out
}

func toOptionalPlayBlindboxPoolDTO(pool *service.PlayBlindboxPool) *playBlindboxPoolDTO {
	if pool == nil {
		return nil
	}
	return toPlayBlindboxPoolDTOPtr(*pool)
}

func (h *PlayHandler) BlindboxRecent(c *gin.Context) {
	wins, err := h.playService.ListRecentBlindboxWins(c.Request.Context(), 20)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]playBlindboxRecentWinDTO, 0, len(wins))
	for _, w := range wins {
		out = append(out, playBlindboxRecentWinDTO{
			User:   w.UserLabel,
			Reward: w.RewardAmount,
			When:   w.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	response.Success(c, out)
}

func (h *PlayHandler) QuizToday(c *gin.Context) {
	var userID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	language := c.GetHeader("Accept-Language")
	today, err := h.playService.GetQuizToday(c.Request.Context(), userID, language)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playQuizTodayDTO{
		Enabled:                   today.Enabled,
		CouponPoolReady:           today.CouponPoolReady,
		Questions:                 make([]playQuizQuestionDTO, 0, len(today.Questions)),
		AlreadySubmitted:          today.AlreadySubmitted,
		PreviousScore:             today.PreviousScore,
		PreviousTotal:             today.PreviousTotal,
		PreviousReward:            today.PreviousReward,
		PreviousRewardType:        today.PreviousRewardType,
		PreviousCoupon:            toPlayCouponRewardDTO(today.PreviousCoupon),
		PreviousCouponPoolVersion: today.PreviousCouponPoolVersion,
		RewardPerCorrect:          today.RewardPerCorrect,
		ServerDate:                today.ServerDate,
	}
	for _, q := range today.Questions {
		out.Questions = append(out.Questions, playQuizQuestionDTO{
			ID:      q.ID,
			Prompt:  q.Prompt,
			Options: q.Options,
		})
	}
	response.Success(c, out)
}

func (h *PlayHandler) QuizSubmit(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playQuizSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	answers := make([]service.PlayQuizAnswer, 0, len(req.Answers))
	for _, a := range req.Answers {
		answers = append(answers, service.PlayQuizAnswer{
			QuestionID:  a.QuestionID,
			ChoiceIndex: a.ChoiceIndex,
		})
	}
	language := c.GetHeader("Accept-Language")
	result, err := h.playService.SubmitQuiz(c.Request.Context(), subject.UserID, language, answers)
	if err != nil {
		response.ErrorFrom(c, playCouponPoolHTTPError(err))
		return
	}
	response.Success(c, playQuizSubmitResultDTO{
		Score:             result.Score,
		Total:             result.Total,
		RewardAmount:      result.RewardAmount,
		RewardType:        result.RewardType,
		Coupon:            toPlayCouponRewardDTO(result.Coupon),
		CouponPoolVersion: result.CouponPoolVersion,
		ServerDate:        result.ServerDate,
	})
}

func playCouponPoolHTTPError(err error) error {
	if errors.Is(err, service.ErrCouponRewardPoolUnavailable) {
		return infraerrors.ServiceUnavailable(
			"COUPON_REWARD_POOL_UNAVAILABLE",
			"coupon reward pool is not published",
		).WithCause(err)
	}
	return err
}

func (h *PlayHandler) TeamMe(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	me, err := h.playService.GetTeamMe(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playTeamMeDTO{Enabled: me.Enabled}
	if me.Team != nil {
		out.Team = toPlayTeamSummaryDTO(me.Team)
	}
	response.Success(c, out)
}

func (h *PlayHandler) TeamCreate(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	team, err := h.playService.CreateTeam(c.Request.Context(), subject.UserID, req.Name)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayTeamSummaryDTO(team))
}

func (h *PlayHandler) TeamJoin(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamJoinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	team, err := h.playService.JoinTeam(c.Request.Context(), subject.UserID, req.InviteCode)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayTeamSummaryDTO(team))
}

func (h *PlayHandler) TeamLeave(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if err := h.playService.LeaveTeam(c.Request.Context(), subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *PlayHandler) TeamTransfer(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamMemberActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.TransferTeamCaptain(c.Request.Context(), subject.UserID, req.TargetUserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *PlayHandler) TeamRemove(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamMemberActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.RemoveTeamMember(c.Request.Context(), subject.UserID, req.TargetUserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}

func (h *PlayHandler) TeamSettlements(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	records, err := h.playService.ListUserTeamRewardSettlements(c.Request.Context(), subject.UserID, 24)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toPlayUserTeamSettlementDTOs(records))
}

func toPlayUserTeamSettlementDTOs(records []service.PlayUserTeamSettlementRecord) []playUserTeamSettlementDTO {
	out := make([]playUserTeamSettlementDTO, 0, len(records))
	for _, record := range records {
		var paidAt *string
		if record.Allocation.PaidAt != nil {
			value := record.Allocation.PaidAt.Format("2006-01-02T15:04:05Z07:00")
			paidAt = &value
		}
		out = append(out, playUserTeamSettlementDTO{
			SettlementID:         record.Settlement.ID,
			TeamID:               record.Settlement.TeamID,
			TeamName:             record.TeamName,
			SettlementMonth:      record.Settlement.PeriodStart.Format("2006-01"),
			TeamSpend:            record.Settlement.TeamSpend.StringFixed(8),
			PoolAmount:           record.Settlement.PoolAmount.StringFixed(8),
			SettlementStatus:     record.Settlement.Status,
			PersonalContribution: record.Allocation.Contribution.StringFixed(8),
			PersonalRatio:        record.Allocation.Ratio.StringFixed(8),
			PersonalReward:       record.Allocation.RewardAmount.StringFixed(8),
			PayoutStatus:         record.Allocation.PayoutStatus,
			PaidAt:               paidAt,
		})
	}
	return out
}

func toPlayTeamSummaryDTO(team *service.PlayTeamSummary) *playTeamSummaryDTO {
	if team == nil {
		return nil
	}
	out := &playTeamSummaryDTO{
		ID:               team.ID,
		Name:             team.Name,
		InviteCode:       team.InviteCode,
		CaptainID:        team.CaptainID,
		MemberCount:      team.MemberCount,
		TokenSum:         team.TokenSum,
		Members:          make([]playTeamMemberDTO, 0, len(team.Members)),
		CurrentMonth:     team.CurrentMonth,
		TeamSpend:        team.TeamSpend.StringFixed(8),
		ReachedThreshold: team.ReachedThreshold.StringFixed(8),
		RewardRate:       team.RewardRate.StringFixed(8),
		NextThreshold:    team.NextThreshold.StringFixed(8),
		EstimatedPool:    team.EstimatedPool.StringFixed(8),
		RewardCap:        team.RewardCap.StringFixed(8),
		RewardTiers:      team.RewardTiers,
	}
	for _, m := range team.Members {
		out.Members = append(out.Members, playTeamMemberDTO{
			UserID:          m.UserID,
			DisplayName:     m.DisplayName,
			AvatarURL:       m.AvatarURL,
			JoinedAt:        m.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
			TokenSum:        m.TokenSum,
			TokenPct:        m.TokenPct,
			Spend:           m.Spend.StringFixed(8),
			SpendPct:        m.SpendPct,
			EstimatedReward: m.EstimatedReward.StringFixed(8),
		})
	}
	if team.Affiliate != nil {
		out.Affiliate = &playTeamAffiliateDTO{
			Enabled:             team.Affiliate.Enabled,
			TokenThreshold:      team.Affiliate.TokenThreshold,
			MilestoneReached:    team.Affiliate.MilestoneReached,
			TokensToMilestone:   team.Affiliate.TokensToMilestone,
			CaptainBonus:        team.Affiliate.CaptainBonus,
			CaptainBonusGranted: team.Affiliate.CaptainBonusGranted,
		}
	}
	return out
}
