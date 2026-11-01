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

type modelBillingAuthUserRepo struct {
	service.UserRepository
	user  *service.User
	reads int
}

func (r *modelBillingAuthUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	r.reads++
	if id != r.user.ID {
		return nil, service.ErrUserNotFound
	}
	user := *r.user
	return &user, nil
}

func (r *modelBillingAuthUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

func TestModelBillingRoutesRequireAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "model-billing-auth-test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 7, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 2}
	userRepo := &modelBillingAuthUserRepo{user: user}
	userService := service.NewUserService(userRepo, nil, nil, nil)
	token, err := authService.GenerateToken(context.Background(), user)
	require.NoError(t, err)

	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Setting: adminhandler.NewSettingHandler(nil, nil, nil, nil, nil, nil, nil),
	}}
	adminAuth := servermiddleware.NewAdminAuthMiddleware(authService, userService, nil, nil)
	auditLog := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, auditLog, stepUp, nil, nil)

	for _, route := range []struct{ method, path string }{
		{method: http.MethodGet, path: "/api/v1/admin/model-billing/config"},
		{method: http.MethodPut, path: "/api/v1/admin/model-billing/config"},
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
			t.Run(route.path+"/"+tc.name, func(t *testing.T) {
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
