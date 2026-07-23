package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type IPRiskHandler struct {
	core        *service.IPRiskService
	admin       *service.IPRiskAdminService
	totpService *service.TotpService
	userService *service.UserService
}

func NewIPRiskHandler(
	core *service.IPRiskService,
) *IPRiskHandler {
	return &IPRiskHandler{core: core}
}

func NewIPRiskManagementHandler(
	core *service.IPRiskService,
	adminService service.AdminService,
	apiKeys service.APIKeyRepository,
	invalidator service.APIKeyAuthCacheInvalidator,
	hasher *service.IPRiskHasher,
	totpService *service.TotpService,
	userService *service.UserService,
	repo service.IPRiskRepository,
) *IPRiskHandler {
	return &IPRiskHandler{
		core:        core,
		admin:       service.NewIPRiskAdminService(repo, core, adminService, apiKeys, invalidator, hasher),
		totpService: totpService,
		userService: userService,
	}
}

func (h *IPRiskHandler) GetOverview(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	item, err := h.admin.Overview(c.Request.Context())
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) GetRuntime(c *gin.Context) {
	if h == nil || h.core == nil {
		response.Success(c, service.IPRiskRuntime{
			Degraded:       true,
			DegradedReason: "service unavailable",
			ShadowMode:     true,
		})
		return
	}
	response.Success(c, h.core.Runtime(c.Request.Context()))
}

func (h *IPRiskHandler) ListCases(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	filter := service.IPRiskCaseFilter{
		Page:     page,
		PageSize: pageSize,
		Level:    strings.TrimSpace(c.Query("level")),
		Status:   strings.TrimSpace(c.Query("status")),
		Signal:   strings.TrimSpace(c.Query("signal")),
		Search:   strings.TrimSpace(c.Query("search")),
	}
	var err error
	if raw := strings.TrimSpace(c.Query("range_start")); raw != "" {
		filter.RangeStart, err = parseIPRiskTime(raw)
		if err != nil {
			response.BadRequest(c, "Invalid range_start, expect RFC3339")
			return
		}
	}
	if raw := strings.TrimSpace(c.Query("range_end")); raw != "" {
		filter.RangeEnd, err = parseIPRiskTime(raw)
		if err != nil {
			response.BadRequest(c, "Invalid range_end, expect RFC3339")
			return
		}
	}
	items, total, err := h.admin.ListCases(c.Request.Context(), filter)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *IPRiskHandler) GetCase(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	id, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	item, err := h.admin.GetCase(c.Request.Context(), id)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

type ipRiskScanRequest struct {
	RangeStart time.Time `json:"range_start" binding:"required"`
	RangeEnd   time.Time `json:"range_end" binding:"required"`
}

func (h *IPRiskHandler) StartScan(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	var req ipRiskScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid scan request: "+err.Error())
		return
	}
	actorID, ok := currentIPRiskActorID(c)
	if !ok {
		response.Unauthorized(c, "Administrator session required")
		return
	}
	item, err := h.admin.StartManualScan(c.Request.Context(), req.RangeStart, req.RangeEnd, actorID)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Accepted(c, item)
}

func (h *IPRiskHandler) GetScan(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	id, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	item, err := h.admin.GetScan(c.Request.Context(), id)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) GetConfig(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	item, err := h.admin.GetConfig(c.Request.Context())
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) UpdateConfig(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	var req service.IPRiskManagedConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid risk config: "+err.Error())
		return
	}
	current, err := h.admin.GetConfig(c.Request.Context())
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	if !current.AutoBlockEnabled && req.AutoBlockEnabled {
		if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
			return
		}
	}
	actorID, ok := currentIPRiskActorID(c)
	if !ok {
		response.Unauthorized(c, "Administrator session required")
		return
	}
	item, err := h.admin.UpdateConfig(c.Request.Context(), req, actorID)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) ListPolicies(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	items, err := h.admin.ListPolicies(c.Request.Context())
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, items)
}

func (h *IPRiskHandler) CreatePolicy(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	var req service.IPRiskPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid policy: "+err.Error())
		return
	}
	if req.Mode == service.IPPolicyBlockRegistration && req.ExpiresAt == nil {
		if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
			return
		}
	}
	if actorID, ok := currentIPRiskActorID(c); ok {
		req.CreatedBy = &actorID
	}
	item, err := h.admin.CreatePolicy(c.Request.Context(), req)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Created(c, item)
}

func (h *IPRiskHandler) UpdatePolicy(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	id, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	var req service.IPRiskPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid policy: "+err.Error())
		return
	}
	if req.Mode == service.IPPolicyBlockRegistration && req.ExpiresAt == nil {
		if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
			return
		}
	}
	item, err := h.admin.UpdatePolicy(c.Request.Context(), id, req)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) DeletePolicy(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	id, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	if err := h.admin.DeletePolicy(c.Request.Context(), id); err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *IPRiskHandler) PreviewAction(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	caseID, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	var req service.IPRiskActionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid action preview: "+err.Error())
		return
	}
	item, err := h.admin.PreviewAction(c.Request.Context(), caseID, req)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) ExecuteAction(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	caseID, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	var req service.IPRiskActionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid risk action: "+err.Error())
		return
	}
	if req.ActionType == service.RiskActionDisableUsers ||
		req.ActionType == service.RiskActionPermanentRegistrationBan {
		if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
			return
		}
	}
	actorID, ok := currentIPRiskActorID(c)
	if !ok {
		response.Unauthorized(c, "Administrator session required")
		return
	}
	item, err := h.admin.ExecuteAction(c.Request.Context(), caseID, actorID, req)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) ListActions(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.admin.ListActions(c.Request.Context(), page, pageSize)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

type ipRiskRollbackRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *IPRiskHandler) RollbackAction(c *gin.Context) {
	if !h.requireAdminService(c) {
		return
	}
	actionID, ok := parseIPRiskID(c, "id")
	if !ok {
		return
	}
	var req ipRiskRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid rollback request: "+err.Error())
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" || len([]rune(req.Reason)) > 1000 {
		response.BadRequest(c, "Rollback reason is required and must not exceed 1000 characters")
		return
	}
	if !middleware.EnforceStepUpAlways(c, h.totpService, h.userService) {
		return
	}
	actorID, ok := currentIPRiskActorID(c)
	if !ok {
		response.Unauthorized(c, "Administrator session required")
		return
	}
	item, err := h.admin.RollbackAction(c.Request.Context(), actionID, actorID, req.Reason)
	if err != nil {
		writeIPRiskError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *IPRiskHandler) requireAdminService(c *gin.Context) bool {
	if h == nil || h.admin == nil {
		response.Error(c, http.StatusServiceUnavailable, "IP risk management is unavailable")
		return false
	}
	return true
}

func currentIPRiskActorID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	return subject.UserID, ok && subject.UserID > 0
}

func parseIPRiskID(c *gin.Context, key string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param(key)), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return 0, false
	}
	return id, true
}

func parseIPRiskTime(raw string) (*time.Time, error) {
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	value = value.UTC()
	return &value, nil
}

func writeIPRiskError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrIPRiskActionPreviewStale),
		errors.Is(err, service.ErrIPRiskActionPreviewExpired):
		response.ErrorWithDetails(c, http.StatusConflict, err.Error(), "risk_action_preview_stale", nil)
	case errors.Is(err, service.ErrIPRiskActionPreviewInvalid):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrIPRiskActionNotRollbackEligible):
		response.ErrorWithDetails(c, http.StatusConflict, err.Error(), "risk_action_not_rollback_eligible", nil)
	case errors.Is(err, sql.ErrNoRows):
		response.NotFound(c, "IP risk record not found")
	case strings.Contains(err.Error(), "already running"):
		response.ErrorWithDetails(c, http.StatusConflict, err.Error(), "ip_risk_scan_running", nil)
	default:
		response.ErrorFrom(c, err)
	}
}
