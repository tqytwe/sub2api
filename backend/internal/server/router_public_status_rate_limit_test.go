package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicStatusRoutesApplyThePublicIPGuardBeforeBothHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	guardCalls := 0
	guard := func(c *gin.Context) {
		guardCalls++
		c.Set("public-ip-guard", true)
		c.Next()
	}
	protected := func(c *gin.Context) {
		guarded, exists := c.Get("public-ip-guard")
		if !exists || guarded != true {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusNoContent)
	}

	registerPublicStatusRoutes(v1, guard, protected, protected)

	for _, path := range []string{
		"/api/v1/public/home-stats",
		"/api/v1/public/status-summary",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNoContent, recorder.Code, path)
	}
	require.Equal(t, 2, guardCalls)
}
