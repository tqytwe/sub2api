package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blindboxHandlerSettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *blindboxHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

type blindboxPoolResponse struct {
	Code int `json:"code"`
	Data struct {
		Enabled bool                     `json:"enabled"`
		Pool    service.PlayBlindboxPool `json:"pool"`
	} `json:"data"`
}

func TestBlindboxPoolAndStatusReturnSameConfiguredPool(t *testing.T) {
	gin.SetMode(gin.TestMode)

	pool := service.PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 0.05, Weight: 4000},
			{Amount: 0.20, Weight: 3000},
			{Amount: 0.50, Weight: 1800},
			{Amount: 1, Weight: 800},
			{Amount: 3, Weight: 300},
			{Amount: 10, Weight: 90},
			{Amount: 20, Weight: 10},
		},
	}
	poolJSON, err := json.Marshal(pool)
	require.NoError(t, err)

	settingService := service.NewSettingService(&blindboxHandlerSettingRepo{values: map[string]string{
		service.SettingKeyPlayBlindboxPoolJSON: string(poolJSON),
	}}, nil)
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	playHandler := handler.NewPlayHandler(playService, nil)

	authCalls := 0
	jwtAuth := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})

	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{
		Play: playHandler,
	}, jwtAuth)

	publicRecorder := httptest.NewRecorder()
	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/play/blindbox/pool", nil)
	router.ServeHTTP(publicRecorder, publicRequest)
	require.Equal(t, http.StatusOK, publicRecorder.Code)
	require.Zero(t, authCalls)

	var publicResponse blindboxPoolResponse
	require.NoError(t, json.Unmarshal(publicRecorder.Body.Bytes(), &publicResponse))
	require.Equal(t, 0, publicResponse.Code)
	require.False(t, publicResponse.Data.Enabled)
	require.Equal(t, 20.0, publicResponse.Data.Pool.Tiers[6].Amount)

	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/v1/play/blindbox/status", nil)
	router.ServeHTTP(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	require.Equal(t, 1, authCalls)

	var statusResponse blindboxPoolResponse
	require.NoError(t, json.Unmarshal(statusRecorder.Body.Bytes(), &statusResponse))
	require.Equal(t, 0, statusResponse.Code)
	require.Equal(t, publicResponse.Data.Enabled, statusResponse.Data.Enabled)
	require.Equal(t, publicResponse.Data.Pool, statusResponse.Data.Pool)
}
