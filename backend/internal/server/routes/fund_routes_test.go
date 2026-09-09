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
		"GET /api/v1/admin/funds/refund-requests/:request_no",
		"POST /api/v1/admin/funds/refund-requests/:request_no/approve",
		"POST /api/v1/admin/funds/refund-requests/:request_no/reject",
		"GET /api/v1/admin/funds/refund-requests/:request_no/payout-sensitive",
		"POST /api/v1/admin/funds/refund-requests/:request_no/mark-paid",
		"POST /api/v1/admin/funds/gifts",
		"POST /api/v1/admin/funds/compensations",
		"POST /api/v1/admin/funds/offline-recharges",
		"GET /api/v1/admin/funds/accounts/search",
		"GET /api/v1/admin/funds/operations",
		"GET /api/v1/admin/funds/operations/:operation_no",
		"GET /api/v1/admin/funds/operations/:operation_no/sensitive",
		"POST /api/v1/admin/funds/operations/:operation_no/corrections",
		"POST /api/v1/admin/funds/operations/:operation_no/corrections/retry",
		"POST /api/v1/admin/funds/operations/:operation_no/corrections/cancel",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route: %s", route)
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/FR-20260908-ABC/approve"},
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/FR-20260908-ABC/reject"},
		{http.MethodGet, "/api/v1/admin/funds/refund-requests/FR-20260908-ABC/payout-sensitive"},
		{http.MethodPost, "/api/v1/admin/funds/refund-requests/FR-20260908-ABC/mark-paid"},
		{http.MethodPost, "/api/v1/admin/funds/gifts"},
		{http.MethodPost, "/api/v1/admin/funds/compensations"},
		{http.MethodPost, "/api/v1/admin/funds/offline-recharges"},
		{http.MethodGet, "/api/v1/admin/funds/operations/FM-20260908-ABC/sensitive"},
		{http.MethodPost, "/api/v1/admin/funds/operations/FM-20260908-ABC/corrections"},
		{http.MethodPost, "/api/v1/admin/funds/operations/FM-20260908-ABC/corrections/retry"},
		{http.MethodPost, "/api/v1/admin/funds/operations/FM-20260908-ABC/corrections/cancel"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, tc.path)
	}
}
