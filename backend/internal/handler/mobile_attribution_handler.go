package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileAttributionEventService interface {
	RecordEvent(context.Context, int64, service.MobileAttributionEventInput) (service.MobileAttributionRecordResult, error)
}

type MobileAttributionHandler struct{ service mobileAttributionEventService }

type mobileAttributionEventRequest struct {
	InstallationID   string            `json:"installation_id" binding:"required"`
	EventType        string            `json:"event_type" binding:"required"`
	IdempotencyKey   string            `json:"idempotency_key" binding:"required"`
	Platform         string            `json:"platform" binding:"required"`
	AppVersion       string            `json:"app_version"`
	Locale           string            `json:"locale"`
	AttributionToken string            `json:"attribution_token"`
	OccurredAt       *time.Time        `json:"occurred_at"`
	Metadata         map[string]string `json:"metadata"`
}

func NewMobileAttributionHandler(svc mobileAttributionEventService) *MobileAttributionHandler {
	return &MobileAttributionHandler{service: svc}
}

func (h *MobileAttributionHandler) Event(c *gin.Context) {
	if h == nil || h.service == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("MOBILE_ATTRIBUTION_UNAVAILABLE", "mobile attribution is unavailable"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req mobileAttributionEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("MOBILE_ATTRIBUTION_INVALID", "mobile attribution event is invalid"))
		return
	}
	userID := int64(0)
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		userID = subject.UserID
	}
	result, err := h.service.RecordEvent(c.Request.Context(), userID, service.MobileAttributionEventInput{
		InstallationID: strings.TrimSpace(req.InstallationID), EventType: strings.TrimSpace(req.EventType), IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		Platform: strings.TrimSpace(req.Platform), AppVersion: strings.TrimSpace(req.AppVersion), Locale: strings.TrimSpace(req.Locale),
		AttributionToken: strings.TrimSpace(req.AttributionToken), OccurredAt: req.OccurredAt, Metadata: req.Metadata,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}

type mobileAttributionAdminService interface {
	ListInstallations(context.Context, service.MobileAttributionAdminFilter) ([]service.MobileAttributionInstallationRow, int64, error)
	Funnel(context.Context, service.MobileAttributionAdminFilter) ([]service.MobileAttributionFunnelStep, error)
}

type MobileAttributionAdminHandler struct{ service mobileAttributionAdminService }

func NewMobileAttributionAdminHandler(svc mobileAttributionAdminService) *MobileAttributionAdminHandler {
	return &MobileAttributionAdminHandler{service: svc}
}

func (h *MobileAttributionAdminHandler) Installations(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := parseMobileAttributionAdminFilter(c)
	filter.Page, filter.PageSize = page, pageSize
	items, total, err := h.service.ListInstallations(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *MobileAttributionAdminHandler) Funnel(c *gin.Context) {
	filter := parseMobileAttributionAdminFilter(c)
	filter.EventType = ""
	items, err := h.service.Funnel(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"steps": items})
}

func parseMobileAttributionAdminFilter(c *gin.Context) service.MobileAttributionAdminFilter {
	filter := service.MobileAttributionAdminFilter{EventType: strings.TrimSpace(c.Query("event_type")), Platform: strings.TrimSpace(c.Query("platform"))}
	if value, err := strconv.ParseInt(c.Query("campaign_id"), 10, 64); err == nil && value > 0 {
		filter.CampaignID = &value
	}
	if value, err := strconv.ParseInt(c.Query("user_id"), 10, 64); err == nil && value > 0 {
		filter.UserID = &value
	}
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			filter.From = &parsed
		}
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			filter.To = &parsed
		}
	}
	return filter
}
