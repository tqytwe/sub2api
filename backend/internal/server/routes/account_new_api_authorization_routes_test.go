package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNewAPIAuthorizationRoutesAreRegisteredOnAdminAccountGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Account: &adminhandler.AccountHandler{}}}
	registerAccountRoutes(admin, handlers, middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }))

	paths := map[string]bool{}
	for _, route := range router.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	require.True(t, paths["GET /api/v1/admin/accounts/:id/upstream-billing-probe/config"])
	require.True(t, paths["POST /api/v1/admin/accounts/:id/upstream-billing-probe/config/preview"])
	require.True(t, paths["PUT /api/v1/admin/accounts/:id/upstream-billing-probe/config"])
	require.True(t, paths["DELETE /api/v1/admin/accounts/:id/upstream-billing-probe/config"])
}
