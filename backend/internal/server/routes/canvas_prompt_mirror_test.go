package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCanvasPromptMirrorRoutesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1/mobile/canvas-prompts")
	RegisterCanvasPromptMirrorRoutes(v1, &handler.Handlers{
		CanvasPromptMirror: handler.NewCanvasPromptMirrorHandler(nil),
	})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /api/v1/mobile/canvas-prompts/manifest",
		"GET /api/v1/mobile/canvas-prompts/catalog",
		"GET /api/v1/mobile/canvas-prompts/catalog/delta",
		"GET /api/v1/mobile/canvas-prompts/categories",
		"GET /api/v1/mobile/canvas-prompts/:id/cover",
		"GET /api/v1/mobile/canvas-prompts/:id",
	} {
		_, ok := routes[expected]
		require.True(t, ok, expected)
	}
}
