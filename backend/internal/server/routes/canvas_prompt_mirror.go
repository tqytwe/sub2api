package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterCanvasPromptMirrorRoutes exposes the shared Creation Space prompt
// mirror for mobile image and video creation. The mirror is authenticated
// metadata and stays distinct from administrator-managed prompts.
func RegisterCanvasPromptMirrorRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	if h == nil || h.CanvasPromptMirror == nil {
		return
	}
	v1.GET("/manifest", h.CanvasPromptMirror.Manifest)
	v1.GET("/catalog", h.CanvasPromptMirror.Catalog)
	v1.GET("/catalog/delta", h.CanvasPromptMirror.Delta)
	v1.GET("/categories", h.CanvasPromptMirror.Categories)
	v1.GET("/:id/cover", h.CanvasPromptMirror.Cover)
	v1.GET("/:id", h.CanvasPromptMirror.Get)
}
