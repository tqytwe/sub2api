package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
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
		Admin: &handler.AdminHandlers{
			PromptLibrary: adminhandler.NewPromptLibraryHandler(nil),
		},
	}
	jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() })

	RegisterPromptLibraryRoutes(v1, handlers, jwt)
	RegisterPromptLibrarySEORoutes(router, handlers)
	admin := v1.Group("/admin")
	registerAdminPromptLibraryRoutes(admin, handlers)

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
		"GET /llms.txt",
		"GET /api/v1/admin/prompts",
		"POST /api/v1/admin/prompts",
		"GET /api/v1/admin/prompts/:id",
		"PUT /api/v1/admin/prompts/:id",
		"POST /api/v1/admin/prompts/:id/submit-review",
		"POST /api/v1/admin/prompts/:id/approve",
		"POST /api/v1/admin/prompts/:id/offline",
		"POST /api/v1/admin/prompts/:id/rollback",
		"GET /api/v1/admin/prompt-categories",
		"POST /api/v1/admin/prompt-categories",
		"PUT /api/v1/admin/prompt-categories/:id",
		"DELETE /api/v1/admin/prompt-categories/:id",
		"POST /api/v1/admin/prompts/import-jobs",
		"GET /api/v1/admin/prompts/import-jobs/:id",
		"GET /api/v1/admin/prompts/import-items",
		"POST /api/v1/admin/prompts/import-items/:id/approve",
		"POST /api/v1/admin/prompts/import-items/:id/reject",
		"GET /api/v1/admin/prompts/reports",
		"POST /api/v1/admin/prompts/reports/:id/resolve",
	} {
		_, ok := got[route]
		require.True(t, ok, route)
	}
}
