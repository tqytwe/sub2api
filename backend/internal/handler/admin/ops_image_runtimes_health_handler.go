package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// GetImageRuntimesHealth reports readiness without exposing credentials.
// GET /api/v1/admin/ops/image-runtimes/health
func (h *OpsHandler) GetImageRuntimesHealth(c *gin.Context) {
	if h == nil || h.imageRuntimesHealth == nil {
		response.Error(c, http.StatusServiceUnavailable, "Image runtimes health is unavailable")
		return
	}
	health, err := h.imageRuntimesHealth.GetImageRuntimesHealth(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Image runtimes health is unavailable")
		return
	}
	response.Success(c, health)
}
