//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestModelCatalogMediaCapabilitiesMigrationUpgradesAndReplaysPostgres(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for migration integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))

	_, err = db.ExecContext(ctx, `
		CREATE TABLE site_model_catalog (
			id BIGSERIAL PRIMARY KEY,
			model_name TEXT NOT NULL,
			platform TEXT NOT NULL,
			display_name TEXT,
			visible_auth BOOLEAN NOT NULL DEFAULT FALSE,
			source TEXT NOT NULL DEFAULT 'system',
			UNIQUE (model_name, platform)
		);
		INSERT INTO site_model_catalog (model_name, platform, display_name, visible_auth, source)
		VALUES
			('SenseNova-U1.5-Lite', 'openai', 'existing row', TRUE, 'operator'),
			('grok-imagine-video', 'grok', 'legacy video row', TRUE, 'operator');
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("260_model_catalog_media_capabilities.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)
	require.NoError(t, applyModelCatalogMediaCapabilitiesMigration(ctx, db, migrationSQL))

	for _, check := range []struct {
		model            string
		platform         string
		adapter          string
		modality         string
		operation        string
		expectSizes      int
		expectReferences int
	}{
		{"sensenova-u1.5-lite", "openai", "sensenova", "image", "edit", 0, 4},
		{"sensenova-u1-fast", "openai", "sensenova", "image", "create", 11, 0},
		{"grok-imagine-video", "grok", "grok_video", "video", "generate", 2, -1},
		{"grok-imagine-video-1.5", "grok", "grok_video", "video", "generate", 3, -1},
		{"agnes-video-v2.0", "openai", "agnes_video", "video", "generate", 0, -1},
	} {
		t.Run(check.model, func(t *testing.T) {
			var adapter string
			var declaredModality, declaredOperation bool
			var supportedSizeCount, referenceLimit int
			require.NoError(t, db.QueryRowContext(ctx, `
				SELECT
					media_capabilities->>'adapter',
					(media_capabilities->'modalities') ? $3,
					(media_capabilities->$3->'operations') ? $4,
					COALESCE(jsonb_array_length(media_capabilities->'image'->'supported_sizes'), 0),
					COALESCE((media_capabilities->'image'->>'max_reference_images')::integer, -1)
				FROM site_model_catalog
				WHERE LOWER(model_name) = $1 AND LOWER(platform) = $2`,
				check.model, check.platform, check.modality, check.operation,
			).Scan(&adapter, &declaredModality, &declaredOperation, &supportedSizeCount, &referenceLimit))
			require.Equal(t, check.adapter, adapter)
			require.True(t, declaredModality)
			require.True(t, declaredOperation)
			if check.expectSizes > 0 {
				require.Equal(t, check.expectSizes, supportedSizeCount)
			}
			if check.expectReferences >= 0 {
				require.Equal(t, check.expectReferences, referenceLimit)
			}
		})
	}

	// An operator-approved declaration always wins over a migration replay.
	_, err = db.ExecContext(ctx, `
		UPDATE site_model_catalog
		SET media_capabilities = '{"version":"operator-v1","adapter":"grok_video","modalities":["video"],"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}}'::jsonb
		WHERE model_name = 'grok-imagine-video-1.5' AND platform = 'grok'`)
	require.NoError(t, err)
	require.NoError(t, applyModelCatalogMediaCapabilitiesMigration(ctx, db, migrationSQL))

	var version string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT media_capabilities->>'version'
		FROM site_model_catalog
		WHERE model_name = 'grok-imagine-video-1.5' AND platform = 'grok'`).Scan(&version))
	require.Equal(t, "operator-v1", version)

	var displayName, source string
	var visibleAuth bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT display_name, source, visible_auth
		FROM site_model_catalog
		WHERE model_name = 'SenseNova-U1.5-Lite' AND platform = 'openai'`).Scan(&displayName, &source, &visibleAuth))
	require.Equal(t, "existing row", displayName)
	require.Equal(t, "operator", source)
	require.True(t, visibleAuth)

	for _, model := range []string{
		"sensenova-u1.5-lite",
		"sensenova-u1-fast",
		"grok-imagine-video",
		"grok-imagine-video-1.5",
		"agnes-video-v2.0",
	} {
		var count int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM site_model_catalog
			WHERE LOWER(model_name) = $1`, model).Scan(&count))
		require.Equal(t, 1, count, model)
	}
}

func applyModelCatalogMediaCapabilitiesMigration(ctx context.Context, db *sql.DB, migrationSQL string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, migrationSQL); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
