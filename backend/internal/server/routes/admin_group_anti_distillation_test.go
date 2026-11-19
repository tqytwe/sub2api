package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminAntiDistillationGroups struct{ service.GroupRepository }

func (adminAntiDistillationGroups) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	if id != 7 {
		return nil, service.ErrGroupNotFound
	}
	return &service.Group{ID: id, Platform: service.PlatformOpenAI}, nil
}

type adminAntiDistillationSettings struct {
	service.SettingRepository
	values map[string]string
}

func (s *adminAntiDistillationSettings) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (s *adminAntiDistillationSettings) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func TestAdminGroupAntiDistillationRoutesAreAuthenticatedAndRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := &adminAntiDistillationSettings{values: map[string]string{}}
	protection := service.NewGroupAntiDistillationService(settings, adminAntiDistillationGroups{}, nil, nil, nil)
	h := adminhandler.ProvideGroupHandlerWithProtection(nil, nil, nil, nil, protection)
	router := gin.New()
	admin := router.Group("/api/v1/admin", func(c *gin.Context) {
		if c.GetHeader("X-Test-Admin") != "true" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})
	registerGroupRoutes(admin, &handler.Handlers{Admin: &handler.AdminHandlers{Group: h}})

	request := func(method, path, body string, authorized bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authorized {
			req.Header.Set("X-Test-Admin", "true")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	resp := request(http.MethodGet, "/api/v1/admin/groups/7/anti-distillation", "", false)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
	resp = request(http.MethodGet, "/api/v1/admin/groups/7/anti-distillation", "", true)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), `"enabled":false`)
	resp = request(http.MethodPut, "/api/v1/admin/groups/7/anti-distillation", `{"enabled":true,"pause_minutes":15}`, true)
	require.Equal(t, http.StatusOK, resp.Code)
	resp = request(http.MethodGet, "/api/v1/admin/groups/7/anti-distillation", "", true)
	require.Contains(t, resp.Body.String(), `"enabled":true`)
	require.Contains(t, resp.Body.String(), `"pause_minutes":15`)
}
