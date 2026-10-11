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
