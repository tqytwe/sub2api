package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestMobileProtocolIncludesCanonicalLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
			Version      int                      `json:"version"`
			Session      map[string]any           `json:"session"`
			TaskStatuses []string                 `json:"task_statuses"`
			Endpoints    []mobileProtocolEndpoint `json:"endpoints"`
			Privacy      map[string]any           `json:"privacy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != 0 || envelope.Data.Version != mobileProtocolVersion {
		t.Fatalf("unexpected envelope: code=%d version=%d", envelope.Code, envelope.Data.Version)
	}
	if got := envelope.Data.Session["refresh_path"]; got != "/api/v1/auth/refresh" {
		t.Fatalf("refresh path = %v", got)
	}
	assertStringSliceContains(t, envelope.Data.TaskStatuses, "streaming")
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodGet, "/api/v1/mobile/account-summary", "canonical")
	assertEndpointContains(t, envelope.Data.Endpoints, http.MethodPost, "/api/v1/play/mobile-feedback", "legacy")
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
				Admin struct {
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
				Admin struct {
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
