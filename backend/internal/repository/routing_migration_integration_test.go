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
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const routingMigrationName = "279_gateway_routing_evidence.sql"

type routingMigrationResult struct {
	err     error
	elapsed time.Duration
}

// Only this disposable loopback fixture is rewound, never an application DSN.
func routingMigrationFixture(t *testing.T) (*sql.DB, fstest.MapFS) {
	t.Helper()
	db := ledgertest.New(t)
	_, err := db.Exec(`DROP TABLE gateway_request_routing;
 ALTER TABLE gateway_requests DROP COLUMN routing_coverage_version, DROP COLUMN routing_summary`)
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile(routingMigrationName)
	require.NoError(t, err)
	return db, fstest.MapFS{routingMigrationName: &fstest.MapFile{Data: raw}}
}

func routingMigrationRun(db *sql.DB, fsys fstest.MapFS) <-chan routingMigrationResult {
	done := make(chan routingMigrationResult, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		start := time.Now()
		err := applyMigrationsFS(ctx, db, fsys)
		done <- routingMigrationResult{err, time.Since(start)}
	}()
	return done
}

func routingMigrationAwait(t *testing.T, done <-chan routingMigrationResult) routingMigrationResult {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(12 * time.Second):
		t.Fatal("migration did not terminate")
		return routingMigrationResult{}
	}
}

func routingMigrationRolledBack(t *testing.T, db *sql.DB) {
	t.Helper()
	var columns, tables, records int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='gateway_requests' AND column_name IN ('routing_coverage_version','routing_summary')`).Scan(&columns))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM pg_class WHERE oid=to_regclass('public.gateway_request_routing')`).Scan(&tables))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE filename=$1`, routingMigrationName).Scan(&records))
	require.Zero(t, columns, "no partial ALTER may survive")
	require.Zero(t, tables, "no partial evidence table may survive")
	require.Zero(t, records, "failed migration must not be registered")
}

// Exercise the pre-279 Begin/Finish SQL and the same three-second write deadline.
// A new binary requires the new columns, so using it before migration would test
// missing-column errors instead of the old application's continuing admission.
func routingLegacyBegin(db *sql.DB) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	owner, id := uuid.NewString(), uuid.NewString()
	var confirmed string
	err = tx.QueryRowContext(ctx, `INSERT INTO gateway_ledger_instances(id,lease_until)
 VALUES($1,clock_timestamp()+interval '90 seconds') ON CONFLICT(id) DO UPDATE
 SET lease_until=EXCLUDED.lease_until WHERE gateway_ledger_instances.lease_until>clock_timestamp()
 RETURNING id`, owner).Scan(&confirmed)
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO gateway_requests(id,parent_id,kind,turn_no,route,method,user_id,api_key_id,instance_id)
 VALUES($1,$2,$3,NULLIF($4,0),$5,$6,NULLIF($7,0),NULLIF($8,0),$9)`, id, nil, "http", 0, "/v1/responses", "POST", 0, 0, confirmed)
	if err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func routingLegacyFinish(db *sql.DB, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, `WITH closing AS (
 SELECT id, CASE WHEN $2<>'succeeded' THEN $2
 WHEN EXISTS(SELECT 1 FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.phase<>'auxiliary' AND a.execution_state='inflight') THEN 'interrupted'
 ELSE COALESCE((SELECT execution_state FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.phase<>'auxiliary' ORDER BY attempt_no DESC LIMIT 1),$2) END AS state,
 (SELECT error_code FROM gateway_request_attempts a WHERE a.request_id=r.id AND a.phase<>'auxiliary' ORDER BY attempt_no DESC LIMIT 1) AS last_code
 FROM gateway_requests r WHERE id=$1 AND execution_state='inflight' FOR UPDATE
 ), finished AS (
 UPDATE gateway_requests r SET execution_state=c.state,ended_at=clock_timestamp(),http_status=NULLIF($3,0),
 error_code=CASE WHEN c.state='interrupted' THEN COALESCE(NULLIF(c.last_code,''),'interrupted') WHEN $4='' AND c.state<>'succeeded' THEN COALESCE(NULLIF(c.last_code,''),'upstream_error') ELSE $4 END,
 usage_state=CASE WHEN usage_state='pending' THEN 'usage_unknown' ELSE usage_state END
 FROM closing c WHERE r.id=c.id RETURNING r.id,r.execution_state)
 UPDATE gateway_request_attempts a SET usage_state=CASE WHEN a.usage_state='pending' THEN 'usage_unknown' ELSE a.usage_state END,execution_state=f.execution_state,ended_at=clock_timestamp(),
 error_code=CASE WHEN f.execution_state IN ('cancelled','timeout','interrupted') THEN f.execution_state ELSE 'upstream_error' END
 FROM finished f WHERE a.request_id=f.id AND a.execution_state='inflight'`, id, "succeeded", 200, "")
	return err
}

func routingMigrationRetryAndCompatibility(t *testing.T, db *sql.DB, fsys fstest.MapFS, historicalID string) {
	t.Helper()
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys), "registered retry must be idempotent")
	var checksum, version, status string
	require.NoError(t, db.QueryRow(`SELECT checksum FROM schema_migrations WHERE filename=$1`, routingMigrationName).Scan(&checksum))
	hash := sha256.Sum256([]byte(strings.TrimSpace(string(fsys[routingMigrationName].Data))))
	require.Equal(t, hex.EncodeToString(hash[:]), checksum)
	require.NoError(t, db.QueryRow(`SELECT routing_coverage_version,routing_summary->>'status' FROM gateway_requests WHERE id=$1`, historicalID).Scan(&version, &status))
	require.Empty(t, version)
	require.Equal(t, "not_recorded", status)
	oldID, err := routingLegacyBegin(db)
	require.NoError(t, err)
	require.NoError(t, routingLegacyFinish(db, oldID))
	require.NoError(t, db.QueryRow(`SELECT routing_coverage_version,routing_summary->>'status' FROM gateway_requests WHERE id=$1`, oldID).Scan(&version, &status))
	require.Empty(t, version)
	require.Equal(t, "not_recorded", status)
	ledger := requestledger.New(db)
	defer ledger.Stop()
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	require.NoError(t, h.Finish(context.Background(), "succeeded", 200, ""))
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE id IN ($1,$2,$3)`, historicalID, oldID, h.ID).Scan(&count))
	require.Equal(t, 3, count, "retry preserves history and both writer generations")
}

func TestRequestLedgerRoutingMigrationBusyKeepsAdmission(t *testing.T) {
	for _, mode := range []string{"reader", "writer"} {
		t.Run(mode, func(t *testing.T) {
			db, fsys := routingMigrationFixture(t)
			oldID, err := routingLegacyBegin(db)
			require.NoError(t, err)
			blocker, err := db.Begin()
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback() }()
			if mode == "reader" {
				var count int
				require.NoError(t, blocker.QueryRow(`SELECT count(*) FROM gateway_requests`).Scan(&count))
			} else {
				_, err = blocker.Exec(`INSERT INTO gateway_requests(id,kind,route,method,instance_id) SELECT $1,'http','/v1/responses','POST',instance_id FROM gateway_requests WHERE id=$2`, uuid.NewString(), oldID)
				require.NoError(t, err)
			}
			done := routingMigrationRun(db, fsys)
			// The old migration queues AccessExclusive behind the blocker. The fixed
			// migration finishes immediately; both branches are observable, not sleeps.
			var result *routingMigrationResult
			require.Eventually(t, func() bool {
				select {
				case r := <-done:
					result = &r
					return true
				default:
				}
				var queued bool
				err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='gateway_requests'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`).Scan(&queued)
				return err == nil && queued
			}, 2*time.Second, 5*time.Millisecond)
			beginDone, finishDone := make(chan error, 1), make(chan error, 1)
			go func() {
				id, err := routingLegacyBegin(db)
				if err == nil {
					err = routingLegacyFinish(db, id)
				}
				beginDone <- err
			}()
			go func() { finishDone <- routingLegacyFinish(db, oldID) }()
			beginErr, finishErr := <-beginDone, <-finishDone
			if result == nil {
				r := routingMigrationAwait(t, done)
				result = &r
			}
			t.Logf("%s contention migration=%s begin=%v finish=%v", mode, result.elapsed, beginErr, finishErr)
			require.NoError(t, beginErr, "DDL must not queue ahead of ordinary admission")
			require.NoError(t, finishErr, "DDL must not queue ahead of ordinary finalization")
			require.Less(t, result.elapsed, 500*time.Millisecond)
			var pgErr *pq.Error
			require.ErrorAs(t, result.err, &pgErr)
			require.Equal(t, pq.ErrorCode("55P03"), pgErr.Code)
			routingMigrationRolledBack(t, db)
			require.NoError(t, blocker.Rollback())
			routingMigrationRetryAndCompatibility(t, db, fsys, oldID)
		})
	}
}

func TestRequestLedgerRoutingMigrationExecutionTimeoutRollsBack(t *testing.T) {
	db, fsys := routingMigrationFixture(t)
	oldID, err := routingLegacyBegin(db)
	require.NoError(t, err)
	// Pause CREATE only after both ALTER additions are visible in its transaction.
	// This proves statement cancellation rolls back work already performed.
	_, err = db.Exec(`CREATE FUNCTION routing_test_slow_create() RETURNS event_trigger LANGUAGE plpgsql AS $$
 BEGIN
 IF position('CREATE TABLE IF NOT EXISTS gateway_request_routing' IN current_query()) > 0 THEN
   IF (SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='gateway_requests' AND column_name IN ('routing_coverage_version','routing_summary')) <> 2 THEN
     RAISE EXCEPTION 'fixture expected both uncommitted columns';
   END IF;
   PERFORM pg_sleep(2);
 END IF;
 END $$;
 CREATE EVENT TRIGGER routing_test_slow_create ON ddl_command_start WHEN TAG IN ('CREATE TABLE') EXECUTE FUNCTION routing_test_slow_create()`)
	require.NoError(t, err)
	done := routingMigrationRun(db, fsys)
	require.Eventually(t, func() bool {
		var held bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='gateway_requests'::regclass AND mode='AccessExclusiveLock' AND granted)`).Scan(&held)
		return err == nil && held
	}, time.Second, 5*time.Millisecond)
	beginDone, finishDone := make(chan error, 1), make(chan error, 1)
	go func() {
		id, err := routingLegacyBegin(db)
		if err == nil {
			err = routingLegacyFinish(db, id)
		}
		beginDone <- err
	}()
	go func() { finishDone <- routingLegacyFinish(db, oldID) }()
	result := routingMigrationAwait(t, done)
	require.NoError(t, <-beginDone)
	require.NoError(t, <-finishDone)
	t.Logf("slow DDL migration=%s error=%v", result.elapsed, result.err)
	var pgErr *pq.Error
	require.ErrorAs(t, result.err, &pgErr)
	require.Equal(t, pq.ErrorCode("57014"), pgErr.Code)
	require.Less(t, result.elapsed, 1500*time.Millisecond, "actual DDL must release the parent lock before the three-second admission deadline")
	routingMigrationRolledBack(t, db)
	_, err = db.Exec(`DROP EVENT TRIGGER routing_test_slow_create; DROP FUNCTION routing_test_slow_create()`)
	require.NoError(t, err)
	routingMigrationRetryAndCompatibility(t, db, fsys, oldID)
}
