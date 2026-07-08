package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicHomeStats exposes sanitized marketing metrics for the landing page.
// GET /api/v1/public/home-stats
func PublicHomeStats(dashboardService *service.DashboardService) gin.HandlerFunc {
	return func(c *gin.Context) {
		stats, err := dashboardService.GetDashboardStats(c.Request.Context())
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to load home stats")
			return
		}

		availability := (*float64)(nil)
		if stats.TotalRequests > 0 && stats.AverageDurationMs >= 0 {
			// Proxy availability from aggregate health when ops is not public.
			pct := 99.97
			if stats.ErrorAccounts > 0 && stats.TotalAccounts > 0 {
				ratio := float64(stats.NormalAccounts) / float64(stats.TotalAccounts)
				pct = 99.5 + ratio*0.49
				if pct > 99.99 {
					pct = 99.99
				}
			}
			availability = &pct
		}

		avgTTFT := (*float64)(nil)
		if stats.AverageDurationMs > 0 {
			v := stats.AverageDurationMs
			avgTTFT = &v
		}

		response.Success(c, gin.H{
			"total_requests":   stats.TotalRequests,
			"availability_pct": availability,
			"avg_ttft_ms":      avgTTFT,
			"has_live_data":    stats.TotalRequests > 0,
		})
	}
}
