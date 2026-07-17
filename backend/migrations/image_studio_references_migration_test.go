package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioReferencesMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("193_image_studio_references.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists image_studio_references",
		"user_id",
		"storage_key",
		"original_filename",
		"content_type",
		"byte_size",
		"expires_at",
		"references users(id) on delete cascade",
		"content_type like 'image/%'",
		"idx_image_studio_references_user_expiry",
	} {
		require.Contains(t, sql, fragment)
	}

	require.NotContains(t, sql, "public_url")
	require.NotContains(t, sql, "data bytea")
}
