//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/requestledger/ledgertest"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const wsEndMigrationName = "282_gateway_ws_attempt_end_evidence.sql"

func wsEndMigrationFixture(t *testing.T) (*sql.DB, fstest.MapFS, *requestledger.Handle, *requestledger.Attempt) {
	t.Helper()
	db := ledgertest.New(t)
	_, err := db.Exec(`DROP TABLE gateway_request_ws_end_evidence`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile(wsEndMigrationName)
	require.NoError(t, err)
	l := requestledger.New(db)
	h, err := l.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	a, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET upstream_request_id='req_historical',identifier_conflict=TRUE WHERE attempt_id=$1`, a.ID)
	require.NoError(t, err)
	return db, fstest.MapFS{wsEndMigrationName: &fstest.MapFile{Data: raw}}, h, a
}
func wsEndMigrationRollback(t *testing.T, db *sql.DB) {
	t.Helper()
	var present bool
	var entries int
	require.NoError(t, db.QueryRow(`SELECT to_regclass('gateway_request_ws_end_evidence') IS NOT NULL`).Scan(&present))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE filename=$1`, wsEndMigrationName).Scan(&entries))
	require.False(t, present)
	require.Zero(t, entries)
}
func wsEndMigrationRetry(t *testing.T, db *sql.DB, fsys fstest.MapFS, h *requestledger.Handle, historical *requestledger.Attempt) {
	t.Helper()
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	var checksum, first, snapshot string
	var conflict bool
	var count int
	require.NoError(t, db.QueryRow(`SELECT checksum FROM schema_migrations WHERE filename=$1`, wsEndMigrationName).Scan(&checksum))
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(fsys[wsEndMigrationName].Data))))
	require.Equal(t, hex.EncodeToString(sum[:]), checksum)
	require.NoError(t, db.QueryRow(`SELECT upstream_request_id,identifier_conflict FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, historical.ID).Scan(&first, &conflict))
	require.Equal(t, "req_historical", first)
	require.True(t, conflict)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_ws_end_evidence`).Scan(&count))
	require.Zero(t, count, "no historical reconstruction")
	ctx := requestledger.WithHandle(context.Background(), h)
	a, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	a.ObserveIdentifiers(ctx, "req_new", "resp_new")
	a.Receipt().ObserveWSEnd(requestledger.WSEndObservation{Stage: "read_upstream", StageSource: "relay_exit"})
	require.Eventually(t, func() bool {
		var version, requestID string
		return db.QueryRow(`SELECT w.snapshot->>'version',e.upstream_request_id FROM gateway_request_ws_end_evidence w JOIN gateway_request_attempt_evidence e USING(request_id,attempt_no) WHERE w.attempt_id=$1`, a.ID).Scan(&version, &requestID) == nil && version == requestledger.WSEndSnapshotVersion && requestID == "req_new"
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, db.QueryRow(`SELECT snapshot::text FROM gateway_request_ws_end_evidence WHERE attempt_id=$1`, a.ID).Scan(&snapshot))
	before := snapshot
	// Previous identifier writer remains byte-for-byte independent of the WS table.
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET
 identifier_conflict=identifier_conflict OR $5 OR (upstream_request_id IS NOT NULL AND $3<>'' AND upstream_request_id<>$3)
 OR (upstream_response_id IS NOT NULL AND $4<>'' AND upstream_response_id<>$4),
 upstream_request_id=COALESCE(upstream_request_id,NULLIF($3,'')),
 upstream_response_id=COALESCE(upstream_response_id,NULLIF($4,''))
 WHERE (request_id,attempt_no) IN (SELECT request_id,attempt_no FROM gateway_request_attempt_evidence
 WHERE request_id=$1 AND attempt_no=$2 FOR UPDATE SKIP LOCKED)`, h.ID, a.Number, "req_old_writer_different", "resp_new", false)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT e.upstream_request_id,e.identifier_conflict,w.snapshot::text FROM gateway_request_attempt_evidence e JOIN gateway_request_ws_end_evidence w USING(request_id,attempt_no) WHERE e.attempt_id=$1`, a.ID).Scan(&first, &conflict, &snapshot))
	require.Equal(t, "req_new", first)
	require.True(t, conflict)
	require.Equal(t, before, snapshot)
	legacy, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	require.NoError(t, legacy.Finish(ctx, 200, nil))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_request_ws_end_evidence WHERE attempt_id=$1`, legacy.ID).Scan(&count))
	require.Zero(t, count, "old writer remains valid with no WS evidence")
	_, err = db.Exec(`INSERT INTO gateway_request_ws_end_evidence(request_id,attempt_no,attempt_id,snapshot) VALUES($1,$2,$3,jsonb_build_object('body',repeat('x',4096)))`, h.ID, legacy.Number, legacy.ID)
	var constraint *pq.Error
	require.ErrorAs(t, err, &constraint)
	require.Equal(t, "23514", string(constraint.Code))
}
func TestRequestLedgerWSEndMigrationCompatibilityAndBusy(t *testing.T) {
	for _, mode := range []string{"reader", "writer"} {
		t.Run(mode, func(t *testing.T) {
			db, fsys, h, historical := wsEndMigrationFixture(t)
			blocker, err := db.Begin()
			require.NoError(t, err)
			defer blocker.Rollback()
			if mode == "reader" {
				_, err = blocker.Exec(`SELECT 1 FROM gateway_request_attempts WHERE request_id=$1`, h.ID)
			} else {
				_, err = blocker.Exec(`UPDATE gateway_request_attempts SET execution_state=execution_state WHERE request_id=$1`, h.ID)
			}
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = applyMigrationsFS(ctx, db, fsys)
			var busy *pq.Error
			require.ErrorAs(t, err, &busy)
			require.Equal(t, "55P03", string(busy.Code))
			wsEndMigrationRollback(t, db)
			a, err := requestledger.BeginAttempt(requestledger.WithHandle(context.Background(), h), 234, 234)
			require.NoError(t, err)
			require.NoError(t, a.Finish(context.Background(), 200, nil))
			require.NoError(t, blocker.Rollback())
			wsEndMigrationRetry(t, db, fsys, h, historical)
		})
	}
}
func TestRequestLedgerWSEndMigrationExecutionTimeoutRollsBack(t *testing.T) {
	db, fsys, h, historical := wsEndMigrationFixture(t)
	require.NoError(t, applyMigrationsFS(context.Background(), db, fstest.MapFS{}))
	_, err := db.Exec(`CREATE FUNCTION fixture_ws_end_slow_ddl() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(1); END $$;
 CREATE EVENT TRIGGER fixture_ws_end_slow_ddl ON ddl_command_end WHEN TAG IN ('CREATE TABLE') EXECUTE FUNCTION fixture_ws_end_slow_ddl()`)
	require.NoError(t, err)
	err = applyMigrationsFS(context.Background(), db, fsys)
	var timeout *pq.Error
	require.ErrorAs(t, err, &timeout)
	require.Equal(t, "57014", string(timeout.Code))
	wsEndMigrationRollback(t, db)
	_, err = db.Exec(`DROP EVENT TRIGGER fixture_ws_end_slow_ddl; DROP FUNCTION fixture_ws_end_slow_ddl()`)
	require.NoError(t, err)
	wsEndMigrationRetry(t, db, fsys, h, historical)
}
