//go:build integration

package requestledger

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

// The child is this test binary; it never loads application configuration.
func TestLedgerKilledProcessChild(t *testing.T) {
	if os.Getenv("LEDGER_KILL_CHILD") != "1" {
		t.Skip("subprocess fixture only")
	}
	db, err := sql.Open("postgres", os.Getenv("LEDGER_LOCAL_DSN"))
	require.NoError(t, err)
	defer db.Close()
	l := New(db)
	l.Start()
	defer l.Stop()
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx = WithHandle(ctx, h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	req, err := http.NewRequestWithContext(ctx, "POST", os.Getenv("LEDGER_LOCAL_UPSTREAM"), nil)
	require.NoError(t, err)
	client := &http.Client{Transport: Transport(http.DefaultTransport, 234)}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	t.Fatal("upstream should remain open until this process is killed")
}

func TestLedgerKilledProcessRecoveryRetainsEvidence(t *testing.T) {
	db, dsn := ledgertest.NewWithDSN(t)
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var count int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE execution_state='inflight'`).Scan(&count))
		require.Equal(t, 1, count, "attempt must be committed before upstream accepts request")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"fixture\"}\n\n"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer upstream.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestLedgerKilledProcessChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "LEDGER_KILL_CHILD=1", "LEDGER_LOCAL_DSN="+dsn, "LEDGER_LOCAL_UPSTREAM="+upstream.URL)
	require.NoError(t, child.Start())
	t.Cleanup(func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	select {
	case <-started:
	case <-time.After(20 * time.Second):
		t.Fatal("child never reached fake upstream")
	}
	require.Eventually(t, func() bool {
		var observed bool
		err := db.QueryRow(`SELECT output_observed FROM gateway_requests`).Scan(&observed)
		return err == nil && observed
	}, 5*time.Second, 20*time.Millisecond)
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	var id, state, usage, settlement string
	require.NoError(t, db.QueryRow(`SELECT id,execution_state,usage_state,settlement_state FROM gateway_requests`).Scan(&id, &state, &usage, &settlement))
	require.Equal(t, "inflight", state)
	restarted := New(db)
	require.NoError(t, restarted.Recover(context.Background()))
	require.NoError(t, db.QueryRow(`SELECT execution_state FROM gateway_requests WHERE id=$1`, id).Scan(&state))
	require.Equal(t, "inflight", state, "live lease is respected")
	// Advance only this isolated fixture's lease; production uses DB clock + 90s.
	_, err := db.Exec(`UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	for range 2 {
		require.NoError(t, restarted.Recover(context.Background()))
	}
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,settlement_state FROM gateway_requests WHERE id=$1`, id).Scan(&state, &usage, &settlement))
	require.Equal(t, "interrupted", state)
	require.Equal(t, "usage_unknown", usage)
	require.Equal(t, "settlement_pending", settlement)
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts WHERE execution_state='interrupted' AND ended_at IS NOT NULL`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&count))
	require.Equal(t, 1, count, "recovery neither deletes evidence nor resends requests")
}
