//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestRequestLedgerRoutingAfterDetailPaginationMigration(t *testing.T) {
	db, fsys := routingMigrationFixture(t)
	const indexMigration = "278_gateway_request_task_page_index_notx.sql"
	raw, err := migrations.FS.ReadFile(indexMigration)
	require.NoError(t, err)
	fsys[indexMigration] = &fstest.MapFile{Data: raw}
	historicalID, err := routingLegacyBegin(db)
	require.NoError(t, err)
	require.NoError(t, routingLegacyFinish(db, historicalID))
	// The real runner sorts 278 before 279, executes CONCURRENTLY outside the
	// transaction, then uses the unchanged transactional routing migration.
	require.NoError(t, applyMigrationsFS(context.Background(), db, fsys))
	var valid, ready bool
	require.NoError(t, db.QueryRow(`SELECT indisvalid,indisready FROM pg_index WHERE indexrelid='gateway_request_tasks_page_idx'::regclass`).Scan(&valid, &ready))
	require.True(t, valid)
	require.True(t, ready)
	var checksum string
	require.NoError(t, db.QueryRow(`SELECT checksum FROM schema_migrations WHERE filename=$1`, indexMigration).Scan(&checksum))
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(raw))))
	require.Equal(t, hex.EncodeToString(sum[:]), checksum)
	// Reuse the existing real-runner assertions for 279 registration, idempotent
	// restart, history preservation and both writer generations' Begin/Finish.
	routingMigrationRetryAndCompatibility(t, db, fsys, historicalID)
}
