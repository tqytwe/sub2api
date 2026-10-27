package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerIdentifierAdminRouteProtection(t *testing.T) {
	r := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{}, RequestLedger: &handler.RequestLedgerHandler{}}
	var mu sync.Mutex
	chains := map[string][]string{}
	auth := middleware.AdminAuthMiddleware(func(c *gin.Context) {
		mu.Lock()
		names := c.HandlerNames()
		chains[c.FullPath()] = append([]string(nil), names[:len(names)-1]...)
		mu.Unlock()
		if c.GetHeader("X-Test-Role") == "" {
			c.AbortWithStatus(401)
			return
		}
		c.AbortWithStatus(403)
	})
	RegisterAdminRoutes(r.Group("/api/v1"), h, auth,
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	server := httptest.NewServer(r)
	defer server.Close()
	for _, suffix := range []string{"", "/identifiers"} {
		for _, role := range []string{"", "ordinary"} {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/admin/requests/00000000-0000-4000-8000-000000000001"+suffix, nil)
			require.NoError(t, err)
			req.Header.Set("X-Test-Role", role)
			resp, err := server.Client().Do(req)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			want := 401
			if role != "" {
				want = 403
			}
			require.Equal(t, want, resp.StatusCode)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, chains["/api/v1/admin/requests/:id"])
	require.Equal(t, chains["/api/v1/admin/requests/:id"], chains["/api/v1/admin/requests/:id/identifiers"])
}

func TestRequestLedgerRoutesAreAdministratorOnly(t *testing.T) {
	r := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{}, RequestLedger: &handler.RequestLedgerHandler{}}
	allow := func(c *gin.Context) { c.Next() }
	RegisterUserRoutes(r.Group("/api/v1"), h,
		middleware.JWTAuthMiddleware(allow),
		middleware.AuditLogMiddleware(allow), nil,
		&middleware.PanelRateLimiter{},
	)
	RegisterAdminRoutes(r.Group("/api/v1"), h,
		middleware.AdminAuthMiddleware(allow),
		middleware.AuditLogMiddleware(allow),
		middleware.StepUpAuthMiddleware(allow), nil,
		&middleware.PanelRateLimiter{},
	)

	paths := make(map[string]bool)
	for _, route := range r.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	for _, path := range []string{
		"GET /api/v1/requests",
		"GET /api/v1/requests/:id",
		"GET /api/v1/requests/:id/usage/:usage_id",
	} {
		require.False(t, paths[path], "ordinary user request-ledger route must not be registered: %s", path)
	}
	for _, path := range []string{
		"GET /api/v1/admin/requests",
		"GET /api/v1/admin/requests/:id",
		"GET /api/v1/admin/requests/:id/routing",
		"GET /api/v1/admin/requests/:id/identifiers",
		"GET /api/v1/admin/requests/:id/usage/:usage_id",
	} {
		require.True(t, paths[path], "administrator request-ledger route must remain registered: %s", path)
	}
}
