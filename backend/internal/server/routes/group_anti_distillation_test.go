package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type antiDistillationRouteGroups struct{ service.GroupRepository }

func (antiDistillationRouteGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	return &service.Group{ID: id, Platform: service.PlatformOpenAI}, nil
}

type antiDistillationRouteSettings struct{ service.SettingRepository }

func (antiDistillationRouteSettings) GetValue(context.Context, string) (string, error) {
	return `{"enabled":true,"pause_minutes":5}`, nil
}

type antiDistillationRouteCache struct {
	service.GroupAntiDistillationCache
}

func (antiDistillationRouteCache) PauseRemaining(context.Context, int64) (time.Duration, error) {
	return time.Minute, nil
}

func (antiDistillationRouteCache) Observe(context.Context, int64, string, bool, time.Duration, time.Time) (bool, time.Time, error) {
	return false, time.Time{}, nil
}

func TestGroupAntiDistillationMountedOnOpenAIGatewayRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	apiKeyService := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, &config.Config{})
	apiKeyService.SetGroupAntiDistillationService(service.NewGroupAntiDistillationService(
		antiDistillationRouteSettings{}, antiDistillationRouteGroups{}, antiDistillationRouteCache{}, nil, nil,
	))
	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
			AsyncImage:    handler.NewAsyncImageHandler(nil, nil, nil),
		},
		middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				UserID:  7,
				GroupID: &groupID,
				Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		apiKeyService, nil, nil, nil, nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1024, TextMaxBodySize: 1024}},
	)

	for _, path := range []string{"/v1/responses", "/responses", "/backend-api/codex/responses"} {
		t.Run(strings.ReplaceAll(strings.Trim(path, "/"), "/", "_"), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"model":"gpt-test"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusTooManyRequests, w.Code)
			require.Equal(t, "60", w.Header().Get("Retry-After"))
			require.Contains(t, w.Body.String(), "group_anti_distillation_paused")
		})
	}
}
