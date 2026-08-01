package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PlayHandler serves play/engagement endpoints (check-in, arena, public models).
type PlayHandler struct {
	playService          *service.PlayService
	billingService       *service.BillingService
	feedbackAssetService *service.AnnouncementAssetService
}

func NewPlayHandler(playService *service.PlayService, billingService *service.BillingService, feedbackAssetService ...*service.AnnouncementAssetService) *PlayHandler {
	var assetService *service.AnnouncementAssetService
	if len(feedbackAssetService) > 0 {
		assetService = feedbackAssetService[0]
	}
	return &PlayHandler{playService: playService, billingService: billingService, feedbackAssetService: assetService}
}

const (
	mobileFeedbackMaxRequestBytes = 8 << 20
	mobileFeedbackMaxFileBytes    = service.AnnouncementAssetMaxBytes
	mobileFeedbackMaxScreenshots  = 3
)

type playCheckinStatusDTO struct {
	Enabled                bool    `json:"enabled"`
	Eligible               bool    `json:"eligible"`
	IneligibleReason       string  `json:"ineligible_reason,omitempty"`
	CheckedInToday         bool    `json:"checked_in_today"`
	RewardAmount           float64 `json:"reward_amount"`
	CouponPoolReady        bool    `json:"coupon_pool_ready"`
	CouponWeightBP         int     `json:"coupon_weight_bp"`
	RedeemCodeWeightBP     int     `json:"redeem_code_weight_bp"`
	BalanceWeightBP        int     `json:"balance_weight_bp"`
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
	RewardAmount      float64                  `json:"reward_amount"`
	BalanceAdded      float64                  `json:"balance_added"`
	RewardType        service.PlayRewardType   `json:"reward_type"`
	Coupon            *playCouponRewardDTO     `json:"coupon,omitempty"`
	RedeemCode        *playRedeemCodeRewardDTO `json:"redeem_code,omitempty"`
	CouponPoolVersion string                   `json:"coupon_pool_version,omitempty"`
	ServerDate        string                   `json:"server_date"`
	StreakCount       int                      `json:"streak_count,omitempty"`
	MilestoneBonus    float64                  `json:"milestone_bonus,omitempty"`
}

type playArenaPeriodDTO struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	StartAt    string  `json:"start_at"`
	EndAt      string  `json:"end_at"`
	Status     string  `json:"status"`
	PeriodType string  `json:"period_type,omitempty"`
	SettledAt  *string `json:"settled_at,omitempty"`
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
	DisplayName string `json:"display_name,omitempty"`
	Anonymous   bool   `json:"anonymous,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	TokenSum    int64  `json:"token_sum"`
	IsMine      bool   `json:"is_mine,omitempty"`
}

type playArenaLeaderboardDTO struct {
	Enabled bool                `json:"enabled"`
	Period  *playArenaPeriodDTO `json:"period,omitempty"`
	Rows    []playArenaScoreDTO `json:"rows"`
}

type playArenaDailyRewardSummaryDTO struct {
	Enabled bool                                    `json:"enabled"`
	Recent  *playArenaDailyRecentRewardSummaryDTO   `json:"recent,omitempty"`
	Current *playArenaDailyCurrentRewardEstimateDTO `json:"current,omitempty"`
}

type playTeamRewardShowcaseDTO struct {
	Winners []playTeamRewardShowcaseWinnerDTO `json:"winners"`
}

type playTeamRewardShowcaseWinnerDTO struct {
	SettlementMonth string  `json:"settlement_month"`
	TeamName        string  `json:"team_name"`
	DisplayName     string  `json:"display_name"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	Amount          float64 `json:"amount"`
	PaidAt          *string `json:"paid_at,omitempty"`
}

type playArenaMonthlyRewardSummaryDTO struct {
	Enabled      bool                              `json:"enabled"`
	Period       *playArenaPeriodDTO               `json:"period,omitempty"`
	SettledAt    *string                           `json:"settled_at,omitempty"`
	WinnersCount int                               `json:"winners_count"`
	TotalAmount  float64                           `json:"total_amount"`
	Winners      []playArenaMonthlyRewardWinnerDTO `json:"winners"`
}

type playArenaMonthlyRewardWinnerDTO struct {
	Rank        int     `json:"rank"`
	DisplayName string  `json:"display_name"`
	Anonymous   bool    `json:"anonymous,omitempty"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	Amount      float64 `json:"amount"`
	PaidAt      *string `json:"paid_at,omitempty"`
}

type playArenaDailyRecentRewardSummaryDTO struct {
	Period       *playArenaPeriodDTO             `json:"period,omitempty"`
	SettledAt    *string                         `json:"settled_at,omitempty"`
	PaidToday    bool                            `json:"paid_today"`
	WinnersCount int                             `json:"winners_count"`
	TotalAmount  float64                         `json:"total_amount"`
	Winners      []playArenaDailyRewardWinnerDTO `json:"winners"`
}

type playArenaDailyRewardWinnerDTO struct {
	Rank        int     `json:"rank"`
	DisplayName string  `json:"display_name"`
	Anonymous   bool    `json:"anonymous,omitempty"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	TokenSum    int64   `json:"token_sum"`
	Amount      float64 `json:"amount"`
}

type playArenaDailyCurrentRewardEstimateDTO struct {
	Period *playArenaPeriodDTO               `json:"period,omitempty"`
	Rows   []playArenaDailyRewardEstimateDTO `json:"rows"`
}

type playArenaDailyRewardEstimateDTO struct {
	Rank            int     `json:"rank"`
	DisplayName     string  `json:"display_name"`
	Anonymous       bool    `json:"anonymous,omitempty"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	TokenSum        int64   `json:"token_sum"`
	EstimatedReward float64 `json:"estimated_reward"`
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

// SubmitMobileFeedback stores Android app feedback with optional screenshots.
// POST /api/v1/play/mobile-feedback
func (h *PlayHandler) SubmitMobileFeedback(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Sunset-Policy", "legacy; use /api/v1/mobile/support/tickets")
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.playService == nil {
		response.ErrorFrom(c, service.ErrMobileFeedbackUnavailable)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileFeedbackMaxRequestBytes)
	if err := c.Request.ParseMultipartForm(mobileFeedbackMaxRequestBytes); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID", "invalid feedback request"))
		return
	}

	deviceInfo := map[string]any{}
	if raw := strings.TrimSpace(c.PostForm("device_info")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &deviceInfo); err != nil {
			response.ErrorFrom(c, infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID", "invalid device info"))
			return
		}
	}

	var groupID *int64
	if raw := strings.TrimSpace(c.PostForm("group_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.ErrorFrom(c, infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID", "invalid group id"))
			return
		}
		groupID = &parsed
	}

	screenshots, err := h.uploadMobileFeedbackScreenshots(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	created, err := h.playService.CreateMobileFeedback(c.Request.Context(), subject.UserID, service.MobileFeedbackInput{
		Title:          c.PostForm("title"),
		Category:       c.PostForm("category"),
		Content:        c.PostForm("content"),
		AppVersion:     c.PostForm("app_version"),
		InstallationID: c.PostForm("installation_id"),
		Channel:        c.PostForm("channel"),
		Referrer:       c.PostForm("referrer"),
		Platform:       c.PostForm("platform"),
		DeviceModel:    c.PostForm("device_model"),
		AndroidVersion: c.PostForm("android_version"),
		SystemVersion:  c.PostForm("system_version"),
		GroupName:      c.PostForm("group_name"),
		GroupID:        groupID,
		BackendURL:     c.PostForm("backend_url"),
		LastError:      c.PostForm("last_error"),
		CrashLog:       c.PostForm("crash_log"),
		DeviceInfo:     deviceInfo,
		Screenshots:    screenshots,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created)
}

func (h *PlayHandler) uploadMobileFeedbackScreenshots(c *gin.Context) ([]service.MobileFeedbackScreenshot, error) {
	if c.Request.MultipartForm == nil || c.Request.MultipartForm.File == nil {
		return []service.MobileFeedbackScreenshot{}, nil
	}
	files := c.Request.MultipartForm.File["screenshots"]
	if len(files) == 0 {
		return []service.MobileFeedbackScreenshot{}, nil
	}
	if len(files) > mobileFeedbackMaxScreenshots {
		return nil, infraerrors.BadRequest("MOBILE_FEEDBACK_TOO_MANY_SCREENSHOTS", "too many screenshots")
	}
	if h.feedbackAssetService == nil {
		return nil, service.ErrAnnouncementAssetStorageUnavailable
	}

	out := make([]service.MobileFeedbackScreenshot, 0, len(files))
	for _, header := range files {
		if header == nil {
			continue
		}
		if header.Size > mobileFeedbackMaxFileBytes {
			return nil, service.ErrAnnouncementAssetTooLarge
		}
		file, err := header.Open()
		if err != nil {
			return nil, infraerrors.BadRequest("MOBILE_FEEDBACK_INVALID_SCREENSHOT", "invalid screenshot")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, mobileFeedbackMaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(data) > mobileFeedbackMaxFileBytes {
			return nil, service.ErrAnnouncementAssetTooLarge
		}
		asset, err := h.feedbackAssetService.Upload(c.Request.Context(), header.Filename, header.Header.Get("Content-Type"), data)
		if err != nil {
			return nil, err
		}
		out = append(out, service.MobileFeedbackScreenshot{
			URL:         asset.URL,
			FileName:    header.Filename,
			ContentType: asset.ContentType,
			ByteSize:    asset.ByteSize,
		})
	}
	return out, nil
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
		Eligible:               status.Eligible,
		IneligibleReason:       status.IneligibleReason,
		CheckedInToday:         status.CheckedInToday,
		RewardAmount:           status.RewardAmount,
		CouponPoolReady:        status.CouponPoolReady,
		CouponWeightBP:         status.CouponWeightBP,
		RedeemCodeWeightBP:     status.RedeemCodeWeightBP,
		BalanceWeightBP:        status.BalanceWeightBP,
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
		RewardAmount:      result.RewardAmount,
		BalanceAdded:      result.BalanceAdded,
		RewardType:        result.RewardType,
		Coupon:            toPlayCouponRewardDTO(result.Coupon),
		RedeemCode:        toPlayRedeemCodeRewardDTO(result.RedeemCode),
		CouponPoolVersion: result.CouponPoolVersion,
		ServerDate:        result.ServerDate,
		StreakCount:       result.StreakCount,
		MilestoneBonus:    result.MilestoneBonus,
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
		RewardAmount:      result.RewardAmount,
		BalanceAdded:      result.BalanceAdded,
		RewardType:        result.RewardType,
		Coupon:            toPlayCouponRewardDTO(result.Coupon),
		RedeemCode:        toPlayRedeemCodeRewardDTO(result.RedeemCode),
		CouponPoolVersion: result.CouponPoolVersion,
		ServerDate:        result.ServerDate,
		StreakCount:       result.StreakCount,
		MilestoneBonus:    result.MilestoneBonus,
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
		out.Rows = append(out.Rows, toPlayArenaScoreDTO(row))
	}
	response.Success(c, out)
}

func toPlayArenaScoreDTO(row service.PlayArenaScoreRow) playArenaScoreDTO {
	return playArenaScoreDTO{
		Rank:        row.Rank,
		DisplayName: row.DisplayName,
		Anonymous:   row.Anonymous,
		AvatarURL:   row.AvatarURL,
		TokenSum:    row.TokenSum,
		IsMine:      row.IsMine,
	}
}

func toPlayArenaPeriodDTO(p *service.PlayArenaPeriod) *playArenaPeriodDTO {
	if p == nil {
		return nil
	}
	var settledAt *string
	if p.SettledAt != nil {
		formatted := p.SettledAt.Format("2006-01-02T15:04:05Z07:00")
		settledAt = &formatted
	}
	return &playArenaPeriodDTO{
		ID:         p.ID,
		Name:       p.Name,
		StartAt:    p.StartAt.Format("2006-01-02T15:04:05Z07:00"),
		EndAt:      p.EndAt.Format("2006-01-02T15:04:05Z07:00"),
		Status:     p.Status,
		PeriodType: p.PeriodType,
		SettledAt:  settledAt,
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
