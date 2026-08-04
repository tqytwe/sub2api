package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type teamRouteIdempotencyStoreUnavailable struct{}

func (teamRouteIdempotencyStoreUnavailable) CreateProcessing(context.Context, *service.IdempotencyRecord) (bool, error) {
	return false, errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) GetByScopeAndKeyHash(context.Context, string, string) (*service.IdempotencyRecord, error) {
	return nil, errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) TryReclaim(context.Context, int64, string, time.Time, time.Time, time.Time) (bool, error) {
	return false, errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) ExtendProcessingLock(context.Context, int64, string, time.Time, time.Time) (bool, error) {
	return false, errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) MarkSucceeded(context.Context, int64, int, string, time.Time) error {
	return errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) MarkFailedRetryable(context.Context, int64, string, time.Time, time.Time) error {
	return errors.New("idempotency store unavailable")
}

func (teamRouteIdempotencyStoreUnavailable) DeleteExpired(context.Context, time.Time, int) (int64, error) {
	return 0, errors.New("idempotency store unavailable")
}

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

func TestTeamMutationRoutesEchoClientRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playHandler := handler.NewPlayHandler(service.NewPlayService(nil, nil, nil, nil, nil, nil), nil)
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: playHandler}, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	}))

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/play/teams/applications", body: `{"team_id":0}`},
		{method: http.MethodPost, path: "/api/v1/play/teams/applications/9/decision", body: `{"decision":""}`},
		{method: http.MethodPost, path: "/api/v1/play/teams/invite/rotate", body: `{}`},
		{method: http.MethodPut, path: "/api/v1/play/teams/recruiting", body: `{}`},
	} {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			httpRequest := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
			httpRequest.Header.Set("Content-Type", "application/json")
			httpRequest.Header.Set("X-Client-Request-ID", "mobile-team-write-42")

			router.ServeHTTP(recorder, httpRequest)

			require.GreaterOrEqual(t, recorder.Code, http.StatusBadRequest)
			require.Equal(t, "mobile-team-write-42", recorder.Header().Get("X-Client-Request-ID"))
		})
	}
}

func TestTeamMutationRoutesFailClosedWhenAnIdempotencyKeyCannotBeStored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousCoordinator := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(teamRouteIdempotencyStoreUnavailable{}, service.DefaultIdempotencyConfig()))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(previousCoordinator)
	})

	playHandler := handler.NewPlayHandler(service.NewPlayService(nil, nil, nil, nil, nil, nil), nil)
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: playHandler}, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	}))

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/play/teams/applications", body: `{"team_id":1}`},
		{method: http.MethodPost, path: "/api/v1/play/teams/applications/9/decision", body: `{"decision":"approve"}`},
		{method: http.MethodPost, path: "/api/v1/play/teams/invite/rotate", body: `{}`},
		{method: http.MethodPut, path: "/api/v1/play/teams/recruiting", body: `{"recruiting":true}`},
	} {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			httpRequest := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
			httpRequest.Header.Set("Content-Type", "application/json")
			httpRequest.Header.Set("Idempotency-Key", "mobile-team-write-key")
			httpRequest.Header.Set("X-Client-Request-ID", "mobile-team-write-42")

			router.ServeHTTP(recorder, httpRequest)

			require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
			require.Equal(t, "mobile-team-write-42", recorder.Header().Get("X-Client-Request-ID"))
			require.Contains(t, recorder.Body.String(), "IDEMPOTENCY_STORE_UNAVAILABLE")
		})
	}
}

func TestTeamMutationRoutesRemainOnCanonicalPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	routes.RegisterPlayRoutes(v1, &handler.Handlers{Play: handler.NewPlayHandler(nil, nil)}, middleware.JWTAuthMiddleware(func(c *gin.Context) {
		c.Next()
	}))

	registered := make(map[string]struct{})
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		"POST /api/v1/play/teams/applications",
		"POST /api/v1/play/teams/applications/:id/decision",
		"POST /api/v1/play/teams/invite/rotate",
		"PUT /api/v1/play/teams/recruiting",
	} {
		_, ok := registered[route]
		require.Truef(t, ok, "missing canonical team mutation route: %s", route)
	}
}
