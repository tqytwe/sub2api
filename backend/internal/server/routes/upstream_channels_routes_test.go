package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUpstreamChannelRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "channel-operations-auth-test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 7, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 2}
	userRepo := &modelBillingAuthUserRepo{user: user}
	userService := service.NewUserService(userRepo, nil, nil, nil)
	token, err := authService.GenerateToken(context.Background(), user)
	require.NoError(t, err)

	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Channel: adminhandler.NewChannelHandler(nil, nil, nil),
	}}
	adminAuth := servermiddleware.NewAdminAuthMiddleware(authService, userService, nil, nil)
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)

	for _, route := range []struct{ method, path string }{
		{method: http.MethodGet, path: "/api/v1/admin/profit-operations/snapshot"},
		{method: http.MethodGet, path: "/api/v1/admin/upstream-channels"},
		{method: http.MethodPost, path: "/api/v1/admin/upstream-channels"},
		{method: http.MethodGet, path: "/api/v1/admin/upstream-channels/1"},
		{method: http.MethodPut, path: "/api/v1/admin/upstream-channels/1"},
		{method: http.MethodDelete, path: "/api/v1/admin/upstream-channels/1"},
		{method: http.MethodGet, path: "/api/v1/admin/upstream-channels/1/members"},
		{method: http.MethodGet, path: "/api/v1/admin/upstream-channel-bindings"},
		{method: http.MethodPut, path: "/api/v1/admin/upstream-channel-bindings"},
	} {
		for _, tc := range []struct {
			name, auth string
			wantStatus int
			wantCode   string
			wantReads  int
		}{
			{name: "unauthenticated", wantStatus: http.StatusUnauthorized, wantCode: "UNAUTHORIZED"},
			{name: "invalid-token", auth: "Bearer invalid-token", wantStatus: http.StatusUnauthorized, wantCode: "INVALID_TOKEN"},
			{name: "authenticated-non-admin", auth: "Bearer " + token, wantStatus: http.StatusForbidden, wantCode: "FORBIDDEN", wantReads: 1},
		} {
			t.Run(route.method+"/"+route.path+"/"+tc.name, func(t *testing.T) {
				readsBefore := userRepo.reads
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(route.method, route.path, nil)
				if tc.auth != "" {
					request.Header.Set("Authorization", tc.auth)
				}
				router.ServeHTTP(recorder, request)
				require.Equal(t, tc.wantStatus, recorder.Code)
				require.Contains(t, recorder.Body.String(), tc.wantCode)
				require.Equal(t, tc.wantReads, userRepo.reads-readsBefore, "valid JWT must load the current user's role before authorization")
			})
		}
	}
}
