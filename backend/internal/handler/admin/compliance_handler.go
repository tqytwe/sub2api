package admin

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ComplianceHandler struct {
	settingService *service.SettingService
}

const adminComplianceAcceptOperation = "admin.compliance.accept"

func NewComplianceHandler(settingService *service.SettingService) *ComplianceHandler {
	return &ComplianceHandler{settingService: settingService}
}

type AcceptAdminComplianceRequest struct {
	Phrase   string `json:"phrase" binding:"required"`
	Language string `json:"language"`
}

// acceptAdminComplianceIdempotencyPayload deliberately contains only the
// acknowledgement's semantic inputs and acting admin. Request-scoped metadata
// such as IP and user agent must not make a retry conflict with the original
// acknowledgement.
type acceptAdminComplianceIdempotencyPayload struct {
	AdminUserID int64  `json:"admin_user_id"`
	Phrase      string `json:"phrase"`
	Language    string `json:"language"`
}

func (h *ComplianceHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	status, err := h.settingService.GetAdminComplianceStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

func (h *ComplianceHandler) Accept(c *gin.Context) {
	// This is a security acknowledgement rather than a generic resource create;
	// use a stable, searchable audit action for both original attempts and replays.
	middleware.SetAuditAction(c, adminComplianceAcceptOperation)

	var req AcceptAdminComplianceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	input := service.AdminComplianceAcceptInput{
		AdminUserID: subject.UserID,
		Phrase:      req.Phrase,
		Language:    req.Language,
		IPAddress:   ip.GetClientIP(c),
		UserAgent:   strings.TrimSpace(c.GetHeader("User-Agent")),
	}

	executeAdminIdempotentJSON(
		c,
		adminComplianceAcceptOperation,
		acceptAdminComplianceIdempotencyPayload{
			AdminUserID: subject.UserID,
			Phrase:      req.Phrase,
			Language:    req.Language,
		},
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			return h.settingService.AcceptAdminCompliance(ctx, input)
		},
	)
}
