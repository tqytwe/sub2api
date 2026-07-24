package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIPRiskRuntimeHandlerRemainsReadOnlyShadowMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/ip-risk/runtime", NewIPRiskHandler(nil).GetRuntime)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ip-risk/runtime", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Data struct {
			Enabled          bool   `json:"enabled"`
			ShadowMode       bool   `json:"shadow_mode"`
			AutoBlockEnabled bool   `json:"auto_block_enabled"`
			Degraded         bool   `json:"degraded"`
			DegradedReason   string `json:"degraded_reason"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.Enabled)
	require.True(t, body.Data.ShadowMode)
	require.False(t, body.Data.AutoBlockEnabled)
	require.True(t, body.Data.Degraded)
	require.Equal(t, "service unavailable", body.Data.DegradedReason)
}
