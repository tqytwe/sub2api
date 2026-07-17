package handler

import (
	"context"
	"sort"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PlayHandler serves play/engagement endpoints (check-in, arena, public models).
type PlayHandler struct {
	playService    *service.PlayService
	billingService *service.BillingService
}

func NewPlayHandler(playService *service.PlayService, billingService *service.BillingService) *PlayHandler {
	return &PlayHandler{playService: playService, billingService: billingService}
}

type playCheckinStatusDTO struct {
	Enabled                bool    `json:"enabled"`
	CheckedInToday         bool    `json:"checked_in_today"`
	RewardAmount           float64 `json:"reward_amount"`
	ServerDate             string  `json:"server_date"`
	StreakCount            int     `json:"streak_count,omitempty"`
	NextMilestoneDays      int     `json:"next_milestone_days,omitempty"`
	NextMilestoneBonus     float64 `json:"next_milestone_bonus,omitempty"`
	CanMakeup              bool    `json:"can_makeup,omitempty"`
	MakeupDate             string  `json:"makeup_date,omitempty"`
	RechargeBoostActive    bool    `json:"recharge_boost_active,omitempty"`
	BoostCheckinMultiplier float64 `json:"boost_checkin_multiplier,omitempty"`
}

type playCheckinResultDTO struct {
	RewardAmount   float64 `json:"reward_amount"`
	BalanceAdded   float64 `json:"balance_added"`
	ServerDate     string  `json:"server_date"`
	StreakCount    int     `json:"streak_count,omitempty"`
	MilestoneBonus float64 `json:"milestone_bonus,omitempty"`
}

type playArenaPeriodDTO struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
	Status  string `json:"status"`
}

type playArenaCurrentDTO struct {
	Enabled              bool                `json:"enabled"`
	Period               *playArenaPeriodDTO `json:"period,omitempty"`
	TokenSum             int64               `json:"token_sum,omitempty"`
	DisplayTokenSum      int64               `json:"display_token_sum,omitempty"`
	Rank                 int                 `json:"rank,omitempty"`
	TokensToPrevRank     int64               `json:"tokens_to_prev_rank,omitempty"`
	EstimatedReward      float64             `json:"estimated_reward,omitempty"`
	RechargeBoostActive  bool                `json:"recharge_boost_active,omitempty"`
	ArenaScoreMultiplier float64             `json:"arena_score_multiplier,omitempty"`
	CampaignActive       bool                `json:"campaign_active,omitempty"`
}

type playArenaScoreDTO struct {
	Rank        int    `json:"rank"`
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	TokenSum    int64  `json:"token_sum"`
}

type playArenaLeaderboardDTO struct {
	Enabled bool                `json:"enabled"`
	Period  *playArenaPeriodDTO `json:"period,omitempty"`
	Rows    []playArenaScoreDTO `json:"rows"`
}

type publicModelPlatformSection struct {
	Platform        string               `json:"platform"`
	SupportedModels []userSupportedModel `json:"supported_models"`
}

type publicModelChannel struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Platforms   []publicModelPlatformSection `json:"platforms"`
}

// PublicModels lists official reference pricing for guests (no group multipliers).
// GET /api/v1/public/models
func (h *PlayHandler) PublicModels(c *gin.Context) {
	channels, err := h.playService.ListPublicModels(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]publicModelChannel, 0, len(channels))
	for _, ch := range channels {
		sections := buildPublicPlatformSections(ch)
		if len(sections) == 0 {
			continue
		}
		out = append(out, publicModelChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}
	response.Success(c, out)
}

// PublicModelPricing lists official catalog prices and site reference prices for /models.
// GET /api/v1/public/model-pricing
func (h *PlayHandler) PublicModelPricing(c *gin.Context) {
	if h.playService == nil || h.billingService == nil {
		response.Success(c, []service.PublicModelPricingRow{})
		return
	}
	rows := h.playService.ListPublicModelPricing(c.Request.Context(), h.billingService)
	response.Success(c, rows)
}

// CheckinStatus returns today's check-in state for the current user.
// GET /api/v1/play/checkin/status
func (h *PlayHandler) CheckinStatus(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	status, err := h.playService.GetCheckinStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playCheckinStatusDTO{
		Enabled:                status.Enabled,
		CheckedInToday:         status.CheckedInToday,
		RewardAmount:           status.RewardAmount,
		ServerDate:             status.ServerDate,
		StreakCount:            status.StreakCount,
		NextMilestoneDays:      status.NextMilestoneDays,
		NextMilestoneBonus:     status.NextMilestoneBonus,
		CanMakeup:              status.CanMakeup,
		MakeupDate:             status.MakeupDate,
		RechargeBoostActive:    status.RechargeBoostActive,
		BoostCheckinMultiplier: status.BoostCheckinMultiplier,
	})
}

// CheckinMakeup backfills yesterday's missed check-in after a recent recharge.
// POST /api/v1/play/checkin/makeup
func (h *PlayHandler) CheckinMakeup(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.playService.CheckinMakeup(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playCheckinResultDTO{
		RewardAmount:   result.RewardAmount,
		BalanceAdded:   result.BalanceAdded,
		ServerDate:     result.ServerDate,
		StreakCount:    result.StreakCount,
		MilestoneBonus: result.MilestoneBonus,
	})
}

// Checkin grants the daily balance reward.
// POST /api/v1/play/checkin
func (h *PlayHandler) Checkin(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	result, err := h.playService.Checkin(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, playCheckinResultDTO{
		RewardAmount:   result.RewardAmount,
		BalanceAdded:   result.BalanceAdded,
		ServerDate:     result.ServerDate,
		StreakCount:    result.StreakCount,
		MilestoneBonus: result.MilestoneBonus,
	})
}

// ArenaCurrent returns the active arena period and optional user score.
// GET /api/v1/play/arena/current
func (h *PlayHandler) ArenaCurrent(c *gin.Context) {
	var userID int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}

	current, err := h.playService.GetArenaCurrent(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := playArenaCurrentDTO{
		Enabled:              current.Enabled,
		TokenSum:             current.TokenSum,
		DisplayTokenSum:      current.DisplayTokenSum,
		Rank:                 current.Rank,
		TokensToPrevRank:     current.TokensToPrevRank,
		EstimatedReward:      current.EstimatedReward,
		RechargeBoostActive:  current.RechargeBoostActive,
		ArenaScoreMultiplier: current.ArenaScoreMultiplier,
		CampaignActive:       current.CampaignActive,
	}
	if current.Period != nil {
		out.Period = toPlayArenaPeriodDTO(current.Period)
	}
	response.Success(c, out)
}

// ArenaLeaderboard returns ranked token usage for the active period.
// GET /api/v1/play/arena/leaderboard
func (h *PlayHandler) ArenaLeaderboard(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}

	rows, period, err := h.playService.ListArenaLeaderboard(c.Request.Context(), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rt := h.playService.GetRuntime(c.Request.Context())
	out := playArenaLeaderboardDTO{
		Enabled: rt.ArenaEnabled,
		Rows:    make([]playArenaScoreDTO, 0, len(rows)),
	}
	if period != nil {
		out.Period = toPlayArenaPeriodDTO(period)
	}
	for _, row := range rows {
		out.Rows = append(out.Rows, playArenaScoreDTO{
			Rank:        row.Rank,
			UserID:      row.UserID,
			DisplayName: row.DisplayName,
			AvatarURL:   row.AvatarURL,
			TokenSum:    row.TokenSum,
		})
	}
	response.Success(c, out)
}

func toPlayArenaPeriodDTO(p *service.PlayArenaPeriod) *playArenaPeriodDTO {
	if p == nil {
		return nil
	}
	return &playArenaPeriodDTO{
		ID:      p.ID,
		Name:    p.Name,
		StartAt: p.StartAt.Format("2006-01-02T15:04:05Z07:00"),
		EndAt:   p.EndAt.Format("2006-01-02T15:04:05Z07:00"),
		Status:  p.Status,
	}
}

func buildPublicPlatformSections(ch service.AvailableChannel) []publicModelPlatformSection {
	platformSet := make(map[string]struct{}, 4)
	for _, m := range ch.SupportedModels {
		if m.Platform == "" {
			continue
		}
		platformSet[m.Platform] = struct{}{}
	}
	if len(platformSet) == 0 {
		return nil
	}

	platforms := make([]string, 0, len(platformSet))
	for p := range platformSet {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	sections := make([]publicModelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		platformFilter := map[string]struct{}{platform: {}}
		models := toUserSupportedModels(ch.SupportedModels, platformFilter)
		if len(models) == 0 {
			continue
		}
		sections = append(sections, publicModelPlatformSection{
			Platform:        platform,
			SupportedModels: models,
		})
	}
	return sections
}

func countPublicModels(channels []service.AvailableChannel) int { //nolint:unused // Used by unit-tagged regression tests.
	seen := make(map[string]struct{})
	for _, ch := range channels {
		for _, model := range ch.SupportedModels {
			if model.Name == "" {
				continue
			}
			platform := model.Platform
			if platform == "" {
				platform = "_"
			}
			key := model.Name + "::" + platform
			seen[key] = struct{}{}
		}
	}
	return len(seen)
}

// PublicModelCount returns unique public model count for marketing endpoints.
func (h *PlayHandler) PublicModelCount(ctx context.Context) int {
	if h == nil || h.playService == nil {
		return 0
	}
	return h.playService.PublicMarketingModelCount(ctx)
}
