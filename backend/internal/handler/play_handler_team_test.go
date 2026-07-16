package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTeamLifecycleRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{
		Play: handler.NewPlayHandler(nil, nil),
	}, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	}))

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/play/teams/leave"},
		{method: http.MethodPost, path: "/api/v1/play/teams/transfer", body: `{"target_user_id":9}`},
		{method: http.MethodPost, path: "/api/v1/play/teams/remove", body: `{"target_user_id":9}`},
	} {
		recorder := httptest.NewRecorder()
		httpRequest := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		httpRequest.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, httpRequest)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, request.path)
	}
}

func TestTeamLifecycleHandlersRejectInvalidRequestsBeforeCallingService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playHandler := handler.NewPlayHandler(nil, nil)

	for _, invoke := range []struct {
		name string
		call func(*gin.Context)
	}{
		{name: "transfer", call: playHandler.TeamTransfer},
		{name: "remove", call: playHandler.TeamRemove},
	} {
		t.Run(invoke.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"target_user_id":0}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})

			invoke.call(ctx)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
