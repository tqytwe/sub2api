package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type publicHomeStatsRepoSequence struct {
	mu      sync.Mutex
	results []service.PublicHomeStatsRaw
	errors  []error
	calls   int
}

func (s *publicHomeStatsRepoSequence) GetPublicHomeStats(context.Context, time.Time) (service.PublicHomeStatsRaw, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.calls
	s.calls++
	if index < len(s.errors) && s.errors[index] != nil {
		return service.PublicHomeStatsRaw{}, s.errors[index]
	}
	if index < len(s.results) {
		return s.results[index], nil
	}
	return service.PublicHomeStatsRaw{}, errors.New("unexpected repository call")
}

type publicHomeStatsEnvelope struct {
	Code int                     `json:"code"`
	Data service.PublicHomeStats `json:"data"`
}

type publicStatusSummaryRepoSequence struct {
	results []service.PublicStatusSummaryRaw
	errors  []error
	calls   int
}

func (s *publicStatusSummaryRepoSequence) GetPublicStatusSummary(context.Context) (service.PublicStatusSummaryRaw, error) {
	index := s.calls
	s.calls++
	if index < len(s.errors) && s.errors[index] != nil {
		return service.PublicStatusSummaryRaw{}, s.errors[index]
	}
	if index < len(s.results) {
		return s.results[index], nil
	}
	return service.PublicStatusSummaryRaw{}, errors.New("unexpected repository call")
}

func (s *publicStatusSummaryRepoSequence) ListPublicStatusSnapshotWindowEnds(context.Context, time.Time, time.Time) ([]time.Time, error) {
	return nil, nil
}

func (s *publicStatusSummaryRepoSequence) IsPublicStatusSnapshotWindowReady(context.Context, time.Time) (bool, error) {
	return false, nil
}

func (s *publicStatusSummaryRepoSequence) RefreshPublicStatusSnapshot(context.Context, time.Time) error {
	return nil
}

type publicStatusSummaryEnvelope struct {
	Code int                         `json:"code"`
	Data service.PublicStatusSummary `json:"data"`
}

func TestPublicHomeStatsMapsRealSnapshotAndNulls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	through := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	repo := &publicHomeStatsRepoSequence{results: []service.PublicHomeStatsRaw{{
		TotalRequests:  42,
		OpsDataThrough: &through,
	}}}
	svc := service.NewPublicHomeStatsService(repo)
	now := time.Date(2026, 7, 16, 2, 0, 0, 0, time.UTC)
	handler := newPublicHomeStatsHandler(svc, func() time.Time { return now })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/home-stats", nil)
	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "99.97")
	require.NotContains(t, recorder.Body.String(), "has_live_data")
	var body publicHomeStatsEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, int64(42), body.Data.TotalRequests)
	require.Nil(t, body.Data.AvailabilityPct)
	require.Nil(t, body.Data.AvgTTFTMs)
	require.Equal(t, &through, body.Data.OpsDataThrough)
	require.False(t, body.Data.ComputedAt.IsZero())
}

func TestPublicHomeStatsFallsBackOnlyToLastRealSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &publicHomeStatsRepoSequence{
		results: []service.PublicHomeStatsRaw{{TotalRequests: 17}},
		errors:  []error{nil, errors.New("database unavailable")},
	}
	svc := service.NewPublicHomeStatsService(repo)
	now := time.Date(2026, 7, 16, 2, 0, 0, 0, time.UTC)
	handler := newPublicHomeStatsHandler(svc, func() time.Time { return now })

	first := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(first)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/home-stats", nil)
	handler(c1)
	require.Equal(t, http.StatusOK, first.Code)

	now = now.Add(61 * time.Second)
	second := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(second)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/home-stats", nil)
	handler(c2)
	require.Equal(t, http.StatusOK, second.Code)
	require.JSONEq(t, first.Body.String(), second.Body.String())
	require.Equal(t, 2, repo.calls)
}

func TestPublicHomeStatsFailsWithoutRealCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &publicHomeStatsRepoSequence{errors: []error{errors.New("database unavailable")}}
	svc := service.NewPublicHomeStatsService(repo)
	handler := newPublicHomeStatsHandler(svc, time.Now)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/home-stats", nil)
	handler(c)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "total_requests")
}

func TestPublicStatusSummaryReturnsPersistedWindowAndSamples(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now().UTC()
	through := now.Add(-time.Hour)
	availability := 99.5
	p50 := 420.0
	p95 := 910.0
	repo := &publicStatusSummaryRepoSequence{results: []service.PublicStatusSummaryRaw{{
		HasSnapshot:             true,
		TotalRequests:           42,
		AvailabilityPct:         &availability,
		AvailabilitySampleCount: 40,
		TTFTP50Ms:               &p50,
		TTFTP95Ms:               &p95,
		TTFTSampleCount:         9,
		WindowEnd:               now.Truncate(time.Hour),
		DataThrough:             &through,
		ComputedAt:              now.Add(-time.Minute),
	}}}
	svc := service.NewPublicStatusSummaryService(repo)
	h := newPublicStatusSummaryHandler(svc, func() time.Time { return now })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/status-summary", nil)
	h(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body publicStatusSummaryEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, int64(42), body.Data.TotalRequests)
	require.Equal(t, int64(40), body.Data.Availability.SampleCount)
	require.Equal(t, int64(9), body.Data.TTFT.SampleCount)
	require.Equal(t, now.Truncate(time.Hour), *body.Data.Availability.WindowEnd)
	require.Equal(t, now.Truncate(time.Hour).Add(-30*24*time.Hour), *body.Data.Availability.WindowStart)
	require.Equal(t, now.Truncate(time.Hour), *body.Data.TTFT.WindowEnd)
	require.Equal(t, now.Truncate(time.Hour).Add(-24*time.Hour), *body.Data.TTFT.WindowStart)
	require.Equal(t, service.PublicStatusFreshnessFresh, body.Data.Freshness)
}

func TestPublicStatusSummaryRecomputesFreshnessWhenServingCachedSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initialNow := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	through := initialNow.Add(-89*time.Minute - 50*time.Second)
	availability := 99.5
	p50 := 420.0
	p95 := 910.0
	repo := &publicStatusSummaryRepoSequence{results: []service.PublicStatusSummaryRaw{{
		HasSnapshot:             true,
		AvailabilityPct:         &availability,
		AvailabilitySampleCount: 40,
		TTFTP50Ms:               &p50,
		TTFTP95Ms:               &p95,
		TTFTSampleCount:         9,
		WindowEnd:               initialNow.Truncate(time.Hour),
		DataThrough:             &through,
		ComputedAt:              initialNow,
	}}, errors: []error{nil, errors.New("database unavailable")}}
	svc := service.NewPublicStatusSummaryService(repo)
	now := initialNow
	h := newPublicStatusSummaryHandler(svc, func() time.Time { return now })

	first := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(first)
	c1.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/status-summary", nil)
	h(c1)
	require.Equal(t, http.StatusOK, first.Code)

	// Expire the 60-second HTTP cache while crossing the 90-minute
	// business-watermark boundary, forcing the repository-error fallback.
	now = initialNow.Add(61 * time.Second)
	second := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(second)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/status-summary", nil)
	h(c2)
	require.Equal(t, http.StatusOK, second.Code)
	var body publicStatusSummaryEnvelope
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &body))
	require.Equal(t, service.PublicStatusFreshnessDelayed, body.Data.Freshness)
}
