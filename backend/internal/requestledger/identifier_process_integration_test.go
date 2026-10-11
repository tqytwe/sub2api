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

func TestLedgerPostgresIdentifierSnapshotCrashChild(t *testing.T) {
	if os.Getenv("IDENTIFIER_SNAPSHOT_CHILD") != "1" {
		t.Skip("subprocess fixture only")
	}
	db, err := sql.Open("postgres", os.Getenv("IDENTIFIER_FIXTURE_DSN"))
	require.NoError(t, err)
	defer db.Close()
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	a, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	a.Receipt().ObserveOpsIdentifiers(ctx, "req_saved", "resp_saved", "http_error")
	waitAttemptIdentifiers(t, a)
	dir := os.Getenv("IDENTIFIER_FIXTURE_SIGNALS")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "first"), []byte("ready"), 0600))
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, "next")); return err == nil }, 20*time.Second, 10*time.Millisecond)
	a.Receipt().ObserveOpsIdentifiers(ctx, "req_lost", "resp_saved", "failover")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "second"), []byte("observed"), 0600))
	select {} // The parent kills this process with the later evidence write blocked.
}

func TestLedgerPostgresIdentifierSnapshotKilledAfterFirstWrite(t *testing.T) {
	db, dsn := ledgertest.NewWithDSN(t)
	dir := t.TempDir()
	child := exec.Command(os.Args[0], "-test.run=^TestLedgerPostgresIdentifierSnapshotCrashChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "IDENTIFIER_SNAPSHOT_CHILD=1", "IDENTIFIER_FIXTURE_DSN="+dsn, "IDENTIFIER_FIXTURE_SIGNALS="+dir)
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
	require.NoError(t, db.QueryRow(`SELECT request_id FROM gateway_request_attempt_evidence WHERE upstream_request_id='req_saved'`).Scan(&id))
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_attempt_evidence IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "next"), []byte("observe"), 0600))
	waitSignal("second")
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	require.NoError(t, blocker.Rollback())
	restarted := New(db)
	page, err := restarted.IdentifierEvidence(context.Background(), Viewer{Admin: true}, id, 0)
	require.NoError(t, err)
	require.Equal(t, "unknown", page.Completeness)
	require.Equal(t, "persisted_snapshot", page.Attempts[0].Status)
	require.False(t, *page.Attempts[0].IdentifierConflict, "only the first snapshot survived; false is not proof of no later conflict")
	require.Nil(t, page.Attempts[0].Request.FirstDifferent)
	require.Equal(t, "req_saved", page.Attempts[0].Request.First.Value)
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

func TestLedgerPostgresIdentifierSnapshotQueueLossAfterFirstWrite(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	victim, err := BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	victim.Receipt().ObserveOpsIdentifiers(ctx, "req_saved", "resp_saved", "http_error")
	waitAttemptIdentifiers(t, victim)
	attempts := make([]*Attempt, 300)
	for i := range attempts {
		attempts[i], err = BeginAttempt(ctx, 234, 234)
		require.NoError(t, err)
	}
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_attempt_evidence IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	start := time.Now()
	for _, a := range attempts {
		a.Receipt().ObserveOpsIdentifiers(ctx, "req_fill", "resp_fill", "http_error")
	}
	victim.Receipt().ObserveOpsIdentifiers(ctx, "req_lost", "resp_saved", "failover")
	require.Less(t, time.Since(start), 200*time.Millisecond)
	victim.evidenceMu.Lock()
	dropped := victim.evidenceUnavailable
	victim.evidenceMu.Unlock()
	require.True(t, dropped)
	require.LessOrEqual(t, len(identifierWrites.queue), 256)
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
	page, err := ledger.IdentifierEvidence(ctx, Viewer{Admin: true}, h.ID, 0)
	require.NoError(t, err)
	require.Equal(t, "unknown", page.Completeness)
	require.Equal(t, "persisted_snapshot", page.Attempts[0].Status)
	require.False(t, *page.Attempts[0].IdentifierConflict)
	require.Nil(t, page.Attempts[0].Request.FirstDifferent)
}
