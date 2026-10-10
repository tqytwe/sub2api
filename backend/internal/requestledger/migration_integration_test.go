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
