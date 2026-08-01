package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

type cachedPublicTeamDirectoryRepo struct {
	service.PlayRepository
	calls atomic.Int32
}

func (r *cachedPublicTeamDirectoryRepo) ListPublicTeamDirectory(context.Context, time.Time, time.Time, int) ([]service.PlayTeamDirectoryBase, error) {
	r.calls.Add(1)
	return []service.PlayTeamDirectoryBase{{
		TeamID:      9,
		TeamName:    "缓存验证战队",
		MemberCount: 3,
		Recruiting:  true,
		Spend:       decimal.RequireFromString("12.50000000"),
	}}, nil
}

func TestPublicTeamDirectoryUsesShortCacheAndConditionalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &cachedPublicTeamDirectoryRepo{}
	playHandler := handler.NewPlayHandler(service.NewPlayService(repo, nil, nil, nil, nil, nil), nil)
	router := gin.New()
	router.GET("/play/teams/directory", playHandler.TeamDirectory)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/play/teams/directory", nil))
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, "public, max-age=15, stale-while-revalidate=30", first.Header().Get("Cache-Control"))
	etag := first.Header().Get("ETag")
	require.NotEmpty(t, etag)

	secondRequest := httptest.NewRequest(http.MethodGet, "/play/teams/directory", nil)
	secondRequest.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	router.ServeHTTP(second, secondRequest)
	require.Equal(t, http.StatusNotModified, second.Code)
	require.Empty(t, second.Body.String())
	require.Equal(t, int32(1), repo.calls.Load(), "conditional request must reuse the short-lived aggregate")
}
