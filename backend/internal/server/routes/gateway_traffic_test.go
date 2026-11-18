package routes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayTrafficRepository struct {
	service.UsageLogRepository
	events chan service.TrafficEvent
}

func (r *gatewayTrafficRepository) RecordTrafficBatch(_ context.Context, events []service.TrafficEvent) error {
	for _, event := range events {
		r.events <- event
	}
	return nil
}

func TestGatewayRoutesMeterAuthenticatedBodyTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &gatewayTrafficRepository{events: make(chan service.TrafficEvent, 16)}
	usage := admin.NewUsageHandler(service.NewUsageService(repo, nil, nil, nil), nil, nil, nil)
	dir := t.TempDir()
	stop, err := usage.ConfigureTrafficDelivery(func() string { return dir })
	require.NoError(t, err)
	t.Cleanup(stop)

	router := gin.New()
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "" {
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 12, UserID: 34})
		}
		_, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.AbortWithStatus(http.StatusForbidden)
		_, _ = c.Writer.WriteString("denied")
	})
	RegisterGatewayRoutes(router, &handler.Handlers{
		Admin:         &handler.AdminHandlers{Usage: usage},
		Gateway:       &handler.GatewayHandler{},
		OpenAIGateway: &handler.OpenAIGatewayHandler{},
		AsyncImage:    handler.NewAsyncImageHandler(nil, nil, nil),
	}, auth, nil, nil, nil, nil, nil, &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1024, TextMaxBodySize: 1024}})

	request := func(method, path string, authenticated bool) {
		req := httptest.NewRequest(method, path, strings.NewReader("payload"))
		if authenticated {
			req.Header.Set("Authorization", "Bearer fixture")
			req.Header.Set("Upgrade", "websocket")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	}
	request(http.MethodPost, "/v1/messages", false)
	request(http.MethodGet, "/v1/responses", true)
	request(http.MethodPost, "/backend-api/codex/realtime/calls", true)
	for _, path := range []string{"/v1/messages", "/responses", "/backend-api/codex/responses", "/images/generations", "/antigravity/v1/messages"} {
		request(http.MethodPost, path, true)
		select {
		case event := <-repo.events:
			require.Equal(t, path, event.Endpoint)
			require.EqualValues(t, 34, event.UserID)
			require.EqualValues(t, 12, event.APIKeyID)
			require.EqualValues(t, len("payload"), event.UploadBytes)
			require.EqualValues(t, len("denied"), event.DownloadBytes)
			require.Equal(t, "http_error", event.Outcome)
		case <-time.After(3 * time.Second):
			t.Fatalf("gateway route %s did not record traffic", path)
		}
	}
}
