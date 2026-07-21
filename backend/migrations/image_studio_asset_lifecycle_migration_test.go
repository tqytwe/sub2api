package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioAssetLifecycleMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("208_image_studio_asset_lifecycle.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	normalized := strings.Join(strings.Fields(sql), " ")

	for _, fragment := range []string{
		"add column if not exists group_id bigint",
		"add column if not exists platform text not null default ''",
		"add column if not exists capability_profile_id text not null default ''",
		"add column if not exists capability_revision text not null default ''",
		"set expires_at = null where status in ('pending', 'running')",
		"set expires_at = coalesce(finished_at, created_at) + interval '7 days'",
		"add column if not exists expires_at timestamptz",
		"add column if not exists purged_at timestamptz",
		"add column if not exists filename text",
		"set expires_at = created_at + interval '24 hours'",
		"values ('image_studio_asset_purge_enabled', 'false')",
	} {
		require.Contains(t, normalized, fragment)
	}
	require.NotContains(t, normalized, "platform = 'openai'")
	require.NotContains(t, normalized, "drop column")
	require.NotContains(t, normalized, "delete from image_studio")
	require.NotContains(t, normalized, "begin")
	require.NotContains(t, normalized, "commit")

	raw, err = os.ReadFile("208_image_studio_asset_lifecycle_indexes_notx.sql")
	require.NoError(t, err)
	indexSQL := strings.ToLower(string(raw))
	indexNormalized := strings.Join(strings.Fields(indexSQL), " ")
	require.Contains(t, indexNormalized, "create index concurrently if not exists idx_image_studio_assets_expiry_live")
	require.Contains(t, indexNormalized, "on image_studio_assets(expires_at, id)")
	require.Contains(t, indexNormalized, "and purged_at is null")
	require.Contains(t, indexNormalized, "create index concurrently if not exists idx_image_studio_jobs_record_expiry")
	require.NotContains(t, indexNormalized, "begin")
	require.NotContains(t, indexNormalized, "commit")
}
