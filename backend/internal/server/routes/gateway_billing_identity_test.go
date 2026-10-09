package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayBillingIdentityCoversGatewayMounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	var identity string
	router.Use(func(c *gin.Context) {
		c.Next()
		identity, _ = c.Request.Context().Value(ctxkey.UsageBillingRequestID).(string)
	})
	RegisterGatewayRoutes(router, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil, nil)},
		middleware.APIKeyAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }), nil, nil, nil, nil, nil,
		&config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1024, TextMaxBodySize: 1024}})
	for _, path := range []string{
		"/v1/messages", "/v1/responses", "/v1/chat/completions", "/v1beta/models/gemini:generateContent",
		"/responses", "/chat/completions", "/embeddings", "/images/generations", "/images/generations/async",
		"/backend-api/codex/responses", "/videos", "/tts", "/stt", "/api/v3/contents/generations/tasks",
		"/v3/contents/generations/tasks", "/contents/generations/tasks", "/antigravity/v1/messages", "/antigravity/v1beta/models/gemini:generateContent",
	} {
		t.Run(path, func(t *testing.T) {
			identity = ""
			request := httptest.NewRequest(http.MethodPost, path, nil)
			request.Header.Set("X-Client-Request-ID", "shared")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.NotEqual(t, http.StatusNotFound, response.Code)
			require.NotEmpty(t, identity, "every mounted gateway must create its private settlement identity")
			require.Equal(t, "shared", response.Header().Get("X-Client-Request-ID"))
		})
	}
	for _, path := range []string{"/v1/responses", "/responses", "/backend-api/codex/responses", "/v1/realtime", "/realtime"} {
		t.Run("GET "+path, func(t *testing.T) {
			identity = ""
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.Header.Set("X-Client-Request-ID", "shared")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.NotEqual(t, http.StatusNotFound, response.Code)
			require.NotEmpty(t, identity)
			require.Equal(t, "shared", response.Header().Get("X-Client-Request-ID"))
		})
	}

}
