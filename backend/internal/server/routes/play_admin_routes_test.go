package routes

import (
	"net/http"
	"net/http/httptest"
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
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers, func(c *gin.Context) {})

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

func TestAdminPlayFinancialWritesRequireStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Play: adminhandler.NewAdminPlayHandler(nil, nil, nil)}}
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusPreconditionRequired)
	})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/play/campaigns"},
		{http.MethodPut, "/api/v1/admin/play/campaigns/12"},
		{http.MethodDelete, "/api/v1/admin/play/campaigns/12"},
		{http.MethodPut, "/api/v1/admin/play/membership/vip-config"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, tc.method+" "+tc.path)
	}
}
