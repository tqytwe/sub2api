//go:build integration

package requestledger

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresWSEndCrashChild(t *testing.T) {
	if os.Getenv("WS_END_SNAPSHOT_CHILD") != "1" {
		t.Skip("subprocess fixture only")
	}
	db, err := sql.Open("postgres", os.Getenv("WS_END_FIXTURE_DSN"))
	require.NoError(t, err)
	defer db.Close()
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	a, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	require.NoError(t, a.Finish(ctx, 0, context.Canceled))
	dir := os.Getenv("WS_END_FIXTURE_SIGNALS")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "first"), []byte("ready"), 0600))
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, "next")); return err == nil }, 20*time.Second, 10*time.Millisecond)
	a.Receipt().ObserveWSEnd(WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit", Reason: "cancelled", ReasonSource: "finish_cause", JoinedUnfinished: true})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "second"), []byte("observed"), 0600))
	select {} // The parent kills this process with the optional evidence write blocked.
}

func TestLedgerPostgresWSEndKilledBeforeWrite(t *testing.T) {
	db, dsn := ledgertest.NewWithDSN(t)
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestLedgerPostgresWSEndCrashChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "WS_END_SNAPSHOT_CHILD=1", "WS_END_FIXTURE_DSN="+dsn, "WS_END_FIXTURE_SIGNALS="+dir)
	require.NoError(t, child.Start())
	t.Cleanup(func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	waitSignal := func(name string) {
		require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, name)); return err == nil }, 15*time.Second, 10*time.Millisecond)
	}
	waitSignal("first")
	var id string
	require.NoError(t, db.QueryRow(`SELECT request_id FROM gateway_request_attempt_evidence `).Scan(&id))
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_ws_end_evidence IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "next"), []byte("observe"), 0600))
	waitSignal("second")
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	require.NoError(t, blocker.Rollback())
	restarted := New(db)
	page, err := restarted.WSEndEvidence(context.Background(), Viewer{Admin: true}, id, 0)
	require.NoError(t, err)
	require.Equal(t, "unknown", page.Completeness)
	require.Equal(t, "not_recorded", page.Attempts[0].Status)
	require.Nil(t, page.Attempts[0].Snapshot, "process exit loses only optional evidence, not the durable attempt")
	_, err = db.Exec(`UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	require.NoError(t, restarted.Recover(context.Background()))
	var state, usage string
	var attempts int
	require.NoError(t, db.QueryRow(`SELECT execution_state,usage_state,attempt_count FROM gateway_requests WHERE id=$1`, id).Scan(&state, &usage, &attempts))
	require.Equal(t, "interrupted", state)
	require.Equal(t, "usage_unknown", usage)
	require.Equal(t, 1, attempts)
}
