//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCountPublicModels(t *testing.T) {
	channels := []service.AvailableChannel{
		{
			SupportedModels: []service.SupportedModel{
				{Name: "gpt-4", Platform: "OpenAI"},
				{Name: "claude-sonnet", Platform: "Anthropic"},
			},
		},
		{
			SupportedModels: []service.SupportedModel{
				{Name: "gpt-4", Platform: "OpenAI"},
				{Name: "gemini-flash", Platform: "Google"},
			},
		},
	}
	require.Equal(t, 3, countPublicModels(channels))
}

func TestPublicGrowthTeaserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerPublicRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:              "true",
			service.SettingPaymentEnabled:                   "true",
			service.SettingKeyPublicModelsEnabled:              "true",
			service.SettingKeyPlayCheckinEnabled:               "true",
			service.SettingKeyPlayCheckinDailyReward:           "0.5",
			service.SettingKeyAffiliateEnabled:                 "true",
			service.SettingKeyAffiliateRebateRate:              "10",
			service.SettingKeyDefaultBalance:                   "0.2",
			service.SettingKeyAuthSourceDefaultEmailGrantOnSignup: "true",
			service.SettingKeyAuthSourceDefaultEmailBalance:    "0.2",
		},
	}
	settingSvc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserBalance: 0}})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/growth-teaser", nil)

	handler := PublicGrowthTeaser(settingSvc, nil, nil)
	handler(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data service.PublicGrowthTeaser `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Data.RegistrationEnabled)
	require.True(t, body.Data.SignupGrantEnabled)
	require.InDelta(t, 0.2, body.Data.SignupBalanceUSD, 0.001)
	require.True(t, body.Data.CheckinEnabled)
	require.InDelta(t, 0.5, body.Data.CheckinDailyReward, 0.001)
	require.True(t, body.Data.AffiliateEnabled)
	require.InDelta(t, 10, body.Data.AffiliateRebatePct, 0.001)
}

func TestPlayHandler_PublicModelCount(t *testing.T) {
	h := &PlayHandler{playService: nil}
	require.Equal(t, 0, h.PublicModelCount(context.Background()))
}
