package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicGrowthTeaser exposes signup perks and play highlights for landing pages.
// GET /api/v1/public/growth-teaser
func PublicGrowthTeaser(
	settingService *service.SettingService,
	dashboardService *service.DashboardService,
	playHandler *PlayHandler,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		modelCount := 0
		if playHandler != nil {
			modelCount = playHandler.PublicModelCount(c.Request.Context())
		}

		var totalRequests int64
		hasLiveStats := false
		if dashboardService != nil {
			if stats, err := dashboardService.GetDashboardStats(c.Request.Context()); err == nil && stats != nil {
				totalRequests = stats.TotalRequests
				hasLiveStats = stats.TotalRequests > 0
			}
		}

		teaser, err := settingService.BuildPublicGrowthTeaser(c.Request.Context(), modelCount, totalRequests, hasLiveStats)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to load growth teaser")
			return
		}
		response.Success(c, teaser)
	}
}
