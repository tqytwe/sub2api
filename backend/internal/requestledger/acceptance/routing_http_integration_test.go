//go:build integration

package acceptance

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerRoutingHTTPAdminIsolationAndDetailFailure(t *testing.T) {
	db := ledgertest.New(t)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	l := requestledger.New(db)
	defer l.Stop()
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	require.NoError(t, h.Finish(ctx, "failed", 401, "unauthorized"))
	handler := handler.NewRequestLedgerHandler(l)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		role := c.GetHeader("X-Test-Role")
		if role != "" {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 101})
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
	})
	router.GET("/api/v1/admin/requests/:id/routing", handler.RoutingAdmin)
	router.GET("/api/v1/admin/requests/:id", handler.GetAdmin)
	router.GET("/api/v1/requests/:id", handler.GetUser)
	server := httptest.NewServer(router)
	defer server.Close()
	get := func(path, role string) (int, string, string) {
		req, err := http.NewRequest("GET", server.URL+path, nil)
		require.NoError(t, err)
		req.Header.Set("X-Test-Role", role)
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.NoError(t, res.Body.Close())
		return res.StatusCode, string(data), res.Header.Get("Cache-Control")
	}
	path := "/api/v1/admin/requests/" + h.ID + "/routing"
	for _, tc := range []struct {
		role   string
		status int
	}{{"", 401}, {"user", 403}, {"admin", 200}} {
		status, body, cache := get(path, tc.role)
		require.Equal(t, tc.status, status)
		if status == 200 {
			require.Contains(t, body, "not_recorded")
			require.Equal(t, "no-store", cache)
		} else {
			require.NotContains(t, body, h.ID)
		}
	}
	status, _, _ := get("/api/v1/admin/requests/"+uuid.NewString()+"/routing", "admin")
	require.Equal(t, 404, status)
	status, _, _ = get("/api/v1/admin/requests/not-a-uuid/routing", "admin")
	require.Equal(t, 400, status)
	status, _, _ = get("/api/v1/requests/"+h.ID+"/routing", "user")
	require.Equal(t, 404, status)
	_, err = db.Exec(`ALTER TABLE gateway_request_routing RENAME TO routing_fixture_unavailable`)
	require.NoError(t, err)
	status, body, _ := get(path, "admin")
	require.Equal(t, 503, status)
	require.NotContains(t, strings.ToLower(body), "sql")
	status, _, _ = get("/api/v1/admin/requests/"+h.ID, "admin")
	require.Equal(t, 200, status)
	status, _, _ = get("/api/v1/requests/"+h.ID, "user")
	require.Equal(t, 200, status)
}

// A zero schedulable source is not evidence of an empty configured group.
func TestRequestLedgerRoutingRepositoryPoolIsPrefiltered(t *testing.T) {
	db := ledgertest.New(t)
	ctx := context.Background()
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	_, err := db.Exec(`INSERT INTO groups(id,name,platform) VALUES(901,'routing fixture','openai');
 INSERT INTO accounts(id,name,platform,type,schedulable) VALUES(902,'routing fixture','openai','apikey',false);
 INSERT INTO account_groups(account_id,group_id) VALUES(902,901)`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := repository.NewAccountRepository(client, db, nil)
	accounts, err := repo.ListSchedulableByGroupIDAndPlatform(ctx, 901, service.PlatformOpenAI)
	require.NoError(t, err)
	require.Empty(t, accounts)
	var configured int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM account_groups WHERE group_id=901`).Scan(&configured))
	require.Equal(t, 1, configured)
	l := requestledger.New(db)
	defer l.Stop()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	c, end := requestledger.BeginRoutingSelection(requestledger.WithHandle(ctx, h), true)
	requestledger.RoutingPool(c, len(accounts), false)
	end(0, false, true)
	require.NoError(t, h.Finish(ctx, "failed", 503, "internal_error"))
	var e *requestledger.RoutingEvidence
	require.Eventually(t, func() bool {
		e, err = l.RoutingEvidence(ctx, requestledger.Viewer{Admin: true, UserID: 1}, h.ID)
		return err == nil && e.Status == "complete"
	}, time.Second, 10*time.Millisecond)
	require.False(t, e.SourcePopulationComplete)
	require.Zero(t, e.VetoCount)
	require.Len(t, e.Events, 3)
	require.Equal(t, "prefiltered_schedulable_pool", e.Events[1].Reason)
	require.Zero(t, e.Events[1].Value)
}
