//go:build integration

package acceptance

// Opt-in disposable PG fixture. Real recorder, writer, billing, HTTP handlers,
// JWT validation, admin authorization and PG user lookup. Token issuance is a
// test-only endpoint; password login, MFA, session binding, refresh revocation,
// providers, full scheduler coverage and production acceptance are not proven.
import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRoutingUIRealPGFixture363(t *testing.T) {
	if os.Getenv("ROUTING_UI_PG_FIXTURE") != "1" {
		t.Skip("opt-in isolated preparation fixture")
	}
	require.Empty(t, os.Getenv("SUB2API_TEST_POSTGRES_DSN"), "external databases are forbidden")
	require.Empty(t, os.Getenv("SUB2API_TEST_EXTERNAL_MODE"), "external test mode is forbidden")
	dist, stop := os.Getenv("LEDGER_BROWSER_DIST"), os.Getenv("ROUTING_UI_STOP_FILE")
	require.True(t, strings.HasPrefix(dist, "/tmp/ledger-routing363-"))
	require.True(t, strings.HasPrefix(stop, "/tmp/ledger-routing363-"))
	db := ledgertest.New(t)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	l := requestledger.New(db)
	r := gin.New()
	r.Use(gin.Recovery())
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	userRepo := repository.NewUserRepository(client, db)
	cfg := &config.Config{}
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	require.NoError(t, err)
	cfg.JWT.Secret = hex.EncodeToString(secret)
	cfg.JWT.AccessTokenExpireMinutes = 60
	// Secret and generated tokens remain in memory and are never printed or archived.
	auth := service.NewAuthService(nil, userRepo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := service.NewUserService(userRepo, nil, nil, nil)
	for _, uid := range []int{101, 102, 999, 1000} {
		role := "user"
		if uid >= 999 {
			role = "admin"
		}
		_, err := db.Exec(`INSERT INTO users (id,email,password_hash,role,balance,status) VALUES ($1,$2,'', $3,100,'active')`, uid, fmt.Sprintf("fixture-%d@example.invalid", uid), role)
		require.NoError(t, err)
	}
	profile := func(u *service.User) gin.H {
		return gin.H{"id": u.ID, "role": u.Role, "username": "Synthetic reviewer", "email": u.Email, "balance": u.Balance, "status": u.Status}
	}
	// Deliberately test-only token issuance, not a claim of production login coverage.
	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		var in struct {
			Email string `json:"email"`
		}
		if c.ShouldBindJSON(&in) != nil {
			c.AbortWithStatus(400)
			return
		}
		u, err := userRepo.GetByEmail(c.Request.Context(), in.Email)
		if err != nil {
			c.AbortWithStatus(401)
			return
		}
		token, err := auth.GenerateToken(c.Request.Context(), u)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"access_token": token, "expires_in": 3600, "user": profile(u)}})
	})
	jwtAuth := middleware.NewJWTAuthMiddleware(auth, users, nil, nil)
	adminAuth := gin.HandlerFunc(middleware.NewAdminAuthMiddleware(auth, users, nil, nil))
	r.GET("/api/v1/auth/me", gin.HandlerFunc(jwtAuth), func(c *gin.Context) {
		subject, _ := middleware.GetAuthSubjectFromContext(c)
		u, err := userRepo.GetByID(c.Request.Context(), subject.UserID)
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": profile(u)})
	})
	l.Start() // Keep the real instance lease healthy for the pending-state browser case.
	t.Cleanup(l.Stop)
	h := handler.NewRequestLedgerHandler(l)
	r.GET("/api/v1/admin/requests/:id/routing", adminAuth, h.RoutingAdmin)
	r.GET("/api/v1/admin/requests", adminAuth, h.ListAdmin)
	r.GET("/api/v1/admin/requests/:id", adminAuth, h.GetAdmin)
	r.GET("/api/v1/admin/requests/:id/usage/:usage_id", adminAuth, h.UsageAdmin)
	r.GET("/api/v1/requests", gin.HandlerFunc(jwtAuth), h.ListUser)
	r.GET("/api/v1/requests/:id", gin.HandlerFunc(jwtAuth), h.GetUser)
	r.GET("/api/v1/requests/:id/usage/:usage_id", gin.HandlerFunc(jwtAuth), h.UsageUser)
	ids := map[string]string{}
	var completeUsage int64
	for _, state := range []string{"complete", "truncated", "pending", "incomplete", "not_recorded", "unsupported_coverage", "zero_candidates"} {
		route := "/v1/responses"
		if state == "unsupported_coverage" {
			route = "/v1/images/generations"
		}
		request, err := l.Begin(context.Background(), route, "POST", "http")
		require.NoError(t, err)
		ctx := requestledger.WithHandle(context.Background(), request)
		require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
		ids[state] = request.ID
		if state != "not_recorded" && state != "unsupported_coverage" {
			c, end := requestledger.BeginRoutingSelection(ctx, true)
			pool := 2
			if state == "zero_candidates" {
				pool = 0
			}
			requestledger.RoutingPool(c, pool, false)
			switch state {
			case "complete":
				requestledger.RoutingVeto(c, "initial_filter", 235, "model_rate_limited")
				end(234, false, false)
				second, finish := requestledger.BeginRoutingSelection(ctx, true)
				requestledger.RoutingPool(second, 1, false)
				finish(234, false, false)
			case "truncated":
				for i := 0; i < 150; i++ {
					requestledger.RoutingVeto(c, "initial_filter", int64(300+i), "model_rate_limited")
				}
				end(0, false, true)
			case "incomplete":
				// The recorder rejects an unknown reason and declares its own evidence gap.
				requestledger.RoutingVeto(c, "initial_filter", 235, "fixture_invalid_reason")
				end(0, false, true)
			default:
				end(0, false, true)
			}
		}
		if state == "complete" {
			// Controlled fixture usage, real existing billing/usage repositories. No provider call.
			completeUsage = settleFixtureRequest(t, ctx, db, 101, 7, 3)
			for i := 0; i < 25; i++ {
				attempt, e := requestledger.BeginAttempt(ctx, 234, 22)
				require.NoError(t, e)
				require.NoError(t, attempt.Finish(ctx, 200, nil))
			}
			require.NoError(t, requestledger.ObserveUsage(ctx, 234))
		}
		if state != "pending" {
			if state == "complete" {
				require.NoError(t, request.Finish(ctx, "succeeded", 200, ""))
			} else {
				require.NoError(t, request.Finish(ctx, "failed", 503, "internal_error"))
			}
		}
		want := state
		if state == "zero_candidates" {
			want = "complete"
		}
		require.Eventually(t, func() bool {
			evidence, e := l.RoutingEvidence(context.Background(), requestledger.Viewer{Admin: true, UserID: 999}, request.ID)
			return e == nil && evidence.Status == want
		}, 5*time.Second, 10*time.Millisecond, "backend did not derive %s", want)
	}
	// Table availability is changed only inside this disposable PG instance. No
	// response stubbing: 503 must come from the real query and real handler.
	var unavailable atomic.Bool
	r.POST("/__fixture/routing-storage/:state", func(c *gin.Context) {
		target := c.Param("state")
		if target != "unavailable" && target != "available" {
			c.AbortWithStatus(400)
			return
		}
		if target == "unavailable" && !unavailable.Load() {
			_, err := db.Exec(`ALTER TABLE gateway_request_routing RENAME TO routing363_fixture_unavailable`)
			if err != nil {
				c.AbortWithStatus(500)
				return
			}
			unavailable.Store(true)
		} else if target == "available" && unavailable.Load() {
			_, err := db.Exec(`ALTER TABLE routing363_fixture_unavailable RENAME TO gateway_request_routing`)
			if err != nil {
				c.AbortWithStatus(500)
				return
			}
			unavailable.Store(false)
		}
		c.Status(204)
	})
	r.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"site_name": "极速蹬", "custom_menu_items": []gin.H{}}})
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.AbortWithStatus(404)
			return
		}
		p := filepath.Join(dist, filepath.Clean("/"+c.Request.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			c.File(p)
			return
		}
		c.File(filepath.Join(dist, "index.html"))
	})
	server := httptest.NewServer(r)
	defer server.Close()
	read := func(uid, path string) (int, string, string) {
		req, err := http.NewRequest("GET", server.URL+path, nil)
		require.NoError(t, err)
		if uid != "" {
			id, err := strconv.ParseInt(uid, 10, 64)
			require.NoError(t, err)
			u, err := userRepo.GetByID(context.Background(), id)
			require.NoError(t, err)
			token, err := auth.GenerateToken(context.Background(), u)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := server.Client().Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		return res.StatusCode, string(data), res.Header.Get("Cache-Control")
	}
	target := "/api/v1/admin/requests/" + ids["complete"] + "/routing"
	for _, test := range []struct {
		uid  string
		code int
	}{{"", 401}, {"101", 403}, {"102", 403}, {"999", 200}} {
		code, body, cache := read(test.uid, target)
		require.Equal(t, test.code, code)
		if code == 200 {
			require.Equal(t, "no-store", cache)
		} else {
			require.NotContains(t, body, ids["complete"])
			require.NotContains(t, body, "model_rate_limited")
		}
	}
	// Prove current database role, activity and token version override stale JWT claims.
	u, err := userRepo.GetByID(context.Background(), 1000)
	require.NoError(t, err)
	token, err := auth.GenerateToken(context.Background(), u)
	require.NoError(t, err)
	raw := func(value string) int {
		req, e := http.NewRequest("GET", server.URL+target, nil)
		require.NoError(t, e)
		req.Header.Set("Authorization", "Bearer "+value)
		req.Header.Set("X-Ledger-Test-User", "999") // Client-supplied identity cannot elevate access.
		res, e := server.Client().Do(req)
		require.NoError(t, e)
		defer res.Body.Close()
		_, e = io.Copy(io.Discard, res.Body)
		require.NoError(t, e)
		return res.StatusCode
	}
	require.Equal(t, 401, raw("forged.invalid.token"))
	for _, test := range []struct {
		sql    string
		status int
	}{
		{`UPDATE users SET role='user' WHERE id=1000`, 403},
		{`UPDATE users SET role='admin',status='disabled' WHERE id=1000`, 401},
		{`UPDATE users SET status='active',email='changed-fixture-1000@example.invalid' WHERE id=1000`, 401},
	} {
		_, err := db.Exec(test.sql)
		require.NoError(t, err)
		require.Equal(t, test.status, raw(token))
	}
	_, err = db.Exec(`UPDATE users SET email='fixture-1000@example.invalid' WHERE id=1000`)
	require.NoError(t, err)
	require.Equal(t, 200, raw(token))
	fmt.Println("ROUTING_PG_AUTH JWT_signature_role_activity_version_and_client_identity=passed")
	code, _, _ := read("999", "/api/v1/admin/requests/not-a-uuid/routing")
	require.Equal(t, 400, code)
	code, _, _ = read("999", "/api/v1/admin/requests/00000000-0000-4000-8000-000000000099/routing")
	require.Equal(t, 404, code)
	code, _, _ = read("101", "/api/v1/requests/"+ids["complete"]+"/routing")
	require.Equal(t, 404, code)
	code, _, _ = read("102", "/api/v1/requests/"+ids["complete"]+"?view=summary")
	require.Equal(t, 404, code)
	var before float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=101`).Scan(&before))
	require.Equal(t, 98.75, before)
	metadata, _ := json.Marshal(map[string]any{"url": server.URL, "requests": ids, "usage_id": completeUsage, "balance": before, "scope": "real recorder/writer/PG/query/HTTP; real JWT/admin middleware/PG user lookup; test-only token issuance and synthetic scheduler/usage inputs; no password login/MFA/binding/provider/full scheduler proof"})
	fmt.Printf("ROUTING_PG_READY %s\n", metadata)
	timer := time.NewTimer(10 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	done := false
	for !done {
		select {
		case <-timer.C:
			t.Fatal("browser did not finish")
		case <-ticker.C:
			if _, err := os.Stat(stop); err == nil {
				done = true
			}
		}
	}
	require.False(t, unavailable.Load(), "browser failed to restore fixture routing table")
	var after float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=101`).Scan(&after))
	require.Equal(t, before, after)
	fmt.Println("ROUTING_PG_FINAL read_only_balance_unchanged=true")
}
