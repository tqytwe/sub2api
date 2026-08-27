package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterPromptLibraryRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
) {
	v1.GET("/prompts/manifest", h.PromptLibrary.Manifest)
	v1.GET("/prompts/catalog/delta", h.PromptLibrary.CatalogDelta)
	v1.GET("/prompts/catalog", h.PromptLibrary.Catalog)
	v1.GET("/prompts", middleware.OptionalJWTAuth(jwtAuth), h.PromptLibrary.List)
	v1.GET("/prompts/:id", middleware.OptionalJWTAuth(jwtAuth), h.PromptLibrary.Get)
	v1.GET("/prompt-categories", h.PromptLibrary.Categories)

	authenticated := v1.Group("/prompts")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.POST("/:id/favorite", h.PromptLibrary.Favorite)
		authenticated.DELETE("/:id/favorite", h.PromptLibrary.Unfavorite)
		authenticated.POST("/:id/use", h.PromptLibrary.Use)
	}
}

func RegisterPromptLibrarySEORoutes(
	r gin.IRoutes,
	h *handler.Handlers,
) {
	r.GET("/home", h.PromptLibrary.HomeRedirect)
	r.GET("/sitemap.xml", h.PromptLibrary.Sitemap)
	r.GET("/robots.txt", h.PromptLibrary.Robots)
	r.GET("/llms.txt", h.PromptLibrary.LLMSTxt)
	r.GET("/llms-full.txt", h.PromptLibrary.LLMSFullTxt)
	r.GET("/llms.small-txt", h.PromptLibrary.LLMSSmallTxt)
	r.GET("/.well-known/ai.txt", h.PromptLibrary.AITxt)
}
