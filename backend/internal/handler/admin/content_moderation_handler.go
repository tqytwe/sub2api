package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ContentModerationHandler struct {
	service *service.ContentModerationService
}

func NewContentModerationHandler(svc *service.ContentModerationService) *ContentModerationHandler {
	return &ContentModerationHandler{service: svc}
}

type contentModerationConfigRequest struct {
	Engine        *string                                               `json:"engine"`
	EngineConfigs map[string]service.UpdateContentModerationEngineInput `json:"engine_configs"`
	Enabled       *bool                                                 `json:"enabled"`
	Mode          *string                                               `json:"mode"`
	BaseURL       *string                                               `json:"base_url"`
	Model         *string                                               `json:"model"`
	// 审计请求使用的代理服务器：null 不修改；0 清除（直连）；>0 指定代理。
	ProxyID              *int64              `json:"proxy_id"`
	APIKey               *string             `json:"api_key"`
	APIKeys              *[]string           `json:"api_keys"`
	APIKeysMode          string              `json:"api_keys_mode"`
	DeleteAPIKeyHashes   *[]string           `json:"delete_api_key_hashes"`
	ClearAPIKey          bool                `json:"clear_api_key"`
	TimeoutMS            *int                `json:"timeout_ms"`
	SampleRate           *int                `json:"sample_rate"`
	AllGroups            *bool               `json:"all_groups"`
	GroupIDs             *[]int64            `json:"group_ids"`
	RecordNonHits        *bool               `json:"record_non_hits"`
	Thresholds           *map[string]float64 `json:"thresholds"`
	WorkerCount          *int                `json:"worker_count"`
	QueueSize            *int                `json:"queue_size"`
	BlockStatus          *int                `json:"block_status"`
	BlockMessage         *string             `json:"block_message"`
	EmailOnHit           *bool               `json:"email_on_hit"`
	AutoBanEnabled       *bool               `json:"auto_ban_enabled"`
	BanThreshold         *int                `json:"ban_threshold"`
	ViolationWindowHours *int                `json:"violation_window_hours"`
	// cyber_policy 命中是否排除出自动封号计数；前端 RiskControlView 已发送该字段，
	// service.UpdateContentModerationConfigInput 已支持，此前 handler 层缺透传导致开关静默失效。
	CyberPolicyExcludeFromBanCount *bool                                 `json:"cyber_policy_exclude_from_ban_count"`
	RetryCount                     *int                                  `json:"retry_count"`
	HitRetentionDays               *int                                  `json:"hit_retention_days"`
	NonHitRetentionDays            *int                                  `json:"non_hit_retention_days"`
	PreHashCheckEnabled            *bool                                 `json:"pre_hash_check_enabled"`
	BlockedKeywords                *[]string                             `json:"blocked_keywords"`
	KeywordBlockingMode            *string                               `json:"keyword_blocking_mode"`
	ModelFilter                    *service.ContentModerationModelFilter `json:"model_filter"`
}

type contentModerationAPIKeyTestRequest struct {
	Engine     string              `json:"engine"`
	Thresholds *map[string]float64 `json:"thresholds"`
	APIKeys    []string            `json:"api_keys"`
	BaseURL    string              `json:"base_url"`
	Model      string              `json:"model"`
	TimeoutMS  int                 `json:"timeout_ms"`
	ProxyID    *int64              `json:"proxy_id"`
	Prompt     string              `json:"prompt"`
	Images     []string            `json:"images"`
}

type contentModerationHashRequest struct {
	InputHash string `json:"input_hash"`
}

type moderationUserActionRequest struct {
	Action         string `json:"action"`
	Version        *int64 `json:"version" binding:"required"`
	AutoBanExempt  *bool  `json:"auto_ban_exempt"`
	KeywordExempt  *bool  `json:"keyword_exempt"`
	APIAuditExempt *bool  `json:"api_audit_exempt"`
	EmailExempt    *bool  `json:"email_exempt"`
	Reason         string `json:"reason"`
	Confirm        bool   `json:"confirm"`
}

func (h *ContentModerationHandler) GetConfig(c *gin.Context) {
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ContentModerationHandler) UpdateConfig(c *gin.Context) {
	var req contentModerationConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.service.UpdateConfig(c.Request.Context(), service.UpdateContentModerationConfigInput{
		Engine: req.Engine, EngineConfigs: req.EngineConfigs,
		Enabled:                        req.Enabled,
		Mode:                           req.Mode,
		BaseURL:                        req.BaseURL,
		Model:                          req.Model,
		ProxyID:                        req.ProxyID,
		APIKey:                         req.APIKey,
		APIKeys:                        req.APIKeys,
		APIKeysMode:                    req.APIKeysMode,
		DeleteAPIKeyHashes:             req.DeleteAPIKeyHashes,
		ClearAPIKey:                    req.ClearAPIKey,
		TimeoutMS:                      req.TimeoutMS,
		SampleRate:                     req.SampleRate,
		AllGroups:                      req.AllGroups,
		GroupIDs:                       req.GroupIDs,
		RecordNonHits:                  req.RecordNonHits,
		Thresholds:                     req.Thresholds,
		WorkerCount:                    req.WorkerCount,
		QueueSize:                      req.QueueSize,
		BlockStatus:                    req.BlockStatus,
		BlockMessage:                   req.BlockMessage,
		EmailOnHit:                     req.EmailOnHit,
		AutoBanEnabled:                 req.AutoBanEnabled,
		BanThreshold:                   req.BanThreshold,
		ViolationWindowHours:           req.ViolationWindowHours,
		CyberPolicyExcludeFromBanCount: req.CyberPolicyExcludeFromBanCount,
		RetryCount:                     req.RetryCount,
		HitRetentionDays:               req.HitRetentionDays,
		NonHitRetentionDays:            req.NonHitRetentionDays,
		PreHashCheckEnabled:            req.PreHashCheckEnabled,
		BlockedKeywords:                req.BlockedKeywords,
		KeywordBlockingMode:            req.KeywordBlockingMode,
		ModelFilter:                    req.ModelFilter,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ContentModerationHandler) TestAPIKeys(c *gin.Context) {
	var req contentModerationAPIKeyTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.TestAPIKeys(c.Request.Context(), service.TestContentModerationAPIKeysInput{
		Engine: req.Engine, Thresholds: req.Thresholds,
		APIKeys:   req.APIKeys,
		BaseURL:   req.BaseURL,
		Model:     req.Model,
		TimeoutMS: req.TimeoutMS,
		ProxyID:   req.ProxyID,
		Prompt:    req.Prompt,
		Images:    req.Images,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) GetStatus(c *gin.Context) {
	status, err := h.service.GetStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *ContentModerationHandler) ListLogs(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.ContentModerationLogFilter{
		Pagination: pagination.PaginationParams{
			Page:      page,
			PageSize:  pageSize,
			SortOrder: pagination.SortOrderDesc,
		},
		Result:   c.Query("result"),
		Endpoint: c.Query("endpoint"),
		Search:   c.Query("search"),
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "Invalid group_id")
			return
		}
		filter.GroupID = &groupID
	}
	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		t, _, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid from")
			return
		}
		filter.From = &t
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		t, dateOnly, err := parseContentModerationDate(raw)
		if err != nil {
			response.BadRequest(c, "Invalid to")
			return
		}
		if dateOnly {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		filter.To = &t
	}
	items, pageResult, err := h.service.ListLogs(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, pageResult.Total, pageResult.Page, pageResult.PageSize)
}

func (h *ContentModerationHandler) UnbanUser(c *gin.Context) { h.manageUser(c, "unban") }

func (h *ContentModerationHandler) ManageUser(c *gin.Context) { h.manageUser(c, "") }

func (h *ContentModerationHandler) manageUser(c *gin.Context, fixedAction string) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	var req moderationUserActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Version, reason and confirmation are required")
		return
	}
	if req.Version == nil {
		response.BadRequest(c, "Version, reason and confirmation are required")
		return
	}
	if fixedAction != "" {
		req.Action = fixedAction
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Administrator identity required")
		return
	}
	result, err := h.service.ManageModerationUser(c.Request.Context(), userID, service.ModerationUserAction{
		Action: req.Action, Version: *req.Version, Reason: req.Reason, Confirm: req.Confirm, ActorID: subject.UserID,
		Exempt: req.AutoBanExempt, KeywordExempt: req.KeywordExempt, APIAuditExempt: req.APIAuditExempt, EmailExempt: req.EmailExempt,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) GetModerationUser(c *gin.Context) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("user_id")), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	state, err := h.service.GetModerationUser(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *ContentModerationHandler) ListModerationUsers(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	userID, err := strconv.ParseInt(strings.TrimSpace(c.DefaultQuery("user_id", "0")), 10, 64)
	if err != nil || userID < 0 {
		response.BadRequest(c, "Invalid user_id")
		return
	}
	state := c.DefaultQuery("state", "exempt")
	items, total, err := h.service.ListModerationUsers(c.Request.Context(), page, pageSize, state, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *ContentModerationHandler) DeleteFlaggedHash(c *gin.Context) {
	var req contentModerationHashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.DeleteFlaggedInputHash(c.Request.Context(), req.InputHash)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ContentModerationHandler) ClearFlaggedHashes(c *gin.Context) {
	result, err := h.service.ClearFlaggedInputHashes(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseContentModerationDate(raw string) (time.Time, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, false, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	return t, err == nil, err
}
