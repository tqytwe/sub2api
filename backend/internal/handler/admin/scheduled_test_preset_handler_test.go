package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type qualityPresetHandlerRepo struct {
	service.ScheduledTestPlanRepository
	presets []*service.QualityRulePreset
}

func (r *qualityPresetHandlerRepo) ListQualityPresets(context.Context) ([]*service.QualityRulePreset, error) {
	return r.presets, nil
}

func (r *qualityPresetHandlerRepo) CreateQualityPreset(_ context.Context, preset *service.QualityRulePreset) (*service.QualityRulePreset, error) {
	preset.ID = int64(len(r.presets) + 1)
	r.presets = append(r.presets, preset)
	return preset, nil
}

func TestQualityPresetHTTPCreateAndReloadDoesNotCreatePlan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &qualityPresetHandlerRepo{}
	h := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
	r := gin.New()
	r.POST("/admin/account-quality-presets", h.CreateQualityPreset)
	r.GET("/admin/account-quality-presets", h.ListQualityPresets)
	body := `{"name":"Saved check","model_id":"test-model","cron_expression":"*/30 * * * *","max_results":75,
		"account_id":42,"id":99,"pelican_config":{"rule_name":"source-only","model_id":"result-only","trigger_source":"upstream_5xx","credentials":{"token":"synthetic"},
		"question_kind":"text","prompt":"question","parallel_count":1,"reasoning_effort":"high","quality":{"expected_answer":"42","action":"observe_only","judge":{"kind":"expected"}}}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/account-quality-presets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	var saved service.QualityRulePreset
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	require.Equal(t, int64(1), saved.ID)
	require.Empty(t, saved.PelicanConfig.RuleName)
	require.Empty(t, saved.PelicanConfig.ModelID)
	require.NotContains(t, w.Body.String(), "credentials")
	require.NotContains(t, w.Body.String(), "account_id")
	require.Len(t, repo.presets, 1)

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/admin/account-quality-presets", nil))
	require.Equal(t, 200, w.Code, w.Body.String())
	var listed []service.QualityRulePreset
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Equal(t, []service.QualityRulePreset{saved}, listed)
}

func TestQualityPresetHTTPRejectsNonQualityConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &qualityPresetHandlerRepo{}
	h := NewScheduledTestHandler(service.NewScheduledTestService(repo, nil))
	r := gin.New()
	r.POST("/admin/account-quality-presets", h.CreateQualityPreset)
	body := `{"name":"Saved check","model_id":"test-model","cron_expression":"*/30 * * * *","pelican_config":{"prompt":"legacy check"}}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/admin/account-quality-presets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 400, w.Code, w.Body.String())
	require.Empty(t, repo.presets)
}
