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
			email VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX users_email_unique_active
			ON users(email)
			WHERE deleted_at IS NULL;
		CREATE TABLE image_studio_jobs (
			id UUID PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id)
		);
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("199_prompt_library.sql")
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

func TestPromptLibraryPublicSeedMigrationIsIdempotentAndPublished(t *testing.T) {
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
			email VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'user',
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX users_email_unique_active
			ON users(email)
			WHERE deleted_at IS NULL;
		CREATE TABLE image_studio_jobs (
			id UUID PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id)
		);
	`)
	require.NoError(t, err)

	coreSQL, err := dbmigrations.FS.ReadFile("199_prompt_library.sql")
	require.NoError(t, err)
	seedSQL, err := dbmigrations.FS.ReadFile("200_prompt_library_seed.sql")
	require.NoError(t, err)
	publicSeedSQL, err := dbmigrations.FS.ReadFile("201_prompt_library_public_seed.sql")
	require.NoError(t, err)
	require.NoError(t, execSQLTwice(ctx, db, string(coreSQL)))
	require.NoError(t, execSQLTwice(ctx, db, string(seedSQL)))
	require.NoError(t, execSQLTwice(ctx, db, string(publicSeedSQL)))

	var categoryCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_categories`).Scan(&categoryCount))
	require.GreaterOrEqual(t, categoryCount, 120)

	var jobCount int
	var itemCount int
	var approvedCount int
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
		  AND status = 'approved'
		  AND authorization_status = 'curated'
		  AND prompt_id IS NOT NULL
	`).Scan(&approvedCount))
	require.Equal(t, 200, approvedCount)

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
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompts
		WHERE status = 'published'
		  AND published_version = 1
		  AND brand_type = 'curated'
		  AND provenance_type = 'external'
		  AND authorization_status = 'curated'
		  AND source_evidence_verified = TRUE
	`).Scan(&promptCount))
	require.Equal(t, 200, promptCount)

	var originalCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prompts WHERE brand_type = 'original'
	`).Scan(&originalCount))
	require.Equal(t, 0, originalCount)

	var featuredCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM prompts WHERE featured = TRUE
	`).Scan(&featuredCount))
	require.Equal(t, 24, featuredCount)

	var missingPublishedVersionCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompts p
		WHERE p.status = 'published'
		  AND NOT EXISTS (
			SELECT 1
			FROM prompt_versions v
			WHERE v.prompt_id = p.id
			  AND v.version = p.published_version
			  AND v.brand_type = 'curated'
			  AND v.prompt_text <> ''
			  AND array_length(v.models, 1) >= 1
			  AND array_length(v.sizes, 1) >= 1
			  AND v.public_attribution_note LIKE '%极速蹬整理、翻译并完成模型适配%'
		  )
	`).Scan(&missingPublishedVersionCount))
	require.Equal(t, 0, missingPublishedVersionCount)

	for name, query := range map[string]string{
		"category links": `
			SELECT COUNT(*)
			FROM prompts p
			WHERE p.status = 'published'
			  AND (
				SELECT COUNT(*)
				FROM prompt_category_links link
				WHERE link.prompt_id = p.id
				  AND link.version = p.published_version
			  ) < 5`,
		"media": `
			SELECT COUNT(*)
			FROM prompts p
			WHERE p.status = 'published'
			  AND NOT EXISTS (
				SELECT 1 FROM prompt_media media
				WHERE media.prompt_id = p.id
				  AND media.version = p.published_version
			  )`,
		"sources": `
			SELECT COUNT(*)
			FROM prompts p
			WHERE p.status = 'published'
			  AND NOT EXISTS (
				SELECT 1 FROM prompt_sources source
				WHERE source.prompt_id = p.id
				  AND source.version = p.published_version
				  AND source.authorization_status = 'curated'
				  AND source.evidence_verified = TRUE
			  )`,
		"reviews": `
			SELECT COUNT(*)
			FROM prompts p
			WHERE p.status = 'published'
			  AND NOT EXISTS (
				SELECT 1 FROM prompt_review_records review
				WHERE review.prompt_id = p.id
				  AND review.version = p.published_version
				  AND review.decision = 'approve'
			  )`,
	} {
		var missing int
		require.NoError(t, db.QueryRowContext(ctx, query).Scan(&missing), name)
		require.Equal(t, 0, missing, name)
	}

	var publicListCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompts p
		JOIN prompt_versions v
		  ON v.prompt_id = p.id
		 AND v.version = p.published_version
		WHERE p.status = 'published'
	`).Scan(&publicListCount))
	require.Equal(t, 200, publicListCount)

	var completedJobCount int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM prompt_import_jobs
		WHERE source_key = 'jisudeng-gpt-image-2-curated-seed-20260717'
		  AND status = 'completed'
		  AND item_count = 200
	`).Scan(&completedJobCount))
	require.Equal(t, 1, completedJobCount)
}

func execSQLTwice(ctx context.Context, db *sql.DB, sqlText string) error {
	for range 2 {
		if _, err := db.ExecContext(ctx, sqlText); err != nil {
			return err
		}
	}
	return nil
}
