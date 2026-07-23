package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMobileLoginSkipsRequiredTurnstile(t *testing.T) {
	handler, client := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyTurnstileEnabled:   "true",
			service.SettingKeyTurnstileSecretKey: "test-secret",
		},
		configureConfig: func(cfg *config.Config) {
			cfg.Server.Mode = "release"
			cfg.Turnstile.Required = true
		},
	})
	ctx := context.Background()

	passwordHash, err := handler.authService.HashPassword("secret-123")
	require.NoError(t, err)
	_, err = client.User.Create().
		SetEmail("mobile-login@example.com").
		SetUsername("mobile-login").
		SetPasswordHash(passwordHash).
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	webRecorder := httptest.NewRecorder()
	webCtx, _ := gin.CreateTestContext(webRecorder)
	webReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/login",
		bytes.NewBufferString(`{"email":"mobile-login@example.com","password":"secret-123"}`),
	)
	webReq.Header.Set("Content-Type", "application/json")
	webCtx.Request = webReq

	handler.Login(webCtx)

	require.Equal(t, http.StatusServiceUnavailable, webRecorder.Code)

	mobileRecorder := httptest.NewRecorder()
	mobileCtx, _ := gin.CreateTestContext(mobileRecorder)
	mobileReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/mobile/login",
		bytes.NewBufferString(`{"email":"mobile-login@example.com","password":"secret-123"}`),
	)
	mobileReq.Header.Set("Content-Type", "application/json")
	mobileCtx.Request = mobileReq

	handler.MobileLogin(mobileCtx)

	require.Equal(t, http.StatusOK, mobileRecorder.Code)
	data := decodeJSONResponseData(t, mobileRecorder)
	require.NotEmpty(t, data["access_token"])
	require.Equal(t, "Bearer", data["token_type"])
}

func TestMobileAuthValidationMessagesFollowAcceptLanguage(t *testing.T) {
	handler := &AuthHandler{}

	for _, tc := range []struct {
		name         string
		language     string
		wantMessage  string
		wantContains string
	}{
		{
			name:        "default chinese",
			wantMessage: "请求参数无效",
		},
		{
			name:        "english",
			language:    "en-US,en;q=0.9",
			wantMessage: "Invalid request",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/mobile/login",
				bytes.NewBufferString(`{}`),
			)
			req.Header.Set("Content-Type", "application/json")
			if tc.language != "" {
				req.Header.Set("Accept-Language", tc.language)
			}
			c.Request = req

			handler.MobileLogin(c)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			body := decodeJSONBody(t, recorder)
			if tc.wantMessage != "" {
				require.Equal(t, tc.wantMessage, body["message"])
			}
			if tc.wantContains != "" {
				require.Contains(t, body["message"], tc.wantContains)
			}
		})
	}
}

func TestMobileLoginInvalidCredentialsMessagesFollowAcceptLanguage(t *testing.T) {
	handler, _ := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{})

	for _, tc := range []struct {
		name        string
		language    string
		wantMessage string
	}{
		{
			name:        "default chinese",
			wantMessage: "邮箱或密码错误",
		},
		{
			name:        "english",
			language:    "en-US,en;q=0.9",
			wantMessage: "Invalid email or password",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/mobile/login",
				bytes.NewBufferString(`{"email":"missing-mobile-user@example.com","password":"secret-123"}`),
			)
			req.Header.Set("Content-Type", "application/json")
			if tc.language != "" {
				req.Header.Set("Accept-Language", tc.language)
			}
			c.Request = req

			handler.MobileLogin(c)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			body := decodeJSONBody(t, recorder)
			require.Equal(t, tc.wantMessage, body["message"])
			require.Equal(t, "INVALID_CREDENTIALS", body["reason"])
		})
	}
}

func TestMobileRegisterSkipsRequiredTurnstile(t *testing.T) {
	handler, _ := newOAuthPendingFlowTestHandlerWithDependencies(t, oauthPendingFlowTestHandlerOptions{
		settingValues: map[string]string{
			service.SettingKeyTurnstileEnabled:   "true",
			service.SettingKeyTurnstileSecretKey: "test-secret",
		},
		configureConfig: func(cfg *config.Config) {
			cfg.Server.Mode = "release"
			cfg.Turnstile.Required = true
		},
	})

	webRecorder := httptest.NewRecorder()
	webCtx, _ := gin.CreateTestContext(webRecorder)
	webReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		bytes.NewBufferString(`{"email":"mobile-register-web@example.com","password":"secret-123"}`),
	)
	webReq.Header.Set("Content-Type", "application/json")
	webCtx.Request = webReq

	handler.Register(webCtx)

	require.Equal(t, http.StatusServiceUnavailable, webRecorder.Code)

	mobileRecorder := httptest.NewRecorder()
	mobileCtx, _ := gin.CreateTestContext(mobileRecorder)
	mobileReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/mobile/register",
		bytes.NewBufferString(`{"email":"mobile-register@example.com","password":"secret-123"}`),
	)
	mobileReq.Header.Set("Content-Type", "application/json")
	mobileCtx.Request = mobileReq

	handler.MobileRegister(mobileCtx)

	require.Equal(t, http.StatusOK, mobileRecorder.Code)
	data := decodeJSONResponseData(t, mobileRecorder)
	require.NotEmpty(t, data["access_token"])
	require.Equal(t, "Bearer", data["token_type"])
}
