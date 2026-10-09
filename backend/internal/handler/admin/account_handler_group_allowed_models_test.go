package admin

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountUpdatePassesGroupAllowedModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		want       any
	}{
		{"configured", `{"group_allowed_models":{"5":["gpt-5.5","gpt-5.3-*"]}}`, map[string]any{"5": []any{"gpt-5.5", "gpt-5.3-*"}}},
		{"clear", `{"group_allowed_models":{}}`, map[string]any{}},
		{"omitted", `{}`, nil},
		{"null", `{"group_allowed_models":null}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := newStubAdminService()
			handler := NewAccountHandler(stub, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.PUT("/accounts/:id", handler.Update)
			response := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/accounts/1", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, req)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			body, err := json.Marshal(stub.lastUpdateAccountInput)
			require.NoError(t, err)
			var input map[string]any
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, tc.want, input["GroupAllowedModels"])
		})
	}
}
