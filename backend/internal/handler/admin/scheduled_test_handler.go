package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ScheduledTestHandler handles admin scheduled-test-plan management.
type ScheduledTestHandler struct {
	scheduledTestSvc *service.ScheduledTestService
}

type qualityOwnershipRequest struct {
	Fence   string `json:"fence" binding:"required"`
	Confirm bool   `json:"confirm"`
}

func (h *ScheduledTestHandler) QualityOwnershipPreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	out, err := h.scheduledTestSvc.QualityOwnership(c.Request.Context(), id, nil)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *ScheduledTestHandler) QualityOwnershipReconcile(c *gin.Context) {
	h.qualityOwnershipMutation(c, false)
}

func (h *ScheduledTestHandler) QualityOwnershipRestore(c *gin.Context) {
	h.qualityOwnershipMutation(c, true)
}

func (h *ScheduledTestHandler) qualityOwnershipMutation(c *gin.Context, restore bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	var req qualityOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	middleware.SetAuditAction(c, map[bool]string{true: "quality.restore", false: "quality.reconcile"}[restore])
	out, err := h.scheduledTestSvc.QualityOwnership(c.Request.Context(), id, &service.QualityOwnershipRequest{Fence: req.Fence, Confirm: req.Confirm, Reconcile: !restore, Restore: restore})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *ScheduledTestHandler) QualityOwnershipDelete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	var req qualityOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	middleware.SetAuditAction(c, "quality.delete")
	out, err := h.scheduledTestSvc.QualityOwnership(c.Request.Context(), id, &service.QualityOwnershipRequest{Fence: req.Fence, Confirm: req.Confirm, Delete: true})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

// NewScheduledTestHandler creates a new ScheduledTestHandler.
func NewScheduledTestHandler(scheduledTestSvc *service.ScheduledTestService) *ScheduledTestHandler {
	return &ScheduledTestHandler{scheduledTestSvc: scheduledTestSvc}
}

type createScheduledTestPlanRequest struct {
	AccountID      int64                      `json:"account_id" binding:"required"`
	ModelID        string                     `json:"model_id"`
	CronExpression string                     `json:"cron_expression" binding:"required"`
	Enabled        *bool                      `json:"enabled"`
	MaxResults     int                        `json:"max_results"`
	AutoRecover    *bool                      `json:"auto_recover"`
	PelicanConfig  *service.PelicanTestConfig `json:"pelican_config"`
}

type updateScheduledTestPlanRequest struct {
	ModelID           string                     `json:"model_id"`
	CronExpression    string                     `json:"cron_expression"`
	Enabled           *bool                      `json:"enabled"`
	MaxResults        int                        `json:"max_results"`
	AutoRecover       *bool                      `json:"auto_recover"`
	ExpectedUpdatedAt *time.Time                 `json:"expected_updated_at"`
	PelicanConfig     *service.PelicanTestConfig `json:"pelican_config"`
}

// ListByAccount GET /admin/accounts/:id/scheduled-test-plans
func (h *ScheduledTestHandler) ListByAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	plans, err := h.scheduledTestSvc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// ListQualityPlans returns quality rules for the administrator workspace.
func (h *ScheduledTestHandler) ListQualityPlans(c *gin.Context) {
	plans, err := h.scheduledTestSvc.ListQualityPlans(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// TriggerQuality schedules an enabled, idle quality rule for the next runner tick.
func (h *ScheduledTestHandler) TriggerQuality(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if err := h.scheduledTestSvc.TriggerQuality(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListQualityHistory returns the administrator-wide quality operation ledger.
// Only sanitized result snapshots are returned; response bodies are never
// loaded by this endpoint.
func (h *ScheduledTestHandler) ListQualityHistory(c *gin.Context) {
	beforeID, err := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	if err != nil || beforeID < 0 {
		response.BadRequest(c, "invalid before_id")
		return
	}
	page, err := h.scheduledTestSvc.ListQualityHistory(c.Request.Context(), beforeID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, page)
}

// Create POST /admin/scheduled-test-plans
func (h *ScheduledTestHandler) Create(c *gin.Context) {
	var req createScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	plan := &service.ScheduledTestPlan{
		AccountID:      req.AccountID,
		ModelID:        req.ModelID,
		CronExpression: req.CronExpression,
		Enabled:        true,
		MaxResults:     req.MaxResults,
		PelicanConfig:  req.PelicanConfig,
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}
	if req.AutoRecover != nil {
		plan.AutoRecover = *req.AutoRecover
	}

	created, err := h.scheduledTestSvc.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

// Update PUT /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	existing, err := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req updateScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.ModelID != "" {
		existing.ModelID = req.ModelID
	}
	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.MaxResults > 0 {
		existing.MaxResults = req.MaxResults
	}
	if req.AutoRecover != nil {
		existing.AutoRecover = *req.AutoRecover
	}
	if req.PelicanConfig != nil {
		existing.PelicanConfig = req.PelicanConfig
	}

	var updated *service.ScheduledTestPlan
	if existing.PelicanConfig != nil {
		if req.ExpectedUpdatedAt == nil {
			response.ErrorFrom(c, infraerrors.BadRequest("QUALITY_VERSION_REQUIRED", "expected_updated_at is required when editing a quality rule"))
			return
		}
		updated, err = h.scheduledTestSvc.UpdatePlanWithVersion(c.Request.Context(), existing, req.ExpectedUpdatedAt.UTC())
	} else {
		updated, err = h.scheduledTestSvc.UpdatePlan(c.Request.Context(), existing)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.ErrorFrom(c, infraerrors.Conflict("QUALITY_VERSION_CONFLICT", "quality rule changed; reload before saving"))
		} else {
			response.ErrorFrom(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, updated)
}

// Delete DELETE /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if plan, getErr := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID); getErr == nil && plan.PelicanConfig != nil {
		response.ErrorFrom(c, infraerrors.Conflict("QUALITY_OWNERSHIP_REQUIRED", "quality rules must be previewed and deleted through the ownership endpoint"))
		return
	}

	if err := h.scheduledTestSvc.DeletePlan(c.Request.Context(), planID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListResults GET /admin/scheduled-test-plans/:id/results
func (h *ScheduledTestHandler) ListResults(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	results, err := h.scheduledTestSvc.ListResults(c.Request.Context(), planID, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}
