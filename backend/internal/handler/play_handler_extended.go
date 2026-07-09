package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playBlindboxStatusDTO struct {
	Enabled             bool    `json:"enabled"`
	CostAmount          float64 `json:"cost_amount"`
	DailyLimit          int     `json:"daily_limit"`
	EffectiveLimit      int     `json:"effective_limit,omitempty"`
	OpensToday          int     `json:"opens_today"`
	CanOpen             bool    `json:"can_open"`
	ServerDate          string  `json:"server_date"`
	RechargeBoostActive bool    `json:"recharge_boost_active,omitempty"`
	CampaignActive      bool    `json:"campaign_active,omitempty"`
}

type playBlindboxOpenResultDTO struct {
	CostAmount   float64 `json:"cost_amount"`
	RewardAmount float64 `json:"reward_amount"`
	NetAmount    float64 `json:"net_amount"`
	OpensToday   int     `json:"opens_today"`
	ServerDate   string  `json:"server_date"`
}

type playQuizQuestionDTO struct {
	ID      int64    `json:"id"`
	Prompt  string   `json:"prompt"`
	Options []string `json:"options"`
}

type playQuizTodayDTO struct {
	Enabled          bool                  `json:"enabled"`
	Questions        []playQuizQuestionDTO `json:"questions"`
	AlreadySubmitted bool                  `json:"already_submitted"`
	PreviousScore    int                   `json:"previous_score,omitempty"`
	PreviousTotal    int                   `json:"previous_total,omitempty"`
	PreviousReward   float64               `json:"previous_reward,omitempty"`
	RewardPerCorrect float64               `json:"reward_per_correct"`
	ServerDate       string                `json:"server_date"`
}

type playQuizSubmitRequest struct {
	Answers []playQuizAnswerDTO `json:"answers"`
}

type playQuizAnswerDTO struct {
	QuestionID  int64 `json:"question_id"`
	ChoiceIndex int   `json:"choice_index"`
}

type playQuizSubmitResultDTO struct {
	Score        int     `json:"score"`
	Total        int     `json:"total"`
	RewardAmount float64 `json:"reward_amount"`
	ServerDate   string  `json:"server_date"`
}

type playTeamMemberDTO struct {
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	JoinedAt    string `json:"joined_at"`
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
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	InviteCode  string                `json:"invite_code"`
	CaptainID   int64                 `json:"captain_id"`
	MemberCount int                   `json:"member_count"`
	TokenSum    int64                 `json:"token_sum"`
	Members     []playTeamMemberDTO   `json:"members"`
	Affiliate   *playTeamAffiliateDTO `json:"affiliate,omitempty"`
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
		CostAmount:          status.CostAmount,
		DailyLimit:          status.DailyLimit,
		EffectiveLimit:      status.EffectiveLimit,
		OpensToday:          status.OpensToday,
		CanOpen:             status.CanOpen,
		ServerDate:          status.ServerDate,
		RechargeBoostActive: status.RechargeBoostActive,
		CampaignActive:      status.CampaignActive,
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
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playBlindboxOpenResultDTO{
		CostAmount:   result.CostAmount,
		RewardAmount: result.RewardAmount,
		NetAmount:    result.NetAmount,
		OpensToday:   result.OpensToday,
		ServerDate:   result.ServerDate,
	})
}

func (h *PlayHandler) QuizToday(c *gin.Context) {
	var userID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	today, err := h.playService.GetQuizToday(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playQuizTodayDTO{
		Enabled:          today.Enabled,
		Questions:        make([]playQuizQuestionDTO, 0, len(today.Questions)),
		AlreadySubmitted: today.AlreadySubmitted,
		PreviousScore:    today.PreviousScore,
		PreviousTotal:    today.PreviousTotal,
		PreviousReward:   today.PreviousReward,
		RewardPerCorrect: today.RewardPerCorrect,
		ServerDate:       today.ServerDate,
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
	result, err := h.playService.SubmitQuiz(c.Request.Context(), subject.UserID, answers)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playQuizSubmitResultDTO{
		Score:        result.Score,
		Total:        result.Total,
		RewardAmount: result.RewardAmount,
		ServerDate:   result.ServerDate,
	})
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

func toPlayTeamSummaryDTO(team *service.PlayTeamSummary) *playTeamSummaryDTO {
	if team == nil {
		return nil
	}
	out := &playTeamSummaryDTO{
		ID:          team.ID,
		Name:        team.Name,
		InviteCode:  team.InviteCode,
		CaptainID:   team.CaptainID,
		MemberCount: team.MemberCount,
		TokenSum:    team.TokenSum,
		Members:     make([]playTeamMemberDTO, 0, len(team.Members)),
	}
	for _, m := range team.Members {
		out.Members = append(out.Members, playTeamMemberDTO{
			UserID:      m.UserID,
			DisplayName: m.DisplayName,
			AvatarURL:   m.AvatarURL,
			JoinedAt:    m.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
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
