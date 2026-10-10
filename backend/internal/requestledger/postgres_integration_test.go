//go:build integration

package requestledger

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func ledgerPostgres(t *testing.T) *sql.DB { return ledgertest.New(t) }

func TestLedgerPostgresPanicPreservesAlreadyWrittenHTTPStatus(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	for _, written := range []bool{false, true} {
		r := gin.New()
		r.Use(gin.Recovery(), Middleware(l))
		var id string
		r.POST("/v1/responses", func(c *gin.Context) {
			id = FromContext(c.Request.Context()).ID
			if written {
				c.String(200, "synthetic partial response")
				c.Writer.Flush()
			}
			panic("synthetic panic")
		})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil))
		var status int
		var state, code string
		require.NoError(t, db.QueryRow(`SELECT http_status,execution_state,error_code FROM gateway_requests WHERE id=$1`, id).Scan(&status, &state, &code))
		require.Equal(t, w.Code, status, "execution failed but an already-written HTTP status cannot change")
		require.Equal(t, "failed", state)
		require.Equal(t, "internal_error", code)
	}
}

func TestLedgerPostgresHTTPRejectionsPersistAndIsolateIdentity(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	for _, status := range []int{200, 400, 401, 429, 500, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			r := gin.New()
			r.Use(Middleware(l))
			var id string
			r.POST("/v1/responses", func(c *gin.Context) {
				id = FromContext(c.Request.Context()).ID
				var persisted int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE id=$1 AND execution_state='inflight'`, id).Scan(&persisted))
				require.Equal(t, 1, persisted, "must commit before authentication")
				if status != 401 {
					require.NoError(t, BindIdentity(c.Request.Context(), 101, 201))
				}
				c.Status(status)
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/responses?api_key=must-not-persist", strings.NewReader("private prompt"))
			req.Header.Set("Authorization", "Bearer must-not-persist")
			req.Header.Set("X-Request-ID", "client-controlled")
			r.ServeHTTP(w, req)
			require.Equal(t, status, w.Code)
			require.NotEqual(t, "client-controlled", id)
			var state, usage, settlement, metadata string
			var user sql.NullInt64
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,settlement_state,user_id,row_to_json(r)::text FROM gateway_requests r WHERE id=$1`, id).Scan(&state, &usage, &settlement, &user, &metadata))
			require.Equal(t, Outcome(status, nil), state)
			require.Equal(t, "not_applicable", usage)
			require.Equal(t, "not_required", settlement)
			require.Equal(t, status != 401, user.Valid)
			for _, secret := range []string{"must-not-persist", "private prompt", "client-controlled", "Authorization"} {
				require.NotContains(t, metadata, secret)
			}
		})
	}
}

func TestLedgerPostgresAttemptOrderRecoveryAndTerminalIdempotence(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	first, err := BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	require.Equal(t, 1, first.Number)
	require.NoError(t, first.Finish(ctx, 429, nil))
	second, err := BeginAttempt(ctx, 235, 23)
	require.NoError(t, err)
	require.Equal(t, 2, second.Number)
	// A new live instance must not interrupt the old live instance's request.
	restarted := New(db)
	require.NoError(t, restarted.Recover(ctx))
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state))
	require.Equal(t, "inflight", state)
	_, err = db.Exec(`UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()-interval '1 second' WHERE id=$1`, l.instance)
	require.NoError(t, err)
	require.NoError(t, restarted.Recover(ctx))
	var usage, settlement string
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,settlement_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state, &usage, &settlement))
	require.Equal(t, "interrupted", state)
	require.Equal(t, "usage_unknown", usage)
	require.Equal(t, "settlement_pending", settlement)
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
	require.NoError(t, restarted.Recover(ctx))
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state))
	require.Equal(t, "interrupted", state, "late completion must not rewrite terminal execution history")
	_, err = BeginAttempt(ctx, 235, 23)
	require.ErrorIs(t, err, ErrUnavailable, "expired owner cannot send more upstream work")
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&count))
	require.Equal(t, 2, count)
}

func TestLedgerPostgresQueriesEnforceOwnerAndPagination(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	var ids []string
	for i := 0; i < 5; i++ {
		h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
		require.NoError(t, err)
		user := int64(101)
		if i == 4 {
			user = 102
		}
		require.NoError(t, BindIdentity(WithHandle(ctx, h), user, user+100))
		ids = append(ids, h.ID)
	}
	page, err := l.List(ctx, Viewer{UserID: 101}, Filter{Page: 1, PageSize: 2, UserID: 102})
	require.NoError(t, err)
	require.Equal(t, int64(4), page.Total, "query parameter must not replace authenticated owner")
	require.Len(t, page.Items, 2)
	for _, r := range page.Items {
		require.Equal(t, int64(101), *r.UserID)
	}
	second, err := l.List(ctx, Viewer{UserID: 101}, Filter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, second.Items, 2)
	for _, r := range second.Items {
		for _, first := range page.Items {
			require.NotEqual(t, first.ID, r.ID)
		}
	}
	_, err = l.Get(ctx, Viewer{UserID: 101}, ids[4])
	require.ErrorIs(t, err, sql.ErrNoRows)
	admin, err := l.List(ctx, Viewer{Admin: true}, Filter{Page: 1, PageSize: 20, UserID: 102})
	require.NoError(t, err)
	require.Equal(t, int64(1), admin.Total)
	_, err = l.List(ctx, Viewer{}, Filter{})
	require.Error(t, err)
}

func TestLedgerPostgresWSTurnsHaveIndependentDurableIdentities(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	session, err := l.Begin(ctx, "/v1/responses", "GET", "ws_session")
	require.NoError(t, err)
	ctx = WithHandle(ctx, session)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	first, err := AcceptTurn(ctx)
	require.NoError(t, err)
	firstCtx := CurrentContext(ctx)
	require.Equal(t, first.ID, FromContext(firstCtx).ID)
	firstAttempt, err := BeginAttempt(firstCtx, 234, 22)
	require.NoError(t, err)
	require.NoError(t, FinishTurn(firstCtx, "succeeded", nil))
	second, err := AcceptTurn(ctx)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.NotEqual(t, session.ID, second.ID)
	_, err = BeginAttempt(CurrentContext(ctx), 234, 22)
	require.NoError(t, err)
	require.NoError(t, firstAttempt.ObserveUsage(firstCtx))
	require.NoError(t, FinishTurn(CurrentContext(ctx), "cancelled", context.Canceled))
	var firstUsage, secondUsage string
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE id=$1`, first.ID).Scan(&firstUsage))
	require.NoError(t, db.QueryRow(`SELECT usage_state FROM gateway_requests WHERE id=$1`, second.ID).Scan(&secondUsage))
	require.Equal(t, "known", firstUsage, "delayed usage must retain its original turn identity")
	require.Equal(t, "usage_unknown", secondUsage)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(DISTINCT turn_no) FROM gateway_requests WHERE parent_id=$1`, session.ID).Scan(&count))
	require.Equal(t, 2, count)
}

func TestLedgerPostgresExpiredLeaseAdmitsNewGenerationOnly(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	old, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	oldCtx := WithHandle(ctx, old)
	require.NoError(t, BindIdentity(oldCtx, 101, 201))
	_, err = db.Exec(`UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	fresh, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err, "storage recovery must admit a new lease generation without reviving stale requests")
	freshCtx := WithHandle(ctx, fresh)
	require.NoError(t, BindIdentity(freshCtx, 101, 201))
	_, err = BeginAttempt(freshCtx, 234, 22)
	require.NoError(t, err)
	_, err = BeginAttempt(oldCtx, 234, 22)
	require.ErrorIs(t, err, ErrUnavailable)
	require.NoError(t, l.Recover(ctx))
	var state string
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, old.ID).Scan(&state))
	require.Equal(t, "interrupted", state)
}

func TestLedgerPostgresTerminalClosesUnfinishedAttempts(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	_, err = BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
	var state, attempt, usage string
	require.NoError(t, db.QueryRow(`SELECT r.execution_state,a.execution_state,r.usage_state FROM gateway_requests r JOIN gateway_request_attempts a ON a.request_id=r.id WHERE r.id=$1`, h.ID).Scan(&state, &attempt, &usage))
	require.Equal(t, "interrupted", state, "HTTP 200 alone does not prove completion of a still open upstream attempt")
	require.Equal(t, "interrupted", attempt)
	require.Equal(t, "usage_unknown", usage)
	require.NoError(t, h.Finish(ctx, "failed", 500, "internal_error"))
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state))
	require.Equal(t, "interrupted", state)
}

func TestLedgerPostgresAttemptStorageFailureRejectsOnlyRequest(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer upstream.Close()
	r := gin.New()
	r.Use(Middleware(l))
	r.POST("/v1/responses", func(c *gin.Context) {
		require.NoError(t, BindIdentity(c.Request.Context(), 101, 201))
		_, err := db.Exec(`ALTER TABLE gateway_request_attempts RENAME TO ledger_attempts_unavailable`)
		require.NoError(t, err)
		req, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstream.URL, nil)
		_, err = (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
		require.ErrorIs(t, err, ErrUnavailable)
		_, restoreErr := db.Exec(`ALTER TABLE ledger_attempts_unavailable RENAME TO gateway_request_attempts`)
		require.NoError(t, restoreErr)
		// Even a legacy handler's generic mapping must preserve the ingress 503 contract.
		c.JSON(502, gin.H{"error": "legacy transport mapping"})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/v1/responses", nil))
	require.Equal(t, 503, w.Code)
	require.Contains(t, w.Body.String(), "request_ledger_unavailable")
	require.Equal(t, 0, calls)
	var state, code string
	require.NoError(t, db.QueryRow(`SELECT execution_state,error_code FROM gateway_requests`).Scan(&state, &code))
	require.Equal(t, "failed", state)
	require.Equal(t, "request_ledger_unavailable", code)
	healthy := httptest.NewRecorder()
	r.GET("/health", func(c *gin.Context) { c.Status(200) })
	r.ServeHTTP(healthy, httptest.NewRequest("GET", "/health", nil))
	require.Equal(t, 200, healthy.Code)
}

func TestLedgerPostgresTaskAttributionSurvivesRestartWithoutBackfill(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/images/generations/async", "POST", "http")
	require.NoError(t, err)
	submit := WithHandle(ctx, h)
	require.NoError(t, BindIdentity(submit, 101, 201))
	require.NoError(t, BindTask(submit, "image_task", "synthetic-task", 101, 201))
	require.NoError(t, h.Finish(submit, "succeeded", 202, ""))
	restarted := New(db)
	child, err := restarted.BeginTask(ctx, "image_task", "synthetic-task", 101, 201, true)
	require.NoError(t, err)
	require.Equal(t, h.ID, child.ParentID)
	require.NotEqual(t, h.ID, child.ID)
	_, err = BeginAttempt(WithHandle(ctx, child), 234, 22)
	require.NoError(t, err)
	_, err = restarted.BeginTask(ctx, "image_task", "synthetic-task", 102, 202, true)
	require.ErrorIs(t, err, ErrUnavailable, "persisted attribution must not be reassigned")
	legacy, err := restarted.BeginTask(ctx, "image_task", "legacy-task", 101, 201, true)
	require.NoError(t, err)
	require.Empty(t, legacy.ParentID, "current execution evidence must not invent missing historic submission")
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE kind='http'`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestLedgerPostgresHTTPTransportPartialOutputAndCancellation(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	for _, mode := range []string{"success", "truncated", "cancelled", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var count int
				require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE execution_state='inflight'`).Scan(&count))
				require.Greater(t, count, 0, "attempt must commit before upstream receives request")
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"synthetic\"}\n\n"))
				w.(http.Flusher).Flush()
				if mode == "success" {
					_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
				}
				if mode == "cancelled" || mode == "timeout" {
					<-r.Context().Done()
				}
			}))
			defer upstream.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx = WithHandle(ctx, h)
			require.NoError(t, BindIdentity(ctx, 101, 201))
			req, _ := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
			resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
			require.NoError(t, err)
			if mode == "cancelled" {
				buffer := make([]byte, 8)
				_, _ = resp.Body.Read(buffer)
				cancel()
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			require.NoError(t, h.Finish(ctx, Outcome(200, ctx.Err()), 200, ErrorCode(ctx.Err())))
			var state, usage string
			var observed bool
			require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,output_observed FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state, &usage, &observed))
			want := map[string]string{"success": "succeeded", "truncated": "interrupted", "cancelled": "cancelled", "timeout": "timeout"}[mode]
			require.Equal(t, want, state)
			require.Equal(t, "usage_unknown", usage)
			require.True(t, observed)
		})
	}
}

func TestLedgerPostgresUntrustedMethodNeverPersistsRawMetadata(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	r := gin.New()
	r.Use(Middleware(l))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PRIVATE-VERB-WITH-SECRET-SHOULD-NOT-PERSIST", "/v1/unknown", nil))
	require.Equal(t, 404, w.Code)
	var method string
	require.NoError(t, db.QueryRow(`SELECT method FROM gateway_requests`).Scan(&method))
	require.Equal(t, "OTHER", method)
}

func TestLedgerPostgresSSEFailureCannotBeOverwrittenBySuccess(t *testing.T) {
	evidence := streamEvidence{}
	evidence.observe([]byte("event: error\ndata: {\"type\":\"error\"}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	require.Equal(t, "failed", evidence.terminal)
}

func TestLedgerPostgresResumeTaskAfterRestartKeepsExactPrivateIdentity(t *testing.T) {
	db := ledgerPostgres(t)
	ctx := context.Background()
	l := New(db)
	h, err := l.Begin(ctx, "/v1/live", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindTask(ctx, "live_call", "fixture-hash", 101, 201))
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
	resumed, err := New(db).ResumeTask(context.Background(), "live_call", "fixture-hash", 101, 201)
	require.NoError(t, err)
	require.Equal(t, h.ID, FromContext(resumed).ID)
	_, err = New(db).ResumeTask(context.Background(), "live_call", "fixture-hash", 102, 201)
	require.Error(t, err)
	missing, err := New(db).ResumeTask(context.Background(), "live_call", "missing-history", 101, 201)
	require.NoError(t, err)
	require.Nil(t, FromContext(missing))
}

func TestLedgerPostgresExternalSearchHasNoInventedAccount(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/api/v1/mobile/web-search", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 0))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var kind string
		var account, credential sql.NullInt64
		require.NoError(t, db.QueryRow(`SELECT upstream_kind,account_id,credential_account_id FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&kind, &account, &credential))
		require.Equal(t, "external_search", kind)
		require.False(t, account.Valid)
		require.False(t, credential.Valid)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", server.URL, nil)
	require.NoError(t, err)
	client := &http.Client{Transport: ExternalSearchTransport(http.DefaultTransport)}
	resp, err := client.Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
}

func TestLedgerPostgresRetryUsageDoesNotHideUnknownEarlierAttempt(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	first, err := BeginAttempt(ctx, 234, 22)
	require.NoError(t, err)
	ObserveOutput(ctx)
	require.NoError(t, first.Finish(ctx, 500, nil))
	second, err := BeginAttempt(ctx, 235, 23)
	require.NoError(t, err)
	require.NoError(t, second.Finish(ctx, 200, nil))
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
	require.NoError(t, ObserveUsage(ctx))
	record, err := l.Get(ctx, Viewer{UserID: 101}, h.ID)
	require.NoError(t, err)
	require.Equal(t, "usage_unknown", record.UsageState, "known usage from retry must not hide unknown consumption on earlier account")
	var output bool
	var usage string
	require.NoError(t, db.QueryRow(`SELECT output_observed,usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&output, &usage))
	require.True(t, output)
	require.Equal(t, "usage_unknown", usage)
}

func TestLedgerPostgresAdditionalBillingIntentReopensPending(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	require.NoError(t, PrepareBilling(ctx, BillingIntent{RequestID: "first", APIKeyID: 201, UserID: 101, Fingerprint: "first-proof"}))
	_, err = db.Exec(`UPDATE gateway_request_billing_links SET settlement_verified=TRUE WHERE request_id=$1`, h.ID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE gateway_requests SET settlement_state='settled' WHERE id=$1`, h.ID)
	require.NoError(t, err)
	require.NoError(t, PrepareBilling(ctx, BillingIntent{RequestID: "second", APIKeyID: 201, UserID: 101, Fingerprint: "second-proof"}))
	rec, err := l.Get(ctx, Viewer{UserID: 101}, h.ID)
	require.NoError(t, err)
	require.Equal(t, "settlement_pending", rec.SettlementState)
}
