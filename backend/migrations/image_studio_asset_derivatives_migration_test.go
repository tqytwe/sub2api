package migrations

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageStudioAssetDerivativesMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("194_image_studio_asset_derivatives.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(raw))
	normalized := strings.Join(strings.Fields(sql), " ")

	for _, fragment := range []string{
		"alter table image_studio_assets",
		"width",
		"height",
		"thumbnail_storage_key",
		"thumbnail_content_type",
		"thumbnail_byte_size",
		"check (width is null or width > 0)",
		"check (height is null or height > 0)",
		"thumbnail_storage_key is not null",
		"thumbnail_content_type is not null",
		"thumbnail_byte_size is not null",
		"not valid",
		"validate constraint image_studio_assets_width_chk_upgrade",
		"validate constraint image_studio_assets_height_chk_upgrade",
		"validate constraint image_studio_assets_thumbnail_size_chk_upgrade",
		"validate constraint image_studio_assets_dimensions_pair_chk_upgrade",
		"validate constraint image_studio_assets_thumbnail_pair_chk_upgrade",
	} {
		require.Contains(t, normalized, fragment)
	}
	require.NotContains(t, normalized, "create index")
	require.NotContains(t, normalized, "begin")
	require.NotContains(t, normalized, "commit")
	require.NotContains(t, sql, "aspect_ratio")

	raw, err = os.ReadFile("194_image_studio_asset_derivatives_indexes_notx.sql")
	require.NoError(t, err)
	indexSQL := strings.ToLower(string(raw))
	indexNormalized := strings.Join(strings.Fields(indexSQL), " ")
	require.Contains(
		t,
		indexNormalized,
		"create index concurrently if not exists idx_image_studio_jobs_user_created_id",
	)
	require.Contains(t, indexNormalized, "on image_studio_jobs(user_id, created_at desc, id desc)")
	require.NotContains(t, indexNormalized, "drop index")
	require.NotContains(t, indexNormalized, "begin")
	require.NotContains(t, indexNormalized, "commit")
}
