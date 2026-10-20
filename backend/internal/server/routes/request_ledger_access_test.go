package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func routeSet(router *gin.Engine) map[string]struct{} {
	result := make(map[string]struct{})
	for _, route := range router.Routes() {
		result[route.Method+" "+route.Path] = struct{}{}
	}
	return result
}

func TestRequestLedgerIsAdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handlers := &handler.Handlers{
		Admin:         &handler.AdminHandlers{},
		RequestLedger: &handler.RequestLedgerHandler{},
	}
	userRouter := gin.New()
	RegisterUserRoutes(
		userRouter.Group("/api/v1"), handlers,
		middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)
	adminRouter := gin.New()
	RegisterAdminRoutes(
		adminRouter.Group("/api/v1"), handlers,
		middleware.AdminAuthMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)

	userRoutes := routeSet(userRouter)
	for _, route := range []string{
		"GET /api/v1/requests",
		"GET /api/v1/requests/:id",
		"GET /api/v1/requests/:id/usage/:usage_id",
	} {
		_, ok := userRoutes[route]
		require.Falsef(t, ok, "ordinary user route must not exist: %s", route)
	}

	adminRoutes := routeSet(adminRouter)
	for _, route := range []string{
		"GET /api/v1/admin/requests",
		"GET /api/v1/admin/requests/:id",
		"GET /api/v1/admin/requests/:id/usage/:usage_id",
	} {
		_, ok := adminRoutes[route]
		require.Truef(t, ok, "administrator route must exist: %s", route)
	}
}
