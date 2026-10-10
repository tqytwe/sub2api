//go:build integration

package requestledger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresExactAttemptEvidence(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	var call int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call++
		var id string
		require.NoError(t, db.QueryRow(`SELECT attempt_id FROM gateway_request_attempt_evidence WHERE request_id=$1 AND attempt_no=$2`, h.ID, call).Scan(&id), "identity must exist before upstream send")
		_, err := uuid.Parse(id)
		require.NoError(t, err)
		w.Header().Set("X-Request-Id", fmt.Sprintf("req_%d", call))
		w.Header().Set("Content-Type", "text/event-stream")
		terminal := "response.failed"
		if call == 2 {
			terminal = "response.completed"
		}
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"%s\",\"response\":{\"id\":\"resp_%d\"}}\n\n", terminal, call)
	}))
	defer upstream.Close()
	client := &http.Client{Transport: Transport(http.DefaultTransport, 234)}
	for i := 1; i <= 2; i++ {
		req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, strings.NewReader("private prompt"))
		require.NoError(t, err)
		req.Header.Set("X-Request-ID", "forged-client-correlation")
		resp, err := client.Do(req)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		waitAttemptIdentifiers(t, ResponseReceipt(resp).attempt)
	}
	var previous string
	for i, state := range []string{"failed", "succeeded"} {
		var id, requestID, responseID, got, usage, evidence string
		require.NoError(t, db.QueryRow(`SELECT e.attempt_id,e.upstream_request_id,e.upstream_response_id,a.execution_state,a.usage_state,row_to_json(e)::text FROM gateway_request_attempt_evidence e JOIN gateway_request_attempts a USING(request_id,attempt_no) WHERE e.request_id=$1 AND e.attempt_no=$2`, h.ID, i+1).Scan(&id, &requestID, &responseID, &got, &usage, &evidence))
		require.NotEqual(t, previous, id)
		previous = id
		require.Equal(t, fmt.Sprintf("req_%d", i+1), requestID)
		require.Equal(t, fmt.Sprintf("resp_%d", i+1), responseID)
		require.Equal(t, state, got)
		require.Equal(t, "usage_unknown", usage)
		require.NotContains(t, evidence, "private prompt")
		require.NotContains(t, evidence, "forged-client-correlation")
	}
}

func TestLedgerPostgresImmutableReceiptsAndLinks(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req_"+r.URL.Path[1:])
		_, _ = fmt.Fprintf(w, `{"object":"response","id":"resp_%s","status":"failed","error":{"code":"synthetic"}}`, r.URL.Path[1:])
	}))
	defer upstream.Close()
	client := &http.Client{Transport: Transport(http.DefaultTransport, 234)}
	send := func(method, path string) *http.Response {
		req, e := http.NewRequestWithContext(ctx, method, upstream.URL+path, nil)
		require.NoError(t, e)
		resp, e := client.Do(req)
		require.NoError(t, e)
		_, e = io.Copy(io.Discard, resp.Body)
		require.NoError(t, e)
		require.NoError(t, resp.Body.Close())
		return resp
	}
	first := send("POST", "/first")
	firstReceipt := ResponseReceipt(first)
	second := send("POST", "/second")
	aux := send("GET", "/aux")
	require.Equal(t, 1, firstReceipt.Correlation().AttemptNo)
	require.Equal(t, 2, ResponseReceipt(second).Correlation().AttemptNo)
	require.Equal(t, 3, ResponseReceipt(aux).Correlation().AttemptNo)
	require.Equal(t, 2, CurrentAttempt(ctx).Number, "auxiliary send must not steal billing attribution")
	require.Equal(t, firstReceipt.Correlation(), ResponseReceipt(first).Correlation(), "restored old response keeps its identity")
	require.Nil(t, ResponseReceipt(&http.Response{Header: http.Header{"X-Request-Id": {h.ID}}}).Correlation())
	firstReceipt.ObserveIdentifiers(ctx, "req_conflicting", "resp_conflicting")
	waitAttemptIdentifiers(t, firstReceipt.attempt)
	var state, requestID, responseID string
	var conflict bool
	require.NoError(t, db.QueryRow(`SELECT a.execution_state,e.upstream_request_id,e.upstream_response_id,e.identifier_conflict FROM gateway_request_attempts a JOIN gateway_request_attempt_evidence e USING(request_id,attempt_no) WHERE a.request_id=$1 AND a.attempt_no=1`, h.ID).Scan(&state, &requestID, &responseID, &conflict))
	require.Equal(t, "failed", state, "HTTP200 cannot override failure envelope")
	require.Equal(t, "req_first", requestID)
	require.Equal(t, "resp_first", responseID)
	require.True(t, conflict)
	_, err = db.Exec(`CREATE TABLE ops_error_logs(id bigint PRIMARY KEY,user_id bigint,api_key_id bigint); INSERT INTO ops_error_logs VALUES(11,101,201),(12,102,201),(13,101,202)`)
	require.NoError(t, err)
	receipts := []Receipt{firstReceipt, ResponseReceipt(second), ResponseReceipt(aux), firstReceipt}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); LinkOps(ctx, db, 11, receipts) }()
	}
	wg.Wait()
	LinkOps(ctx, db, 12, receipts)
	LinkOps(ctx, db, 13, receipts)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_ops_links`).Scan(&count))
	require.Equal(t, 3, count)
	refs, err := l.Diagnostics(ctx, Viewer{Admin: true}, h.ID)
	require.NoError(t, err)
	require.Len(t, refs, 3)
	userRefs, err := l.Diagnostics(ctx, Viewer{UserID: 101}, h.ID)
	require.NoError(t, err)
	require.Empty(t, userRefs)
	userAttempts, err := l.Attempts(ctx, Viewer{UserID: 101}, h.ID)
	require.NoError(t, err)
	encoded, err := json.Marshal(userAttempts)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "upstream_request_id")
	require.NotContains(t, string(encoded), "upstream_response_id")
	require.NotContains(t, string(encoded), "account_id")
	_, err = db.Exec(`DELETE FROM ops_error_logs WHERE id=11`)
	require.NoError(t, err)
	refs, err = l.Diagnostics(ctx, Viewer{Admin: true}, h.ID)
	require.NoError(t, err)
	require.Len(t, refs, 3)
	require.False(t, refs[0].Available)
	_, err = db.Exec(`ALTER TABLE gateway_request_ops_links RENAME TO fixture_links_unavailable`)
	require.NoError(t, err)
	LinkOps(ctx, db, 11, receipts) // diagnostic failure must never escape to forwarding/settlement
	upstream.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
	require.Equal(t, 4, ErrorReceipt(err).Correlation().AttemptNo)
}

func TestLedgerPostgresDoTimeoutAndIsolation(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(200)
			return
		}
		<-r.Context().Done()
	}))
	defer upstream.Close()
	contexts := make([]context.Context, 2)
	for i := range contexts {
		h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
		require.NoError(t, err)
		contexts[i] = WithHandle(context.Background(), h)
		require.NoError(t, BindIdentity(contexts[i], 101, 201))
	}
	client := &http.Client{Transport: Transport(http.DefaultTransport, 234), Timeout: 500 * time.Millisecond}
	var wg sync.WaitGroup
	for _, ctx := range contexts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL+"/wait", nil)
			require.NoError(t, err)
			resp, err := Do(req, client.Do)
			require.Nil(t, resp)
			require.Error(t, err)
			require.NotNil(t, ErrorReceipt(err).Correlation())
			require.Equal(t, FromContext(ctx).ID, ErrorReceipt(err).Correlation().RequestID)
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout())
			require.ErrorIs(t, err, context.DeadlineExceeded)
		}()
	}
	wg.Wait()
	// A reused client and a subsequent successful attempt cannot inherit failure.
	ctx := contexts[0]
	req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL+"/ok", nil)
	require.NoError(t, err)
	sentinel := errors.New("outer policy failure after successful retry")
	_, err = Do(req, func(r *http.Request) (*http.Response, error) {
		bad := r.Clone(r.Context())
		bad.URL.Path = "/wait"
		_, firstErr := client.Do(bad)
		require.Error(t, firstErr)
		resp, sendErr := client.Do(r)
		require.NoError(t, sendErr)
		require.NoError(t, resp.Body.Close())
		return nil, sentinel
	})
	require.ErrorIs(t, err, sentinel)
	require.Nil(t, ErrorReceipt(err).Correlation(), "a success clears the preceding transport receipt")
	// The production response-header timeout retains its original net.Error too.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 50 * time.Millisecond
	defer transport.CloseIdleConnections()
	headerClient := &http.Client{Transport: Transport(transport, 234)}
	req.URL.Path = "/wait"
	_, err = Do(req, headerClient.Do)
	require.Error(t, err)
	require.NotNil(t, ErrorReceipt(err).Correlation())
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.Timeout())
}

func waitAttemptIdentifiers(t *testing.T, a *Attempt) {
	t.Helper()
	require.Eventually(t, func() bool { a.evidenceMu.Lock(); defer a.evidenceMu.Unlock(); return !a.evidencePending }, 5*time.Second, 10*time.Millisecond)
	a.evidenceMu.Lock()
	defer a.evidenceMu.Unlock()
	require.False(t, a.evidenceUnavailable)
}

func TestLedgerPostgresIdentifierQueueIsBounded(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	attempts := make([]*Attempt, 300)
	for i := range attempts {
		attempts[i], err = BeginAttempt(ctx, 234, 234)
		require.NoError(t, err)
	}
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer blocker.Rollback()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_attempt_evidence IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	started := time.Now()
	for _, a := range attempts {
		a.ObserveIdentifiers(ctx, "req_bounded", "resp_bounded")
	}
	require.Less(t, time.Since(started), 200*time.Millisecond)
	require.LessOrEqual(t, len(identifierWrites.queue), 256)
	dropped := 0
	for _, a := range attempts {
		a.evidenceMu.Lock()
		if a.evidenceUnavailable {
			dropped++
		}
		a.evidenceMu.Unlock()
	}
	require.Positive(t, dropped, "overflow is an explicit evidence gap, never a blocking producer")
	require.NoError(t, blocker.Rollback())
	require.Eventually(t, func() bool {
		for _, a := range attempts {
			a.evidenceMu.Lock()
			pending := a.evidencePending
			a.evidenceMu.Unlock()
			if pending {
				return false
			}
		}
		return true
	}, 5*time.Second, 10*time.Millisecond)
}
