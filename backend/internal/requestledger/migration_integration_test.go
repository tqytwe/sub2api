//go:build integration

package requestledger

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestLedgerPostgresMigrationLockTimeoutPreservesEvidence(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_requests IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("275_gateway_request_ledger.sql")
	require.NoError(t, err)
	started := time.Now()
	_, err = tx.ExecContext(ctx, string(content))
	var postgresErr *pq.Error
	require.ErrorAs(t, err, &postgresErr)
	require.Equal(t, pq.ErrorCode("55P03"), postgresErr.Code)
	require.Less(t, time.Since(started), 6*time.Second)
	require.NoError(t, tx.Rollback())
	require.NoError(t, blocker.Rollback())
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM gateway_requests WHERE id=$1`, h.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestLedgerPostgresCorrelationMigrationCompatibility(t *testing.T) {
	db := ledgerPostgres(t)
	ledger := New(db)
	h, err := ledger.Begin(context.Background(), "/v1/responses", "POST", "http")
	require.NoError(t, err)
	ctx := WithHandle(context.Background(), h)
	require.NoError(t, BindIdentity(ctx, 101, 201))
	// Simulate the previous binary's insert, which has no new evidence row.
	_, err = db.Exec(`INSERT INTO gateway_request_attempts(request_id,attempt_no,account_id,credential_account_id,phase,upstream_kind,usage_state) VALUES($1,1,234,22,'request','account','pending')`, h.ID)
	require.NoError(t, err)
	attempts, err := ledger.Attempts(ctx, Viewer{Admin: true}, h.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Nil(t, attempts[0].AttemptID)
	var indexes int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relname IN ('gateway_request_attempt_evidence','gateway_request_ops_links') AND i.indisvalid AND i.indisready`).Scan(&indexes))
	require.Equal(t, 4, indexes)
	_, err = db.Exec(`INSERT INTO gateway_request_attempt_evidence(request_id,attempt_no,attempt_id,upstream_request_id) VALUES($1,1,$1,E'invalid\nvalue')`, h.ID)
	require.Error(t, err)
	blocker, err := db.Begin()
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.Exec(`LOCK TABLE gateway_request_ops_links IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	content, err := migrations.FS.ReadFile("276_gateway_request_correlation.sql")
	require.NoError(t, err)
	tx, err := db.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(string(content))
	var pgErr *pq.Error
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, pq.ErrorCode("55P03"), pgErr.Code)
	require.NoError(t, tx.Rollback())
	require.NoError(t, blocker.Rollback())
	attempts, err = ledger.Attempts(ctx, Viewer{Admin: true}, h.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Nil(t, attempts[0].AttemptID, "never backfill a guessed identity")
}
