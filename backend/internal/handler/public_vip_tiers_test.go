//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicVIPTiersHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerPublicRepoStub{
		values: map[string]string{
			service.SettingKeyPlayVIPTiers: `[{"tier":0,"label":"V0","min_recharge":0,"recharge_bonus_pct":0,"color_key":"neutral"},{"tier":1,"label":"V1","min_recharge":50,"recharge_bonus_pct":2,"color_key":"emerald","perks":["models_vip_tag"]}]`,
		},
	}
	settingSvc := service.NewSettingService(repo, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/vip-tiers", nil)

	handler := PublicVIPTiers(settingSvc)
	handler(c)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Enabled bool                  `json:"enabled"`
			Tiers   []service.PlayVIPTier `json:"tiers"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Data.Enabled)
	require.Len(t, body.Data.Tiers, 2)
	require.Equal(t, "V1", body.Data.Tiers[1].Label)
	require.Equal(t, 2.0, body.Data.Tiers[1].RechargeBonusPct)
	require.Equal(t, "emerald", body.Data.Tiers[1].ColorKey)
}
