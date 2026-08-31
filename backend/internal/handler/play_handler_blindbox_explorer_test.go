package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blindboxExplorerHandlerRepo struct {
	service.PlayRepository
}

func (r *blindboxExplorerHandlerRepo) GetGrowthEligibilitySignals(_ context.Context, _ int64, _, _, now time.Time) (service.PlayGrowthEligibilitySignals, error) {
	return service.PlayGrowthEligibilitySignals{
		EmailVerified: true,
		CreatedAt:     now.Add(-24 * time.Hour),
	}, nil
}

func (r *blindboxExplorerHandlerRepo) CountBlindboxOpens(context.Context, int64, time.Time) (int, error) {
	return 0, nil
}

func (r *blindboxExplorerHandlerRepo) CreateGrowthEligibilitySnapshot(context.Context, service.PlayGrowthEligibilitySnapshot) (int64, error) {
	return 1, nil
}

func (r *blindboxExplorerHandlerRepo) LinkGrowthEligibilitySnapshot(context.Context, string, int64, time.Time, int64) error {
	return nil
}

func (r *blindboxExplorerHandlerRepo) LinkBlindboxGrowthEligibilitySnapshot(context.Context, int64, string, int64) error {
	return nil
}

func (r *blindboxExplorerHandlerRepo) InsertGrowthEnergyLedger(context.Context, service.PlayGrowthEnergyLedgerEntry) error {
	return nil
}

func TestBlindboxStatusExplorerOmitsRewardPoolFields(t *testing.T) {
	status := &service.PlayBlindboxStatus{
		Enabled:         true,
		CouponPoolReady: false,
		CostAmount:      0.5,
		BlindboxPool:    service.PlayBlindboxPool{Version: "secret", Cost: 0.5, RTPCap: 0.9},
		CurrentPool:     service.PlayBlindboxPool{Version: "secret-current"},
		ExpectedReward:  0.45,
		PoolVersion:     "secret",
		RTPCap:          0.9,
		GrowthEligibility: service.PlayGrowthEligibility{
			Tier:       service.PlayGrowthTierExplorer,
			RewardMode: service.PlayGrowthRewardEnergy,
		},
	}

	payload, err := json.Marshal(toPlayBlindboxStatusDTO(status, true))
	require.NoError(t, err)
	body := string(payload)
	for _, forbidden := range []string{
		"\"pool\"", "\"current_pool\"", "\"next_pool\"", "\"cost_amount\"",
		"\"expected_reward\"", "\"rtp_cap\"", "\"pool_version\"",
		"\"coupon_prizes\"", "\"coupon_weight_bp\"", "\"balance_weight_bp\"",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestBlindboxStatusAuthenticatedUnknownEligibilityOmitsRewardPoolFields(t *testing.T) {
	status := &service.PlayBlindboxStatus{
		Enabled:        false,
		CostAmount:     0.5,
		BlindboxPool:   service.PlayBlindboxPool{Version: "secret", Cost: 0.5, RTPCap: 0.9},
		CurrentPool:    service.PlayBlindboxPool{Version: "secret-current"},
		ExpectedReward: 0.45,
		PoolVersion:    "secret",
		RTPCap:         0.9,
		// Zero-value eligibility represents a disabled or unavailable
		// qualification dependency and must fail closed for authenticated users.
	}

	payload, err := json.Marshal(toPlayBlindboxStatusDTO(status, true))
	require.NoError(t, err)
	body := string(payload)
	for _, forbidden := range []string{
		"\"pool\"", "\"current_pool\"", "\"next_pool\"", "\"cost_amount\"",
		"\"expected_reward\"", "\"rtp_cap\"", "\"pool_version\"",
	} {
		require.NotContains(t, body, forbidden)
	}
}

func TestBlindboxPoolExplorerOmitsRewardPoolFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settings := service.NewSettingService(&blindboxExplorerHandlerSettingRepo{values: map[string]string{
		service.SettingKeyPlayBlindboxEnabled:  "true",
		service.SettingKeyPlayBlindboxPoolJSON: `{"version":"secret","cost":0.5,"rtp_cap":0.9,"tiers":[{"amount":1,"weight":10000}]}`,
	}}, nil)
	playService := service.NewPlayService(&blindboxExplorerHandlerRepo{}, nil, nil, settings, nil, nil)
	playHandler := NewPlayHandler(playService, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/api/v1/play/blindbox/pool", playHandler.BlindboxPool)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/play/blindbox/pool", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "Authorization", recorder.Header().Get("Vary"))
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
	body := recorder.Body.String()
	for _, forbidden := range []string{
		`"pool"`, `"current_pool"`, `"next_pool"`, `"cost_amount"`,
		`"expected_reward"`, `"rtp_cap"`, `"pool_version"`,
		`"coupon_prizes"`, `"coupon_weight_bp"`, `"balance_weight_bp"`,
	} {
		require.NotContains(t, body, forbidden)
	}
}

type blindboxExplorerHandlerSettingRepo struct {
	service.SettingRepository
	values map[string]string
}

func (r *blindboxExplorerHandlerSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}
