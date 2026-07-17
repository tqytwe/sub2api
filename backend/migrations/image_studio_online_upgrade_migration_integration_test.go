//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"strings"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

var imageStudioOnlineUpgradeFiles = []string{
	"192_image_studio_persistent_jobs.sql",
	"192_image_studio_persistent_jobs_indexes_notx.sql",
	"193_image_studio_references.sql",
	"194_image_studio_asset_derivatives.sql",
	"194_image_studio_asset_derivatives_indexes_notx.sql",
	"195_image_studio_billing_reconciliation.sql",
	"196_image_studio_job_references.sql",
	"197_image_studio_object_deletions.sql",
	"198_image_studio_upload_slots.sql",
}

func TestImageStudioOnlineMigrationsUpgradeLegacyPostgreSQL(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for migration integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"pgvector/pgvector:pg16",
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

	bootstrapLegacyImageStudioSchema(t, ctx, db)
	applyImageStudioOnlineUpgrade(t, ctx, db)

	t.Run("preserves legacy rows and makes active jobs recoverable", func(t *testing.T) {
		rows, err := db.QueryContext(ctx, `
			SELECT id::text, status, COALESCE(error_message, ''), finished_at IS NOT NULL
			FROM image_studio_jobs
			ORDER BY created_at, id`)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()

		type jobRow struct {
			id, status, error string
			finished          bool
		}
		var got []jobRow
		for rows.Next() {
			var row jobRow
			require.NoError(t, rows.Scan(&row.id, &row.status, &row.error, &row.finished))
			got = append(got, row)
		}
		require.NoError(t, rows.Err())
		require.Equal(t, []jobRow{
			{
				id:       "00000000-0000-0000-0000-000000000101",
				status:   "failed",
				error:    "legacy image studio job cannot be resumed after durable worker upgrade",
				finished: true,
			},
			{
				id:       "00000000-0000-0000-0000-000000000102",
				status:   "failed",
				error:    "legacy worker stopped",
				finished: true,
			},
			{
				id:     "00000000-0000-0000-0000-000000000103",
				status: "completed",
			},
		}, got)

		var assetCount int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM image_studio_assets
			WHERE id IN (
				'00000000-0000-0000-0000-000000000201',
				'00000000-0000-0000-0000-000000000202'
			)`).Scan(&assetCount))
		require.Equal(t, 2, assetCount)

		var templateID, promptHash, storageKey string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT j.template_id, j.prompt_hash, a.storage_key
			FROM image_studio_jobs j
			JOIN image_studio_assets a ON a.job_id = j.id
			WHERE a.id = '00000000-0000-0000-0000-000000000202'`,
		).Scan(&templateID, &promptHash, &storageKey))
		require.Equal(t, "legacy-template", templateID)
		require.Equal(t, "legacy-hash", promptHash)
		require.Equal(t, "legacy/private.webp", storageKey)
	})

	t.Run("creates and validates durable constraints", func(t *testing.T) {
		for _, constraint := range []string{
			"chk_image_studio_jobs_status",
			"image_studio_jobs_active_payload_chk",
			"image_studio_assets_width_chk",
			"image_studio_assets_height_chk",
			"image_studio_assets_thumbnail_size_chk",
			"image_studio_assets_dimensions_pair_chk",
			"image_studio_assets_thumbnail_pair_chk",
		} {
			var validated bool
			require.NoError(t, db.QueryRowContext(ctx, `
				SELECT convalidated
				FROM pg_constraint
				WHERE conname = $1`,
				constraint,
			).Scan(&validated), constraint)
			require.True(t, validated, constraint)
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO image_studio_jobs (
				id, user_id, template_id, size, status
			)
			VALUES (
				'00000000-0000-0000-0000-000000000104',
				1, 'invalid-active', '1024x1024', 'pending'
			)`)
		requirePostgresMigrationCode(t, err, "23514")

		_, err = db.ExecContext(ctx, `
			INSERT INTO image_studio_jobs (
				id, user_id, template_id, size, status, request_payload_encrypted
			)
			VALUES (
				'00000000-0000-0000-0000-000000000105',
				1, 'valid-active', '1024x1024', 'pending', 'ciphertext'
			)`)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx, `
			UPDATE image_studio_jobs
			SET status = 'partial'
			WHERE id = '00000000-0000-0000-0000-000000000103'`)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx, `
			UPDATE image_studio_jobs
			SET status = 'unknown'
			WHERE id = '00000000-0000-0000-0000-000000000103'`)
		requirePostgresMigrationCode(t, err, "23514")

		_, err = db.ExecContext(ctx, `
			UPDATE image_studio_assets
			SET width = 1024, height = NULL
			WHERE id = '00000000-0000-0000-0000-000000000201'`)
		requirePostgresMigrationCode(t, err, "23514")

		_, err = db.ExecContext(ctx, `
			UPDATE image_studio_assets
			SET thumbnail_storage_key = 'thumb.webp'
			WHERE id = '00000000-0000-0000-0000-000000000201'`)
		requirePostgresMigrationCode(t, err, "23514")
	})

	t.Run("creates valid concurrent indexes", func(t *testing.T) {
		for _, index := range []string{
			"idx_image_studio_jobs_claim",
			"idx_image_studio_jobs_user_active",
			"uq_image_studio_jobs_user_idempotency",
			"idx_image_studio_items_job_status",
			"idx_image_studio_jobs_user_created_id",
		} {
			var valid, ready bool
			require.NoError(t, db.QueryRowContext(ctx, `
				SELECT i.indisvalid, i.indisready
				FROM pg_index i
				JOIN pg_class c ON c.oid = i.indexrelid
				WHERE c.relname = $1`,
				index,
			).Scan(&valid, &ready), index)
			require.True(t, valid, index)
			require.True(t, ready, index)
		}
	})

	t.Run("replaying phased migrations preserves validated constraints", func(t *testing.T) {
		for _, name := range []string{
			"192_image_studio_persistent_jobs.sql",
			"194_image_studio_asset_derivatives.sql",
		} {
			applyImageStudioPhasedMigration(t, ctx, db, name)
		}

		for _, constraint := range []string{
			"chk_image_studio_jobs_status",
			"image_studio_jobs_active_payload_chk",
			"image_studio_assets_width_chk",
			"image_studio_assets_height_chk",
			"image_studio_assets_thumbnail_size_chk",
			"image_studio_assets_dimensions_pair_chk",
			"image_studio_assets_thumbnail_pair_chk",
		} {
			var validated bool
			require.NoError(t, db.QueryRowContext(ctx, `
				SELECT convalidated
				FROM pg_constraint
				WHERE conname = $1`,
				constraint,
			).Scan(&validated), constraint)
			require.True(t, validated, constraint)
		}
	})

	t.Run("replaying notx migrations preserves healthy index identities", func(t *testing.T) {
		indexNames := []string{
			"idx_image_studio_jobs_claim",
			"idx_image_studio_jobs_user_active",
			"uq_image_studio_jobs_user_idempotency",
			"idx_image_studio_items_job_status",
			"idx_image_studio_jobs_user_created_id",
		}
		before := make(map[string]int64, len(indexNames))
		for _, indexName := range indexNames {
			before[indexName] = imageStudioIndexOID(t, ctx, db, indexName)
		}

		applyImageStudioNonTransactionalMigration(
			t,
			ctx,
			db,
			"192_image_studio_persistent_jobs_indexes_notx.sql",
		)
		applyImageStudioNonTransactionalMigration(
			t,
			ctx,
			db,
			"194_image_studio_asset_derivatives_indexes_notx.sql",
		)

		for _, indexName := range indexNames {
			require.Equal(
				t,
				before[indexName],
				imageStudioIndexOID(t, ctx, db, indexName),
				indexName,
			)
		}
	})

	t.Run("retains migrations 193 through 198 and their constraints", func(t *testing.T) {
		for _, table := range []string{
			"image_studio_items",
			"image_studio_references",
			"image_studio_billing_reconciliations",
			"image_studio_job_references",
			"image_studio_object_deletions",
			"image_studio_upload_slots",
		} {
			var exists bool
			require.NoError(t, db.QueryRowContext(ctx, `
				SELECT to_regclass('public.' || $1) IS NOT NULL`,
				table,
			).Scan(&exists))
			require.True(t, exists, table)
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO image_studio_items (
				id, job_id, sort_order, status
			)
			VALUES (
				'00000000-0000-0000-0000-000000000301',
				'00000000-0000-0000-0000-000000000105',
				0, 'persisting'
			)`)
		require.NoError(t, err)

		_, err = db.ExecContext(ctx, `
			INSERT INTO image_studio_upload_slots (
				id, user_id, started_at, lease_expires_at
			)
			VALUES (
				'00000000-0000-0000-0000-000000000401',
				1, NOW(), NOW() - INTERVAL '1 second'
			)`)
		requirePostgresMigrationCode(t, err, "23514")
	})
}

func bootstrapLegacyImageStudioSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE
		);

		CREATE TABLE image_studio_jobs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			template_id TEXT NOT NULL,
			prompt_hash TEXT NOT NULL DEFAULT '',
			size TEXT NOT NULL,
			count INT NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'pending',
			estimated_cost DECIMAL(20, 8),
			actual_cost DECIMAL(20, 8),
			api_key_id BIGINT,
			error_message TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ,
			CONSTRAINT chk_image_studio_jobs_status
				CHECK (status IN ('pending', 'running', 'completed', 'failed'))
		);

		CREATE INDEX idx_image_studio_jobs_user_created
			ON image_studio_jobs(user_id, created_at DESC);

		CREATE TABLE image_studio_assets (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			job_id UUID NOT NULL REFERENCES image_studio_jobs(id) ON DELETE CASCADE,
			url TEXT,
			sort_order INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			storage_key TEXT,
			content_type TEXT,
			byte_size BIGINT NOT NULL DEFAULT 0,
			CONSTRAINT image_studio_assets_url_or_storage_key_chk CHECK (
				(url IS NOT NULL AND btrim(url) <> '')
				OR (storage_key IS NOT NULL AND btrim(storage_key) <> '')
			)
		);

		CREATE INDEX idx_image_studio_assets_job
			ON image_studio_assets(job_id, sort_order);

		INSERT INTO users (id, email)
		VALUES (1, 'legacy-image-studio@example.test');

		INSERT INTO image_studio_jobs (
			id, user_id, template_id, prompt_hash, size, status,
			error_message, created_at
		)
		VALUES
			(
				'00000000-0000-0000-0000-000000000101',
				1, 'legacy-template', 'legacy-hash', '1024x1024',
				'pending', NULL, TIMESTAMPTZ '2026-07-01 00:00:01 UTC'
			),
			(
				'00000000-0000-0000-0000-000000000102',
				1, 'legacy-template', 'legacy-hash', '1024x1024',
				'running', 'legacy worker stopped', TIMESTAMPTZ '2026-07-01 00:00:02 UTC'
			),
			(
				'00000000-0000-0000-0000-000000000103',
				1, 'legacy-template', 'legacy-hash', '1024x1024',
				'completed', NULL, TIMESTAMPTZ '2026-07-01 00:00:03 UTC'
			);

		INSERT INTO image_studio_assets (
			id, job_id, url, sort_order, storage_key, content_type, byte_size
		)
		VALUES
			(
				'00000000-0000-0000-0000-000000000201',
				'00000000-0000-0000-0000-000000000103',
				'https://example.test/legacy.png', 0, NULL, NULL, 0
			),
			(
				'00000000-0000-0000-0000-000000000202',
				'00000000-0000-0000-0000-000000000103',
				NULL, 1, 'legacy/private.webp', 'image/webp', 128
			);
	`)
	require.NoError(t, err)
}

func applyImageStudioOnlineUpgrade(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, name := range imageStudioOnlineUpgradeFiles {
		raw, err := dbmigrations.FS.ReadFile(name)
		require.NoError(t, err, name)
		content := strings.TrimSpace(string(raw))

		if strings.HasSuffix(name, "_notx.sql") {
			applyImageStudioNonTransactionalMigration(t, ctx, db, name)
			continue
		}
		if name == "192_image_studio_persistent_jobs.sql" ||
			name == "194_image_studio_asset_derivatives.sql" {
			applyImageStudioPhasedMigration(t, ctx, db, name)
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err, name)
		_, err = tx.ExecContext(ctx, content)
		if err != nil {
			_ = tx.Rollback()
			require.NoError(t, err, name)
		}
		require.NoError(t, tx.Commit(), name)
	}
}

func applyImageStudioPhasedMigration(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	name string,
) {
	t.Helper()
	raw, err := dbmigrations.FS.ReadFile(name)
	require.NoError(t, err, name)
	for _, statement := range strings.Split(strings.TrimSpace(string(raw)), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err, name)
		_, err = tx.ExecContext(ctx, "SET LOCAL lock_timeout = '5s'")
		if err == nil {
			_, err = tx.ExecContext(ctx, statement)
		}
		if err != nil {
			_ = tx.Rollback()
			require.NoError(t, err, name)
		}
		require.NoError(t, tx.Commit(), name)
	}
}

func applyImageStudioNonTransactionalMigration(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	name string,
) {
	t.Helper()
	raw, err := dbmigrations.FS.ReadFile(name)
	require.NoError(t, err, name)
	for _, statement := range strings.Split(strings.TrimSpace(string(raw)), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		_, err = db.ExecContext(ctx, statement)
		require.NoError(t, err, name)
	}
}

func imageStudioIndexOID(t *testing.T, ctx context.Context, db *sql.DB, indexName string) int64 {
	t.Helper()
	var oid int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT c.oid::bigint
		FROM pg_class c
		JOIN pg_namespace ns ON ns.oid = c.relnamespace
		WHERE ns.nspname = 'public'
		  AND c.relname = $1`,
		indexName,
	).Scan(&oid))
	return oid
}

func requirePostgresMigrationCode(t *testing.T, err error, code pq.ErrorCode) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.ErrorAs(t, err, &pqErr)
	require.Equal(t, code, pqErr.Code)
}
