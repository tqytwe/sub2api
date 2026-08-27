package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelCatalogMediaCapabilitiesMigrationIsAdditiveAndNullable(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("260_model_catalog_media_capabilities.sql"))
	require.NoError(t, err)
	text := string(body)

	require.Contains(t, text, "SET LOCAL lock_timeout = '5s'")
	require.Contains(t, text, "ALTER TABLE site_model_catalog")
	require.Contains(t, text, "ADD COLUMN IF NOT EXISTS media_capabilities JSONB")
	require.NotContains(t, text, "NOT NULL")
	require.Contains(t, text, "UPDATE site_model_catalog")
	require.Contains(t, text, "LOWER(platform) = 'openai'")
	require.Contains(t, text, "'sensenova-u1.5-lite'")
	require.Contains(t, text, "'sensenova-u1-fast'")
	require.Contains(t, text, "'grok-imagine-video'")
	require.Contains(t, text, "'grok-imagine-video-1.5'")
	require.Contains(t, text, "'agnes-video-v2.0'")
	require.Contains(t, text, "media_capabilities IS NULL")
	require.Contains(t, text, "\"adapter\":\"sensenova\"")
	require.Contains(t, text, "\"adapter\":\"grok_video\"")
	require.Contains(t, text, "\"adapter\":\"agnes_video\"")
	// The catalog contract must never promise more U1.5 reference images than
	// Image Studio accepts for either the update or insert seed path.
	require.Equal(t, 2, strings.Count(text, "\"max_reference_images\":4"))
	require.NotContains(t, text, "\"max_reference_images\":8")
	require.Contains(t, text, "INSERT INTO site_model_catalog")
	require.Contains(t, text, "ON CONFLICT (model_name, platform) DO UPDATE")
	require.Contains(t, text, "COALESCE(")
	require.Contains(t, text, "WHERE NOT EXISTS")
	require.NotContains(t, text, "UPDATE groups")
	require.NotContains(t, text, "UPDATE accounts")
	require.NotContains(t, text, "UPDATE channel_model_pricing")
	require.NotContains(t, text, "DELETE")
}
