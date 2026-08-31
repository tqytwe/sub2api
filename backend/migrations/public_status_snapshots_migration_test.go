package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicStatusSnapshotMigrationsKeepSnapshotsImmutableAndTTFTIndexOnline(t *testing.T) {
	snapshot, err := os.ReadFile(filepath.Join("263_public_status_snapshots.sql"))
	require.NoError(t, err)
	snapshotSQL := strings.ToUpper(string(snapshot))
	require.Contains(t, snapshotSQL, "CREATE TABLE IF NOT EXISTS PUBLIC_STATUS_SNAPSHOTS")
	require.Contains(t, snapshotSQL, "WINDOW_END TIMESTAMPTZ PRIMARY KEY")
	require.Contains(t, snapshotSQL, "TTFT_P50_MS")
	require.Contains(t, snapshotSQL, "TTFT_P95_MS")
	require.Contains(t, snapshotSQL, "CREATE OR REPLACE FUNCTION REJECT_PUBLIC_STATUS_SNAPSHOT_MUTATION")
	require.Contains(t, snapshotSQL, "RAISE EXCEPTION 'PUBLIC_STATUS_SNAPSHOTS ARE IMMUTABLE AFTER INSERTION'")
	require.Contains(t, snapshotSQL, "CREATE TRIGGER PUBLIC_STATUS_SNAPSHOTS_IMMUTABLE")
	require.Contains(t, snapshotSQL, "BEFORE UPDATE OR DELETE ON PUBLIC_STATUS_SNAPSHOTS")

	index, err := os.ReadFile(filepath.Join("264_public_status_ttft_window_index_notx.sql"))
	require.NoError(t, err)
	indexSQL := strings.ToUpper(string(index))
	require.Contains(t, indexSQL, "CREATE INDEX CONCURRENTLY IF NOT EXISTS IDX_USAGE_LOGS_PUBLIC_STATUS_TTFT_WINDOW")
	require.Contains(t, indexSQL, "WHERE FIRST_TOKEN_MS IS NOT NULL")
}
