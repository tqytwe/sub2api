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

func TestPublicPelicanShowcaseRoutesRequireAPIKeyBeforeHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	authCalls := 0
	auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		authCalls++
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	showcase := handler.NewPelicanShowcaseHandler(nil)

	RegisterPublicPelicanShowcaseRoutes(v1, &handler.Handlers{PelicanShowcase: showcase}, auth, nil)

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/v1/public/pelican-showcase",
		"HEAD /api/v1/public/pelican-showcase",
		"GET /api/v1/public/pelican-showcase/items/:id",
		"HEAD /api/v1/public/pelican-showcase/items/:id",
	} {
		require.True(t, routes[route], route)
	}

	for _, path := range []string{
		"/api/v1/public/pelican-showcase",
		"/api/v1/public/pelican-showcase/items/1",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, recorder.Code, path)
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"), path)
	}
	require.Equal(t, 2, authCalls)
}
