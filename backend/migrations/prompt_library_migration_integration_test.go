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

func TestPromptLibraryMigrationRunsTwiceAndEnforcesProvenance(t *testing.T) {
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

	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL UNIQUE
		);
		CREATE TABLE image_studio_jobs (
			id UUID PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id)
		);
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("192_prompt_library.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = db.ExecContext(ctx, string(raw))
		require.NoError(t, err)
	}

	for _, table := range []string{
		"prompt_categories", "prompts", "prompt_versions", "prompt_category_links",
		"prompt_media", "prompt_sources", "prompt_favorites", "prompt_use_events",
		"prompt_import_jobs", "prompt_import_items", "prompt_review_records", "prompt_reports",
	} {
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists))
		require.True(t, exists, table)
	}

	for _, column := range []string{"prompt_id", "prompt_version", "model", "quality"} {
		var nullable string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'image_studio_jobs'
			  AND column_name = $1`, column,
		).Scan(&nullable))
		require.Equal(t, "YES", nullable, column)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO prompts (
			title_zh, brand_type, provenance_type, authorization_status, source_evidence_verified
		) VALUES ('伪原创', 'original', 'external', 'unknown', false)
	`)
	require.Error(t, err)

	_, err = db.ExecContext(ctx, `
		WITH created_prompt AS (
			INSERT INTO prompts (
				title_zh, brand_type, provenance_type, authorization_status,
				source_evidence_verified
			) VALUES (
				'已证明原创', 'original', 'external', 'original', true
			)
			RETURNING id
		),
		created_version AS (
			INSERT INTO prompt_versions (
				prompt_id, version, brand_type, provenance_type, authorization_status,
				source_evidence_verified, title_zh, description_zh, prompt_text,
				models
			)
			SELECT id, 1, 'original', 'external', 'original', true,
			       '已证明原创', '已证明原创说明', 'original prompt',
			       ARRAY['gpt-image-1']::text[]
			FROM created_prompt
			RETURNING prompt_id
		)
		INSERT INTO prompt_sources (
			prompt_id, version, source_key, external_id, original_author,
			evidence, authorization_status, evidence_verified
		)
		SELECT prompt_id, 1, 'internal-proof', 'proof-1', '极速蹬内容团队',
		       jsonb_build_object(
		           'summary', '运营复核了原创制作记录',
		           'captured_at', '2026-07-17T04:00:00Z',
		           'proof_type', 'internal_creation_record'
		       ),
		       'original', true
		FROM created_version
	`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO prompts (
			title_zh, brand_type, provenance_type, authorization_status, source_evidence_verified
		) VALUES ('授权内容伪装原创', 'original', 'external', 'authorized', true)
	`)
	require.Error(t, err)

	var referenceRequirementExists bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'prompt_versions'
			  AND column_name = 'reference_requirement'
		)`).Scan(&referenceRequirementExists))
	require.True(t, referenceRequirementExists)

	_, err = db.ExecContext(ctx, `
		INSERT INTO prompt_import_jobs (source_key) VALUES ('source-a');
		INSERT INTO prompt_import_items (
			job_id, source_key, external_id, normalized_hash, normalized_payload
		) VALUES (1, 'source-a', 'external-1', 'hash-1', '{}'::jsonb);
	`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO prompt_import_items (
			job_id, source_key, external_id, normalized_hash, normalized_payload
		) VALUES (1, 'source-a', 'external-1', 'hash-2', '{}'::jsonb)
	`)
	require.Error(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO prompt_import_items (
			job_id, source_key, external_id, normalized_hash, normalized_payload
		) VALUES (1, 'source-b', 'external-2', 'hash-1', '{}'::jsonb)
	`)
	require.Error(t, err)
}

func TestPromptLibrarySeedMigrationIsIdempotentReviewOnly(t *testing.T) {
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

	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL UNIQUE
		);
		CREATE TABLE image_studio_jobs (
			id UUID PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id)
		);
	`)
	require.NoError(t, err)

	coreSQL, err := dbmigrations.FS.ReadFile("192_prompt_library.sql")
	require.NoError(t, err)
	seedSQL, err := dbmigrations.FS.ReadFile("193_prompt_library_seed.sql")
	require.NoError(t, err)
	require.NoError(t, execSQLTwice(ctx, db, string(coreSQL)))
	require.NoError(t, execSQLTwice(ctx, db, string(seedSQL)))

	var categoryCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_categories`).Scan(&categoryCount))
	require.GreaterOrEqual(t, categoryCount, 120)

	var jobCount int
	var itemCount int
	var pendingCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompt_import_jobs
		WHERE source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
	`).Scan(&jobCount))
	require.Equal(t, 1, jobCount)
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompt_import_items
		WHERE source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
	`).Scan(&itemCount))
	require.Equal(t, 200, itemCount)
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompt_import_items
		WHERE source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
		  AND status = 'pending_review'
		  AND authorization_status = 'curated'
	`).Scan(&pendingCount))
	require.Equal(t, 200, pendingCount)

	var missingCategoryCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompt_import_items item
		CROSS JOIN LATERAL (
			VALUES
				('purpose', item.normalized_payload->>'purpose'),
				('style', item.normalized_payload->>'style'),
				('subject', item.normalized_payload->>'subject'),
				('model', item.normalized_payload->'models'->>0),
				('size', item.normalized_payload->'sizes'->>0)
		) AS linked_categories(dimension, slug)
		WHERE item.source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
		  AND NOT EXISTS (
			SELECT 1
			FROM prompt_categories category
			WHERE category.dimension = linked_categories.dimension
			  AND category.slug = linked_categories.slug
		  )
	`).Scan(&missingCategoryCount))
	require.Equal(t, 0, missingCategoryCount)

	var promptCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompts`).Scan(&promptCount))
	require.Equal(t, 0, promptCount)
}

func execSQLTwice(ctx context.Context, db *sql.DB, sqlText string) error {
	for range 2 {
		if _, err := db.ExecContext(ctx, sqlText); err != nil {
			return err
		}
	}
	return nil
}
