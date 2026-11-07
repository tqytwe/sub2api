//go:build integration

package repository_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestQualityIsolationHTTPPreviewConfirmReadback(t *testing.T) {
	plans, plan, db := repository.NewQualityIsolationHTTPFixture(t)
	h := admin.NewScheduledTestHandler(service.NewScheduledTestService(plans, repository.NewScheduledTestResultRepository(db)))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Authentication/authorization remains on the existing admin route group.
	r.GET("/plans/:id/ownership", h.QualityOwnershipPreview)
	r.POST("/plans/:id/ownership/remove-model", h.QualityOwnershipRemoveModel)
	path := fmt.Sprintf("/plans/%d/ownership", plan.ID)
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		encoded, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	preview := func() service.QualityOwnershipPreview {
		w := request("GET", path, nil)
		require.Equal(t, 200, w.Code, w.Body.String())
		for _, secret := range []string{"private-probe", "credentials", "model_rate_limits", "reset_at"} {
			require.NotContains(t, w.Body.String(), secret)
		}
		var response struct {
			Data service.QualityOwnershipPreview `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		return response.Data
	}
	view := preview()
	require.Len(t, view.ModelRemovals, 3)
	for _, body := range []map[string]any{
		{"model": "sol", "reason_code": "orphan_marker_recovery", "fence": view.Fence},
		{"model": "sol", "confirm": true, "fence": view.Fence},
		{"model": "sol", "reason_code": "arbitrary", "confirm": true, "fence": view.Fence},
		{"model": "sol", "reason_code": "retire_model", "confirm": true, "fence": "invalid"},
		{"model": "sol", "reason_code": "retire_model", "confirm": true, "fence": "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"},
	} {
		w := request("POST", path+"/remove-model", body)
		require.Equal(t, 400, w.Code, w.Body.String())
	}
	w := request("POST", path+"/remove-model", map[string]any{"model": "sol", "reason_code": "orphan_marker_recovery", "confirm": true, "fence": view.Fence})
	require.Equal(t, 200, w.Code, w.Body.String())
	fresh := preview()
	require.Len(t, fresh.ModelRemovals, 2)
	require.NotEqual(t, view.Fence, fresh.Fence)
	w = request("POST", path+"/remove-model", map[string]any{"model": "missing", "reason_code": "retire_model", "confirm": true, "fence": view.Fence})
	require.Equal(t, 409, w.Code, w.Body.String())
}
