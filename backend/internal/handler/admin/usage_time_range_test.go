package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseUsageDateBoundary(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	start, err := parseUsageDateBoundary("2024-03-10", "America/New_York", false)
	require.NoError(t, err)
	end, err := parseUsageDateBoundary("2024-03-10", "America/New_York", true)
	require.NoError(t, err)
	require.Equal(t, time.Date(2024, 3, 10, 0, 0, 0, 0, loc), start)
	require.Equal(t, 23*time.Hour, end.Sub(start))

	instant, err := parseUsageDateBoundary("2024-03-10T01:30:00.123456789-05:00", "UTC", true)
	require.NoError(t, err)
	require.Equal(t, 123456789, instant.Nanosecond())
	require.Equal(t, -5*60*60, offsetSeconds(instant))
}

func TestParseTimeRangeRejectsInvalidAndNonIncreasingBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{
		"/?start_date=bad&timezone=UTC",
		"/?start_date=2024-01-02T00:00:00Z&end_date=2024-01-02T00:00:00Z",
		"/?start_date=2024-01-03&end_date=2024-01-02&timezone=UTC",
		"/?start_date=2024-01-02T00:00:00Z&end_date=2024-01-01T23:59:59Z",
	} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
		_, _, err := parseTimeRange(ctx)
		require.Error(t, err, target)
	}
}

func TestDashboardAndUsageCacheKeysKeepNanoseconds(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 123, time.UTC)
	filters := func(v time.Time) usagestats.UsageLogFilters {
		end := v.Add(time.Hour)
		return usagestats.UsageLogFilters{StartTime: &v, EndTime: &end}
	}
	one := usageStatsCacheKey(filters(start))
	two := usageStatsCacheKey(filters(start.Add(time.Nanosecond)))
	require.NotEqual(t, one, two)

	oneDashboard := mustMarshalDashboardCacheKey(dashboardEntityTrendCacheKey{
		StartTime: start.UTC().Format(time.RFC3339Nano),
		EndTime:   start.Add(time.Hour).UTC().Format(time.RFC3339Nano),
	})
	twoDashboard := mustMarshalDashboardCacheKey(dashboardEntityTrendCacheKey{
		StartTime: start.Add(time.Nanosecond).UTC().Format(time.RFC3339Nano),
		EndTime:   start.Add(time.Hour).UTC().Format(time.RFC3339Nano),
	})
	require.NotEqual(t, oneDashboard, twoDashboard)
}

func offsetSeconds(t time.Time) int {
	_, offset := t.Zone()
	return offset
}
