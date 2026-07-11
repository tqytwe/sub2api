package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterImageStudioRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
) {
	studio := v1.Group("/image-studio")
	{
		studio.GET("/templates", h.ImageStudio.Templates)
	}

	authenticated := v1.Group("/image-studio")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.GET("/estimate", h.ImageStudio.Estimate)
		authenticated.POST("/generate", h.ImageStudio.Generate)
		authenticated.GET("/jobs", h.ImageStudio.ListJobs)
		authenticated.GET("/jobs/:id", h.ImageStudio.GetJob)
		authenticated.DELETE("/jobs/:id", h.ImageStudio.DeleteJob)
	}
}
