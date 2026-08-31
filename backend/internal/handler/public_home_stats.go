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

type publicStatusSummaryCache struct {
	mu        sync.Mutex
	snapshot  *service.PublicStatusSummary
	expiresAt time.Time
}

// PublicHomeStats exposes authoritative production metrics for the landing page.
func PublicHomeStats(statsService *service.PublicHomeStatsService) gin.HandlerFunc {
	return newPublicHomeStatsHandler(statsService, time.Now)
}

// PublicStatusSummary exposes the last completed, immutable status snapshot.
func PublicStatusSummary(statsService *service.PublicStatusSummaryService) gin.HandlerFunc {
	return newPublicStatusSummaryHandler(statsService, time.Now)
}

func newPublicStatusSummaryHandler(statsService *service.PublicStatusSummaryService, now func() time.Time) gin.HandlerFunc {
	cache := &publicStatusSummaryCache{}
	return func(c *gin.Context) {
		cache.mu.Lock()
		defer cache.mu.Unlock()

		requestedAt := now()
		if cache.snapshot != nil && requestedAt.Before(cache.expiresAt) {
			response.Success(c, statsService.RecomputeFreshnessAt(cache.snapshot, requestedAt))
			return
		}

		summary, err := statsService.Get(c.Request.Context())
		if err != nil {
			if cache.snapshot != nil {
				response.Success(c, statsService.RecomputeFreshnessAt(cache.snapshot, requestedAt))
				return
			}
			response.Error(c, http.StatusInternalServerError, "failed to load public status")
			return
		}

		cache.snapshot = summary
		cache.expiresAt = requestedAt.Add(publicHomeStatsCacheTTL)
		response.Success(c, summary)
	}
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
