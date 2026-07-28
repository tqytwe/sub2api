package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCouponHandlerRejectsInvalidTemplatePayloadBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewCouponHandler(nil)
	router.POST("/templates", handler.CreateTemplate)

	request := httptest.NewRequest(http.MethodPost, "/templates", strings.NewReader(`{
		"key":"bad-template",
		"name":"Bad template",
		"status":"draft",
		"benefit_type":"fixed_amount",
		"benefit_value":1,
		"currency":"CNY",
		"applicable_scopes":[],
		"validity_mode":"relative_days",
		"validity_days":1
	}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
