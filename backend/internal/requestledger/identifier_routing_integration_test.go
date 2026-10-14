//go:build integration

package requestledger

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A routing end summary and a later exact-receipt identifier update have
// different completeness guarantees, even when both belong to one request.
func TestLedgerPostgresIdentifierSnapshotWithRoutingFinish(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	defer l.Stop()
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	selected, end := BeginRoutingSelection(ctx, true)
	RoutingPool(selected, 2, false)
	RoutingVeto(selected, "initial_filter", 33, "model_not_supported")
	end(234, false, false)
	var sends atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sends.Add(1)
		w.Header().Set("X-Request-ID", "req_combined")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_combined\"}}\n\n")
	}))
	defer upstream.Close()
	req, err := http.NewRequestWithContext(ctx, "POST", upstream.URL, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Transport: Transport(http.DefaultTransport, 234)}).Do(req)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	waitAttemptIdentifiers(t, ResponseAttempt(resp))
	require.NoError(t, h.Finish(ctx, "succeeded", 200, ""))
	viewer := Viewer{Admin: true, UserID: 101}
	var routing *RoutingEvidence
	require.Eventually(t, func() bool {
		routing, err = l.RoutingEvidence(ctx, viewer, h.ID)
		return err == nil && routing.Status == "complete"
	}, 3*time.Second, 10*time.Millisecond)
	require.Equal(t, 1, routing.SelectionCount)
	require.False(t, routing.SourcePopulationComplete)
	foundVeto := false
	for _, event := range routing.Events {
		if event.Reason == "model_not_supported" && event.Stage == "initial_filter" && event.AccountID == 33 {
			require.Equal(t, 1, event.SelectionNo)
			foundVeto = true
		}
	}
	require.True(t, foundVeto, "failed candidate is retained before the successful HTTP attempt")
	ResponseReceipt(resp).ObserveOpsIdentifiers(ctx, "req_late_combined", "resp_combined", "failover")
	waitAttemptIdentifiers(t, ResponseAttempt(resp))
	identifiers, err := l.IdentifierEvidence(ctx, viewer, h.ID, 0)
	require.NoError(t, err)
	require.Equal(t, "unknown", identifiers.Completeness)
	require.Len(t, identifiers.Attempts, 1)
	item := identifiers.Attempts[0]
	require.Equal(t, "persisted_snapshot", item.Status)
	require.Equal(t, ResponseReceipt(resp).Correlation().AttemptID, *item.AttemptID)
	require.True(t, *item.IdentifierConflict)
	require.Equal(t, "req_combined", item.Request.First.Value)
	require.Equal(t, "req_late_combined", item.Request.FirstDifferent.Value)
	require.Nil(t, item.Response.FirstDifferent)
	after, err := l.RoutingEvidence(ctx, viewer, h.ID)
	require.NoError(t, err)
	require.Equal(t, routing, after, "late identifier evidence cannot change routing completion")
	var state, usage string
	var count int
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state FROM gateway_requests WHERE id=$1`, h.ID).Scan(&state, &usage))
	require.Equal(t, "succeeded", state)
	require.Equal(t, "usage_unknown", usage)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE request_id=$1`, h.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.EqualValues(t, 1, sends.Load())
}
