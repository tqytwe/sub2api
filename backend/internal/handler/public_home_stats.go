package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const publicHomeStatsCacheTTL = 60 * time.Second

type publicHomeStatsCache struct {
	mu        sync.Mutex
	snapshot  *service.PublicHomeStats
	expiresAt time.Time
}

// PublicHomeStats exposes authoritative production metrics for the landing page.
func PublicHomeStats(statsService *service.PublicHomeStatsService) gin.HandlerFunc {
	return newPublicHomeStatsHandler(statsService, time.Now)
}

func newPublicHomeStatsHandler(statsService *service.PublicHomeStatsService, now func() time.Time) gin.HandlerFunc {
	cache := &publicHomeStatsCache{}
	return func(c *gin.Context) {
		cache.mu.Lock()
		defer cache.mu.Unlock()

		requestedAt := now()
		if cache.snapshot != nil && requestedAt.Before(cache.expiresAt) {
			response.Success(c, cache.snapshot)
			return
		}

		stats, err := statsService.Get(c.Request.Context())
		if err != nil {
			if cache.snapshot != nil {
				response.Success(c, cache.snapshot)
				return
			}
			response.Error(c, http.StatusInternalServerError, "failed to load home stats")
			return
		}

		cache.snapshot = stats
		cache.expiresAt = requestedAt.Add(publicHomeStatsCacheTTL)
		response.Success(c, stats)
	}
}
