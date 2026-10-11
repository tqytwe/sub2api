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

const identifierMigrationName = "281_gateway_identifier_diagnostics.sql"

func identifierMigrationFixture(t *testing.T) (*sql.DB, fstest.MapFS, *requestledger.Handle, *requestledger.Attempt) {
	t.Helper()
	db := ledgertest.New(t)
	// Rewind only this disposable loopback fixture, never an application DSN.
	_, err := db.Exec(`ALTER TABLE gateway_request_attempt_evidence DROP CONSTRAINT gateway_identifier_diagnostics_bounds, DROP COLUMN identifier_diagnostics`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile(identifierMigrationName)
	require.NoError(t, err)
	ledger := requestledger.New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := requestledger.WithHandle(context.Background(), h)
	require.NoError(t, requestledger.BindIdentity(ctx, 101, 201))
	a, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET upstream_request_id='req_historical',identifier_conflict=TRUE WHERE attempt_id=$1`, a.ID)
	require.NoError(t, err)
	return db, fstest.MapFS{identifierMigrationName: &fstest.MapFile{Data: raw}}, h, a
}

func identifierMigrationRollback(t *testing.T, db *sql.DB) {
	t.Helper()
	var columns, entries int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_name='gateway_request_attempt_evidence' AND column_name='identifier_diagnostics'`).Scan(&columns))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE filename=$1`, identifierMigrationName).Scan(&entries))
	require.Zero(t, columns)
	require.Zero(t, entries)
}

func identifierMigrationRetry(t *testing.T, db *sql.DB, fsys fstest.MapFS, h *requestledger.Handle, historical *requestledger.Attempt) {
	t.Helper()
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	var checksum, first, snapshot string
	var conflict bool
	require.NoError(t, db.QueryRow(`SELECT checksum FROM schema_migrations WHERE filename=$1`, identifierMigrationName).Scan(&checksum))
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(fsys[identifierMigrationName].Data))))
	require.Equal(t, hex.EncodeToString(sum[:]), checksum)
	require.NoError(t, db.QueryRow(`SELECT upstream_request_id,identifier_conflict,identifier_diagnostics::text FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, historical.ID).Scan(&first, &conflict, &snapshot))
	require.Equal(t, "req_historical", first)
	require.True(t, conflict)
	require.Equal(t, "{}", snapshot, "no historical reconstruction or flag clearing")
	ctx := requestledger.WithHandle(context.Background(), h)
	newAttempt, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	newAttempt.Receipt().ObserveOpsIdentifiers(ctx, "req_new", "resp_new", "http_error")
	require.Eventually(t, func() bool {
		var version string
		err := db.QueryRow(`SELECT identifier_diagnostics->>'version' FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, newAttempt.ID).Scan(&version)
		return err == nil && version == requestledger.IdentifierSnapshotVersion
	}, 3*time.Second, 10*time.Millisecond)
	var before string
	require.NoError(t, db.QueryRow(`SELECT identifier_diagnostics::text FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, newAttempt.ID).Scan(&before))
	// The previous writer's statement omits the additive column. Its true
	// conflict and first IDs remain monotonic, and it cannot erase the snapshot.
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET
 identifier_conflict=identifier_conflict OR $5 OR (upstream_request_id IS NOT NULL AND $3<>'' AND upstream_request_id<>$3)
 OR (upstream_response_id IS NOT NULL AND $4<>'' AND upstream_response_id<>$4),
 upstream_request_id=COALESCE(upstream_request_id,NULLIF($3,'')),
 upstream_response_id=COALESCE(upstream_response_id,NULLIF($4,''))
 WHERE (request_id,attempt_no) IN (SELECT request_id,attempt_no FROM gateway_request_attempt_evidence
 WHERE request_id=$1 AND attempt_no=$2 FOR UPDATE SKIP LOCKED)`, h.ID, newAttempt.Number, "req_old_writer_different", "resp_new", false)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT upstream_request_id,identifier_conflict,identifier_diagnostics::text FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, newAttempt.ID).Scan(&first, &conflict, &snapshot))
	require.Equal(t, "req_new", first)
	require.True(t, conflict)
	require.Equal(t, before, snapshot)
	legacyAttempt, err := requestledger.BeginAttempt(ctx, 234, 234)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT identifier_diagnostics::text FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, legacyAttempt.ID).Scan(&snapshot))
	require.Equal(t, "{}", snapshot, "old insert without the column remains valid")
	require.NoError(t, legacyAttempt.Finish(ctx, 200, nil))
	_, err = db.Exec(`UPDATE gateway_request_attempt_evidence SET identifier_diagnostics=jsonb_build_object('body',repeat('x',4096)) WHERE attempt_id=$1`, legacyAttempt.ID)
	var constraint *pq.Error
	require.ErrorAs(t, err, &constraint)
	require.Equal(t, "23514", string(constraint.Code), "NOT VALID still checks new writes")
}

func TestRequestLedgerIdentifierMigrationCompatibilityAndBusy(t *testing.T) {
	for _, mode := range []string{"reader", "writer"} {
		t.Run(mode, func(t *testing.T) {
			db, fsys, h, historical := identifierMigrationFixture(t)
			blocker, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback() }()
			if mode == "reader" {
				_, err = blocker.Exec(`SELECT 1 FROM gateway_request_attempt_evidence WHERE attempt_id=$1`, historical.ID)
			} else {
				_, err = blocker.Exec(`UPDATE gateway_request_attempt_evidence SET identifier_conflict=TRUE WHERE attempt_id=$1`, historical.ID)
			}
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = applyMigrationsFS(ctx, db, fsys)
			var busy *pq.Error
			require.ErrorAs(t, err, &busy)
			require.Equal(t, "55P03", string(busy.Code), "NOWAIT must reject before the caller deadline")
			identifierMigrationRollback(t, db)
			// Admission still uses the unchanged pre-281 INSERT columns and deadline.
			a, err := requestledger.BeginAttempt(requestledger.WithHandle(context.Background(), h), 234, 234)
			require.NoError(t, err)
			require.NoError(t, a.Finish(context.Background(), 200, nil))
			require.NoError(t, blocker.Rollback())
			identifierMigrationRetry(t, db, fsys, h, historical)
		})
	}
}

func TestRequestLedgerIdentifierMigrationExecutionTimeoutRollsBack(t *testing.T) {
	db, fsys, h, historical := identifierMigrationFixture(t)
	_, err := db.Exec(`CREATE FUNCTION fixture_identifier_slow_ddl() RETURNS event_trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(1); END $$;
 CREATE EVENT TRIGGER fixture_identifier_slow_ddl ON ddl_command_end WHEN TAG IN ('ALTER TABLE') EXECUTE FUNCTION fixture_identifier_slow_ddl()`)
	require.NoError(t, err)
	err = applyMigrationsFS(context.Background(), db, fsys)
	var timeout *pq.Error
	require.ErrorAs(t, err, &timeout)
	require.Equal(t, "57014", string(timeout.Code))
	identifierMigrationRollback(t, db)
	_, err = db.Exec(`DROP EVENT TRIGGER fixture_identifier_slow_ddl; DROP FUNCTION fixture_identifier_slow_ddl()`)
	require.NoError(t, err)
	identifierMigrationRetry(t, db, fsys, h, historical)
}
