package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminUsageRepoCapture struct {
	service.UsageLogRepository
	listParams   pagination.PaginationParams
	listFilters  usagestats.UsageLogFilters
	statsFilters usagestats.UsageLogFilters
}

func (s *adminUsageRepoCapture) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	s.listParams = params
	s.listFilters = filters
	return []service.UsageLog{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    0,
	}, nil
}

func (s *adminUsageRepoCapture) GetStatsWithFilters(ctx context.Context, filters usagestats.UsageLogFilters) (*usagestats.UsageStats, error) {
	s.statsFilters = filters
	return &usagestats.UsageStats{}, nil
}

func newAdminUsageRequestTypeTestRouter(repo *adminUsageRepoCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.GET("/admin/usage", handler.List)
	router.GET("/admin/usage/stats", handler.Stats)
	return router
}

func TestAdminUsageListRequestTypePriority(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=ws_v2&stream=false", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeWSV2), *repo.listFilters.RequestType)
	require.Nil(t, repo.listFilters.Stream)
}

func TestAdminUsageListUsesRequestedModelForDisplayModelFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?model=grok-imagine-video-1.5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "grok-imagine-video-1.5", repo.listFilters.Model)
	require.Equal(t, usagestats.ModelSourceRequested, repo.listFilters.ModelFilterSource)
}

func TestAdminUsageListInvalidRequestType(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListInvalidStream(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageStatsLast24HoursUsesOneExclusiveSnapshot(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)
	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?period=last24h&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	after := time.Now().UTC()

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.StartTime)
	require.NotNil(t, repo.statsFilters.EndTime)
	require.Equal(t, 24*time.Hour, repo.statsFilters.EndTime.Sub(*repo.statsFilters.StartTime))
	require.False(t, repo.statsFilters.EndTime.Before(before))
	require.False(t, repo.statsFilters.EndTime.After(after))
}

func TestAdminUsageStatsRejectsNonIncreasingRange(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?start_date=2024-01-02&end_date=2024-01-01&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListNativeCompactionFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_type=stream&native_compaction_v2=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.listFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.listFilters.RequestType)
	require.NotNil(t, repo.listFilters.NativeCompactionV2)
	require.True(t, *repo.listFilters.NativeCompactionV2)
}

func TestAdminUsageListInvalidNativeCompactionFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?native_compaction_v2=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageListExactTotalTrue(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?exact_total=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.listFilters.ExactTotal)
}

func TestAdminUsageListRequestIDFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?request_id=req-0123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "req-0123", repo.listFilters.RequestID)
}

func TestAdminUsageListInvalidExactTotal(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?exact_total=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageStatsRequestTypePriority(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?request_type=stream&stream=bad", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.RequestType)
	require.Equal(t, int16(service.RequestTypeStream), *repo.statsFilters.RequestType)
	require.Nil(t, repo.statsFilters.Stream)
}

func TestAdminUsageStatsNativeCompactionFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?native_compaction_v2=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, repo.statsFilters.NativeCompactionV2)
	require.True(t, *repo.statsFilters.NativeCompactionV2)
}

func TestAdminUsageStatsUsesRequestedModelForDisplayModelFilter(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?model=grok-imagine-video-1.5", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "grok-imagine-video-1.5", repo.statsFilters.Model)
	require.Equal(t, usagestats.ModelSourceRequested, repo.statsFilters.ModelFilterSource)
}

func TestAdminUsageStatsInvalidRequestType(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?request_type=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageStatsInvalidStream(t *testing.T) {
	repo := &adminUsageRepoCapture{}
	router := newAdminUsageRequestTypeTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?stream=oops", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsagePlatformFilterValidationAndPropagation(t *testing.T) {
	platforms := []string{
		service.PlatformAnthropic,
		service.PlatformOpenAI,
		service.PlatformGemini,
		service.PlatformAntigravity,
		service.PlatformGrok,
		service.PlatformKimi,
		service.PlatformZhipu,
		service.PlatformDeepseek,
		service.PlatformMiniMax,
		service.PlatformQwen,
		service.PlatformTypeSafe,
		service.PlatformOpenCodeGo,
		service.PlatformCline,
		service.PlatformCommandCode,
		service.PlatformMistral,
	}
	cases := []struct {
		path, platform string
		wantStatus     int
	}{
		{path: "/admin/usage", platform: "", wantStatus: http.StatusOK},
		{path: "/admin/usage/stats", platform: "unknown", wantStatus: http.StatusOK},
		{path: "/admin/usage", platform: "composite", wantStatus: http.StatusBadRequest},
		{path: "/admin/usage/stats", platform: "invented", wantStatus: http.StatusBadRequest},
	}
	for _, path := range []string{"/admin/usage", "/admin/usage/stats"} {
		for _, platform := range platforms {
			cases = append(cases, struct {
				path, platform string
				wantStatus     int
			}{path: path, platform: platform, wantStatus: http.StatusOK})
		}
	}
	for _, tc := range cases {
		t.Run(tc.path+"/"+tc.platform, func(t *testing.T) {
			repo := &adminUsageRepoCapture{}
			router := newAdminUsageRequestTypeTestRouter(repo)
			query := "?platform=" + tc.platform + "&group_id=7&start_date=2026-11-01&end_date=2026-11-07&timezone=UTC"
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path+query, nil))
			require.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantStatus != http.StatusOK {
				return
			}
			filters := repo.listFilters
			if tc.path == "/admin/usage/stats" {
				filters = repo.statsFilters
			}
			require.Equal(t, tc.platform, filters.Platform)
			require.Equal(t, int64(7), filters.GroupID)
			require.NotNil(t, filters.StartTime)
			require.NotNil(t, filters.EndTime)
			require.Equal(t, 7*24*time.Hour, filters.EndTime.Sub(*filters.StartTime))
		})
	}
}

func TestUsageStatsCacheKeyIncludesPlatform(t *testing.T) {
	base := usagestats.UsageLogFilters{Model: "same-model"}
	openAI := base
	openAI.Platform = service.PlatformOpenAI
	gemini := base
	gemini.Platform = service.PlatformGemini
	unknown := base
	unknown.Platform = "unknown"
	require.NotEqual(t, usageStatsCacheKey(base), usageStatsCacheKey(openAI))
	require.NotEqual(t, usageStatsCacheKey(openAI), usageStatsCacheKey(gemini))
	require.NotEqual(t, usageStatsCacheKey(gemini), usageStatsCacheKey(unknown))
}
