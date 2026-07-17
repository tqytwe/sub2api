package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioUploadSlotsMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("198_image_studio_upload_slots.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))

	for _, fragment := range []string{
		"create table if not exists image_studio_upload_slots",
		"user_id",
		"started_at",
		"lease_expires_at",
		"released_at",
		"references users(id) on delete cascade",
	} {
		require.Contains(t, sql, fragment)
	}
}
