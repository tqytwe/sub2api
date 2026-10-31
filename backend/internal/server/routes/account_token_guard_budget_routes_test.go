package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTokenGuardBudgetResumeRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		AccountTokenGuardV2: adminhandler.NewAccountTokenGuardV2Handler(nil),
	}}
	registerScheduledTestRoutes(router.Group("/api/v1/admin"), handlers)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, ok := routes["POST /api/v1/admin/account-ops/token-guard-v2/accounts/:id/resume-relogin-budget"]
	require.True(t, ok, "resume budget UI endpoint must be registered under admin routes")
}
