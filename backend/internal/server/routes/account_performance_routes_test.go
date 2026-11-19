package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountPerformanceRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	account := adminhandler.NewAccountHandler(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	admin := router.Group("/api/v1/admin")
	stepUp := middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	registerAccountRoutes(admin, &handler.Handlers{Admin: &handler.AdminHandlers{Account: account}}, stepUp)

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"POST /api/v1/admin/accounts/performance",
		"PUT /api/v1/admin/accounts/:id/performance",
		"POST /api/v1/admin/accounts/:id/performance/release",
		"GET /api/v1/admin/accounts/:id/performance/events",
	} {
		require.True(t, registered[route], "missing route %s", route)
	}
}

func TestAccountPerformanceSettingsRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	registerSettingsRoutes(admin, &handler.Handlers{Admin: &handler.AdminHandlers{
		Setting: adminhandler.NewSettingHandler(nil, nil, nil, nil, nil, nil, nil),
	}}, middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }))

	registered := map[string]bool{}
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	require.True(t, registered["GET /api/v1/admin/settings/account-performance"])
	require.True(t, registered["PUT /api/v1/admin/settings/account-performance"])
}
