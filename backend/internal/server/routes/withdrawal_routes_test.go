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

func TestAdminWithdrawalRoutesContractAndSensitiveStepUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			Withdrawal: adminhandler.NewWithdrawalHandler(nil),
		},
	}
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) {
		servermiddleware.AbortWithError(c, http.StatusPreconditionRequired, "STEP_UP_REQUIRED", "step-up required")
	})
	registerWithdrawalRoutes(router.Group("/api/v1/admin"), handlers, stepUp)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/admin/withdrawals",
		"GET /api/v1/admin/withdrawals/settings",
		"PUT /api/v1/admin/withdrawals/settings",
		"POST /api/v1/admin/withdrawals/user-settings/batch",
		"GET /api/v1/admin/withdrawals/users/:id/settings",
		"PUT /api/v1/admin/withdrawals/users/:id/settings",
		"GET /api/v1/admin/withdrawals/:id",
		"POST /api/v1/admin/withdrawals/:id/approve",
		"POST /api/v1/admin/withdrawals/:id/reject",
		"GET /api/v1/admin/withdrawals/:id/payout-sensitive",
		"POST /api/v1/admin/withdrawals/:id/mark-paid",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route: %s", route)
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/withdrawals/12/approve"},
		{http.MethodPost, "/api/v1/admin/withdrawals/12/reject"},
		{http.MethodGet, "/api/v1/admin/withdrawals/12/payout-sensitive"},
		{http.MethodPost, "/api/v1/admin/withdrawals/12/mark-paid"},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(tc.method, tc.path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusPreconditionRequired, recorder.Code, tc.path)
	}
}
