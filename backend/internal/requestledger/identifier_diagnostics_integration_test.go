//go:build integration

package requestledger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresIdentifierSnapshotReceiptIsolation(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	var sends atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		name := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("X-Request-ID", "req_"+name)
		w.Header().Set("Request-ID", "req_not_selected")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_%s\"},\"input\":\"body-secret-sentinel\"}\n\n", name)
		if name == "first" {
			_, _ = io.WriteString(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_second\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_third\"}}\n\n")
		} else {
			_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_%s\"}}\n\n", name)
		}
	}))
	defer upstream.Close()
	client := &http.Client{Transport: Transport(http.DefaultTransport, 234)}
	send := func(method, name string) *http.Response {
		req, e := http.NewRequestWithContext(ctx, method, upstream.URL+"/"+name, nil)
		require.NoError(t, e)
		resp, e := client.Do(req)
		require.NoError(t, e)
		_, e = io.Copy(io.Discard, resp.Body)
		require.NoError(t, e)
		require.NoError(t, resp.Body.Close())
		waitAttemptIdentifiers(t, ResponseAttempt(resp))
		return resp
	}
	first := send("POST", "first")
	second := send("POST", "second")
	_ = send("GET", "auxiliary")
	require.Equal(t, 2, CurrentAttempt(ctx).Number)
	oldReceipt := ResponseReceipt(first)
	oldReceipt.ObserveOpsIdentifiers(ctx, "req_late", "resp_fourth", "failover")
	waitAttemptIdentifiers(t, oldReceipt.attempt)
	page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.Len(t, page.Attempts, 3)
	require.Equal(t, "unknown", page.Completeness)
	a := page.Attempts[0]
	require.Equal(t, oldReceipt.Correlation().AttemptID, *a.AttemptID)
	require.True(t, *a.IdentifierConflict)
	require.Equal(t, "persisted_snapshot", a.Status)
	require.Equal(t, "req_first", a.Request.First.Value)
	require.Equal(t, "x-request-id", a.Request.First.Field)
	require.Equal(t, "http_header", a.Request.First.Source)
	require.Equal(t, "req_late", a.Request.FirstDifferent.Value)
	require.Equal(t, "ops.failover", a.Request.FirstDifferent.EventType)
	require.Equal(t, "resp_first", a.Response.First.Value)
	require.Equal(t, "response.created", a.Response.First.EventType)
	require.Equal(t, "resp_second", a.Response.FirstDifferent.Value)
	require.Equal(t, "response.failed", a.Response.FirstDifferent.EventType)
	require.False(t, *page.Attempts[1].IdentifierConflict, "different retry IDs belong to a distinct attempt")
	require.Equal(t, ResponseReceipt(second).Correlation().AttemptID, *page.Attempts[1].AttemptID)
	require.EqualValues(t, 3, sends.Load())
	var state, usage string
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_request_attempts WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&state, &usage))
	require.Equal(t, "failed", state, "diagnostics cannot overwrite terminal behavior")
	require.Equal(t, "usage_unknown", usage)
	var stored string
	require.NoError(t, db.QueryRow(`SELECT identifier_diagnostics::text FROM gateway_request_attempt_evidence WHERE request_id=$1 AND attempt_no=1`, h.ID).Scan(&stored))
	require.NotContains(t, stored, "body-secret-sentinel")
	require.NotContains(t, stored, "req_not_selected")
	_, err = ledger.IdentifierEvidence(ctx, Viewer{UserID: 101}, h.ID, 0)
	require.Error(t, err, "admin-only evidence is not exposed to its ordinary owner")
}

func TestLedgerPostgresIdentifierSnapshotBothConflictsAndLoss(t *testing.T) {
	for _, failure := range []string{"none", "row_lock", "pool"} {
		t.Run(failure, func(t *testing.T) {
			db := ledgerPostgres(t)
			ledger := New(db)
			h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx := WithHandle(context.Background(), h)
			require.NoError(t, BindIdentity(ctx, 101, 201))
			a, err := BeginAttempt(ctx, 234, 234)
			require.NoError(t, err)
			a.Receipt().ObserveOpsIdentifiers(ctx, "req_first", "resp_first", "http_error")
			waitAttemptIdentifiers(t, a)
			if failure == "none" {
				a.Receipt().ObserveOpsIdentifiers(ctx, "req_second", "resp_first", "failover")
				waitAttemptIdentifiers(t, a)
				a.Receipt().ObserveOpsIdentifiers(ctx, "req_third", "resp_second", "response_error")
				waitAttemptIdentifiers(t, a)
				page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
				require.NoError(t, err)
				require.Equal(t, "req_second", page.Attempts[0].Request.FirstDifferent.Value)
				require.Equal(t, "resp_second", page.Attempts[0].Response.FirstDifferent.Value)
				return
			}
			var release func()
			if failure == "row_lock" {
				tx, err := db.Begin()
				require.NoError(t, err)
				defer func() { _ = tx.Rollback() }()
				_, err = tx.Exec(`SELECT 1 FROM gateway_request_attempt_evidence WHERE request_id=$1 FOR UPDATE`, h.ID)
				require.NoError(t, err)
				release = func() { require.NoError(t, tx.Rollback()) }
			} else {
				db.SetMaxOpenConns(2)
				conn, err := db.Conn(ctx)
				require.NoError(t, err)
				defer func() { _ = conn.Close() }()
				release = func() { require.NoError(t, conn.Close()) }
			}
			started := time.Now()
			a.Receipt().ObserveOpsIdentifiers(ctx, "req_lost", "resp_first", "failover")
			require.Less(t, time.Since(started), 200*time.Millisecond)
			require.Eventually(t, func() bool {
				a.evidenceMu.Lock()
				defer a.evidenceMu.Unlock()
				return a.evidenceUnavailable && !a.evidencePending
			}, 3*time.Second, 10*time.Millisecond)
			release()
			page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
			require.NoError(t, err)
			require.False(t, *page.Attempts[0].IdentifierConflict, "only the older persisted flag survived the dropped update")
			require.Nil(t, page.Attempts[0].Request.FirstDifferent)
			require.Equal(t, "unknown", page.Completeness, "saved first values cannot certify absence of later conflicts")
			require.Equal(t, "persisted_snapshot", page.Attempts[0].Status)
		})
	}
}

func TestLedgerPostgresIdentifierSnapshotLegacyAndQueryFailure(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	a, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET upstream_request_id='req_old',identifier_conflict=TRUE WHERE request_id=$1`, h.ID)
	require.NoError(t, err)
	page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.True(t, *page.Attempts[0].IdentifierConflict)
	require.Equal(t, "not_recorded", page.Attempts[0].Status)
	require.Nil(t, page.Attempts[0].Request.FirstDifferent)
	a.Receipt().ObserveOpsIdentifiers(ctx, "req_new", "resp_new", "failover")
	waitAttemptIdentifiers(t, a)
	page, err = ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.Equal(t, "incomplete", page.Attempts[0].Status, "new provenance cannot fabricate an old first source")
	require.True(t, *page.Attempts[0].IdentifierConflict)
	_, err = db.Exec(`ALTER TABLE gateway_request_attempt_evidence RENAME COLUMN identifier_diagnostics TO fixture_missing_diagnostics`)
	require.NoError(t, err)
	_, err = ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.Error(t, err, "an unavailable query must not become a successful empty list")
	_, err = ledger.Get(ctx, Viewer{Admin: true}, h.ID)
	require.NoError(t, err, "original request metadata stays readable")
}

func TestLedgerPostgresIdentifierSnapshotAfterTerminalAndJSON(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	var sends atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		w.Header().Set("X-Request-ID", "req_transport")
		if r.URL.Path == "/json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"response","id":"resp_json","response_id":"resp_preferred","input":"body-secret-sentinel"}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\"}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_after_terminal\"}}\n\n")
	}))
	defer upstream.Close()
	for _, path := range []string{"/stream", "/json"} {
		t.Run(path, func(t *testing.T) {
			h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
			require.NoError(t, err)
			ctx := WithHandle(context.Background(), h)
			require.NoError(t, BindIdentity(ctx, 101, 201))
			req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL+path, nil)
			require.NoError(t, err)
			resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
			require.NoError(t, err)
			_, err = io.Copy(io.Discard, resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			waitAttemptIdentifiers(t, ResponseAttempt(resp))
			page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
			require.NoError(t, err)
			item := page.Attempts[0]
			require.Equal(t, "persisted_snapshot", item.Status)
			if path == "/stream" {
				require.True(t, *item.IdentifierConflict)
				require.Equal(t, "resp_after_terminal", item.Response.FirstDifferent.Value)
				require.Equal(t, "response.completed", item.Response.FirstDifferent.EventType)
				var state string
				require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&state))
				require.Equal(t, "failed", state, "later ID evidence cannot turn a failed stream into success")
			} else {
				require.False(t, *item.IdentifierConflict)
				require.Equal(t, "resp_preferred", item.Response.First.Value)
				require.Equal(t, "response_id", item.Response.First.Field)
				require.Equal(t, "http_json", item.Response.First.Source)
			}
		})
	}
	require.EqualValues(t, 2, sends.Load())
}
