//go:build unit

package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetImageRuntimesHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := &fakeImageRuntimesHealthProvider{
		health: &service.ImageRuntimesHealth{
			Batch: service.ImageRuntimeComponentHealth{
				Enabled:       true,
				Ready:         true,
				QueueEnabled:  true,
				WorkerRunning: true,
			},
		},
	}
	handler := NewOpsHandler(nil, provider)
	router := gin.New()
	router.GET("/api/v1/admin/ops/image-runtimes/health", handler.GetImageRuntimesHealth)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/image-runtimes/health", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"ready":true`)
	require.Contains(t, recorder.Body.String(), `"worker_running":true`)
}

type fakeImageRuntimesHealthProvider struct {
	health *service.ImageRuntimesHealth
	err    error
}

func (f *fakeImageRuntimesHealthProvider) GetImageRuntimesHealth(context.Context) (*service.ImageRuntimesHealth, error) {
	return f.health, f.err
}
