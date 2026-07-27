package handler

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const mobileProtocolVersion = 1

type mobileProtocolEndpoint struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Replacement string `json:"replacement,omitempty"`
	RemoveAfter string `json:"remove_after,omitempty"`
}

type mobileProtocolResponse struct {
	Version          int                      `json:"version"`
	GeneratedAt      time.Time                `json:"generated_at"`
	Session          map[string]any           `json:"session"`
	TaskKinds        []string                 `json:"task_kinds"`
	TaskStatuses     []string                 `json:"task_statuses"`
	TerminalStatuses []string                 `json:"terminal_statuses"`
	Endpoints        []mobileProtocolEndpoint `json:"endpoints"`
	Privacy          map[string]any           `json:"privacy"`
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
	return mobileProtocolResponse{
		Version:     mobileProtocolVersion,
		GeneratedAt: time.Now().UTC(),
		Session: map[string]any{
			"authenticated": authenticated,
			"user_id":       userID,
			"role":          role,
			"refresh_path":  "/api/v1/auth/refresh",
			"login_path":    "/api/v1/auth/mobile/login",
			"logout_path":   "/api/v1/auth/logout",
		},
		TaskKinds: []string{
			string(service.MobileTaskKindChat),
			string(service.MobileTaskKindImage),
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
		Endpoints: []mobileProtocolEndpoint{
			{Method: http.MethodGet, Path: "/api/v1/mobile/protocol", Status: "canonical", Description: "移动端统一协议与接口生命周期清单"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/session/status", Status: "canonical", Description: "移动端登录态自检，401 时 APP 应先无感刷新后重试"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/account-summary", Status: "canonical", Description: "账户、余额、分组、订阅和套餐消耗聚合"},
			{Method: http.MethodGet, Path: "/api/v1/nextchat/mobile/account-summary", Status: "legacy", Description: "旧 APP 账户聚合兼容路径", Replacement: "/api/v1/mobile/account-summary", RemoveAfter: "2026-09-30"},
			{Method: http.MethodGet, Path: "/api/v1/nextchat/mobile/bootstrap", Status: "canonical", Description: "移动端聊天和生图独立托管会话启动"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/sessions/chat/switch-group", Status: "canonical", Description: "切换聊天分组，不影响生图分组"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/sessions/image/switch-group", Status: "canonical", Description: "切换生图分组，不影响聊天分组"},
			{Method: http.MethodPost, Path: "/api/v1/nextchat/mobile/group", Status: "legacy", Description: "旧全局分组切换兼容路径", Replacement: "/api/v1/mobile/sessions/{purpose}/switch-group", RemoveAfter: "2026-09-30"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/tasks", Status: "canonical", Description: "创建聊天、生图、文件统一任务"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/tasks", Status: "canonical", Description: "统一任务历史"},
			{Method: http.MethodDelete, Path: "/api/v1/mobile/tasks/:id", Status: "canonical", Description: "软删除任务记录"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/tasks/:id/cancel", Status: "canonical", Description: "取消可取消任务"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/tasks/:id/retry", Status: "canonical", Description: "重试失败、取消或部分完成任务"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/image-history", Status: "canonical", Description: "生图任务历史语义化包装"},
			{Method: http.MethodDelete, Path: "/api/v1/mobile/image-history/:id", Status: "canonical", Description: "删除生图历史"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/assets", Status: "canonical", Description: "上传系统分享、图片、PDF、语音和文件素材"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/assets", Status: "canonical", Description: "素材库列表"},
			{Method: http.MethodGet, Path: "/api/v1/mobile/skills", Status: "canonical", Description: "服务端技能目录，skill 必须区别于 agent"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/skills/:slug/use", Status: "canonical", Description: "启用技能并记录最近使用"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/support/tickets", Status: "canonical", Description: "APP 反馈和客服工单提交"},
			{Method: http.MethodPatch, Path: "/api/v1/admin/play/mobile-feedback/:id", Status: "canonical", Description: "玩法运营统一维护 APP 反馈、客服备注和需求项"},
			{Method: http.MethodPost, Path: "/api/v1/play/mobile-feedback", Status: "legacy", Description: "旧 APP 反馈兼容提交", Replacement: "/api/v1/mobile/support/tickets", RemoveAfter: "2026-09-30"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/diagnostics", Status: "canonical", Description: "脱敏移动端网络和崩溃诊断"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/payments/create", Status: "canonical", Description: "创建移动端支付并返回拉起字段"},
			{Method: http.MethodPost, Path: "/api/v1/mobile/payments/:order_id/sync", Status: "canonical", Description: "返回 APP 后查单并同步到账"},
			{Method: http.MethodPost, Path: "/api/v1/redeem-codes/redeem", Status: "canonical", Description: "兑换码、活动码和套餐码兑换"},
		},
		Privacy: map[string]any{
			"diagnostics_redacts": []string{"access_token", "refresh_token", "api_key", "authorization", "cookie", "prompt", "messages", "chat"},
			"chat_content":        "never_upload_in_diagnostics",
			"legacy_policy":       "legacy endpoints are compatibility only and must not receive new capabilities",
		},
	}
}
