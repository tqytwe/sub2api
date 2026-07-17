package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioObjectDeletionsMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("197_image_studio_object_deletions.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists image_studio_object_deletions",
		"user_id",
		"job_id",
		"storage_key",
		"attempts",
		"last_error",
		"unique (job_id, storage_key)",
		"idx_image_studio_object_deletions_pending",
	} {
		require.Contains(t, sql, fragment)
	}
	require.NotContains(t, sql, "references image_studio_jobs")
}
