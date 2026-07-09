package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// PublicVIPTiers exposes configured VIP tiers for public docs and marketing pages.
// GET /api/v1/public/vip-tiers
func PublicVIPTiers(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		rt := settingService.GetPlayRuntime(c.Request.Context())
		tiers := rt.VIPTiers
		response.Success(c, gin.H{
			"enabled": len(tiers) > 0,
			"tiers":   tiers,
		})
	}
}
