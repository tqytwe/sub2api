//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"testing/fstest"
	"time"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestApplyMigrationsFS_ImageStudioOnlinePhasesDoNotCarryAlterLocks(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for migration integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"postgres:18.1-alpine3.23",
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
	db.SetMaxOpenConns(10)
	require.NoError(t, db.PingContext(ctx))

	t.Run("pooled unlock cannot release a lock held by another session", func(t *testing.T) {
		lockDB, err := sql.Open("postgres", dsn)
		require.NoError(t, err)
		t.Cleanup(func() { _ = lockDB.Close() })
		lockDB.SetMaxOpenConns(2)
		lockDB.SetMaxIdleConns(2)
		require.NoError(t, lockDB.PingContext(ctx))

		require.NoError(t, pgAdvisoryLock(ctx, lockDB))

		lockHolder, err := lockDB.Conn(ctx)
		require.NoError(t, err)
		defer func() { _ = lockHolder.Close() }()

		var holderOwnsLock bool
		require.NoError(t, lockHolder.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_locks
				WHERE locktype = 'advisory'
				  AND pid = pg_backend_pid()
				  AND granted
			)`,
		).Scan(&holderOwnsLock))
		require.True(t, holderOwnsLock, "test must reserve the session that acquired the lock")

		unlockErr := pgAdvisoryUnlock(ctx, lockDB)
		require.Error(t, unlockErr)
		require.Contains(t, unlockErr.Error(), "not held by this session")

		var stillLocked bool
		require.NoError(t, lockHolder.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_locks
				WHERE locktype = 'advisory'
				  AND pid = pg_backend_pid()
				  AND granted
			)`,
		).Scan(&stillLocked))
		require.True(t, stillLocked)

		var unlocked bool
		require.NoError(t, lockHolder.QueryRowContext(
			ctx,
			"SELECT pg_advisory_unlock($1)",
			migrationsAdvisoryLockID,
		).Scan(&unlocked))
		require.True(t, unlocked)
	})

	t.Run("192 backfill releases the preceding ALTER lock", func(t *testing.T) {
		const (
			migrationName = "192_image_studio_persistent_jobs.sql"
			tableName     = "migration_lock_backfill_probe"
			advisoryKey   = int64(8120192001)
		)

		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			CREATE TABLE %s (
				id INT PRIMARY KEY,
				marker INT NOT NULL
			);
			INSERT INTO %s (id, marker) VALUES (1, 1), (2, 2);`,
			tableName,
			tableName,
		))
		require.NoError(t, err)

		migrationSQL := fmt.Sprintf(`
			ALTER TABLE %s
				ADD COLUMN IF NOT EXISTS phase_value INT;

			WITH wait_for_test AS MATERIALIZED (
				SELECT pg_advisory_xact_lock(%d)
			)
			UPDATE %s
			SET phase_value = marker
			FROM wait_for_test
			WHERE id = 1;`,
			tableName,
			advisoryKey,
			tableName,
		)

		assertMigrationPhaseAllowsTargetReadsAndWrites(
			t,
			ctx,
			db,
			migrationName,
			migrationSQL,
			tableName,
			advisoryKey,
		)
	})

	t.Run("194 validation releases the preceding ALTER lock", func(t *testing.T) {
		const (
			migrationName = "194_image_studio_asset_derivatives.sql"
			tableName     = "migration_lock_validate_probe"
			functionName  = "migration_lock_validate_wait"
			constraint    = "migration_lock_validate_probe_chk"
			advisoryKey   = int64(8120194001)
		)

		_, err := db.ExecContext(ctx, fmt.Sprintf(`
			CREATE TABLE %s (
				id INT PRIMARY KEY,
				marker INT NOT NULL
			);
			INSERT INTO %s (id, marker) VALUES (1, 1), (2, 2);

			CREATE FUNCTION %s(probe_id INT)
			RETURNS BOOLEAN
			LANGUAGE plpgsql
			VOLATILE
			AS $$
			BEGIN
				IF probe_id = 1 THEN
					PERFORM pg_advisory_xact_lock(%d);
				END IF;
				RETURN TRUE;
			END;
			$$;`,
			tableName,
			tableName,
			functionName,
			advisoryKey,
		))
		require.NoError(t, err)

		migrationSQL := fmt.Sprintf(`
			ALTER TABLE %s
				ADD COLUMN IF NOT EXISTS phase_value INT;

			ALTER TABLE %s
				ADD CONSTRAINT %s
				CHECK (%s(id))
				NOT VALID;

			ALTER TABLE %s
				VALIDATE CONSTRAINT %s;`,
			tableName,
			tableName,
			constraint,
			functionName,
			tableName,
			constraint,
		)

		assertMigrationPhaseAllowsTargetReadsAndWrites(
			t,
			ctx,
			db,
			migrationName,
			migrationSQL,
			tableName,
			advisoryKey,
		)
	})

	t.Run("a partial phased migration retries without being recorded early", func(t *testing.T) {
		const migrationName = "192_image_studio_persistent_jobs.sql"

		_, err := db.ExecContext(ctx, `
			DELETE FROM schema_migrations
			WHERE filename = '192_image_studio_persistent_jobs.sql';
			DROP TABLE IF EXISTS migration_phase_retry_dependency;
			DROP TABLE IF EXISTS migration_phase_retry_marker;`)
		require.NoError(t, err)

		migrationSQL := `
			CREATE TABLE IF NOT EXISTS migration_phase_retry_marker (
				id INT PRIMARY KEY
			);

			INSERT INTO migration_phase_retry_marker (id)
			VALUES (1)
			ON CONFLICT (id) DO NOTHING;

			INSERT INTO migration_phase_retry_dependency (id)
			VALUES (1)
			ON CONFLICT (id) DO NOTHING;`
		fsys := fstest.MapFS{
			migrationName: &fstest.MapFile{Data: []byte(migrationSQL)},
		}

		err = applyMigrationsFS(ctx, db, fsys)
		require.Error(t, err)
		require.Contains(t, err.Error(), "phase 3")

		var markerRows int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM migration_phase_retry_marker`,
		).Scan(&markerRows))
		require.Equal(t, 1, markerRows)

		var migrationRows int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM schema_migrations
			WHERE filename = $1`,
			migrationName,
		).Scan(&migrationRows))
		require.Zero(t, migrationRows)

		_, err = db.ExecContext(ctx, `
			CREATE TABLE migration_phase_retry_dependency (
				id INT PRIMARY KEY
			)`)
		require.NoError(t, err)

		require.NoError(t, applyMigrationsFS(ctx, db, fsys))

		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM migration_phase_retry_dependency`,
		).Scan(&markerRows))
		require.Equal(t, 1, markerRows)
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM schema_migrations
			WHERE filename = $1`,
			migrationName,
		).Scan(&migrationRows))
		require.Equal(t, 1, migrationRows)
	})

	t.Run("the actual 192 and 194 files run and replay through the production runner", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			DELETE FROM schema_migrations
			WHERE filename IN (
				'192_image_studio_persistent_jobs.sql',
				'194_image_studio_asset_derivatives.sql'
			);

			DROP TABLE IF EXISTS image_studio_items;
			DROP TABLE IF EXISTS image_studio_assets;
			DROP TABLE IF EXISTS image_studio_jobs;

			CREATE TABLE image_studio_jobs (
				id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
				user_id BIGINT NOT NULL,
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

			INSERT INTO image_studio_jobs (
				id, user_id, template_id, prompt_hash, size, status
			)
			VALUES (
				'00000000-0000-0000-0000-000000000901',
				1, 'legacy', 'legacy-hash', '1024x1024', 'pending'
			);`)
		require.NoError(t, err)

		fsys := fstest.MapFS{}
		for _, name := range []string{
			imageStudioPersistentJobsMigration,
			imageStudioAssetDerivativesMigration,
		} {
			raw, err := dbmigrations.FS.ReadFile(name)
			require.NoError(t, err)
			fsys[name] = &fstest.MapFile{Data: raw}
		}

		require.NoError(t, applyMigrationsFS(ctx, db, fsys))
		require.NoError(t, applyMigrationsFS(ctx, db, fsys))

		var status string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT status
			FROM image_studio_jobs
			WHERE id = '00000000-0000-0000-0000-000000000901'`,
		).Scan(&status))
		require.Equal(t, "failed", status)

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

		var migrationRows int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM schema_migrations
			WHERE filename IN (
				'192_image_studio_persistent_jobs.sql',
				'194_image_studio_asset_derivatives.sql'
			)`,
		).Scan(&migrationRows))
		require.Equal(t, 2, migrationRows)
	})
}

func assertMigrationPhaseAllowsTargetReadsAndWrites(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	migrationName string,
	migrationSQL string,
	tableName string,
	advisoryKey int64,
) {
	t.Helper()

	blocker, err := db.Conn(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Close() }()
	_, err = blocker.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryKey)
	require.NoError(t, err)

	fsys := fstest.MapFS{
		migrationName: &fstest.MapFile{Data: []byte(migrationSQL)},
	}
	migrationResult := make(chan error, 1)
	go func() {
		migrationResult <- applyMigrationsFS(context.Background(), db, fsys)
	}()

	waitForMigrationAdvisoryWait(t, ctx, db, tableName)

	readCtx, cancelRead := context.WithTimeout(ctx, 750*time.Millisecond)
	var marker int
	readErr := db.QueryRowContext(
		readCtx,
		fmt.Sprintf("SELECT marker FROM %s WHERE id = 2", tableName),
	).Scan(&marker)
	cancelRead()

	writeCtx, cancelWrite := context.WithTimeout(ctx, 750*time.Millisecond)
	_, writeErr := db.ExecContext(
		writeCtx,
		fmt.Sprintf("UPDATE %s SET marker = marker + 1 WHERE id = 2", tableName),
	)
	cancelWrite()

	_, unlockErr := blocker.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", advisoryKey)
	require.NoError(t, unlockErr)

	select {
	case err := <-migrationResult:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("migration did not finish after releasing the phase blocker")
	}

	require.NoError(t, readErr, "ordinary SELECT was blocked by a preceding ALTER TABLE lock")
	require.Equal(t, 2, marker)
	require.NoError(t, writeErr, "ordinary UPDATE was blocked by a preceding ALTER TABLE lock")

	var applied int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM schema_migrations
		WHERE filename = $1`,
		migrationName,
	).Scan(&applied))
	require.Equal(t, 1, applied)
}

func waitForMigrationAdvisoryWait(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	tableName string,
) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_stat_activity
				WHERE datname = current_database()
				  AND pid <> pg_backend_pid()
				  AND state = 'active'
				  AND wait_event_type = 'Lock'
				  AND wait_event = 'advisory'
				  AND query ILIKE '%' || $1 || '%'
			)`,
			tableName,
		).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("migration for %s did not reach its blocked long-running phase", tableName)
}
