package admin

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseGrowthCohortWindowDefaultsToMatureTwoWeekWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/v1/admin/play/growth/cohort", nil)
	before := time.Now().UTC()
	start, end, err := parseGrowthCohortWindow(ctx)
	after := time.Now().UTC()

	require.NoError(t, err)
	require.Equal(t, 14*24*time.Hour, end.Sub(start))
	require.WithinDuration(t, before.Add(-30*24*time.Hour), end, time.Second)
	require.WithinDuration(t, after.Add(-30*24*time.Hour), end, time.Second)
}

func TestParseGrowthCohortWindowPreservesExplicitAuditWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/api/v1/admin/play/growth/cohort?start=2026-06-01&end=2026-06-15", nil)

	start, end, err := parseGrowthCohortWindow(ctx)
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), end)
}
