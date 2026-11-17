package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type latencyReportStub struct {
	service.UsageLogRepository
	filter service.LatencyReportFilter
	calls  int
}

func (r *latencyReportStub) ReadLatencyReport(_ context.Context, f service.LatencyReportFilter) ([]service.LatencyGroup, error) {
	r.calls++
	r.filter = f
	return []service.LatencyGroup{}, nil
}

func TestLatencyReportFiltersTimezoneAndUnknownEvidence(t *testing.T) {
	stub := &latencyReportStub{}
	h := NewUsageHandler(service.NewUsageService(stub, nil, nil, nil), nil, nil, nil)
	router := gin.New()
	router.GET("/latency", h.LatencyReport)
	for _, query := range []string{
		"user_id=0", "api_key_id=invalid", "account_id=-1", "group_id=0",
		"start_date=invalid", "model=" + strings.Repeat("m", 257),
		"start_date=2026-01-01&end_date=2026-02-15",
		"start_date=2026-02-03&end_date=2026-02-01",
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/latency?"+query, nil))
		require.Equal(t, http.StatusBadRequest, w.Code, query)
	}
	require.Zero(t, stub.calls, "invalid filters must not execute a report query")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/latency?start_date=2026-11-01&end_date=2026-11-02&timezone=Asia%2FShanghai&user_id=12&api_key_id=7&model=gpt-fixture&platform=openai", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC), stub.filter.Start.UTC())
	require.Equal(t, time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC), stub.filter.End.UTC())
	require.Equal(t, int64(12), stub.filter.UserID)
	require.Equal(t, int64(7), stub.filter.APIKeyID)
	require.Equal(t, "gpt-fixture", stub.filter.Model)
	require.Equal(t, "openai", stub.filter.Platform)
	var payload struct {
		Data service.LatencyReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Nil(t, payload.Data.EndToEndMS)
	require.Equal(t, "not_collected", payload.Data.EndToEndStatus)
	require.Nil(t, payload.Data.ClientUploadMS)
	require.Equal(t, "not_collected", payload.Data.ContentCapabilities.Status)
}
