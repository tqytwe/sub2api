package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestMobileProtocolIncludesCanonicalLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MOBILE_WEB_SEARCH_ENABLED", "")
	t.Setenv("EXA_API_KEY", "")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mobile/protocol", nil)

	MobileProtocol(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Version         int                        `json:"version"`
			ContractVersion string                     `json:"contract_version"`
			Session         map[string]any             `json:"session"`
			TaskStatuses    []string                   `json:"task_statuses"`
			Endpoints       []mobileProtocolEndpoint   `json:"endpoints"`
			Lifecycle       mobileProtocolLifecycle    `json:"lifecycle"`
			Capabilities    mobileProtocolCapabilities `json:"capabilities"`
			Privacy         map[string]any             `json:"privacy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != 0 || envelope.Data.Version != mobileProtocolVersion {
		t.Fatalf("unexpected envelope: code=%d version=%d", envelope.Code, envelope.Data.Version)
	}
	if envelope.Data.ContractVersion != mobileProtocolContractVersion {
		t.Fatalf("contract_version = %q, want %q", envelope.Data.ContractVersion, mobileProtocolContractVersion)
	}
	if envelope.Data.Lifecycle.RegistryVersion != mobileProtocolLifecycleRegistryVersion {
		t.Fatalf("lifecycle registry_version = %d, want %d", envelope.Data.Lifecycle.RegistryVersion, mobileProtocolLifecycleRegistryVersion)
	}
	assertStringSliceContains(t, envelope.Data.Lifecycle.States, mobileProtocolLifecycleCanonical)
	assertStringSliceContains(t, envelope.Data.Lifecycle.States, mobileProtocolLifecycleLegacy)
	assertStringSliceContains(t, envelope.Data.Lifecycle.States, mobileProtocolLifecycleObserve)
	if got := envelope.Data.Session["refresh_path"]; got != "/api/v1/auth/refresh" {
		t.Fatalf("refresh path = %v", got)
	}
	assertStringSliceContains(t, envelope.Data.TaskStatuses, "streaming")
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodGet, "/api/v1/mobile/account-summary", "canonical")
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodPost, "/api/v1/mobile/web-search", "canonical")
	for _, endpoint := range envelope.Data.Endpoints {
		if endpoint.Method == http.MethodPost && endpoint.Path == "/api/v1/mobile/web-search" {
			if endpoint.OperationID != mobileOperationSearchWeb {
				t.Fatalf("web search operation id = %q, want %q", endpoint.OperationID, mobileOperationSearchWeb)
			}
			if endpoint.Request == nil || endpoint.Request.ClientRequestIDHeader != middleware2.ClientRequestIDHeader || endpoint.Request.IdempotencyHeader != "Idempotency-Key" || endpoint.Request.IdempotencyMode != "required" {
				t.Fatalf("web search endpoint must advertise required replay headers: %#v", endpoint.Request)
			}
		}
		if (endpoint.Method == http.MethodPost && (endpoint.Path == "/api/v1/mobile/assets" || endpoint.Path == "/api/v1/mobile/support/tickets")) ||
			(endpoint.Method == http.MethodPatch && endpoint.Path == "/api/v1/mobile/assets/:id") {
			if endpoint.Request == nil || endpoint.Request.ClientRequestIDHeader != middleware2.ClientRequestIDHeader || endpoint.Request.IdempotencyHeader != "Idempotency-Key" || endpoint.Request.IdempotencyMode != "observe_only" {
				t.Fatalf("idempotent mobile create endpoint contract is incomplete: %#v", endpoint)
			}
		}
	}
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodPost, "/api/v1/play/mobile-feedback", "legacy")
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodPost, "/api/v1/mobile/tasks/:id/status", "observe")
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationTeamApplicationCreate, false, mobileProtocolLifecycleCanonical)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationSearchWeb, false, mobileProtocolLifecycleDisabled)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAssetUpload, false, mobileProtocolLifecycleCanonical)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAssetRename, false, mobileProtocolLifecycleCanonical)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationSupportTicketCreate, false, mobileProtocolLifecycleCanonical)
	for _, operation := range []string{mobileOperationAssetUpload, mobileOperationAssetRename, mobileOperationSupportTicketCreate} {
		grant := findOperationGrant(t, envelope.Data.Capabilities.OperationGrants, operation)
		if grant.ClientRequestIDHeader != middleware2.ClientRequestIDHeader || grant.IdempotencyHeader != "Idempotency-Key" || grant.IdempotencyMode != "observe_only" {
			t.Fatalf("operation %q idempotency contract is incomplete: %#v", operation, grant)
		}
	}
	taskGrant := findOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationTaskSubmit)
	if taskGrant.IdempotencyMode != "client_request_id_body" {
		t.Fatalf("task submit idempotency mode = %q", taskGrant.IdempotencyMode)
	}
	if taskGrant.ClientRequestIDHeader != "" || taskGrant.IdempotencyHeader != "" {
		t.Fatal("task submit must not advertise team-write headers it does not consume")
	}
	if envelope.Data.Capabilities.Search.Configured {
		t.Fatal("search must be unconfigured when the server Exa environment is absent")
	}
	if envelope.Data.Capabilities.Search.Provider != "" {
		t.Fatalf("search provider = %q when no server key is configured", envelope.Data.Capabilities.Search.Provider)
	}
	if envelope.Data.Capabilities.Search.DefaultEnabled {
		t.Fatal("mobile search must be disabled when the server search service is disabled")
	}
	if envelope.Data.Capabilities.Search.ExecutionState != mobileProtocolLifecycleDisabled {
		t.Fatalf("search execution_state = %q", envelope.Data.Capabilities.Search.ExecutionState)
	}
	if strings.Contains(rec.Body.String(), `"request":{}`) {
		t.Fatal("protocol response must omit empty per-endpoint request contracts")
	}
}

func TestMobileProtocolSearchContractIsEnvironmentOnlyAndCanonicalWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("MOBILE_WEB_SEARCH_ENABLED", "true")
	t.Setenv("EXA_API_KEY", "test-exa-key")

	payload := mobileProtocolPayload(true, 42, "user")
	search := payload.Capabilities.Search
	if !search.Configured {
		t.Fatal("search should be configured only when the server toggle and Exa key are present")
	}
	if search.Provider != "exa" {
		t.Fatalf("search provider = %q, want exa", search.Provider)
	}
	if search.ExecutionState != mobileProtocolLifecycleCanonical {
		t.Fatalf("search execution_state = %q, want canonical", search.ExecutionState)
	}
	if !search.DefaultEnabled || !search.ModelToolCallRequired {
		t.Fatalf("unexpected model-tool policy: default_enabled=%v model_tool_call_required=%v", search.DefaultEnabled, search.ModelToolCallRequired)
	}
	assertStringSliceContains(t, search.ResultFields, "title")
	assertStringSliceContains(t, search.ResultFields, "url")
	if search.MaxQueryRunes != mobileWebSearchMaxQueryRunes || search.MaxResults != mobileWebSearchMaxResults || search.TimeoutMS <= 0 {
		t.Fatalf("unexpected search limits: query=%d results=%d timeout=%d", search.MaxQueryRunes, search.MaxResults, search.TimeoutMS)
	}
	if search.ClientRequestIDHeader != "X-Client-Request-ID" {
		t.Fatalf("search client request ID header = %q", search.ClientRequestIDHeader)
	}
	if search.ResponseRequestIDField != "request_id" {
		t.Fatalf("search response request ID field = %q", search.ResponseRequestIDField)
	}
	if search.ToolCallIDField != "tool_call_id" || search.ResponseToolCallIDField != "tool_call_id" {
		t.Fatalf("search tool call fields = request:%q response:%q", search.ToolCallIDField, search.ResponseToolCallIDField)
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal protocol payload: %v", err)
	}
	if strings.Contains(string(serialized), "test-exa-key") {
		t.Fatal("protocol payload must not expose the server search credential")
	}
	if !strings.Contains(strings.ToLower(string(serialized)), "duckduckgo") {
		t.Fatal("protocol payload must document the server-owned DuckDuckGo fallback")
	}
	assertOperationGrant(t, payload.Capabilities.OperationGrants, mobileOperationSearchWeb, true, mobileProtocolLifecycleCanonical)
	searchGrant := findOperationGrant(t, payload.Capabilities.OperationGrants, mobileOperationSearchWeb)
	if searchGrant.ClientRequestIDHeader != middleware2.ClientRequestIDHeader || searchGrant.IdempotencyHeader != "Idempotency-Key" || searchGrant.IdempotencyMode != "required" {
		t.Fatalf("search operation idempotency contract is incomplete: %#v", searchGrant)
	}
}

func TestMobileProtocolSearchContractSupportsExplicitDuckDuckGo(t *testing.T) {
	t.Setenv("MOBILE_WEB_SEARCH_ENABLED", "1")
	t.Setenv("EXA_API_KEY", "")
	t.Setenv("MOBILE_WEB_SEARCH_PROVIDER", "duckduckgo")

	search := mobileProtocolPayload(true, 42, "user").Capabilities.Search
	if !search.Configured || search.Provider != "duckduckgo" {
		t.Fatalf("unexpected DuckDuckGo capability: %#v", search)
	}
	if search.ExecutionState != mobileProtocolLifecycleCanonical {
		t.Fatalf("execution_state = %q, want canonical", search.ExecutionState)
	}
	if !search.DefaultEnabled || !search.ModelToolCallRequired {
		t.Fatalf("DuckDuckGo must remain model-tool enabled without a client toggle: %#v", search)
	}
}

func TestMobileProtocolSearchContractFallsBackToDuckDuckGoWithoutExaKey(t *testing.T) {
	t.Setenv("MOBILE_WEB_SEARCH_ENABLED", "true")
	t.Setenv("EXA_API_KEY", "")
	t.Setenv("MOBILE_WEB_SEARCH_PROVIDER", "")

	search := mobileProtocolPayload(true, 42, "user").Capabilities.Search
	if !search.Configured || search.Provider != "duckduckgo" {
		t.Fatalf("unexpected automatic DuckDuckGo fallback capability: %#v", search)
	}
	if search.ExecutionState != mobileProtocolLifecycleCanonical || !search.DefaultEnabled || !search.ModelToolCallRequired {
		t.Fatalf("unexpected fallback model-tool policy: %#v", search)
	}
}

func TestMobileSessionStatusUsesAuthenticatedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mobile/session/status", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
	c.Set(string(middleware2.ContextKeyUserRole), "admin")

	MobileSessionStatus(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Session      map[string]any `json:"session"`
			Capabilities struct {
				OperationGrants []mobileProtocolOperationGrant `json:"operation_grants"`
				Admin           struct {
					Available      bool   `json:"available"`
					APIBasePath    string `json:"api_base_path"`
					StepUpPath     string `json:"step_up_path"`
					CompliancePath string `json:"compliance_path"`
				} `json:"admin"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Session["authenticated"] != true {
		t.Fatalf("authenticated = %v", envelope.Data.Session["authenticated"])
	}
	if envelope.Data.Session["user_id"] != float64(42) {
		t.Fatalf("user_id = %v", envelope.Data.Session["user_id"])
	}
	if envelope.Data.Session["role"] != "admin" {
		t.Fatalf("role = %v", envelope.Data.Session["role"])
	}
	if !envelope.Data.Capabilities.Admin.Available {
		t.Fatal("admin capability must be server-declared for an administrator")
	}
	if envelope.Data.Capabilities.Admin.APIBasePath != "/api/v1/admin" {
		t.Fatalf("admin API base path = %q", envelope.Data.Capabilities.Admin.APIBasePath)
	}
	if envelope.Data.Capabilities.Admin.StepUpPath != "/api/v1/user/totp/step-up" {
		t.Fatalf("step-up path = %q", envelope.Data.Capabilities.Admin.StepUpPath)
	}
	if envelope.Data.Capabilities.Admin.CompliancePath != "/api/v1/admin/compliance" {
		t.Fatalf("compliance path = %q", envelope.Data.Capabilities.Admin.CompliancePath)
	}
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAdminConsoleRead, true, mobileProtocolLifecycleCanonical)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAdminStepUpWrite, true, mobileProtocolLifecycleCanonical)
}

func TestMobileSessionStatusDoesNotGrantAdminCapabilityToRegularUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mobile/session/status", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	c.Set(string(middleware2.ContextKeyUserRole), "user")

	MobileSessionStatus(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var envelope struct {
		Data struct {
			Capabilities struct {
				OperationGrants []mobileProtocolOperationGrant `json:"operation_grants"`
				Admin           struct {
					Available      bool   `json:"available"`
					CompliancePath string `json:"compliance_path"`
				} `json:"admin"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Capabilities.Admin.Available {
		t.Fatal("regular user must not receive administrator capability")
	}
	if envelope.Data.Capabilities.Admin.CompliancePath != "" {
		t.Fatal("regular user must not receive an administrator compliance path")
	}
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAdminConsoleRead, false, mobileProtocolLifecycleCanonical)
	assertOperationGrant(t, envelope.Data.Capabilities.OperationGrants, mobileOperationAdminStepUpWrite, false, mobileProtocolLifecycleCanonical)
}

func TestMobileSessionStatusRejectsMissingContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mobile/session/status", nil)

	MobileSessionStatus(c)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMobileProtocolLifecycleRegistryCoversCanonicalLegacyAndObserveRoutes(t *testing.T) {
	if err := validateMobileProtocolLifecycleRegistry(); err != nil {
		t.Fatalf("mobile protocol lifecycle registry is invalid: %v", err)
	}

	endpoints := mobileProtocolEndpoints()
	assertEndpointContains(t, endpoints, http.MethodGet, "/api/v1/mobile/protocol", mobileProtocolLifecycleCanonical)
	assertEndpointContains(t, endpoints, http.MethodPost, "/api/v1/nextchat/mobile/group", mobileProtocolLifecycleLegacy)
	assertEndpointContains(t, endpoints, http.MethodPost, "/api/v1/mobile/tasks/:id/status", mobileProtocolLifecycleObserve)
}

func assertEndpointContains(t *testing.T, endpoints []mobileProtocolEndpoint, method, path, status string) {
	t.Helper()
	for _, endpoint := range endpoints {
		if endpoint.Method == method && endpoint.Path == path && endpoint.Status == status {
			return
		}
	}
	t.Fatalf("endpoint %s %s with status %s not found in %#v", method, path, status, endpoints)
}

func assertStringSliceContains(t *testing.T, values []string, needle string) {
	t.Helper()
	for _, value := range values {
		if value == needle {
			return
		}
	}
	t.Fatalf("%q not found in %#v", needle, values)
}

func assertOperationGrant(t *testing.T, grants []mobileProtocolOperationGrant, operation string, granted bool, lifecycle string) {
	grant := findOperationGrant(t, grants, operation)
	if grant.Granted != granted {
		t.Fatalf("operation %q granted = %v, want %v", operation, grant.Granted, granted)
	}
	if grant.Lifecycle != lifecycle {
		t.Fatalf("operation %q lifecycle = %q, want %q", operation, grant.Lifecycle, lifecycle)
	}
}

func findOperationGrant(t *testing.T, grants []mobileProtocolOperationGrant, operation string) mobileProtocolOperationGrant {
	t.Helper()
	for _, grant := range grants {
		if grant.ID != operation {
			continue
		}
		return grant
	}
	t.Fatalf("operation %q not found in %#v", operation, grants)
	return mobileProtocolOperationGrant{}
}
