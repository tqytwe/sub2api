package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type automationSettingsFixture struct {
	service.SettingRepository
	raw string
}

func (s *automationSettingsFixture) GetValue(context.Context, string) (string, error) {
	if s.raw == "" {
		return "", service.ErrSettingNotFound
	}
	return s.raw, nil
}
func (s *automationSettingsFixture) Set(_ context.Context, _ string, value string) error {
	s.raw = value
	return nil
}

type automationHistoryFixture struct {
	service.AccountRepository
	events []service.AutoConfigEvent
}

func (s automationHistoryFixture) ListAutoConfigEvents(context.Context, int64, string, int) ([]service.AutoConfigEvent, error) {
	return s.events, nil
}
func automationHandlerFixture(t *testing.T) (*gin.Engine, *automationSettingsFixture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := service.DefaultOAuthAutoConfig()
	cfg.Revision = "fixture"
	cfg.Concurrency = 7
	cfg.SelectedModels = []string{"model-fixture"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	settings := &automationSettingsFixture{raw: string(raw)}
	svc := service.NewOAuthAutomationService(settings, automationHistoryFixture{events: []service.AutoConfigEvent{{ID: 3}, {ID: 2}}}, structGroupsFixture{})
	handler := &AccountHandler{automation: svc}
	r := gin.New()
	r.GET("/config", handler.GetAutoConfig)
	r.PUT("/config", handler.SaveAutoConfig)
	r.GET("/history", handler.ListAutoConfigEvents)
	return r, settings
}

type structGroupsFixture struct{ service.GroupRepository }

func TestOAuthAutomationHandlerPartialUpdatePreservesStoredFields(t *testing.T) {
	r, settings := automationHandlerFixture(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(`{"priority":12}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var cfg service.OAuthAutoConfig
	require.NoError(t, json.Unmarshal([]byte(settings.raw), &cfg))
	require.Equal(t, 12, cfg.Priority)
	require.Equal(t, 7, cfg.Concurrency)
	require.Equal(t, []string{"model-fixture"}, cfg.SelectedModels)
	require.False(t, cfg.Enabled)
	require.False(t, cfg.OpenAIAPIKeyFallback.Enabled)
	require.NotEqual(t, "fixture", cfg.Revision)
}
func TestOAuthAutomationHistoryCursorAndEventValidation(t *testing.T) {
	r, _ := automationHandlerFixture(t)
	for _, query := range []string{"before=-1", "limit=101", "limit=0", "kind=unknown"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history?"+query, nil))
		require.Equal(t, http.StatusBadRequest, w.Code, query)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/history?limit=1&kind=manual_recovery", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"has_more":true`)
}
func TestOAuthAutomationUnavailableIsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/config", (&AccountHandler{}).GetAutoConfig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}
