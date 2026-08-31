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

func TestAdminPlayGrowthGovernanceRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Play: adminhandler.NewAdminPlayHandler(nil, nil, nil)}}
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers, func(c *gin.Context) {})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, want := range []string{
		"GET /api/v1/admin/play/growth/cohort",
		"GET /api/v1/admin/play/growth/governance",
		"POST /api/v1/admin/play/growth/governance/approve",
		"POST /api/v1/admin/play/growth/governance/revoke",
	} {
		_, ok := routes[want]
		require.Truef(t, ok, "missing route: %s", want)
	}
}

func TestAdminPlayGrowthGovernanceWritesRequireStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Play: adminhandler.NewAdminPlayHandler(nil, nil, nil)}}
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusPreconditionRequired)
	})
	for _, path := range []string{
		"/api/v1/admin/play/growth/governance/approve",
		"/api/v1/admin/play/growth/governance/revoke",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, path)
	}
}
