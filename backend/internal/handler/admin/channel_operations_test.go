package admin

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelOperationsHTTPRepository struct {
	service.ChannelRepository
	called   bool
	mutation service.ChannelOperationsMutation
	err      error
}

func (r *channelOperationsHTTPRepository) ChannelOperationsCatalog(context.Context, int64, service.ChannelOperationsQuery) (*service.ChannelOperationsCatalog, error) {
	r.called = true
	return &service.ChannelOperationsCatalog{}, nil
}

func (r *channelOperationsHTTPRepository) MutateChannelOperations(_ context.Context, _ int64, mutation service.ChannelOperationsMutation) (int64, error) {
	r.called = true
	r.mutation = mutation
	return 43, r.err
}

func (r *channelOperationsHTTPRepository) ReadChannelOperations(context.Context, service.ChannelOperationsFilter) *service.ChannelOperationsSnapshot {
	r.called = true
	return &service.ChannelOperationsSnapshot{Quality: service.ChannelOperationsSection{Status: "unsupported_scope"}}
}

func newChannelOperationsHTTP(t *testing.T, repo *channelOperationsHTTPRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil)
	router.GET("/channels/:id/operations", handler.OperationsCatalog)
	router.POST("/channels/:id/operations", handler.MutateOperations)
	router.GET("/snapshot", handler.OperationsSnapshot)
	return router
}

func TestChannelOperationsHTTPValidationBeforeRepository(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{method: "GET", path: "/channels/not-an-id/operations"},
		{method: "GET", path: "/channels/7/operations?page=0"},
		{method: "GET", path: "/channels/7/operations?page_size=101"},
		{method: "GET", path: "/channels/7/operations?membership=credentials"},
		{method: "POST", path: "/channels/7/operations", body: `{"action":"delete_upstream","revision":0,"upstream_id":2}`},
		{method: "POST", path: "/channels/7/operations", body: `{"action":"assign","revision":0,"upstream_id":2,"account_id":9,"expected_upstream_id":3}`},
		{method: "POST", path: "/channels/7/operations", body: `{"action":"create_upstream","name":"Provider"}`},
		{method: "POST", path: "/channels/7/operations", body: `{"action":"probe","revision":0}`},
		{method: "POST", path: "/channels/7/operations", body: `{"revision":`},
		{method: "GET", path: "/snapshot?from=bad&until=bad"},
		{method: "GET", path: "/snapshot?from=2026-01-01T00:00:00Z&until=2026-04-01T00:00:00Z"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			repo := &channelOperationsHTTPRepository{}
			router := newChannelOperationsHTTP(t, repo)
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 400, response.Code)
			require.False(t, repo.called)
		})
	}
}

func TestChannelOperationsSnapshotHTTPReturnsUnsupportedQualityExplicitly(t *testing.T) {
	repo := &channelOperationsHTTPRepository{}
	router := newChannelOperationsHTTP(t, repo)
	from := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	until := time.Now().UTC().Format(time.RFC3339)
	request := httptest.NewRequest("GET", "/snapshot?from="+from+"&until="+until, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.True(t, repo.called)
	require.Contains(t, response.Body.String(), `"status":"unsupported_scope"`)
}

func TestChannelOperationsHTTPConflictAndExplicitZeroRevision(t *testing.T) {
	repo := &channelOperationsHTTPRepository{err: service.ErrChannelOperationsConflict}
	router := newChannelOperationsHTTP(t, repo)
	request := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/channels/7/operations", strings.NewReader(`{"action":"detach","revision":0,"account_id":9,"expected_upstream_id":3,"confirm":true}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, req)
		return response
	}
	require.Equal(t, 409, request().Code)
	require.True(t, repo.called)
	require.NotNil(t, repo.mutation.Revision)
	require.Zero(t, *repo.mutation.Revision)

	repo.err = nil
	response := request()
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"revision":43`)
}

func TestChannelOperationsMutationAddsSafeAuditSummary(t *testing.T) {
	repo := &channelOperationsHTTPRepository{}
	router := gin.New()
	var action string
	var extra map[string]any
	router.Use(func(c *gin.Context) {
		c.Next()
		if value, ok := c.Get("audit_action"); ok {
			action, _ = value.(string)
		}
		if value, ok := c.Get("audit_extra"); ok {
			extra, _ = value.(map[string]any)
		}
	})
	handler := NewChannelHandler(service.NewChannelService(repo, nil, nil, nil, nil), nil, nil)
	router.POST("/channels/:id/operations", handler.MutateOperations)
	request := httptest.NewRequest("POST", "/channels/7/operations", strings.NewReader(`{"action":"assign","revision":12,"upstream_id":2,"account_id":9}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.Equal(t, "channel_operations.assign", action)
	require.Equal(t, map[string]any{"config_version": int64(12), "operation": "assign", "confirm": false}, extra)
}
