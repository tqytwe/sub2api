package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const publicTeamCompetitionCacheTTL = 15 * time.Second

type publicTeamCompetitionCacheEntry struct {
	payload   any
	etag      string
	expiresAt time.Time
}

// publicTeamCompetitionCache only stores team-level, anonymous aggregates.
// Personal progress and authenticated team data never enter this cache.
type publicTeamCompetitionCache struct {
	mu      sync.Mutex
	entries map[string]publicTeamCompetitionCacheEntry
}

func (cache *publicTeamCompetitionCache) load(key string, loader func() (any, error)) (any, string, error) {
	now := time.Now()
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok && now.Before(entry.expiresAt) {
		cache.mu.Unlock()
		return entry.payload, entry.etag, nil
	}
	cache.mu.Unlock()

	payload, err := loader()
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`

	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = make(map[string]publicTeamCompetitionCacheEntry)
	}
	// Keep an unbounded query-string fanout from becoming an in-process cache leak.
	if len(cache.entries) >= 64 {
		cache.entries = make(map[string]publicTeamCompetitionCacheEntry)
	}
	cache.entries[key] = publicTeamCompetitionCacheEntry{payload: payload, etag: etag, expiresAt: now.Add(publicTeamCompetitionCacheTTL)}
	cache.mu.Unlock()
	return payload, etag, nil
}

func respondPublicTeamCompetition(c *gin.Context, payload any, etag string) {
	c.Header("Cache-Control", "public, max-age=15, stale-while-revalidate=30")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	response.Success(c, payload)
}
