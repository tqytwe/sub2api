package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountOpsRoutesRejectGuestAndOrdinaryUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "account-ops-auth-fixture", ExpireHour: 1}}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	user := &service.User{ID: 7, Role: service.RoleUser, Status: service.StatusActive}
	repo := &modelBillingAuthUserRepo{user: user}
	token, err := auth.GenerateToken(context.Background(), user)
	require.NoError(t, err)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{AccountOps: adminhandler.NewAccountOpsHandler(nil, nil)}}
	adminAuth := middleware.NewAdminAuthMiddleware(auth, service.NewUserService(repo, nil, nil, nil), nil, nil)
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth,
		func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() }, nil, nil)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/config"},
		{http.MethodPut, "/config"},
		{http.MethodPut, "/notification-settings"},
		{http.MethodPut, "/webhooks/fixture"},
		{http.MethodDelete, "/webhooks/fixture"},
		{http.MethodPost, "/webhooks/fixture/test"},
		{http.MethodPut, "/rules/batch"},
		{http.MethodPut, "/rules/1"},
		{http.MethodDelete, "/rules/1"},
		{http.MethodGet, "/alerts"},
		{http.MethodGet, "/balance-accounts"},
		{http.MethodGet, "/threshold-accounts"},
		{http.MethodGet, "/groups"},
		{http.MethodPut, "/groups"},
	} {
		for _, loggedIn := range []bool{false, true} {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(route.method, "/api/v1/admin/account-ops"+route.path, nil)
			want := http.StatusUnauthorized
			if loggedIn {
				request.Header.Set("Authorization", "Bearer "+token)
				want = http.StatusForbidden
			}
			router.ServeHTTP(recorder, request)
			require.Equal(t, want, recorder.Code, route.path)
		}
	}
}
