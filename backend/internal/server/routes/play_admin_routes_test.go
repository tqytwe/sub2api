package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminPlayTeamRepairRoutesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Play: adminhandler.NewAdminPlayHandler(nil, nil, nil),
		},
	}
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, route := range []string{
		"GET /api/v1/admin/play/teams/:id/member-candidates",
		"POST /api/v1/admin/play/teams/:id/members",
		"GET /api/v1/admin/play/teams/:id/events",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route: %s", route)
	}
}
