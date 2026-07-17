//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestImageStudioPersistentJobsIndexesMigrationRecoversInvalidIndex(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)

	raw, err := dbmigrations.FS.ReadFile(imageStudioPersistentJobsIndexesMigration)
	require.NoError(t, err)
	fsys := fstest.MapFS{
		imageStudioPersistentJobsIndexesMigration: &fstest.MapFile{Data: raw},
	}

	user, err := client.User.Create().
		SetEmail("image-studio-migration-retry-" + uuid.NewString() + "@example.test").
		SetPasswordHash("test-password-hash").
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(
			context.Background(),
			"DELETE FROM image_studio_jobs WHERE user_id = $1",
			user.ID,
		)
		_, _ = integrationDB.ExecContext(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		_, _ = integrationDB.ExecContext(
			context.Background(),
			"DELETE FROM schema_migrations WHERE filename = $1",
			imageStudioPersistentJobsIndexesMigration,
		)
		if cleanupErr := applyMigrationsFS(context.Background(), integrationDB, fsys); cleanupErr != nil {
			t.Errorf("restore image studio index migration: %v", cleanupErr)
		}
	})

	healthyIndexes := []string{
		"idx_image_studio_jobs_claim",
		"idx_image_studio_jobs_user_active",
		"idx_image_studio_items_job_status",
	}
	healthyOIDs := make(map[string]int64, len(healthyIndexes))
	for _, indexName := range healthyIndexes {
		healthyOIDs[indexName] = publicIndexOID(t, ctx, integrationDB, indexName)
	}

	_, err = integrationDB.ExecContext(
		ctx,
		"DROP INDEX CONCURRENTLY IF EXISTS uq_image_studio_jobs_user_idempotency",
	)
	require.NoError(t, err)

	firstJobID := uuid.New()
	secondJobID := uuid.New()
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO image_studio_jobs (
			id, user_id, template_id, prompt_hash, size, status,
			idempotency_key_hash, idempotency_fingerprint
		)
		VALUES
			($1, $3, 'migration-retry', 'hash-a', '1024x1024', 'completed',
			 'duplicate-key-hash', 'fingerprint-a'),
			($2, $3, 'migration-retry', 'hash-b', '1024x1024', 'completed',
			 'duplicate-key-hash', 'fingerprint-b')`,
		firstJobID,
		secondJobID,
		user.ID,
	)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
		CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uq_image_studio_jobs_user_idempotency
			ON image_studio_jobs(user_id, idempotency_key_hash)
			WHERE idempotency_key_hash IS NOT NULL`)
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, pq.ErrorCode("23505"), pqErr.Code)

	invalid, err := indexIsInvalid(ctx, integrationDB, "uq_image_studio_jobs_user_idempotency")
	require.NoError(t, err)
	require.True(t, invalid)
	invalidOID := publicIndexOID(t, ctx, integrationDB, "uq_image_studio_jobs_user_idempotency")

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE image_studio_jobs
		SET idempotency_key_hash = 'repaired-key-hash'
		WHERE id = $1`,
		secondJobID,
	)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(
		ctx,
		"DELETE FROM schema_migrations WHERE filename = $1",
		imageStudioPersistentJobsIndexesMigration,
	)
	require.NoError(t, err)

	require.NoError(t, applyMigrationsFS(ctx, integrationDB, fsys))

	invalid, err = indexIsInvalid(ctx, integrationDB, "uq_image_studio_jobs_user_idempotency")
	require.NoError(t, err)
	require.False(t, invalid)
	recreatedOID := publicIndexOID(t, ctx, integrationDB, "uq_image_studio_jobs_user_idempotency")
	require.NotEqual(t, invalidOID, recreatedOID)

	for _, indexName := range healthyIndexes {
		require.Equal(
			t,
			healthyOIDs[indexName],
			publicIndexOID(t, ctx, integrationDB, indexName),
			indexName,
		)
	}

	var valid, ready, unique bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT i.indisvalid, i.indisready, i.indisunique
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indexrelid
		WHERE c.relname = 'uq_image_studio_jobs_user_idempotency'`,
	).Scan(&valid, &ready, &unique))
	require.True(t, valid)
	require.True(t, ready)
	require.True(t, unique)

	var migrationRows int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE filename = $1`,
		imageStudioPersistentJobsIndexesMigration,
	).Scan(&migrationRows))
	require.Equal(t, 1, migrationRows)
}

func publicIndexOID(t *testing.T, ctx context.Context, db *sql.DB, indexName string) int64 {
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
