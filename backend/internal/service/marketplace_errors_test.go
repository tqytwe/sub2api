//go:build unit

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceFeatureDisabledError_StableReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/marketplace/contracts", nil)

	require.True(t, response.ErrorFrom(ctx, fmt.Errorf("marketplace gate: %w", ErrMarketplaceFeatureDisabled)))
	require.Equal(t, http.StatusNotFound, recorder.Code)

	var body response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.StatusNotFound, body.Code)
	require.Equal(t, "MARKETPLACE_FEATURE_DISABLED", body.Reason)
}
