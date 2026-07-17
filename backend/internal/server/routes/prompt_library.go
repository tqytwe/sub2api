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
	v1.GET("/prompts", middleware.OptionalJWTAuth(jwtAuth), h.PromptLibrary.List)
	v1.GET("/prompts/:id", middleware.OptionalJWTAuth(jwtAuth), h.PromptLibrary.Get)
	v1.GET("/prompt-categories", h.PromptLibrary.Categories)

	authenticated := v1.Group("/prompts")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.POST("/:id/favorite", h.PromptLibrary.Favorite)
		authenticated.DELETE("/:id/favorite", h.PromptLibrary.Unfavorite)
		authenticated.POST("/:id/use", h.PromptLibrary.Use)
		authenticated.POST("/:id/report", h.PromptLibrary.Report)
	}
}

func RegisterPromptLibrarySEORoutes(
	r gin.IRoutes,
	h *handler.Handlers,
) {
	r.GET("/sitemap.xml", h.PromptLibrary.Sitemap)
	r.GET("/robots.txt", h.PromptLibrary.Robots)
}

func registerAdminPromptLibraryRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	prompts := admin.Group("/prompts")
	{
		prompts.GET("", h.Admin.PromptLibrary.List)
		prompts.POST("", h.Admin.PromptLibrary.Create)
		prompts.GET("/:id", h.Admin.PromptLibrary.Get)
		prompts.PUT("/:id", h.Admin.PromptLibrary.Update)
		prompts.POST("/:id/submit-review", h.Admin.PromptLibrary.SubmitReview)
		prompts.POST("/:id/approve", h.Admin.PromptLibrary.Approve)
		prompts.POST("/:id/offline", h.Admin.PromptLibrary.Offline)
		prompts.POST("/:id/rollback", h.Admin.PromptLibrary.Rollback)

		prompts.POST("/import-jobs", h.Admin.PromptLibrary.CreateImportJob)
		prompts.GET("/import-jobs", h.Admin.PromptLibrary.ListImportJobs)
		prompts.GET("/import-jobs/:id", h.Admin.PromptLibrary.GetImportJob)
		prompts.GET("/import-items", h.Admin.PromptLibrary.ListImportItems)
		prompts.POST("/import-items/:id/approve", h.Admin.PromptLibrary.ApproveImportItem)
		prompts.POST("/import-items/:id/reject", h.Admin.PromptLibrary.RejectImportItem)
		prompts.GET("/reports", h.Admin.PromptLibrary.ListReports)
		prompts.POST("/reports/:id/resolve", h.Admin.PromptLibrary.ResolveReport)
	}

	categories := admin.Group("/prompt-categories")
	{
		categories.GET("", h.Admin.PromptLibrary.ListCategories)
		categories.POST("", h.Admin.PromptLibrary.CreateCategory)
		categories.PUT("/:id", h.Admin.PromptLibrary.UpdateCategory)
		categories.DELETE("/:id", h.Admin.PromptLibrary.DeleteCategory)
	}
}
