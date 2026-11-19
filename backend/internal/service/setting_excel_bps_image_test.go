package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/stretchr/testify/require"
)

type excelBPSImageSettingsRepo struct {
	SettingRepository
	mu     sync.Mutex
	values map[string]string
	err    error
}

func (r *excelBPSImageSettingsRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = r.values[key]
	}
	return values, r.err
}

func (r *excelBPSImageSettingsRepo) GetAll(_ context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, r.err
}

func (r *excelBPSImageSettingsRepo) GetValue(ctx context.Context, key string) (string, error) {
	values, err := r.GetMultiple(ctx, []string{key})
	return values[key], err
}

func (r *excelBPSImageSettingsRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func TestExcelBPSImageSettingsRejectInvalidUpdatesAtomically(t *testing.T) {
	ctx := context.Background()
	repo := &excelBPSImageSettingsRepo{}
	settings := NewSettingService(repo, &config.Config{})
	require.NoError(t, settings.UpdateSettings(ctx, &SystemSettings{ExcelBPSImageMode: ExcelBPSImageModeRelay, ExcelBPSImageRelayEnabled: true, ExcelBPSImageBaseURL: "https://images.example"}))
	for _, limits := range []struct{ body, budget, requests int }{
		{basispoints.MaxImageBodyMiB + 1, basispoints.MaxImageBudgetMiB, 32}, {64, 511, 32}, {64, basispoints.MaxImageBudgetMiB + 1, 32}, {64, 512, basispoints.MaxImageRequests + 1}, {128, 512, 32},
	} {
		err := settings.UpdateSettings(ctx, &SystemSettings{
			ExcelBPSImageMode: ExcelBPSImageModeRelay, ExcelBPSImageRelayEnabled: true, ExcelBPSImageBaseURL: "https://images.example",
			ExcelBPSImageBodyLimitMiB: limits.body, ExcelBPSImageBudgetMiB: limits.budget, ExcelBPSImageMaxRequests: limits.requests, ExcelBPSImageMaxImages: 20,
		})
		require.Error(t, err)
		runtime, err := settings.GetExcelBPSImageRelaySettings(ctx)
		require.NoError(t, err)
		require.Equal(t, DefaultExcelBPSImageBodyLimitMiB, runtime.BodyLimitMiB)
		require.Equal(t, DefaultExcelBPSImageBudgetMiB, runtime.BudgetMiB)
		require.Equal(t, DefaultExcelBPSImageMaxRequests, runtime.MaxRequests)
		require.Equal(t, basispoints.DefaultImageRelayLimits().MaxImages, runtime.Limits.MaxImages)
	}
	for _, origin := range []string{"", "http://images.example", "https://images.example/v1", "https://user:secret@images.example", "https://images.example?token=secret"} {
		err := settings.UpdateSettings(ctx, &SystemSettings{ExcelBPSImageMode: ExcelBPSImageModeRelay, ExcelBPSImageRelayEnabled: true, ExcelBPSImageBaseURL: origin})
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
		runtime, err := settings.GetExcelBPSImageRelaySettings(ctx)
		require.NoError(t, err)
		require.True(t, runtime.Enabled)
		require.Equal(t, "https://images.example", runtime.BaseURL)
	}
	repo.err = errors.New("database-private-error")
	runtime, err := settings.GetExcelBPSImageRelaySettings(ctx)
	require.Error(t, err)
	require.False(t, runtime.Enabled)
	require.NotContains(t, err.Error(), "database-private-error")
}
