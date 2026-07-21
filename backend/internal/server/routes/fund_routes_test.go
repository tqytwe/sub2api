package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminFundRoutesContractAndSensitiveStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Fund: adminhandler.NewFundHandler(nil),
		},
	}
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		servermiddleware.AbortWithError(c, http.StatusPreconditionRequired, "STEP_UP_REQUIRED", "step-up required")
	})
	registerFundRoutes(router.Group("/api/v1/admin"), handlers, stepUp)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/admin/funds/refund-requests",
		"GET /api/v1/admin/funds/refund-requests/:id",
		"POST /api/v1/admin/funds/refund-requests/:id/approve",
		"POST /api/v1/admin/funds/refund-requests/:id/reject",
		"GET /api/v1/admin/funds/refund-requests/:id/payout-sensitive",
		"POST /api/v1/admin/funds/refund-requests/:id/mark-paid",
		"POST /api/v1/admin/funds/gifts",
		"POST /api/v1/admin/funds/offline-recharges",
		"GET /api/v1/admin/funds/classifications/signup-gift-30/preview",
		"POST /api/v1/admin/funds/classifications/signup-gift-30/execute",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route: %s", route)
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/12/approve"},
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/12/reject"},
		{http.MethodGet, "/api/v1/admin/funds/refund-requests/12/payout-sensitive"},
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/12/mark-paid"},
		{http.MethodPost, "/api/v1/admin/funds/gifts"},
		{http.MethodPost, "/api/v1/admin/funds/offline-recharges"},
		{http.MethodPost, "/api/v1/admin/funds/classifications/signup-gift-30/execute"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, tc.path)
	}
}
