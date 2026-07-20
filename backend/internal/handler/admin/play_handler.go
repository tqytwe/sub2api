package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type adminArenaSettleRequest struct {
	PeriodID int64 `json:"period_id"`
}

type adminTeamMemberRepairRequest struct {
	UserID               int64      `json:"user_id"`
	Operation            string     `json:"operation"`
	EffectiveAt          *time.Time `json:"effective_at"`
	Reason               string     `json:"reason"`
	ExpectedSourceTeamID *int64     `json:"expected_source_team_id"`
}

type adminPlayCampaignRequest struct {
	Name    string                    `json:"name"`
	StartAt time.Time                 `json:"start_at"`
	EndAt   time.Time                 `json:"end_at"`
	Rules   service.PlayCampaignRules `json:"rules"`
	Enabled bool                      `json:"enabled"`
}

type adminPlayCampaignDTO struct {
	ID        int64                     `json:"id"`
	Name      string                    `json:"name"`
	StartAt   string                    `json:"start_at"`
	EndAt     string                    `json:"end_at"`
	Rules     service.PlayCampaignRules `json:"rules"`
	Enabled   bool                      `json:"enabled"`
	CreatedAt string                    `json:"created_at"`
}

type adminArenaSettleResultDTO struct {
	PeriodID     int64   `json:"period_id"`
	PeriodName   string  `json:"period_name"`
	WinnersCount int     `json:"winners_count"`
	TotalAwarded float64 `json:"total_awarded"`
}

type adminArenaLeaderboardDTO struct {
	Period  *adminArenaPeriodDTO              `json:"period,omitempty"`
	Rewards []service.PlayArenaSettlementTier `json:"rewards"`
	Rows    []adminArenaScoreDTO              `json:"rows"`
}

type adminArenaPeriodDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
	Status  string `json:"status"`
}

type adminArenaScoreDTO struct {
	Rank            int     `json:"rank"`
	UserID          int64   `json:"user_id"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email,omitempty"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	TokenSum        int64   `json:"token_sum"`
	EstimatedReward float64 `json:"estimated_reward"`
}

type adminTeamListDTO struct {
	Items    []adminTeamListItemDTO `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

type adminPlayOpsSummaryDTO struct {
	TotalTeams               int     `json:"total_teams"`
	ActiveTeams              int     `json:"active_teams"`
	MonthSpend               string  `json:"month_spend"`
	EstimatedSharedPool      string  `json:"estimated_shared_pool"`
	PendingFailedSettlements int     `json:"pending_failed_settlements"`
	MonthlyArenaRewardBudget float64 `json:"monthly_arena_reward_budget"`
	DailyArenaRewardBudget   float64 `json:"daily_arena_reward_budget"`
}

type adminTeamListItemDTO struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	InviteCode         string  `json:"invite_code"`
	CaptainID          int64   `json:"captain_id"`
	CaptainDisplayName string  `json:"captain_display_name"`
	CaptainAvatarURL   string  `json:"captain_avatar_url,omitempty"`
	CaptainEmail       string  `json:"captain_email,omitempty"`
	MemberCount        int     `json:"member_count"`
	TokenSum           int64   `json:"token_sum"`
	TeamSpend          string  `json:"team_spend"`
	EstimatedPool      string  `json:"estimated_pool"`
	CreatedAt          string  `json:"created_at"`
	ArchivedAt         *string `json:"archived_at,omitempty"`
}

type adminTeamDetailDTO struct {
	Team        *adminTeamSummaryDTO           `json:"team"`
	CreatedAt   string                         `json:"created_at"`
	ArchivedAt  *string                        `json:"archived_at,omitempty"`
	Settlements []adminTeamSettlementRecordDTO `json:"settlements"`
}

type adminTeamSummaryDTO struct {
	ID               int64                    `json:"id"`
	Name             string                   `json:"name"`
	InviteCode       string                   `json:"invite_code"`
	CaptainID        int64                    `json:"captain_id"`
	MemberCount      int                      `json:"member_count"`
	TokenSum         int64                    `json:"token_sum"`
	Members          []adminTeamMemberDTO     `json:"members"`
	CurrentMonth     string                   `json:"current_month"`
	TeamSpend        string                   `json:"team_spend"`
	ReachedThreshold string                   `json:"reached_threshold"`
	RewardRate       string                   `json:"reward_rate"`
	NextThreshold    string                   `json:"next_threshold"`
	EstimatedPool    string                   `json:"estimated_pool"`
	RewardCap        string                   `json:"reward_cap"`
	RewardTiers      []service.TeamRewardTier `json:"reward_tiers"`
}

type adminTeamMemberDTO struct {
	UserID          int64  `json:"user_id"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email,omitempty"`
	AvatarURL       string `json:"avatar_url,omitempty"`
	JoinedAt        string `json:"joined_at"`
	TokenSum        int64  `json:"token_sum"`
	TokenPct        int    `json:"token_pct"`
	Spend           string `json:"spend"`
	SpendPct        int    `json:"spend_pct"`
	EstimatedReward string `json:"estimated_reward"`
}

type adminTeamSettlementRecordDTO struct {
	Settlement  service.PlayTeamSettlement     `json:"settlement"`
	Allocations []adminTeamRewardAllocationDTO `json:"allocations"`
}

type adminTeamRewardAllocationDTO struct {
	ID           int64   `json:"id"`
	SettlementID int64   `json:"settlement_id"`
	UserID       int64   `json:"user_id"`
	DisplayName  string  `json:"display_name,omitempty"`
	AvatarURL    string  `json:"avatar_url,omitempty"`
	Email        string  `json:"email,omitempty"`
	Contribution string  `json:"contribution"`
	Ratio        string  `json:"ratio"`
	RewardAmount string  `json:"reward_amount"`
	PayoutStatus string  `json:"payout_status"`
	PaidAt       *string `json:"paid_at,omitempty"`
	LastError    string  `json:"last_error,omitempty"`
}

// AdminPlayHandler serves admin play operations.
type AdminPlayHandler struct {
	playService *service.PlayService
	totpService *service.TotpService
	userService *service.UserService
}

func NewAdminPlayHandler(
	playService *service.PlayService,
	totpService *service.TotpService,
	userService *service.UserService,
) *AdminPlayHandler {
	return &AdminPlayHandler{
		playService: playService,
		totpService: totpService,
		userService: userService,
	}
}

// GetBlindboxPool returns the effective editable blindbox pool.
// GET /api/v1/admin/play/blindbox/pool
func (h *AdminPlayHandler) GetBlindboxPool(c *gin.Context) {
	pool, err := h.playService.GetBlindboxPoolConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, pool)
}

// UpdateBlindboxPool validates and replaces the editable blindbox pool.
// PUT /api/v1/admin/play/blindbox/pool
func (h *AdminPlayHandler) UpdateBlindboxPool(c *gin.Context) {
	var pool service.PlayBlindboxPool
	if err := c.ShouldBindJSON(&pool); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid blindbox pool request"))
		return
	}
	updated, err := h.playService.UpdateBlindboxPoolConfig(c.Request.Context(), pool)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

// ArenaSettle settles an arena period and distributes rank rewards.
// POST /api/v1/admin/play/arena/settle
func (h *AdminPlayHandler) ArenaSettle(c *gin.Context) {
	var req adminArenaSettleRequest
	_ = c.ShouldBindJSON(&req)
	result, err := h.playService.SettleArenaPeriod(c.Request.Context(), req.PeriodID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminArenaSettleResultDTO{
		PeriodID:     result.PeriodID,
		PeriodName:   result.PeriodName,
		WinnersCount: result.WinnersCount,
		TotalAwarded: result.TotalAwarded,
	})
}

func (h *AdminPlayHandler) ArenaLeaderboard(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 50)
	periodID := parsePositiveInt64(c.Query("period_id"))
	periodType := strings.ToLower(strings.TrimSpace(c.Query("period_type")))
	rows, period, rewards, err := h.playService.ListAdminArenaLeaderboard(c.Request.Context(), periodType, periodID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := adminArenaLeaderboardDTO{
		Rewards: rewards,
		Rows:    make([]adminArenaScoreDTO, 0, len(rows)),
	}
	if period != nil {
		out.Period = &adminArenaPeriodDTO{
			ID:      period.ID,
			Name:    period.Name,
			StartAt: period.StartAt.Format("2006-01-02T15:04:05Z07:00"),
			EndAt:   period.EndAt.Format("2006-01-02T15:04:05Z07:00"),
			Status:  period.Status,
		}
	}
	for _, row := range rows {
		out.Rows = append(out.Rows, adminArenaScoreDTO{
			Rank:            row.Rank,
			UserID:          row.UserID,
			DisplayName:     row.DisplayName,
			Email:           row.Email,
			AvatarURL:       row.AvatarURL,
			TokenSum:        row.TokenSum,
			EstimatedReward: arenaRewardForRankAdmin(row.Rank, rewards),
		})
	}
	response.Success(c, out)
}

func (h *AdminPlayHandler) ListCampaigns(c *gin.Context) {
	campaigns, err := h.playService.ListAdminCampaigns(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminPlayCampaignDTOs(campaigns))
}

func (h *AdminPlayHandler) CreateCampaign(c *gin.Context) {
	var req adminPlayCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid campaign request"))
		return
	}
	created, err := h.playService.CreateAdminCampaign(c.Request.Context(), service.PlayCampaign{
		Name:    req.Name,
		StartAt: req.StartAt,
		EndAt:   req.EndAt,
		Rules:   req.Rules,
		Enabled: req.Enabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminPlayCampaignDTO(*created))
}

func (h *AdminPlayHandler) UpdateCampaign(c *gin.Context) {
	id := parsePositiveInt64(c.Param("id"))
	if id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid campaign id"))
		return
	}
	var req adminPlayCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid campaign request"))
		return
	}
	updated, err := h.playService.UpdateAdminCampaign(c.Request.Context(), service.PlayCampaign{
		ID:      id,
		Name:    req.Name,
		StartAt: req.StartAt,
		EndAt:   req.EndAt,
		Rules:   req.Rules,
		Enabled: req.Enabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminPlayCampaignDTO(*updated))
}

func (h *AdminPlayHandler) DeleteCampaign(c *gin.Context) {
	id := parsePositiveInt64(c.Param("id"))
	if id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid campaign id"))
		return
	}
	if err := h.playService.DeleteAdminCampaign(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *AdminPlayHandler) GetTeamRewardSettings(c *gin.Context) {
	response.Success(c, h.playService.GetTeamRewardSettings(c.Request.Context()))
}

func (h *AdminPlayHandler) UpdateTeamRewardSettings(c *gin.Context) {
	var settings service.PlayTeamRewardSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team reward settings request"))
		return
	}
	updated, err := h.playService.UpdateTeamRewardSettings(c.Request.Context(), settings)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *AdminPlayHandler) ListTeamRewardSettlements(c *gin.Context) {
	records, err := h.playService.ListAdminTeamRewardSettlements(c.Request.Context(), 100)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminTeamSettlementRecordDTOs(records))
}

func (h *AdminPlayHandler) ListTeams(c *gin.Context) {
	page := parsePositiveInt(c.Query("page"), 1)
	pageSize := parsePositiveInt(c.Query("page_size"), 20)
	list, err := h.playService.ListAdminTeams(c.Request.Context(), c.Query("status"), c.Query("q"), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, toAdminTeamListDTO(list))
}

func (h *AdminPlayHandler) Summary(c *gin.Context) {
	summary, err := h.playService.GetAdminOpsSummary(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminPlayOpsSummaryDTO{
		TotalTeams:               summary.TotalTeams,
		ActiveTeams:              summary.ActiveTeams,
		MonthSpend:               summary.MonthSpend.StringFixed(8),
		EstimatedSharedPool:      summary.EstimatedSharedPool.StringFixed(8),
		PendingFailedSettlements: summary.PendingFailedSettlements,
		MonthlyArenaRewardBudget: summary.MonthlyArenaRewardBudget,
		DailyArenaRewardBudget:   summary.DailyArenaRewardBudget,
	})
}

func (h *AdminPlayHandler) GetTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || teamID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team id"))
		return
	}
	detail, err := h.playService.GetAdminTeamDetail(c.Request.Context(), teamID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if detail == nil {
		response.ErrorFrom(c, infraerrors.NotFound("PLAY_TEAM_NOT_FOUND", "team not found"))
		return
	}
	response.Success(c, toAdminTeamDetailDTO(detail))
}

func (h *AdminPlayHandler) GetTeamSettlements(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || teamID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team id"))
		return
	}
	detail, err := h.playService.GetAdminTeamDetail(c.Request.Context(), teamID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if detail == nil {
		response.ErrorFrom(c, infraerrors.NotFound("PLAY_TEAM_NOT_FOUND", "team not found"))
		return
	}
	response.Success(c, toAdminTeamSettlementRecordDTOs(detail.Settlements))
}

func (h *AdminPlayHandler) ListTeamMemberCandidates(c *gin.Context) {
	teamID := parsePositiveInt64(c.Param("id"))
	if teamID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team id"))
		return
	}
	var effectiveAt *time.Time
	if raw := strings.TrimSpace(c.Query("effective_at")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("PLAY_TEAM_EFFECTIVE_AT_INVALID", "invalid effective time"))
			return
		}
		effectiveAt = &parsed
	}
	result, err := h.playService.ListAdminTeamMemberCandidates(c.Request.Context(), service.PlayAdminTeamMemberCandidateQuery{
		TargetTeamID: teamID,
		Query:        c.Query("q"),
		Operation:    c.Query("operation"),
		EffectiveAt:  effectiveAt,
		Limit:        parsePositiveInt(c.Query("limit"), 20),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AdminPlayHandler) AddOrMoveTeamMember(c *gin.Context) {
	teamID := parsePositiveInt64(c.Param("id"))
	if teamID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team id"))
		return
	}
	idempotencyKey, err := service.NormalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if idempotencyKey == "" {
		response.ErrorFrom(c, infraerrors.BadRequest("IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required"))
		return
	}
	c.Request.Header.Set("Idempotency-Key", idempotencyKey)
	var req adminTeamMemberRepairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team member repair request"))
		return
	}
	req.Operation = strings.ToLower(strings.TrimSpace(req.Operation))
	req.Reason = strings.TrimSpace(req.Reason)
	if req.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid user id"))
		return
	}
	if req.Operation != service.AdminTeamMemberOperationAdd && req.Operation != service.AdminTeamMemberOperationMove {
		response.ErrorFrom(c, service.ErrPlayAdminTeamInvalidOperation)
		return
	}
	reasonLen := len([]rune(req.Reason))
	if reasonLen < 10 || reasonLen > 500 {
		response.ErrorFrom(c, service.ErrPlayAdminTeamReasonInvalid)
		return
	}
	middleware.SetAuditAction(c, adminTeamMemberAuditAction(req.Operation))
	auditExtra := map[string]any{
		"target_team_id": teamID,
		"target_user_id": req.UserID,
		"operation":      req.Operation,
		"reason_code":    service.PlayTeamEventReasonAdminManualMembershipRepair,
	}
	if req.ExpectedSourceTeamID != nil && *req.ExpectedSourceTeamID > 0 {
		auditExtra["source_team_id"] = *req.ExpectedSourceTeamID
	}
	if req.EffectiveAt != nil {
		auditExtra["effective_at"] = req.EffectiveAt.Format(time.RFC3339Nano)
	}
	middleware.SetAuditExtra(c, auditExtra)
	if req.Operation == service.AdminTeamMemberOperationMove || req.EffectiveAt != nil {
		if c.GetString("auth_method") == service.AuditAuthMethodAdminAPIKey {
			_ = middleware.EnforceStepUpAlways(c, h.totpService, h.userService)
			return
		}
		if h.totpService == nil || h.userService == nil {
			response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "Authorization required"))
			return
		}
		if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
			return
		}
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "Authorization required"))
		return
	}

	input := service.AdminTeamMemberRepairInput{
		TargetTeamID:         teamID,
		UserID:               req.UserID,
		ActorUserID:          subject.UserID,
		Operation:            req.Operation,
		EffectiveAt:          req.EffectiveAt,
		Reason:               req.Reason,
		ExpectedSourceTeamID: req.ExpectedSourceTeamID,
	}
	executeAdminIdempotentJSON(
		c,
		fmt.Sprintf("admin:play:team-member-repair:%d", teamID),
		input,
		24*time.Hour,
		func(ctx context.Context) (any, error) {
			return h.playService.RepairAdminTeamMember(ctx, input)
		},
	)
}

func adminTeamMemberAuditAction(operation string) string {
	if operation == service.AdminTeamMemberOperationMove {
		return service.AuditActionAdminPlayTeamMemberMove
	}
	return service.AuditActionAdminPlayTeamMemberAdd
}

func (h *AdminPlayHandler) ListTeamEvents(c *gin.Context) {
	teamID := parsePositiveInt64(c.Param("id"))
	if teamID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid team id"))
		return
	}
	events, err := h.playService.ListAdminTeamEvents(
		c.Request.Context(),
		teamID,
		parsePositiveInt(c.Query("limit"), 100),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, events)
}

func (h *AdminPlayHandler) RetryTeamRewardSettlement(c *gin.Context) {
	settlementID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || settlementID <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_REQUEST", "invalid settlement id"))
		return
	}
	settlement, err := h.playService.PayoutTeamRewardSettlement(c.Request.Context(), settlementID)
	if err != nil && settlement == nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settlement)
}

func parsePositiveInt(raw string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return n
	}
	return fallback
}

func parsePositiveInt64(raw string) int64 {
	if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && n > 0 {
		return n
	}
	return 0
}

func arenaRewardForRankAdmin(rank int, tiers []service.PlayArenaSettlementTier) float64 {
	for _, tier := range tiers {
		if rank <= tier.RankMax {
			return tier.Amount
		}
	}
	return 0
}

func toAdminTeamListDTO(list *service.PlayAdminTeamList) adminTeamListDTO {
	out := adminTeamListDTO{
		Total:    list.Total,
		Page:     list.Page,
		PageSize: list.PageSize,
		Items:    make([]adminTeamListItemDTO, 0, len(list.Items)),
	}
	for _, item := range list.Items {
		out.Items = append(out.Items, toAdminTeamListItemDTO(item))
	}
	return out
}

func toAdminPlayCampaignDTOs(items []service.PlayCampaign) []adminPlayCampaignDTO {
	out := make([]adminPlayCampaignDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toAdminPlayCampaignDTO(item))
	}
	return out
}

func toAdminPlayCampaignDTO(item service.PlayCampaign) adminPlayCampaignDTO {
	return adminPlayCampaignDTO{
		ID:        item.ID,
		Name:      item.Name,
		StartAt:   item.StartAt.Format("2006-01-02T15:04:05Z07:00"),
		EndAt:     item.EndAt.Format("2006-01-02T15:04:05Z07:00"),
		Rules:     item.Rules,
		Enabled:   item.Enabled,
		CreatedAt: item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func toAdminTeamListItemDTO(item service.PlayAdminTeamListItem) adminTeamListItemDTO {
	out := adminTeamListItemDTO{
		ID:                 item.ID,
		Name:               item.Name,
		InviteCode:         item.InviteCode,
		CaptainID:          item.CaptainID,
		CaptainDisplayName: item.CaptainDisplayName,
		CaptainAvatarURL:   item.CaptainAvatarURL,
		CaptainEmail:       item.CaptainEmail,
		MemberCount:        item.MemberCount,
		TokenSum:           item.TokenSum,
		TeamSpend:          item.TeamSpend.StringFixed(8),
		EstimatedPool:      item.EstimatedPool.StringFixed(8),
		CreatedAt:          item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if item.ArchivedAt != nil {
		value := item.ArchivedAt.Format("2006-01-02T15:04:05Z07:00")
		out.ArchivedAt = &value
	}
	return out
}

func toAdminTeamDetailDTO(detail *service.PlayAdminTeamDetail) adminTeamDetailDTO {
	out := adminTeamDetailDTO{
		Team:        toAdminTeamSummaryDTO(detail.Team),
		CreatedAt:   detail.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Settlements: toAdminTeamSettlementRecordDTOs(detail.Settlements),
	}
	if detail.ArchivedAt != nil {
		value := detail.ArchivedAt.Format("2006-01-02T15:04:05Z07:00")
		out.ArchivedAt = &value
	}
	return out
}

func toAdminTeamSettlementRecordDTOs(records []service.PlayTeamSettlementRecord) []adminTeamSettlementRecordDTO {
	out := make([]adminTeamSettlementRecordDTO, 0, len(records))
	for _, record := range records {
		dto := adminTeamSettlementRecordDTO{
			Settlement:  record.Settlement,
			Allocations: make([]adminTeamRewardAllocationDTO, 0, len(record.Allocations)),
		}
		for _, allocation := range record.Allocations {
			var paidAt *string
			if allocation.PaidAt != nil {
				value := allocation.PaidAt.Format("2006-01-02T15:04:05Z07:00")
				paidAt = &value
			}
			dto.Allocations = append(dto.Allocations, adminTeamRewardAllocationDTO{
				ID:           allocation.ID,
				SettlementID: allocation.SettlementID,
				UserID:       allocation.UserID,
				DisplayName:  allocation.DisplayName,
				AvatarURL:    allocation.AvatarURL,
				Email:        allocation.Email,
				Contribution: allocation.Contribution.StringFixed(8),
				Ratio:        allocation.Ratio.StringFixed(8),
				RewardAmount: allocation.RewardAmount.StringFixed(8),
				PayoutStatus: allocation.PayoutStatus,
				PaidAt:       paidAt,
				LastError:    allocation.LastError,
			})
		}
		out = append(out, dto)
	}
	return out
}

func toAdminTeamSummaryDTO(team *service.PlayTeamSummary) *adminTeamSummaryDTO {
	if team == nil {
		return nil
	}
	out := &adminTeamSummaryDTO{
		ID:               team.ID,
		Name:             team.Name,
		InviteCode:       team.InviteCode,
		CaptainID:        team.CaptainID,
		MemberCount:      team.MemberCount,
		TokenSum:         team.TokenSum,
		Members:          make([]adminTeamMemberDTO, 0, len(team.Members)),
		CurrentMonth:     team.CurrentMonth,
		TeamSpend:        team.TeamSpend.StringFixed(8),
		ReachedThreshold: team.ReachedThreshold.StringFixed(8),
		RewardRate:       team.RewardRate.StringFixed(8),
		NextThreshold:    team.NextThreshold.StringFixed(8),
		EstimatedPool:    team.EstimatedPool.StringFixed(8),
		RewardCap:        team.RewardCap.StringFixed(8),
		RewardTiers:      team.RewardTiers,
	}
	for _, member := range team.Members {
		out.Members = append(out.Members, adminTeamMemberDTO{
			UserID:          member.UserID,
			DisplayName:     member.DisplayName,
			Email:           member.Email,
			AvatarURL:       member.AvatarURL,
			JoinedAt:        member.JoinedAt.Format("2006-01-02T15:04:05Z07:00"),
			TokenSum:        member.TokenSum,
			TokenPct:        member.TokenPct,
			Spend:           member.Spend.StringFixed(8),
			SpendPct:        member.SpendPct,
			EstimatedReward: member.EstimatedReward.StringFixed(8),
		})
	}
	return out
}
