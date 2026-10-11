//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerDetailPaginationMigrationRecoversInterruptedIndex(t *testing.T) {
	const migration = "278_gateway_request_task_page_index_notx.sql"
	const index = "gateway_request_tasks_page_idx"
	ctx := context.Background()
	raw, err := dbmigrations.FS.ReadFile(migration)
	require.NoError(t, err)
	fsys := fstest.MapFS{migration: &fstest.MapFile{Data: raw}}
	// Restore only this disposable fixture's migration, including after a RED.
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS "+index)
		_, _ = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename=$1", migration)
		require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))
	})
	_, err = integrationDB.ExecContext(ctx, "DROP INDEX CONCURRENTLY IF EXISTS "+index)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename=$1", migration)
	require.NoError(t, err)
	// Hold a writer while the actual non-unique concurrent build creates its
	// invalid catalog entry and waits for writers. Cancel that real build.
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, "LOCK TABLE gateway_request_tasks IN ROW EXCLUSIVE MODE")
	require.NoError(t, err)
	buildCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := integrationDB.ExecContext(buildCtx, string(raw)); done <- err }()
	require.Eventually(t, func() bool {
		invalid, err := indexIsInvalid(ctx, integrationDB, index)
		return err == nil && invalid
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent build did not stop")
	}
	require.NoError(t, tx.Rollback())
	invalid, err := indexIsInvalid(ctx, integrationDB, index)
	require.NoError(t, err)
	require.True(t, invalid)
	invalidOID := publicIndexOID(t, ctx, integrationDB, index)
	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))
	invalid, err = indexIsInvalid(ctx, integrationDB, index)
	require.NoError(t, err)
	require.False(t, invalid, "a retry must rebuild the interrupted index before recording success")
	healthyOID := publicIndexOID(t, ctx, integrationDB, index)
	require.NotEqual(t, invalidOID, healthyOID)
	var checksum string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename=$1", migration).Scan(&checksum))
	hash := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
	require.Equal(t, hex.EncodeToString(hash[:]), checksum)
	// A completed index without its history row is replayed without dropping it.
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM schema_migrations WHERE filename=$1", migration)
	require.NoError(t, err)
	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))
	require.Equal(t, healthyOID, publicIndexOID(t, ctx, integrationDB, index))
	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations WHERE filename=$1", migration).Scan(&count))
	require.Equal(t, 1, count)
}
