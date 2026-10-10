//go:build integration

package acceptance

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ledgerFixtureRouter(t *testing.T) (*gin.Engine, *requestledger.Ledger, *sql.DB) {
	t.Helper()
	db := ledgertest.New(t)
	require.NoError(t, repository.ApplyMigrations(context.Background(), db))
	l := requestledger.New(db)
	h := &handler.RequestLedgerHandler{Ledger: l}
	r := gin.New()
	r.Use(requestledger.Middleware(l))
	// Explicitly synthetic identity injection; no real login, token issuance or account access.
	r.Use(func(c *gin.Context) {
		uid, _ := strconv.ParseInt(c.GetHeader("X-Ledger-Test-User"), 10, 64)
		if uid > 0 {
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: uid})
			role := "user"
			if uid == 999 {
				role = "admin"
			}
			c.Set(string(middleware.ContextKeyUserRole), role)
		}
	})
	r.GET("/api/v1/requests", h.ListUser)
	r.GET("/api/v1/requests/:id", h.GetUser)
	r.GET("/api/v1/requests/:id/usage/:usage_id", h.UsageUser)
	r.GET("/api/v1/admin/requests", h.ListAdmin)
	r.GET("/api/v1/admin/requests/:id", h.GetAdmin)
	r.GET("/api/v1/admin/requests/:id/usage/:usage_id", h.UsageAdmin)
	r.POST("/v1/responses", func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			c.Status(401)
			return
		}
		require.NoError(t, requestledger.BindIdentity(c.Request.Context(), subject.UserID, subject.UserID+100))
		status, _ := strconv.Atoi(c.Query("status"))
		if status < 100 {
			status = 200
		}
		if status == 200 {
			a, err := requestledger.BeginAttempt(c.Request.Context(), 234, 22)
			require.NoError(t, err)
			require.NoError(t, a.Finish(c.Request.Context(), 200, nil))
		}
		c.JSON(status, gin.H{"ok": status == 200})
	})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var persisted int
		err := db.QueryRow(`SELECT count(*) FROM gateway_request_attempts a JOIN gateway_requests r ON r.id=a.request_id WHERE a.execution_state='inflight' AND a.account_id=234 AND a.credential_account_id=22 AND r.user_id IS NOT NULL`).Scan(&persisted)
		if err != nil || persisted == 0 {
			t.Error("upstream observed a send without precommitted request and attempt evidence")
			w.WriteHeader(500)
			return
		}
		if req.Header.Get("X-Request-ID") != "" {
			t.Error("private identity unexpectedly forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":7,"output_tokens":3}}`)
	}))
	t.Cleanup(upstream.Close)
	client := &http.Client{Transport: requestledger.Transport(upstream.Client().Transport, 234), Timeout: 5 * time.Second}
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		if !ok {
			c.Status(401)
			return
		}
		ctx := requestledger.WithAccount(c.Request.Context(), 234, 22)
		require.NoError(t, requestledger.BindIdentity(ctx, subject.UserID, subject.UserID+100))
		req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		if err != nil {
			c.Status(502)
			return
		}
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, resp.Body.Close())
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		var result struct {
			Usage struct {
				Input  int `json:"input_tokens"`
				Output int `json:"output_tokens"`
			} `json:"usage"`
		}
		require.NoError(t, json.Unmarshal(body, &result))
		require.Equal(t, 7, result.Usage.Input)
		require.Equal(t, 3, result.Usage.Output)
		if c.Query("auxiliary") == "1" {
			download, err := http.NewRequestWithContext(ctx, "GET", upstream.URL, nil)
			require.NoError(t, err)
			downloaded, err := client.Do(download)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, downloaded.Body)
			require.NoError(t, err)
			require.NoError(t, downloaded.Body.Close())
		}
		require.NoError(t, requestledger.ObserveUsage(ctx, 234))
		settleFixtureRequest(t, ctx, db, subject.UserID, result.Usage.Input, result.Usage.Output)
		c.JSON(200, result)
	})
	return r, l, db
}

// Only disposable identities; the synthetic API key cannot authenticate at this fixture.
// Use the real existing billing repository and usage-log writer, not a fabricated settled flag.
func seedBilledRequest(t *testing.T, r *gin.Engine, db *sql.DB, uid int64, auxiliary ...bool) (string, int64) {
	t.Helper()
	w := httptest.NewRecorder()
	path := "/v1/chat/completions"
	if len(auxiliary) > 0 && auxiliary[0] {
		path += "?auxiliary=1"
	}
	req := httptest.NewRequest("POST", path, nil)
	req.Header.Set("X-Ledger-Test-User", strconv.FormatInt(uid, 10))
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	var id string
	var usageID int64
	require.NoError(t, db.QueryRow(`SELECT r.id,u.id FROM gateway_requests r JOIN gateway_request_billing_links b ON b.request_id=r.id JOIN usage_logs u ON u.request_id=b.billing_request_id AND u.api_key_id=b.api_key_id AND u.billing_request_fingerprint=b.request_fingerprint WHERE r.user_id=$1 AND r.route='/v1/chat/completions' ORDER BY r.started_at DESC LIMIT 1`, uid).Scan(&id, &usageID))
	return id, usageID
}

func settleFixtureRequest(t *testing.T, ctx context.Context, db *sql.DB, uid int64, inputTokens, outputTokens int) int64 {
	t.Helper()
	keyID := uid + 100
	_, err := db.Exec(`INSERT INTO users(id,email,password_hash,balance) VALUES($1,$2,'disabled-fixture',100) ON CONFLICT DO NOTHING`, uid, fmt.Sprintf("fixture-%d@example.invalid", uid))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO api_keys(id,user_id,key,name) VALUES($1,$2,$3,'disabled fixture') ON CONFLICT DO NOTHING`, keyID, uid, fmt.Sprintf("not-a-credential-fixture-%d", uid))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO accounts(id,name,platform,type) VALUES(234,'disabled fixture','openai','apikey') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	cmd := &service.UsageBillingCommand{RequestID: uuid.NewString(), UserID: uid, APIKeyID: keyID, AccountID: 234, AccountType: service.AccountTypeAPIKey, Model: "synthetic-usage-fixture", InputTokens: inputTokens, OutputTokens: outputTokens, BilledCost: 1.25, BalanceCost: 1.25}
	billing := repository.NewUsageBillingRepositoryWithLedger(client, db, service.NewBalanceLedgerService(db, nil, nil))
	proof, err := billing.Apply(ctx, cmd)
	require.NoError(t, err)
	usage := &service.UsageLog{BillingSettled: proof.SettlementVerified, UserID: uid, APIKeyID: keyID, AccountID: 234, RequestID: cmd.RequestID, BillingRequestFingerprint: cmd.RequestFingerprint, Model: cmd.Model, InputTokens: inputTokens, OutputTokens: outputTokens, BilledCost: 1.25, ActualCost: 1.25}
	_, err = repository.NewUsageLogRepository(client, db).Create(ctx, usage)
	require.NoError(t, err)
	return usage.ID
}

func TestRequestLedgerHTTPLinkedUsageAndRetention(t *testing.T) {
	r, _, db := ledgerFixtureRouter(t)
	id, usageID := seedBilledRequest(t, r, db, 101)
	read := func(user, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-Ledger-Test-User", user)
		r.ServeHTTP(w, req)
		return w
	}
	detail := read("101", "/api/v1/requests/"+id)
	require.Equal(t, 200, detail.Code)
	for _, evidence := range []string{`"execution_state":"succeeded"`, `"usage_state":"known"`, `"settlement_state":"settled"`, `"attempt_count":1`, `"output_observed":true`} {
		require.Contains(t, detail.Body.String(), evidence)
	}
	var balance float64
	require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=101`).Scan(&balance))
	require.InDelta(t, 98.75, balance, 0.000001)
	path := fmt.Sprintf("/api/v1/requests/%s/usage/%d", id, usageID)
	w := read("101", path)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"input_tokens":7`)
	require.Contains(t, w.Body.String(), `"billed_cost":1.25`)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, 404, read("102", path).Code)
	require.Equal(t, 404, read("101", fmt.Sprintf("/api/v1/requests/%s/usage/%d", id, usageID+1000)).Code)
	require.Equal(t, 200, read("999", strings.Replace(path, "/api/v1/", "/api/v1/admin/", 1)).Code)
	// Existing usage retention must neither cascade the new ledger nor invent a replacement log.
	_, err := db.Exec(`DELETE FROM usage_logs WHERE id=$1`, usageID)
	require.NoError(t, err)
	require.Equal(t, 404, read("101", path).Code)
	w = read("101", "/api/v1/requests/"+id)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"usage_log_id":null`)
	require.Contains(t, w.Body.String(), `"settled":true`)
	require.Contains(t, w.Body.String(), `"billed_cost":null`)
}

func TestRequestLedgerHTTPAPIRolesPaginationAndRefresh(t *testing.T) {
	r, _, _ := ledgerFixtureRouter(t)
	request := func(method, path, user string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("X-Ledger-Test-User", user)
		r.ServeHTTP(w, req)
		return w
	}
	for _, status := range []int{200, 400, 429, 500, 504} {
		require.Equal(t, status, request("POST", fmt.Sprintf("/v1/responses?status=%d", status), "101").Code)
	}
	require.Equal(t, 401, request("POST", "/v1/responses", "").Code)
	require.Equal(t, 200, request("POST", "/v1/responses", "102").Code)
	require.Equal(t, 401, request("GET", "/api/v1/requests", "").Code)
	require.Equal(t, 403, request("GET", "/api/v1/admin/requests", "101").Code)
	parse := func(w *httptest.ResponseRecorder) requestledger.Page {
		require.Equal(t, 200, w.Code)
		var envelope struct {
			Data requestledger.Page `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
		return envelope.Data
	}
	own := parse(request("GET", "/api/v1/requests?user_id=102&page_size=2", "101"))
	require.Equal(t, int64(5), own.Total)
	require.Len(t, own.Items, 2)
	for _, item := range own.Items {
		require.Equal(t, int64(101), *item.UserID)
	}
	next := parse(request("GET", "/api/v1/requests?page=2&page_size=2", "101"))
	require.NotEqual(t, own.Items[0].ID, next.Items[0].ID)
	refreshed := parse(request("GET", "/api/v1/requests?page_size=2", "101"))
	require.Equal(t, own.Items[0].ID, refreshed.Items[0].ID)
	foreign := parse(request("GET", "/api/v1/requests", "102"))
	require.Equal(t, 404, request("GET", "/api/v1/requests/"+foreign.Items[0].ID, "101").Code)
	all := parse(request("GET", "/api/v1/admin/requests", "999"))
	require.Equal(t, int64(7), all.Total)
	scoped := parse(request("GET", "/api/v1/admin/requests?user_id=101&account_id=22", "999"))
	require.Equal(t, int64(1), scoped.Total)
	ownDetail := request("GET", "/api/v1/requests/"+scoped.Items[0].ID, "101")
	require.Equal(t, 200, ownDetail.Code)
	require.NotContains(t, ownDetail.Body.String(), "credential_account_id")
	adminDetail := request("GET", "/api/v1/admin/requests/"+scoped.Items[0].ID, "999")
	require.Equal(t, 200, adminDetail.Code)
	require.Contains(t, adminDetail.Body.String(), "credential_account_id")
	require.Equal(t, 400, request("GET", "/api/v1/requests?page=bad", "101").Code)
	require.Equal(t, 400, request("GET", "/api/v1/requests?private_id=bad", "101").Code)
}

// Explicit opt-in, disposable PG and loopback only. Serves a production frontend
// build against the real ledger API for browser evidence; never production acceptance.
func TestRequestLedgerBrowserFixture(t *testing.T) {
	if os.Getenv("LEDGER_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in browser fixture")
	}
	r, ledger, db := ledgerFixtureRouter(t)
	r.GET("/api/v1/auth/me", func(c *gin.Context) {
		id, _ := strconv.Atoi(c.GetHeader("X-Ledger-Test-User"))
		role := "user"
		if id == 999 {
			role = "admin"
		}
		var balance float64
		require.NoError(t, db.QueryRow(`SELECT balance FROM users WHERE id=$1`, id).Scan(&balance))
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"id": id, "username": "Synthetic reviewer", "email": "fixture@example.invalid", "role": role, "balance": balance, "status": "active"}})
	})
	r.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"site_name": "极速蹬", "custom_menu_items": []gin.H{}}})
	})
	for _, uid := range []string{"101", "102", "999", ""} {
		for _, status := range []string{"200", "400", "429", "500", "504"} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/responses?status="+status, nil)
			req.Header.Set("X-Ledger-Test-User", uid)
			r.ServeHTTP(w, req)
		}
	}
	for _, uid := range []int64{101, 999} {
		for _, state := range []string{"inflight", "interrupted", "cancelled"} {
			h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx := requestledger.WithHandle(context.Background(), h)
			require.NoError(t, requestledger.BindIdentity(ctx, uid, uid+100))
			_, err = requestledger.BeginAttempt(ctx, 234, 22)
			require.NoError(t, err)
			if state != "inflight" {
				require.NoError(t, h.Finish(ctx, state, 0, state))
			}
		}
		seedBilledRequest(t, r, db, uid, true)
	}
	dist := os.Getenv("LEDGER_BROWSER_DIST")
	require.True(t, strings.HasPrefix(dist, "/tmp/ledger-"))
	r.NoRoute(func(c *gin.Context) {
		p := filepath.Join(dist, filepath.Clean("/"+c.Request.URL.Path))
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			c.File(p)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(200, gin.H{"code": 0, "data": gin.H{"items": []gin.H{}, "total": 0}})
			return
		}
		c.File(filepath.Join(dist, "index.html"))
	})
	server := &http.Server{Addr: "127.0.0.1:18762", Handler: r, ReadHeaderTimeout: 5 * time.Second}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.ListenAndServe() }()
	fmt.Println("LEDGER_BROWSER_FIXTURE_READY 127.0.0.1:18762")
	stop := time.NewTimer(20 * time.Minute)
	defer stop.Stop()
	for {
		select {
		case <-stop.C:
			return
		case <-time.After(time.Second):
			if _, err := os.Stat("/tmp/ledger-browser-stop"); err == nil {
				return
			}
		}
	}
}
