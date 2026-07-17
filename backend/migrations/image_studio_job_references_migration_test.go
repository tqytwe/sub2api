package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioJobReferencesMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("196_image_studio_job_references.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists image_studio_job_references",
		"job_id",
		"storage_key",
		"content_type",
		"byte_size",
		"sort_order",
		"references image_studio_jobs(id) on delete cascade",
		"unique (job_id, sort_order)",
	} {
		require.Contains(t, sql, fragment)
	}

	require.NotContains(t, sql, "source_reference_id")
	require.NotContains(t, sql, "source_storage_key")
	require.NotContains(t, sql, "data bytea")
	require.NotContains(t, sql, "expires_at")
}
