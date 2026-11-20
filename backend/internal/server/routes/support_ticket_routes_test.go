package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSupportTicketRoutesRequireTheExpectedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}, SupportTicket: &handler.SupportTicketHandler{}}
	pass := func(c *gin.Context) { c.Next() }
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	admin := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("X-Test-Session") == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.AbortWithStatus(http.StatusForbidden)
	})
	RegisterUserRoutes(router.Group("/api/v1"), handlers, jwt, middleware.AuditLogMiddleware(pass), nil, nil)
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, admin, middleware.AuditLogMiddleware(pass), middleware.StepUpAuthMiddleware(pass), nil, nil)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/support-tickets"},
		{http.MethodPost, "/api/v1/support-tickets"},
		{http.MethodGet, "/api/v1/support-tickets/summary"},
		{http.MethodGet, "/api/v1/support-tickets/42"},
		{http.MethodPost, "/api/v1/support-tickets/42/messages"},
		{http.MethodPost, "/api/v1/support-tickets/42/close"},
		{http.MethodPost, "/api/v1/support-tickets/42/reopen"},
		{http.MethodGet, "/api/v1/admin/support-tickets"},
		{http.MethodGet, "/api/v1/admin/support-tickets/summary"},
		{http.MethodGet, "/api/v1/admin/support-tickets/42"},
		{http.MethodPost, "/api/v1/admin/support-tickets/42/messages"},
		{http.MethodPost, "/api/v1/admin/support-tickets/42/status"},
		{http.MethodDelete, "/api/v1/admin/support-tickets/42"},
	} {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			anonymous := httptest.NewRequest(request.method, request.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, anonymous)
			require.Equal(t, http.StatusUnauthorized, response.Code)
			if strings.HasPrefix(request.path, "/api/v1/admin/") {
				ordinary := httptest.NewRequest(request.method, request.path, nil)
				ordinary.Header.Set("X-Test-Session", "ordinary")
				response = httptest.NewRecorder()
				router.ServeHTTP(response, ordinary)
				require.Equal(t, http.StatusForbidden, response.Code)
			}
		})
	}
}
