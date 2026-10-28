package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelOperationsRouteRepository struct{ service.ChannelRepository }

func TestChannelOperationsRoutesAreRegisteredInsideAdminGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	repo := &channelOperationsRouteRepository{}
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Channel: adminhandler.NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil),
	}}
	admin := router.Group("/api/v1/admin", func(c *gin.Context) {
		if c.GetHeader("X-Admin-Test") != "allow" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})
	registerChannelRoutes(admin, handlers)

	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/admin/channels/7/operations"},
		{"POST", "/api/v1/admin/channels/7/operations"},
		{"GET", "/api/v1/admin/channels/operations/snapshot"},
	} {
		request := httptest.NewRequest(tc.method, tc.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusUnauthorized, response.Code, tc.method+" "+tc.path)
	}
}
