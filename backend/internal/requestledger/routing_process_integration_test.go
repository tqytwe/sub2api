//go:build integration

package requestledger

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresRoutingCrashChild(t *testing.T) {
	if os.Getenv("ROUTING_LEDGER_CHILD") != "1" {
		t.Skip("subprocess fixture")
	}
	db, err := sql.Open("postgres", os.Getenv("ROUTING_LEDGER_LOCAL_DSN"))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	l := New(db)
	defer l.Stop()
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx, end := BeginRoutingSelection(WithHandle(context.Background(), h), true)
	RoutingVeto(ctx, "initial_filter", 42, "model_not_supported")
	end(0, false, true)
	_, err = fmt.Fprintln(os.Stdout, h.ID)
	require.NoError(t, err)
	<-time.After(30 * time.Second)
	t.Fatal("parent should kill the process before Finish")
}

func TestLedgerPostgresRoutingKilledProcessIsIncomplete(t *testing.T) {
	db, dsn := ledgertest.NewWithDSN(t)
	child := exec.Command(os.Args[0], "-test.run=^TestLedgerPostgresRoutingCrashChild$", "-test.timeout=35s")
	child.Env = append(os.Environ(), "ROUTING_LEDGER_CHILD=1", "ROUTING_LEDGER_LOCAL_DSN="+dsn)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	ids := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if _, err := uuid.Parse(scanner.Text()); err == nil {
				ids <- scanner.Text()
				return
			}
		}
	}()
	var id string
	select {
	case id = <-ids:
	case <-time.After(15 * time.Second):
		t.Fatal("fixture child did not admit")
	}
	require.NoError(t, child.Process.Kill())
	require.Error(t, child.Wait())
	_, err = db.Exec(`UPDATE gateway_ledger_instances SET lease_until=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	l := New(db)
	defer l.Stop()
	require.NoError(t, l.Recover(context.Background()))
	e, err := l.RoutingEvidence(context.Background(), Viewer{Admin: true, UserID: 1}, id)
	require.NoError(t, err)
	require.Equal(t, "incomplete", e.Status)
	require.True(t, e.Incomplete)
	require.Equal(t, RoutingCoverageVersion, e.CoverageVersion)
	var requests, attempts, billing int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&requests))
	require.Equal(t, 1, requests)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_attempts`).Scan(&attempts))
	require.Zero(t, attempts)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_billing_links`).Scan(&billing))
	require.Zero(t, billing)
}

func TestLedgerPostgresRoutingMigrationOldWriterAndLock(t *testing.T) {
	db := ledgerPostgres(t)
	l := New(db)
	defer l.Stop()
	ctx := context.Background()
	h, err := l.Begin(ctx, "/v1/responses", "POST", "http")
	require.NoError(t, err)
	oldID := uuid.NewString()
	_, err = db.Exec(`INSERT INTO gateway_requests(id,kind,route,method,instance_id) VALUES($1,'http','/v1/responses','POST',$2)`, oldID, h.owner)
	require.NoError(t, err)
	e, err := l.RoutingEvidence(ctx, Viewer{Admin: true, UserID: 1}, oldID)
	require.NoError(t, err)
	require.Equal(t, "not_recorded", e.Status)
	require.Empty(t, e.CoverageVersion)
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_requests IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("279_gateway_routing_evidence.sql")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	start := time.Now()
	_, err = tx.Exec(string(migration))
	require.Less(t, time.Since(start), 6*time.Second)
	var pgErr *pq.Error
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, pq.ErrorCode("55P03"), pgErr.Code)
	require.NoError(t, tx.Rollback())
	require.NoError(t, blocker.Rollback())
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&count))
	require.Equal(t, 2, count)
}
