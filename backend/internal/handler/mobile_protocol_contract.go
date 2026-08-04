package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
)

// Keep the numeric version stable for installed clients. New fields are added
// under contract_version so clients that only understand version=2 keep their
// existing behavior.
const (
	mobileProtocolVersion                  = 2
	mobileProtocolContractVersion          = "2026-08-04.4"
	mobileProtocolLifecycleRegistryVersion = 1

	mobileProtocolLifecycleCanonical = "canonical"
	mobileProtocolLifecycleLegacy    = "legacy"
	mobileProtocolLifecycleObserve   = "observe"
	mobileProtocolLifecycleDisabled  = "disabled"

	mobileOperationSessionStatusRead       = "mobile.session.status.read"
	mobileOperationAccountSummaryRead      = "mobile.account.summary.read"
	mobileOperationTaskSubmit              = "mobile.task.submit"
	mobileOperationAssetUpload             = "mobile.asset.upload"
	mobileOperationSupportTicketCreate     = "mobile.support.ticket.create"
	mobileOperationTaskClientStatusObserve = "mobile.task.client_status.observe"
	mobileOperationTeamApplicationCreate   = "play.team.application.create"
	mobileOperationTeamApplicationDecide   = "play.team.application.decide"
	mobileOperationTeamInviteRotate        = "play.team.invite.rotate"
	mobileOperationTeamRecruitingUpdate    = "play.team.recruiting.update"
	mobileOperationSearchWeb               = "mobile.search.web"
	mobileOperationAdminConsoleRead        = "admin.console.read"
	mobileOperationAdminStepUpWrite        = "admin.step_up.write"
	mobileOperationAdminRefundApprove      = "admin.funds.refund.approve"
	mobileOperationAdminRefundReject       = "admin.funds.refund.reject"
	mobileOperationAdminRefundMarkPaid     = "admin.funds.refund.mark_paid"
	mobileOperationAdminWithdrawalApprove  = "admin.funds.withdrawal.approve"
	mobileOperationAdminWithdrawalReject   = "admin.funds.withdrawal.reject"
	mobileOperationAdminWithdrawalMarkPaid = "admin.funds.withdrawal.mark_paid"
)

type mobileProtocolAdminCapabilities struct {
	Available       bool     `json:"available"`
	APIBasePath     string   `json:"api_base_path,omitempty"`
	StepUpPath      string   `json:"step_up_path,omitempty"`
	CompliancePath  string   `json:"compliance_path,omitempty"`
	WriteOperations []string `json:"write_operations,omitempty"`
}

// mobileProtocolOperationGrant is intentionally identifier-based. Mobile
// clients may only recognize these fixed IDs and must not execute arbitrary
// server-supplied URLs. Granted is a coarse session/role grant; each write is
// still authorized by its canonical handler and domain policy.
type mobileProtocolOperationGrant struct {
	ID                    string   `json:"id"`
	Granted               bool     `json:"granted"`
	Lifecycle             string   `json:"lifecycle"`
	RiskLevel             string   `json:"risk_level"`
	Authorization         []string `json:"authorization,omitempty"`
	ClientRequestIDHeader string   `json:"client_request_id_header,omitempty"`
	IdempotencyHeader     string   `json:"idempotency_header,omitempty"`
	IdempotencyMode       string   `json:"idempotency_mode,omitempty"`
}

// mobileProtocolSearchCapability describes the one canonical mobile search
// route. Configuration is server-only; no API key or provider fallback is ever
// included in this payload.
type mobileProtocolSearchCapability struct {
	Configured             bool     `json:"configured"`
	Provider               string   `json:"provider,omitempty"`
	ExecutionState         string   `json:"execution_state"`
	DefaultEnabled         bool     `json:"default_enabled"`
	UserOptInRequired      bool     `json:"user_opt_in_required"`
	ResultFields           []string `json:"result_fields"`
	MaxQueryRunes          int      `json:"max_query_runes"`
	MaxResults             int      `json:"max_results"`
	TimeoutMS              int      `json:"timeout_ms"`
	ClientRequestIDHeader  string   `json:"client_request_id_header"`
	ResponseRequestIDField string   `json:"response_request_id_field"`
}

type mobileProtocolCapabilities struct {
	Admin           mobileProtocolAdminCapabilities `json:"admin"`
	Search          mobileProtocolSearchCapability  `json:"search"`
	OperationGrants []mobileProtocolOperationGrant  `json:"operation_grants"`
}

type mobileProtocolLifecycle struct {
	RegistryVersion       int      `json:"registry_version"`
	ContractVersion       string   `json:"contract_version"`
	States                []string `json:"states"`
	ClientRequestIDHeader string   `json:"client_request_id_header"`
	IdempotencyHeader     string   `json:"idempotency_header"`
	MissingKeyPolicy      string   `json:"missing_key_policy"`
}

type mobileProtocolEndpointLifecycle struct {
	State                  string `json:"state"`
	DeclaredInContract     string `json:"declared_in_contract"`
	NewCapabilitiesAllowed bool   `json:"new_capabilities_allowed"`
}

type mobileProtocolEndpoint struct {
	Method      string                          `json:"method"`
	Path        string                          `json:"path"`
	Status      string                          `json:"status"`
	Description string                          `json:"description"`
	Replacement string                          `json:"replacement,omitempty"`
	RemoveAfter string                          `json:"remove_after,omitempty"`
	OperationID string                          `json:"operation_id,omitempty"`
	RiskLevel   string                          `json:"risk_level,omitempty"`
	Request     *mobileProtocolEndpointRequest  `json:"request,omitempty"`
	Lifecycle   mobileProtocolEndpointLifecycle `json:"lifecycle"`
}

type mobileProtocolEndpointRequest struct {
	ClientRequestIDHeader string `json:"client_request_id_header,omitempty"`
	IdempotencyHeader     string `json:"idempotency_header,omitempty"`
	IdempotencyMode       string `json:"idempotency_mode,omitempty"`
}

func mobileProtocolLifecycleMetadata() mobileProtocolLifecycle {
	return mobileProtocolLifecycle{
		RegistryVersion:       mobileProtocolLifecycleRegistryVersion,
		ContractVersion:       mobileProtocolContractVersion,
		States:                []string{mobileProtocolLifecycleCanonical, mobileProtocolLifecycleLegacy, mobileProtocolLifecycleObserve, mobileProtocolLifecycleDisabled},
		ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
		IdempotencyHeader:     "Idempotency-Key",
		MissingKeyPolicy:      "observe_only_until_server_enforcement_is_enabled",
	}
}

func mobileProtocolSearchCapabilityFromEnvironment() mobileProtocolSearchCapability {
	enabled := mobileWebSearchEnvBool(os.Getenv("MOBILE_WEB_SEARCH_ENABLED"))
	hasExaKey := strings.TrimSpace(os.Getenv("EXA_API_KEY")) != ""
	configuredProvider := strings.ToLower(strings.TrimSpace(os.Getenv("MOBILE_WEB_SEARCH_PROVIDER")))
	configured := enabled && ((hasExaKey && (configuredProvider == "" || configuredProvider == "exa")) || configuredProvider == "duckduckgo")

	capability := mobileProtocolSearchCapability{
		Configured:             configured,
		ExecutionState:         mobileProtocolLifecycleDisabled,
		DefaultEnabled:         false,
		UserOptInRequired:      true,
		ResultFields:           []string{"title", "url", "snippet", "page_age"},
		MaxQueryRunes:          mobileWebSearchMaxQueryRunes,
		MaxResults:             mobileWebSearchMaxResults,
		TimeoutMS:              int(mobileWebSearchTimeout / time.Millisecond),
		ClientRequestIDHeader:  middleware2.ClientRequestIDHeader,
		ResponseRequestIDField: "request_id",
	}
	if capability.Configured {
		if configuredProvider == "duckduckgo" && !hasExaKey {
			capability.Provider = "duckduckgo"
		} else {
			capability.Provider = "exa"
		}
		capability.ExecutionState = mobileProtocolLifecycleCanonical
	}
	return capability
}

func mobileProtocolOperationGrants(authenticated, isAdmin, searchConfigured bool) []mobileProtocolOperationGrant {
	requestID := middleware2.ClientRequestIDHeader
	idempotency := "Idempotency-Key"
	searchLifecycle := mobileProtocolLifecycleDisabled
	if searchConfigured {
		searchLifecycle = mobileProtocolLifecycleCanonical
	}
	grants := []mobileProtocolOperationGrant{
		{
			ID:            mobileOperationSessionStatusRead,
			Granted:       authenticated,
			Lifecycle:     mobileProtocolLifecycleCanonical,
			RiskLevel:     "low",
			Authorization: []string{"authenticated"},
		},
		{
			ID:            mobileOperationAccountSummaryRead,
			Granted:       authenticated,
			Lifecycle:     mobileProtocolLifecycleCanonical,
			RiskLevel:     "low",
			Authorization: []string{"authenticated"},
		},
		{
			ID:              mobileOperationTaskSubmit,
			Granted:         authenticated,
			Lifecycle:       mobileProtocolLifecycleCanonical,
			RiskLevel:       "medium",
			Authorization:   []string{"authenticated", "quota_and_model_capability"},
			IdempotencyMode: "client_request_id_body",
		},
		{
			ID:                    mobileOperationAssetUpload,
			Granted:               authenticated,
			Lifecycle:             mobileProtocolLifecycleCanonical,
			RiskLevel:             "medium",
			Authorization:         []string{"authenticated", "file_policy"},
			ClientRequestIDHeader: requestID,
			IdempotencyHeader:     idempotency,
			IdempotencyMode:       "observe_only",
		},
		{
			ID:                    mobileOperationSupportTicketCreate,
			Granted:               authenticated,
			Lifecycle:             mobileProtocolLifecycleCanonical,
			RiskLevel:             "medium",
			Authorization:         []string{"authenticated", "support_content_policy"},
			ClientRequestIDHeader: requestID,
			IdempotencyHeader:     idempotency,
			IdempotencyMode:       "observe_only",
		},
		{
			ID:            mobileOperationTaskClientStatusObserve,
			Granted:       false,
			Lifecycle:     mobileProtocolLifecycleObserve,
			RiskLevel:     "high",
			Authorization: []string{"compatibility_only", "new_clients_must_not_write_task_state"},
		},
		teamOperationGrant(mobileOperationTeamApplicationCreate, authenticated, "medium", []string{"authenticated", "team_admission_policy"}),
		teamOperationGrant(mobileOperationTeamApplicationDecide, authenticated, "high", []string{"authenticated", "team_captain", "team_admission_policy"}),
		teamOperationGrant(mobileOperationTeamInviteRotate, authenticated, "high", []string{"authenticated", "team_captain"}),
		teamOperationGrant(mobileOperationTeamRecruitingUpdate, authenticated, "medium", []string{"authenticated", "team_captain"}),
		{
			ID:                    mobileOperationSearchWeb,
			Granted:               authenticated && searchConfigured,
			Lifecycle:             searchLifecycle,
			RiskLevel:             "medium",
			Authorization:         []string{"configured_server_tool", "explicit_user_opt_in", "authenticated_mobile_route", "server_budget"},
			ClientRequestIDHeader: requestID,
			IdempotencyHeader:     idempotency,
			IdempotencyMode:       "required",
		},
		{
			ID:            mobileOperationAdminConsoleRead,
			Granted:       isAdmin,
			Lifecycle:     mobileProtocolLifecycleCanonical,
			RiskLevel:     "high",
			Authorization: []string{"role:admin"},
		},
		{
			ID:                    mobileOperationAdminStepUpWrite,
			Granted:               isAdmin,
			Lifecycle:             mobileProtocolLifecycleCanonical,
			RiskLevel:             "critical",
			Authorization:         []string{"role:admin", "totp_step_up", "route_specific_rbac"},
			ClientRequestIDHeader: requestID,
			IdempotencyHeader:     idempotency,
			IdempotencyMode:       "route_specific",
		},
		adminWriteOperationGrant(mobileOperationAdminRefundApprove, isAdmin),
		adminWriteOperationGrant(mobileOperationAdminRefundReject, isAdmin),
		adminWriteOperationGrant(mobileOperationAdminRefundMarkPaid, isAdmin),
		adminWriteOperationGrant(mobileOperationAdminWithdrawalApprove, isAdmin),
		adminWriteOperationGrant(mobileOperationAdminWithdrawalReject, isAdmin),
		adminWriteOperationGrant(mobileOperationAdminWithdrawalMarkPaid, isAdmin),
	}

	return grants
}

func adminWriteOperationGrant(id string, granted bool) mobileProtocolOperationGrant {
	return mobileProtocolOperationGrant{
		ID:                    id,
		Granted:               granted,
		Lifecycle:             mobileProtocolLifecycleCanonical,
		RiskLevel:             "critical",
		Authorization:         []string{"role:admin", "totp_step_up", "route_specific_rbac"},
		ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
		IdempotencyHeader:     "Idempotency-Key",
		IdempotencyMode:       "required",
	}
}

func teamOperationGrant(id string, granted bool, riskLevel string, authorization []string) mobileProtocolOperationGrant {
	return mobileProtocolOperationGrant{
		ID:                    id,
		Granted:               granted,
		Lifecycle:             mobileProtocolLifecycleCanonical,
		RiskLevel:             riskLevel,
		Authorization:         authorization,
		ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
		IdempotencyHeader:     "Idempotency-Key",
		IdempotencyMode:       "observe_only",
	}
}

func mobileEndpoint(method, path, state, description string) mobileProtocolEndpoint {
	return mobileProtocolEndpoint{
		Method:      method,
		Path:        path,
		Status:      state,
		Description: description,
		Lifecycle: mobileProtocolEndpointLifecycle{
			State:                  state,
			DeclaredInContract:     mobileProtocolContractVersion,
			NewCapabilitiesAllowed: state == mobileProtocolLifecycleCanonical,
		},
	}
}

func mobileProtocolEndpoints() []mobileProtocolEndpoint {
	canonical := mobileProtocolLifecycleCanonical
	legacy := mobileProtocolLifecycleLegacy
	observe := mobileProtocolLifecycleObserve
	endpoints := []mobileProtocolEndpoint{
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/protocol", canonical, "移动端统一协议与接口生命周期清单"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/mobile/register", canonical, "移动端注册"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/mobile/login", canonical, "移动端账号密码登录"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/mobile/send-verify-code", canonical, "移动端发送验证码"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/mobile/forgot-password", canonical, "移动端发起密码找回"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/mobile/reset-password", canonical, "移动端重置密码"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/refresh", canonical, "移动端登录态无感刷新"),
		mobileEndpoint(http.MethodPost, "/api/v1/auth/logout", canonical, "服务端登录态注销；本地凭据保留策略由 APP 控制"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/session/status", canonical, "移动端登录态自检，401 时 APP 应先无感刷新后重试"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/account-summary", canonical, "账户、余额、分组、订阅和套餐消耗聚合"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/sessions", canonical, "移动端聊天和生图独立托管会话启动"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/mobile/bootstrap", canonical, "移动端聊天和生图独立托管会话启动"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/sessions/chat/switch-group", canonical, "切换聊天分组，不影响生图分组"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/sessions/image/switch-group", canonical, "切换生图分组，不影响聊天分组"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/sessions/:purpose/switch-group", canonical, "按用途切换分组，不创建或切换聊天会话"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/tasks", canonical, "创建聊天、生图、文件统一任务"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/tasks", canonical, "统一任务历史"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/tasks/:id", canonical, "读取单个统一任务"),
		mobileEndpoint(http.MethodDelete, "/api/v1/mobile/tasks/:id", canonical, "软删除任务记录"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/tasks/:id/cancel", canonical, "取消可取消任务"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/tasks/:id/retry", canonical, "重试失败、取消或部分完成任务"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/image-history", canonical, "生图任务历史语义化包装"),
		mobileEndpoint(http.MethodDelete, "/api/v1/mobile/image-history/:id", canonical, "删除生图历史"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/image-history/:id/retry", canonical, "重试生图历史任务"),
		idempotentMobileEndpoint(http.MethodPost, "/api/v1/mobile/assets", mobileOperationAssetUpload, "medium", "上传系统分享、图片、PDF、语音和文件素材"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/assets", canonical, "素材库列表"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/assets/:id", canonical, "读取单个素材元数据"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/assets/:id/content", canonical, "读取素材内容"),
		mobileEndpoint(http.MethodDelete, "/api/v1/mobile/assets/:id", canonical, "删除素材"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/skills", canonical, "服务端技能目录，skill 必须区别于 agent"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/skills/:slug", canonical, "读取单个技能"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/skills/:slug/install", canonical, "安装技能"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/skills/:slug/use", canonical, "启用技能并记录最近使用"),
		mobileEndpoint(http.MethodDelete, "/api/v1/mobile/skills/:slug/install", canonical, "卸载技能"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/support/tickets", canonical, "读取 APP 反馈和客服工单"),
		idempotentMobileEndpoint(http.MethodPost, "/api/v1/mobile/support/tickets", mobileOperationSupportTicketCreate, "medium", "APP 反馈和客服工单提交"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/support/tickets/:id", canonical, "读取工单详情"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/support/tickets/:id/messages", canonical, "追加工单消息"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/support/tickets/:id/close", canonical, "关闭工单"),
		{
			Method:      http.MethodPost,
			Path:        "/api/v1/mobile/web-search",
			Status:      canonical,
			Description: "用户明确确认后执行服务端 Exa 联网搜索",
			OperationID: mobileOperationSearchWeb,
			RiskLevel:   "medium",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  canonical,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: true,
			},
			Request: &mobileProtocolEndpointRequest{
				ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
				IdempotencyHeader:     "Idempotency-Key",
				IdempotencyMode:       "required",
			},
		},
		mobileEndpoint(http.MethodPatch, "/api/v1/admin/play/mobile-feedback/:id", canonical, "玩法运营统一维护 APP 反馈、客服备注和需求项"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/diagnostics", canonical, "脱敏移动端网络和崩溃诊断"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/attribution/events", canonical, "记录脱敏安装和归因事件"),
		mobileEndpoint(http.MethodPut, "/api/v1/mobile/devices/:installation_id", canonical, "注册移动设备安装标识"),
		mobileEndpoint(http.MethodDelete, "/api/v1/mobile/devices/:installation_id", canonical, "删除移动设备安装标识"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/payments/create", canonical, "创建移动端支付并返回拉起字段"),
		mobileEndpoint(http.MethodGet, "/api/v1/mobile/payments/:order_id", canonical, "读取移动端支付订单"),
		mobileEndpoint(http.MethodPost, "/api/v1/mobile/payments/:order_id/sync", canonical, "返回 APP 后查单并同步到账"),
		mobileEndpoint(http.MethodPost, "/api/v1/redeem-codes/redeem", canonical, "兑换码、活动码和套餐码兑换"),
		mobileEndpoint(http.MethodGet, "/api/v1/redeem-codes/history", canonical, "兑换码历史"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/models", canonical, "移动端生图模型能力清单"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/estimate", canonical, "移动端生图费用和能力预估"),
		mobileEndpoint(http.MethodPost, "/api/v1/nextchat/image-studio/generate", canonical, "提交移动端生图任务"),
		mobileEndpoint(http.MethodPost, "/api/v1/nextchat/image-studio/references", canonical, "上传生图参考素材"),
		mobileEndpoint(http.MethodDelete, "/api/v1/nextchat/image-studio/references/:id", canonical, "删除生图参考素材"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/jobs/active", canonical, "读取当前活跃生图任务"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/jobs", canonical, "读取生图任务列表"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/jobs/:id", canonical, "读取单个生图任务"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/jobs/:id/download", canonical, "下载生图任务结果"),
		mobileEndpoint(http.MethodPost, "/api/v1/nextchat/image-studio/jobs/:id/cancel", canonical, "取消生图任务"),
		mobileEndpoint(http.MethodDelete, "/api/v1/nextchat/image-studio/jobs/:id", canonical, "删除生图任务"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/assets/:id/thumbnail", canonical, "读取生图素材缩略图"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/assets/:id/content", canonical, "读取生图素材内容"),
		mobileEndpoint(http.MethodGet, "/api/v1/nextchat/image-studio/assets/:id/download", canonical, "下载生图素材"),
		{
			Method:      http.MethodGet,
			Path:        "/api/v1/nextchat/mobile/account-summary",
			Status:      legacy,
			Description: "旧 APP 账户聚合兼容路径",
			Replacement: "/api/v1/mobile/account-summary",
			RemoveAfter: "2026-09-30",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  legacy,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: false,
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/api/v1/nextchat/mobile/group",
			Status:      legacy,
			Description: "旧全局分组切换兼容路径",
			Replacement: "/api/v1/mobile/sessions/{purpose}/switch-group",
			RemoveAfter: "2026-09-30",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  legacy,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: false,
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/api/v1/nextchat/mobile/sessions/:purpose/group",
			Status:      legacy,
			Description: "旧移动端按用途分组切换兼容路径",
			Replacement: "/api/v1/mobile/sessions/:purpose/switch-group",
			RemoveAfter: "2026-09-30",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  legacy,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: false,
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/api/v1/play/mobile-feedback",
			Status:      legacy,
			Description: "旧 APP 反馈兼容提交；canonical 不存在时可复用原幂等键安全降级",
			Replacement: "/api/v1/mobile/support/tickets",
			RemoveAfter: "2026-09-30",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  legacy,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: false,
			},
		},
		{
			Method:      http.MethodPost,
			Path:        "/api/v1/mobile/tasks/:id/status",
			Status:      observe,
			Description: "兼容期客户端任务状态写入；新客户端不得报告伪造运行或完成状态",
			OperationID: mobileOperationTaskClientStatusObserve,
			RiskLevel:   "high",
			Lifecycle: mobileProtocolEndpointLifecycle{
				State:                  observe,
				DeclaredInContract:     mobileProtocolContractVersion,
				NewCapabilitiesAllowed: false,
			},
		},
		teamEndpoint(http.MethodPost, "/api/v1/play/teams/applications", mobileOperationTeamApplicationCreate, "medium", "提交战队申请；可安全携带请求关联与幂等键"),
		teamEndpoint(http.MethodPost, "/api/v1/play/teams/applications/:id/decision", mobileOperationTeamApplicationDecide, "high", "队长批准或拒绝战队申请；可安全携带请求关联与幂等键"),
		teamEndpoint(http.MethodPost, "/api/v1/play/teams/invite/rotate", mobileOperationTeamInviteRotate, "high", "轮换战队邀请口令；可安全携带请求关联与幂等键"),
		teamEndpoint(http.MethodPut, "/api/v1/play/teams/recruiting", mobileOperationTeamRecruitingUpdate, "medium", "更新战队招募状态；可安全携带请求关联与幂等键"),
	}

	return endpoints
}

func teamEndpoint(method, path, operationID, riskLevel, description string) mobileProtocolEndpoint {
	endpoint := mobileEndpoint(method, path, mobileProtocolLifecycleCanonical, description)
	endpoint.OperationID = operationID
	endpoint.RiskLevel = riskLevel
	endpoint.Request = &mobileProtocolEndpointRequest{
		ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
		IdempotencyHeader:     "Idempotency-Key",
		IdempotencyMode:       "observe_only",
	}
	return endpoint
}

func idempotentMobileEndpoint(method, path, operationID, riskLevel, description string) mobileProtocolEndpoint {
	endpoint := mobileEndpoint(method, path, mobileProtocolLifecycleCanonical, description)
	endpoint.OperationID = operationID
	endpoint.RiskLevel = riskLevel
	endpoint.Request = &mobileProtocolEndpointRequest{
		ClientRequestIDHeader: middleware2.ClientRequestIDHeader,
		IdempotencyHeader:     "Idempotency-Key",
		IdempotencyMode:       "observe_only",
	}
	return endpoint
}

func validateMobileProtocolLifecycleRegistry() error {
	seen := make(map[string]struct{})
	for _, endpoint := range mobileProtocolEndpoints() {
		key := endpoint.Method + " " + endpoint.Path
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate mobile protocol endpoint %s", key)
		}
		seen[key] = struct{}{}
		if endpoint.Method == "" || !strings.HasPrefix(endpoint.Path, "/api/v1/") {
			return fmt.Errorf("invalid mobile protocol endpoint %s", key)
		}
		if endpoint.Status != endpoint.Lifecycle.State {
			return fmt.Errorf("endpoint %s lifecycle state %q does not match status %q", key, endpoint.Lifecycle.State, endpoint.Status)
		}
		if endpoint.Lifecycle.DeclaredInContract != mobileProtocolContractVersion {
			return fmt.Errorf("endpoint %s declares unexpected contract version %q", key, endpoint.Lifecycle.DeclaredInContract)
		}
		switch endpoint.Status {
		case mobileProtocolLifecycleCanonical:
			if !endpoint.Lifecycle.NewCapabilitiesAllowed {
				return fmt.Errorf("canonical endpoint %s must allow new capabilities", key)
			}
		case mobileProtocolLifecycleLegacy:
			if endpoint.Replacement == "" || endpoint.RemoveAfter == "" || endpoint.Lifecycle.NewCapabilitiesAllowed {
				return fmt.Errorf("legacy endpoint %s requires replacement, removal date, and no new capabilities", key)
			}
		case mobileProtocolLifecycleObserve:
			if endpoint.Lifecycle.NewCapabilitiesAllowed {
				return fmt.Errorf("observe endpoint %s must not allow new capabilities", key)
			}
		default:
			return fmt.Errorf("endpoint %s has unknown lifecycle status %q", key, endpoint.Status)
		}
	}
	return nil
}
