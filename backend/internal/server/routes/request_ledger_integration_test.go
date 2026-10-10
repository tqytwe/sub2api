//go:build integration

package routes

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

// Enumerate the real registered gateway routes, including aliases, and reject
// before handlers. This proves ingress coverage; it does not claim protocol success.
func TestRequestLedgerRealGatewayRouteRejectionMatrix(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	cfg := &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 64, TextMaxBodySize: 64}}
	for _, status := range []int{401, 429} {
		r := gin.New()
		r.Use(requestledger.Middleware(l))
		auth := middleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			require.NotNil(t, requestledger.FromContext(c.Request.Context()))
			if status == 429 {
				require.NoError(t, requestledger.BindIdentity(c.Request.Context(), 101, 201))
			}
			c.AbortWithStatus(status)
		})
		RegisterGatewayRoutes(r, &handler.Handlers{Gateway: &handler.GatewayHandler{}, OpenAIGateway: &handler.OpenAIGatewayHandler{}, AsyncImage: handler.NewAsyncImageHandler(nil, nil, nil)}, auth, nil, nil, nil, nil, nil, cfg)
		routes := r.Routes()
		require.Greater(t, len(routes), 100)
		for _, route := range routes {
			path := route.Path
			for _, param := range []string{":model", ":name", ":id", ":call_id", ":task_id", ":request_id", ":batch_id", ":item_id", ":asset_id", ":voice_id", "*path", "*action", "*suffix"} {
				path = strings.ReplaceAll(path, param, "fixture")
			}
			t.Run(route.Method+" "+route.Path, func(t *testing.T) {
				require.True(t, requestledger.ManagedPath(path), "registered managed gateway route missing from durable admission")
				var before int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&before))
				w := httptest.NewRecorder()
				req := httptest.NewRequest(route.Method, path, strings.NewReader("{}"))
				req.Header.Set("Accept", "application/json")
				r.ServeHTTP(w, req)
				var after int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&after))
				require.Equal(t, before+1, after)
				var state string
				var code int
				require.NoError(t, db.QueryRow(`SELECT execution_state,http_status FROM gateway_requests ORDER BY started_at DESC LIMIT 1`).Scan(&state, &code))
				require.NotEqual(t, "inflight", state)
				require.Equal(t, w.Code, code)
			})
		}
	}
	require.NoError(t, l.Recover(context.Background()))
}

func TestRequestLedgerActualAuthAndBodyLimitRejections(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	auth := middleware.NewAPIKeyAuthMiddleware(service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, cfg)
	r := gin.New()
	r.Use(requestledger.Middleware(l))
	r.POST("/v1/responses", gin.HandlerFunc(auth), func(c *gin.Context) { t.Fatal("anonymous request passed auth") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil))
	require.Equal(t, 401, w.Code)
	var code int
	var uid *int64
	require.NoError(t, db.QueryRow(`SELECT http_status,user_id FROM gateway_requests`).Scan(&code, &uid))
	require.Equal(t, 401, code)
	require.Nil(t, uid)

	bodyRouter := gin.New()
	bodyRouter.Use(requestledger.Middleware(l))
	bodyRouter.POST("/v1/images/edits", middleware.RequestBodyLimit(4), func(c *gin.Context) {
		_, err := c.GetRawData()
		require.Error(t, err)
		c.AbortWithStatus(400)
	})
	oversized := httptest.NewRecorder()
	bodyRouter.ServeHTTP(oversized, httptest.NewRequest("POST", "/v1/images/edits", strings.NewReader("oversize-body")))
	require.Equal(t, 400, oversized.Code)
	require.NoError(t, db.QueryRow(`SELECT http_status,user_id FROM gateway_requests WHERE route='/v1/images/edits'`).Scan(&code, &uid))
	require.Equal(t, 400, code)
	require.Nil(t, uid)
}

func TestRequestLedgerImageStudioAndBFFRouteRejectionMatrix(t *testing.T) {
	db := ledgertest.New(t)
	l := requestledger.New(db)
	r := gin.New()
	r.Use(requestledger.Middleware(l), func(c *gin.Context) { c.AbortWithStatus(401) })
	auth := middleware.JWTAuthMiddleware(func(c *gin.Context) { c.AbortWithStatus(401) })
	v1 := r.Group("/api/v1")
	studio := &handler.ImageStudioHandler{}
	RegisterImageStudioRoutes(v1, &handler.Handlers{ImageStudio: studio}, auth)
	RegisterNextChatRoutes(v1, auth, nil, nil, nil, studio, nil, &config.Config{}, nil)
	count := 0
	for _, route := range r.Routes() {
		if !strings.Contains(route.Path, "/image-studio/") {
			continue
		}
		count++
		t.Run(route.Method+" "+route.Path, func(t *testing.T) {
			require.True(t, requestledger.ManagedPath(route.Path))
			path := strings.ReplaceAll(route.Path, ":id", "fixture")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(route.Method, path, nil))
			require.Equal(t, 401, w.Code)
			var persisted int
			require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE route=$1 AND method=$2 AND http_status=401 AND user_id IS NULL`, route.Path, route.Method).Scan(&persisted))
			require.Equal(t, 1, persisted)
		})
	}
	require.Greater(t, count, 25)
}
