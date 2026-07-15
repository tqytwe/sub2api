package handler

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type playBlindboxStatusDTO struct {
	Enabled             bool                     `json:"enabled"`
	CostAmount          float64                  `json:"cost_amount"`
	DailyLimit          int                      `json:"daily_limit"`
	EffectiveLimit      int                      `json:"effective_limit,omitempty"`
	OpensToday          int                      `json:"opens_today"`
	CanOpen             bool                     `json:"can_open"`
	ServerDate          string                   `json:"server_date"`
	RechargeBoostActive bool                     `json:"recharge_boost_active,omitempty"`
	CampaignActive      bool                     `json:"campaign_active,omitempty"`
	PaidEnabled         bool                     `json:"paid_enabled"`
	RegionEnabled       bool                     `json:"region_enabled"`
	TicketBalance       int                      `json:"ticket_balance"`
	Pool                service.PlayBlindboxPool `json:"pool"`
}

type playBlindboxOpenResultDTO struct {
	CostAmount   float64 `json:"cost_amount"`
	RewardAmount float64 `json:"reward_amount"`
	NetAmount    float64 `json:"net_amount"`
	OpensToday   int     `json:"opens_today"`
	ServerDate   string  `json:"server_date"`
	PoolVersion  string  `json:"pool_version"`
	OpenSource   string  `json:"open_source"`
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
	UserID       int64  `json:"user_id"`
	DisplayName  string `json:"display_name"`
	AvatarURL    string `json:"avatar_url,omitempty"`
	JoinedAt     string `json:"joined_at"`
	TokenSum     int64  `json:"token_sum"`
	TokenPct     int    `json:"token_pct"`
	RequestCount int64  `json:"request_count"`
	ActiveDays   int    `json:"active_days"`
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
	ID           int64                           `json:"id"`
	Name         string                          `json:"name"`
	InviteCode   string                          `json:"invite_code"`
	CaptainID    int64                           `json:"captain_id"`
	MemberCount  int                             `json:"member_count"`
	TokenSum     int64                           `json:"token_sum"`
	Members      []playTeamMemberDTO             `json:"members"`
	Affiliate    *playTeamAffiliateDTO           `json:"affiliate,omitempty"`
	RequestCount int64                           `json:"request_count"`
	ActiveDays   int                             `json:"active_days"`
	Level        int                             `json:"level"`
	MaxMembers   int                             `json:"max_members"`
	IsPublic     bool                            `json:"is_public"`
	Weekly       *service.PlayTeamWeeklyProgress `json:"weekly,omitempty"`
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

type playTeamReviewRequest struct {
	RequestID int64 `json:"request_id"`
	Approve   bool  `json:"approve"`
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
		PaidEnabled:         status.PaidEnabled,
		RegionEnabled:       status.RegionEnabled,
		TicketBalance:       status.TicketBalance,
		Pool:                status.Pool,
	})
}

func (h *PlayHandler) BlindboxPool(c *gin.Context) {
	status, err := h.playService.GetBlindboxStatus(c.Request.Context(), 0)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playBlindboxStatusDTO{
		Enabled: status.Enabled, CostAmount: status.CostAmount, DailyLimit: status.DailyLimit,
		PaidEnabled: status.PaidEnabled, RegionEnabled: status.RegionEnabled, Pool: status.Pool,
		ServerDate: status.ServerDate,
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
		PoolVersion:  result.PoolVersion,
		OpenSource:   result.OpenSource,
	})
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
	language := c.GetHeader("Accept-Language")
	result, err := h.playService.SubmitQuiz(c.Request.Context(), subject.UserID, language, answers)
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

func (h *PlayHandler) TeamDiscover(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	teams, err := h.playService.DiscoverTeams(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, teams)
}

func (h *PlayHandler) TeamRequestJoin(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		TeamID int64 `json:"team_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TeamID <= 0 {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.RequestTeamJoin(c.Request.Context(), subject.UserID, req.TeamID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *PlayHandler) TeamJoinRequests(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	items, err := h.playService.ListTeamJoinRequests(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *PlayHandler) TeamReviewRequest(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req playTeamReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RequestID <= 0 {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.ReviewTeamJoinRequest(c.Request.Context(), subject.UserID, req.RequestID, req.Approve); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
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
	response.Success(c, gin.H{"ok": true})
}

func (h *PlayHandler) TeamTransfer(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		UserID int64 `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.TransferTeamCaptain(c.Request.Context(), subject.UserID, req.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *PlayHandler) TeamRemoveMember(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req struct {
		UserID int64 `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.playService.RemoveTeamMember(c.Request.Context(), subject.UserID, req.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *PlayHandler) TeamActivity(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := strconv.ParseInt(c.Query("limit"), 10, 64)
	items, err := h.playService.ListMyTeamActivity(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func toPlayTeamSummaryDTO(team *service.PlayTeamSummary) *playTeamSummaryDTO {
	if team == nil {
		return nil
	}
	out := &playTeamSummaryDTO{
		ID:           team.ID,
		Name:         team.Name,
		InviteCode:   team.InviteCode,
		CaptainID:    team.CaptainID,
		MemberCount:  team.MemberCount,
		TokenSum:     team.TokenSum,
		Members:      make([]playTeamMemberDTO, 0, len(team.Members)),
		RequestCount: team.RequestCount,
		ActiveDays:   team.ActiveDays,
		Level:        team.Level,
		MaxMembers:   team.MaxMembers,
		IsPublic:     team.IsPublic,
		Weekly:       team.Weekly,
	}
	for _, m := range team.Members {
		out.Members = append(out.Members, playTeamMemberDTO{
			UserID:       m.UserID,
			DisplayName:  m.DisplayName,
			AvatarURL:    m.AvatarURL,
			JoinedAt:     m.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
			TokenSum:     m.TokenSum,
			TokenPct:     m.TokenPct,
			RequestCount: m.RequestCount,
			ActiveDays:   m.ActiveDays,
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
