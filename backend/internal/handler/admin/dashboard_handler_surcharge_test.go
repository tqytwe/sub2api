package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type surchargeReportRepoCapture struct {
	service.UsageLogRepository
	start, end, today time.Time
	params            usagestats.BillingSurchargeReportParams
}

func (r *surchargeReportRepoCapture) GetBillingSurchargeReport(
	_ context.Context,
	start, end, today time.Time,
	params usagestats.BillingSurchargeReportParams,
) (*usagestats.BillingSurchargeReport, error) {
	r.start, r.end, r.today, r.params = start, end, today, params
	return &usagestats.BillingSurchargeReport{Items: []usagestats.BillingSurchargeDetail{}}, nil
}

func newSurchargeReportRouter(repo *surchargeReportRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/dashboard/billing-surcharge", NewDashboardHandler(service.NewDashboardService(repo, nil, nil, nil), nil).GetBillingSurchargeReport)
	return router
}

func TestGetBillingSurchargeReportAcceptsRollingTimestampAndFilters(t *testing.T) {
	repo := &surchargeReportRepoCapture{}
	router := newSurchargeReportRouter(repo)
	start := "2026-11-02T15:08:00.123Z"
	end := "2026-11-03T15:08:00.123Z"
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/billing-surcharge?start_date="+start+"&end_date="+end+"&search=alice&sort_by=billed_cost&sort_order=asc&page=2&page_size=25", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, mustParseSurchargeTime(t, start), repo.start)
	require.Equal(t, mustParseSurchargeTime(t, end), repo.end)
	require.Equal(t, "alice", repo.params.Search)
	require.Equal(t, pagination.PaginationParams{Page: 2, PageSize: 25, SortBy: "billed_cost", SortOrder: "asc"}, repo.params.Pagination)
}

func TestGetBillingSurchargeReportRejectsInvalidFilters(t *testing.T) {
	for _, query := range []string{
		"sort_by=created_at%3Bdrop+table",
		"sort_order=sideways",
		"search=" + strings.Repeat("x", 121),
	} {
		t.Run(query[:min(len(query), 24)], func(t *testing.T) {
			router := newSurchargeReportRouter(&surchargeReportRepoCapture{})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/dashboard/billing-surcharge?"+query, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

func mustParseSurchargeTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	require.NoError(t, err)
	return parsed
}
