package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type qualityPresetRequest struct {
	Name           string                     `json:"name" binding:"required"`
	ModelID        string                     `json:"model_id" binding:"required"`
	CronExpression string                     `json:"cron_expression" binding:"required"`
	Enabled        *bool                      `json:"enabled"`
	MaxResults     int                        `json:"max_results"`
	PelicanConfig  *service.PelicanTestConfig `json:"pelican_config" binding:"required"`
}

func (h *ScheduledTestHandler) ListQualityPresets(c *gin.Context) {
	presets, err := h.scheduledTestSvc.ListQualityPresets(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if presets == nil {
		presets = []*service.QualityRulePreset{}
	}
	c.JSON(http.StatusOK, presets)
}

func (h *ScheduledTestHandler) CreateQualityPreset(c *gin.Context) {
	var req qualityPresetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	middleware.SetAuditAction(c, "quality.preset.create")
	created, err := h.scheduledTestSvc.CreateQualityPreset(c.Request.Context(), &service.QualityRulePreset{
		Name: req.Name, ModelID: req.ModelID, CronExpression: req.CronExpression,
		Enabled: enabled, MaxResults: req.MaxResults, PelicanConfig: req.PelicanConfig,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, created)
}
