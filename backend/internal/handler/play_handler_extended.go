package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playBlindboxStatusDTO struct {
	Enabled             bool                             `json:"enabled"`
	CouponPoolReady     bool                             `json:"coupon_pool_ready"`
	CouponPrizes        []service.PlayCouponPrizePreview `json:"coupon_prizes,omitempty"`
	CouponWeightBP      int                              `json:"coupon_weight_bp,omitempty"`
	BalanceWeightBP     int                              `json:"balance_weight_bp,omitempty"`
	CostAmount          float64                          `json:"cost_amount,omitempty"`
	Pool                *playBlindboxPoolDTO             `json:"pool,omitempty"`
	CurrentPool         *playBlindboxPoolDTO             `json:"current_pool,omitempty"`
	NextPool            *playBlindboxPoolDTO             `json:"next_pool,omitempty"`
	VIPTier             service.PlayVIPStatus            `json:"vip_tier"`
	ExpectedReward      float64                          `json:"expected_reward,omitempty"`
	NextExpectedReward  float64                          `json:"next_expected_reward,omitempty"`
	PoolVersion         string                           `json:"pool_version,omitempty"`
	RTPCap              float64                          `json:"rtp_cap,omitempty"`
	DailyLimit          int                              `json:"daily_limit"`
	EffectiveLimit      int                              `json:"effective_limit,omitempty"`
	OpensToday          int                              `json:"opens_today"`
	CanOpen             bool                             `json:"can_open"`
	ServerDate          string                           `json:"server_date"`
	GrowthEligibility   service.PlayGrowthEligibility    `json:"growth_eligibility"`
	RechargeBoostActive bool                             `json:"recharge_boost_active,omitempty"`
	CampaignActive      bool                             `json:"campaign_active,omitempty"`
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
	Enabled            bool                             `json:"enabled"`
	CouponPoolReady    bool                             `json:"coupon_pool_ready"`
	CouponPrizes       []service.PlayCouponPrizePreview `json:"coupon_prizes"`
	CouponWeightBP     int                              `json:"coupon_weight_bp"`
	BalanceWeightBP    int                              `json:"balance_weight_bp"`
	Pool               playBlindboxPoolDTO              `json:"pool"`
	CurrentPool        playBlindboxPoolDTO              `json:"current_pool"`
	NextPool           *playBlindboxPoolDTO             `json:"next_pool,omitempty"`
	VIPTier            service.PlayVIPStatus            `json:"vip_tier"`
	ExpectedReward     float64                          `json:"expected_reward,omitempty"`
	NextExpectedReward float64                          `json:"next_expected_reward,omitempty"`
	PoolVersion        string                           `json:"pool_version,omitempty"`
	RTPCap             float64                          `json:"rtp_cap,omitempty"`
}

// playBlindboxPublicPreviewDTO is the deliberately small anonymous/Explorer
// contract. Pool configuration, odds, costs, and expected-value data are
// server-side reward controls rather than public marketing content.
type playBlindboxPublicPreviewDTO struct {
	Enabled         bool `json:"enabled"`
	CouponPoolReady bool `json:"coupon_pool_ready"`
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
	CostAmount        float64                  `json:"cost_amount"`
	RewardAmount      float64                  `json:"reward_amount"`
	NetAmount         float64                  `json:"net_amount"`
	RewardType        service.PlayRewardType   `json:"reward_type"`
	Coupon            *playCouponRewardDTO     `json:"coupon,omitempty"`
	RedeemCode        *playRedeemCodeRewardDTO `json:"redeem_code,omitempty"`
	CouponPoolVersion string                   `json:"coupon_pool_version,omitempty"`
	OpensToday        int                      `json:"opens_today"`
	ServerDate        string                   `json:"server_date"`
	PoolVersion       string                   `json:"pool_version"`
	OpenSource        string                   `json:"open_source"`
	VIPTier           service.PlayVIPStatus    `json:"vip_tier"`
	ExpectedReward    float64                  `json:"expected_reward,omitempty"`
	RTPCap            float64                  `json:"rtp_cap,omitempty"`
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

type playRedeemCodeRewardDTO struct {
	ID                int64      `json:"id"`
	Code              string     `json:"code"`
	Type              string     `json:"type"`
	Value             float64    `json:"value"`
	Status            string     `json:"status"`
	BatchName         string     `json:"batch_name,omitempty"`
	IssuedAt          *time.Time `json:"issued_at,omitempty"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	RewardPoolVersion string     `json:"reward_pool_version,omitempty"`
}

type playBlindboxRecentWinDTO struct {
	User       string                 `json:"user"`
	Reward     float64                `json:"reward"`
	RewardType service.PlayRewardType `json:"reward_type"`
	CouponName string                 `json:"coupon_name,omitempty"`
	When       string                 `json:"when"`
}

type playQuizQuestionDTO struct {
	ID      int64    `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
}

type playQuizTodayDTO struct {
	Enabled                   bool                          `json:"enabled"`
	CouponPoolReady           bool                          `json:"coupon_pool_ready"`
	Questions                 []playQuizQuestionDTO         `json:"questions"`
	AlreadySubmitted          bool                          `json:"already_submitted"`
	PreviousScore             int                           `json:"previous_score,omitempty"`
	PreviousTotal             int                           `json:"previous_total,omitempty"`
	PreviousReward            float64                       `json:"previous_reward,omitempty"`
	PreviousRewardType        service.PlayRewardType        `json:"previous_reward_type,omitempty"`
	PreviousGrowthEnergy      int64                         `json:"previous_growth_energy,omitempty"`
	PreviousCoupon            *playCouponRewardDTO          `json:"previous_coupon,omitempty"`
	PreviousRedeemCode        *playRedeemCodeRewardDTO      `json:"previous_redeem_code,omitempty"`
	PreviousCouponPoolVersion string                        `json:"previous_coupon_pool_version,omitempty"`
	RewardPerCorrect          float64                       `json:"reward_per_correct"`
	ServerDate                string                        `json:"server_date"`
	GrowthEligibility         service.PlayGrowthEligibility `json:"growth_eligibility"`
}

type playQuizSubmitRequest struct {
	Answers []playQuizAnswerDTO `json:"answers"`
}

type playQuizAnswerDTO struct {
	QuestionID  int64 `json:"question_id"`
	ChoiceIndex int   `json:"choice_index"`
}

type playQuizSubmitResultDTO struct {
	Score             int                           `json:"score"`
	Total             int                           `json:"total"`
	RewardAmount      float64                       `json:"reward_amount"`
	RewardType        service.PlayRewardType        `json:"reward_type"`
	Coupon            *playCouponRewardDTO          `json:"coupon,omitempty"`
	RedeemCode        *playRedeemCodeRewardDTO      `json:"redeem_code,omitempty"`
	CouponPoolVersion string                        `json:"coupon_pool_version,omitempty"`
	ServerDate        string                        `json:"server_date"`
	GrowthEnergy      int64                         `json:"growth_energy,omitempty"`
	GrowthEligibility service.PlayGrowthEligibility `json:"growth_eligibility"`
}

type playTeamMemberDTO struct {
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
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
	InviteCode       string                   `json:"invite_code,omitempty"`
	IsCaptain        bool                     `json:"is_captain"`
	CanManage        bool                     `json:"can_manage"`
	Recruiting       bool                     `json:"is_recruiting"`
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

type playTeamApplicationRequest struct {
	TeamID  int64  `json:"team_id" binding:"required,gt=0"`
	Message string `json:"message"`
}

type playTeamApplicationDecisionRequest struct {
	Decision string `json:"decision" binding:"required"`
	Note     string `json:"note"`
}

type playTeamRecruitingRequest struct {
	Recruiting *bool `json:"recruiting" binding:"required"`
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
	response.Success(c, toPlayBlindboxStatusDTO(status, true))
}

// toPlayBlindboxStatusDTO keeps authenticated responses reward-blind unless the
// service has explicitly established redeemable eligibility. Empty eligibility
// (for example while the feature is disabled or a dependency is unavailable)
// must fail closed instead of being treated as qualified.
func toPlayBlindboxStatusDTO(status *service.PlayBlindboxStatus, authenticated bool) playBlindboxStatusDTO {
	if status == nil {
		return playBlindboxStatusDTO{}
	}
	// The DTO must fail closed for both anonymous and authenticated callers.
	// Reward details are an account-scoped, redeemable-only contract.
	rewardDetailsVisible := authenticated && status.GrowthEligibility.RewardMode == service.PlayGrowthRewardRedeemable
	out := playBlindboxStatusDTO{
		Enabled:             status.Enabled,
		CouponPoolReady:     status.CouponPoolReady,
		DailyLimit:          status.DailyLimit,
		EffectiveLimit:      status.EffectiveLimit,
		OpensToday:          status.OpensToday,
		CanOpen:             status.CanOpen,
		ServerDate:          status.ServerDate,
		GrowthEligibility:   status.GrowthEligibility,
		RechargeBoostActive: status.RechargeBoostActive,
		CampaignActive:      status.CampaignActive,
	}
	if rewardDetailsVisible {
		out.CouponPrizes = status.CouponPrizes
		out.CouponWeightBP = status.CouponWeightBP
		out.BalanceWeightBP = status.BalanceWeightBP
		out.CostAmount = status.CostAmount
		out.Pool = toPlayBlindboxPoolDTOPtr(status.BlindboxPool)
		out.CurrentPool = toPlayBlindboxPoolDTOPtr(status.CurrentPool)
		out.NextPool = toOptionalPlayBlindboxPoolDTO(status.NextPool)
		out.VIPTier = status.VIPTier
		out.ExpectedReward = status.ExpectedReward
		out.NextExpectedReward = status.NextExpectedReward
		out.PoolVersion = status.PoolVersion
		out.RTPCap = status.RTPCap
	}
	return out
}

func (h *PlayHandler) BlindboxPool(c *gin.Context) {
	// The response varies by optional authentication. Prevent shared caches
	// from replaying a qualified account's detailed pool to an Explorer.
	c.Writer.Header().Add("Vary", "Authorization")
	c.Header("Cache-Control", "private, no-store")
	subject, authenticated := middleware.GetAuthSubjectFromContext(c)
	userID := int64(0)
	if authenticated {
		userID = subject.UserID
	}
	status, err := h.playService.GetBlindboxStatus(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// Anonymous requests and authenticated Explorer accounts must not receive
	// reward-pool internals. Qualified accounts may use the established detail
	// response, while the canonical authenticated status endpoint applies the
	// same qualification-aware DTO contract.
	if !authenticated || status.GrowthEligibility.RewardMode != service.PlayGrowthRewardRedeemable {
		response.Success(c, playBlindboxPublicPreviewDTO{
			Enabled:         status.Enabled,
			CouponPoolReady: status.CouponPoolReady,
		})
		return
	}
	response.Success(c, playBlindboxPoolResponseDTO{
		Enabled:            status.Enabled,
		CouponPoolReady:    status.CouponPoolReady,
		CouponPrizes:       status.CouponPrizes,
		CouponWeightBP:     status.CouponWeightBP,
		BalanceWeightBP:    status.BalanceWeightBP,
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
		RedeemCode:        toPlayRedeemCodeRewardDTO(result.RedeemCode),
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

func toPlayRedeemCodeRewardDTO(code *service.PlayRedeemCodeRewardSummary) *playRedeemCodeRewardDTO {
	if code == nil {
		return nil
	}
	return &playRedeemCodeRewardDTO{
		ID:                code.ID,
		Code:              code.Code,
		Type:              code.Type,
		Value:             code.Value,
		Status:            code.Status,
		BatchName:         code.BatchName,
		IssuedAt:          code.IssuedAt,
		ExpiresAt:         code.ExpiresAt,
		RewardPoolVersion: code.RewardPoolVersion,
	}
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
			User:       w.UserLabel,
			Reward:     w.RewardAmount,
			RewardType: w.RewardType,
			CouponName: w.CouponName,
			When:       w.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
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
		PreviousGrowthEnergy:      today.PreviousGrowthEnergy,
		PreviousCoupon:            toPlayCouponRewardDTO(today.PreviousCoupon),
		PreviousRedeemCode:        toPlayRedeemCodeRewardDTO(today.PreviousRedeemCode),
		PreviousCouponPoolVersion: today.PreviousCouponPoolVersion,
		RewardPerCorrect:          today.RewardPerCorrect,
		ServerDate:                today.ServerDate,
		GrowthEligibility:         today.GrowthEligibility,
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
		RedeemCode:        toPlayRedeemCodeRewardDTO(result.RedeemCode),
		CouponPoolVersion: result.CouponPoolVersion,
		ServerDate:        result.ServerDate,
		GrowthEnergy:      result.GrowthEnergy,
		GrowthEligibility: result.GrowthEligibility,
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
		out.Team = toPlayTeamSummaryDTOForActor(me.Team, subject.UserID)
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
	response.Success(c, toPlayTeamSummaryDTOForActor(team, subject.UserID))
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
	response.Success(c, toPlayTeamSummaryDTOForActor(team, subject.UserID))
}

func (h *PlayHandler) TeamAdmissionEligibility(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	eligibility, err := h.playService.GetTeamAdmissionEligibility(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, eligibility)
}

func (h *PlayHandler) TeamApply(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team join application request"))
		return
	}
	executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamApplicationCreate), req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.playService.ApplyToTeam(ctx, subject.UserID, service.PlayTeamJoinApplicationInput{
			TeamID:  req.TeamID,
			Message: req.Message,
		})
	})
}

func (h *PlayHandler) TeamMyApplications(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	applications, err := h.playService.ListMyTeamJoinApplications(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, applications)
}

func (h *PlayHandler) TeamApplicationWithdraw(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	applicationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || applicationID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team join application ID"))
		return
	}
	application, err := h.playService.WithdrawTeamJoinApplication(c.Request.Context(), subject.UserID, applicationID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, application)
}

func (h *PlayHandler) TeamCaptainApplications(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	applications, err := h.playService.ListCaptainTeamJoinApplications(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, applications)
}

func (h *PlayHandler) TeamApplicationDecision(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	applicationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || applicationID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team join application ID"))
		return
	}
	var req playTeamApplicationDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team join application decision request"))
		return
	}
	executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamApplicationDecide), struct {
		ApplicationID int64  `json:"application_id"`
		Decision      string `json:"decision"`
		Note          string `json:"note"`
	}{
		ApplicationID: applicationID,
		Decision:      req.Decision,
		Note:          req.Note,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.playService.DecideTeamJoinApplication(ctx, subject.UserID, applicationID, req.Decision, req.Note)
	})
}

func (h *PlayHandler) TeamInviteRotate(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamInviteRotate), struct{}{}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.playService.RotateTeamInvite(ctx, subject.UserID)
	})
}

func (h *PlayHandler) TeamRecruiting(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamRecruitingRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Recruiting == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team recruiting request"))
		return
	}
	executeUserIdempotentJSONOptionalKey(c, mobileUserIdempotencyScope(c, mobileOperationTeamRecruitingUpdate), struct {
		Recruiting bool `json:"recruiting"`
	}{Recruiting: *req.Recruiting}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.playService.SetTeamRecruiting(ctx, subject.UserID, *req.Recruiting); err != nil {
			return nil, err
		}
		return map[string]bool{"recruiting": *req.Recruiting}, nil
	})
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

func (h *PlayHandler) TeamLeaderboard(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	leaderboard, err := h.playService.TeamLeaderboard(c.Request.Context(), subject.UserID, 50)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, leaderboard)
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

func toPlayTeamSummaryDTOForActor(team *service.PlayTeamSummary, actorUserID int64) *playTeamSummaryDTO {
	if team == nil {
		return nil
	}
	isCaptain := actorUserID > 0 && team.CaptainID == actorUserID
	out := &playTeamSummaryDTO{
		ID:               team.ID,
		Name:             team.Name,
		IsCaptain:        isCaptain,
		CanManage:        isCaptain,
		Recruiting:       team.Recruiting,
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
	if isCaptain {
		out.InviteCode = team.InviteCode
	}
	for _, m := range team.Members {
		out.Members = append(out.Members, playTeamMemberDTO{
			DisplayName: m.DisplayName,
			AvatarURL:   m.AvatarURL,
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
