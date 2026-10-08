package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckinMilestoneProductionRouteDiagnosis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Play: adminhandler.NewAdminPlayHandler(nil, nil, nil)}}
	registerAdminPlayRoutes(router.Group("/api/v1/admin"), handlers, func(c *gin.Context) { c.AbortWithStatus(http.StatusPreconditionRequired) })
	routes := map[string]bool{}
	for _, r := range router.Routes() {
		routes[r.Method+" "+r.Path] = true
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			require.True(t, routes[method+" /api/v1/admin/play/checkin/milestones"], "route must be registered")
		})
	}
	t.Run("PUT_requires_stepup", func(t *testing.T) {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/v1/admin/play/checkin/milestones", nil))
		require.Equal(t, http.StatusPreconditionRequired, w.Code)
	})
}
