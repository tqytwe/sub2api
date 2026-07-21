package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserWalletRoutesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Wallet: handler.NewWalletHandler(nil),
		Fund:   handler.NewFundHandler(nil),
	}
	RegisterUserRoutes(
		router.Group("/api/v1"),
		handlers,
		middleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.AbortWithStatus(http.StatusUnauthorized)
		}),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"GET /api/v1/user/wallet/summary",
		"GET /api/v1/user/wallet/transactions",
		"GET /api/v1/user/wallet/withdrawals/availability",
		"GET /api/v1/user/wallet/withdrawal-account",
		"PUT /api/v1/user/wallet/withdrawal-account",
		"GET /api/v1/user/wallet/withdrawals",
		"POST /api/v1/user/wallet/withdrawals",
		"GET /api/v1/user/wallet/withdrawals/:id",
		"POST /api/v1/user/wallet/withdrawals/:id/cancel",
		"GET /api/v1/user/wallet/refund-requests",
		"POST /api/v1/user/wallet/refund-requests",
		"GET /api/v1/user/wallet/refund-requests/:id",
		"POST /api/v1/user/wallet/refund-requests/:id/cancel",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route: %s", route)
	}

	for _, path := range []string{
		"/api/v1/user/wallet/summary",
		"/api/v1/user/wallet/transactions",
		"/api/v1/user/wallet/withdrawals/availability",
		"/api/v1/user/wallet/withdrawal-account",
		"/api/v1/user/wallet/withdrawals",
		"/api/v1/user/wallet/refund-requests",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, path)
	}
}
