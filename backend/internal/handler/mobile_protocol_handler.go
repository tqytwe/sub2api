package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileProtocolResponse struct {
	Version          int                        `json:"version"`
	ContractVersion  string                     `json:"contract_version"`
	GeneratedAt      time.Time                  `json:"generated_at"`
	Session          map[string]any             `json:"session"`
	Capabilities     mobileProtocolCapabilities `json:"capabilities"`
	Lifecycle        mobileProtocolLifecycle    `json:"lifecycle"`
	TaskKinds        []string                   `json:"task_kinds"`
	TaskStatuses     []string                   `json:"task_statuses"`
	TerminalStatuses []string                   `json:"terminal_statuses"`
	Endpoints        []mobileProtocolEndpoint   `json:"endpoints"`
	Privacy          map[string]any             `json:"privacy"`
}

func MobileProtocol(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, mobileProtocolPayload(false, 0, ""))
}

func MobileSessionStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return
	}
	role, _ := middleware2.GetUserRoleFromContext(c)
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, mobileProtocolPayload(true, subject.UserID, role))
}

func mobileProtocolPayload(authenticated bool, userID int64, role string) mobileProtocolResponse {
	isAdmin := authenticated && role == service.RoleAdmin
	adminCapabilities := mobileProtocolAdminCapabilities{Available: isAdmin}
	if isAdmin {
		adminCapabilities.APIBasePath = "/api/v1/admin"
		adminCapabilities.StepUpPath = "/api/v1/user/totp/step-up"
		adminCapabilities.CompliancePath = "/api/v1/admin/compliance"
		adminCapabilities.WriteOperations = []string{
			mobileOperationAdminRefundApprove,
			mobileOperationAdminRefundReject,
			mobileOperationAdminRefundMarkPaid,
			mobileOperationAdminWithdrawalApprove,
			mobileOperationAdminWithdrawalReject,
			mobileOperationAdminWithdrawalMarkPaid,
		}
	}
	searchCapability := mobileProtocolSearchCapabilityFromEnvironment()

	return mobileProtocolResponse{
		Version:         mobileProtocolVersion,
		ContractVersion: mobileProtocolContractVersion,
		GeneratedAt:     time.Now().UTC(),
		Session: map[string]any{
			"authenticated": authenticated,
			"user_id":       userID,
			"role":          role,
			"refresh_path":  "/api/v1/auth/refresh",
			"login_path":    "/api/v1/auth/mobile/login",
			"logout_path":   "/api/v1/auth/logout",
		},
		Capabilities: mobileProtocolCapabilities{
			Admin:           adminCapabilities,
			Search:          searchCapability,
			OperationGrants: mobileProtocolOperationGrants(authenticated, isAdmin, searchCapability.Configured),
		},
		Lifecycle: mobileProtocolLifecycleMetadata(),
		TaskKinds: []string{
			string(service.MobileTaskKindChat),
			string(service.MobileTaskKindImage),
			string(service.MobileTaskKindVideo),
			string(service.MobileTaskKindFile),
		},
		TaskStatuses: []string{
			string(service.MobileTaskStatusQueued),
			string(service.MobileTaskStatusRunning),
			string(service.MobileTaskStatusStreaming),
			string(service.MobileTaskStatusCompleted),
			string(service.MobileTaskStatusPartial),
			string(service.MobileTaskStatusFailed),
			string(service.MobileTaskStatusCancelled),
		},
		TerminalStatuses: []string{
			string(service.MobileTaskStatusCompleted),
			string(service.MobileTaskStatusPartial),
			string(service.MobileTaskStatusFailed),
			string(service.MobileTaskStatusCancelled),
		},
		Endpoints: mobileProtocolEndpoints(),
		Privacy: map[string]any{
			"diagnostics_redacts": []string{"access_token", "refresh_token", "api_key", "authorization", "cookie", "prompt", "messages", "chat"},
			"chat_content":        "never_upload_in_diagnostics",
			"legacy_policy":       "legacy endpoints are compatibility only and must not receive new capabilities",
		},
	}
}
