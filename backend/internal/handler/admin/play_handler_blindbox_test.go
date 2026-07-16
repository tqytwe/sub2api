package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminBlindboxPoolGetAndPut(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newAdminBlindboxSettingRepo(approvedAdminBlindboxPool())
	settingService := service.NewSettingService(repo, nil)
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	handler := NewAdminPlayHandler(playService)
	router := gin.New()
	router.GET("/api/v1/admin/play/blindbox/pool", handler.GetBlindboxPool)
	router.PUT("/api/v1/admin/play/blindbox/pool", handler.UpdateBlindboxPool)

	getRecorder := httptest.NewRecorder()
	router.ServeHTTP(getRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/play/blindbox/pool", nil))

	require.Equal(t, http.StatusOK, getRecorder.Code)
	require.JSONEq(t, `{
		"code": 0,
		"message": "success",
		"data": {
			"version": "season-1-v1",
			"cost": 0.5,
			"rtp_cap": 0.9,
			"tiers": [
				{"amount": 0.05, "weight": 4000},
				{"amount": 0.2, "weight": 3000},
				{"amount": 0.5, "weight": 1800},
				{"amount": 1, "weight": 800},
				{"amount": 3, "weight": 300},
				{"amount": 10, "weight": 90},
				{"amount": 20, "weight": 10}
			]
		}
	}`, getRecorder.Body.String())

	updated := approvedAdminBlindboxPool()
	updated.Version = "season-2-v1"
	body, err := json.Marshal(updated)
	require.NoError(t, err)
	putRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/play/blindbox/pool", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(putRecorder, request)

	require.Equal(t, http.StatusOK, putRecorder.Code)
	require.Equal(t, "season-2-v1", repo.pool().Version)
	require.Contains(t, putRecorder.Body.String(), `"version":"season-2-v1"`)
}

func TestAdminBlindboxPoolRejectsInvalidUpdateWithoutChangingStoredValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newAdminBlindboxSettingRepo(approvedAdminBlindboxPool())
	settingService := service.NewSettingService(repo, nil)
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	handler := NewAdminPlayHandler(playService)
	router := gin.New()
	router.PUT("/api/v1/admin/play/blindbox/pool", handler.UpdateBlindboxPool)

	invalid := approvedAdminBlindboxPool()
	invalid.Tiers[0].Weight = 3999
	body, err := json.Marshal(invalid)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/play/blindbox/pool", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "season-1-v1", repo.pool().Version)
	require.Equal(t, int64(4000), repo.pool().Tiers[0].Weight)
	require.Contains(t, recorder.Body.String(), `"reason":"PLAY_BLINDBOX_POOL_INVALID"`)
}

func TestAdminBlindboxPoolPutReturnsSavedValueWithoutPostCommitRead(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newAdminBlindboxSettingRepo(approvedAdminBlindboxPool())
	repo.getMultipleErr = errors.New("post-commit reads are unavailable")
	settingService := service.NewSettingService(repo, nil)
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	handler := NewAdminPlayHandler(playService)
	router := gin.New()
	router.PUT("/api/v1/admin/play/blindbox/pool", handler.UpdateBlindboxPool)

	updated := approvedAdminBlindboxPool()
	updated.Version = " season-2-v1 "
	body, err := json.Marshal(updated)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/play/blindbox/pool", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 0, repo.getMultipleCalls)
	require.Equal(t, "season-2-v1", repo.pool().Version)
	require.Contains(t, recorder.Body.String(), `"version":"season-2-v1"`)
}

func approvedAdminBlindboxPool() service.PlayBlindboxPool {
	return service.PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []service.PlayBlindboxTier{
			{Amount: 0.05, Weight: 4000},
			{Amount: 0.2, Weight: 3000},
			{Amount: 0.5, Weight: 1800},
			{Amount: 1, Weight: 800},
			{Amount: 3, Weight: 300},
			{Amount: 10, Weight: 90},
			{Amount: 20, Weight: 10},
		},
	}
}

type adminBlindboxSettingRepo struct {
	values           map[string]string
	getMultipleErr   error
	getMultipleCalls int
}

func newAdminBlindboxSettingRepo(pool service.PlayBlindboxPool) *adminBlindboxSettingRepo {
	raw, err := json.Marshal(pool)
	if err != nil {
		panic(err)
	}
	return &adminBlindboxSettingRepo{
		values: map[string]string{
			service.SettingKeyPlayBlindboxPoolJSON: string(raw),
		},
	}
}

func (r *adminBlindboxSettingRepo) pool() service.PlayBlindboxPool {
	var pool service.PlayBlindboxPool
	if err := json.Unmarshal([]byte(r.values[service.SettingKeyPlayBlindboxPoolJSON]), &pool); err != nil {
		panic(err)
	}
	return pool
}

func (r *adminBlindboxSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *adminBlindboxSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *adminBlindboxSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *adminBlindboxSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.getMultipleCalls++
	if r.getMultipleErr != nil {
		return nil, r.getMultipleErr
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *adminBlindboxSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *adminBlindboxSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *adminBlindboxSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}
