package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPromptLibraryRoutesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	handlers := &handler.Handlers{
		PromptLibrary: handler.NewPromptLibraryHandler(nil),
	}
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })

	RegisterPromptLibraryRoutes(v1, handlers, jwt)
	RegisterPromptLibrarySEORoutes(router, handlers)
	got := make(map[string]struct{})
	for _, route := range router.Routes() {
		got[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/prompts",
		"GET /api/v1/prompts/:id",
		"GET /api/v1/prompt-categories",
		"POST /api/v1/prompts/:id/favorite",
		"DELETE /api/v1/prompts/:id/favorite",
		"POST /api/v1/prompts/:id/use",
		"GET /sitemap.xml",
		"GET /robots.txt",
		"GET /home",
		"GET /llms.txt",
		"GET /llms-full.txt",
		"GET /llms.small-txt",
		"GET /.well-known/ai.txt",
	} {
		_, ok := got[route]
		require.True(t, ok, route)
	}
	for route := range got {
		require.NotContains(t, route, "/api/v1/admin/prompts")
		require.NotContains(t, route, "/api/v1/admin/prompt-categories")
	}
}
