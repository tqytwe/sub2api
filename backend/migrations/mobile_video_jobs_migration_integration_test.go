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

func TestMobileVideoJobsMigrationUpgradesAndReplaysPostgres(t *testing.T) {
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
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE mobile_tasks (
			id UUID PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			kind VARCHAR(16) NOT NULL,
			CONSTRAINT mobile_tasks_kind_check CHECK (kind IN ('chat', 'image', 'file'))
		);
		INSERT INTO users (id) VALUES (7);
		INSERT INTO mobile_tasks (id, user_id, kind)
		VALUES ('00000000-0000-0000-0000-000000000001', 7, 'chat');
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("259_mobile_video_jobs.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)
	require.NoError(t, applyMobileVideoJobsMigration(ctx, db, migrationSQL))

	var chatTasks int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mobile_tasks WHERE kind = 'chat'`).Scan(&chatTasks))
	require.Equal(t, 1, chatTasks)

	_, err = db.ExecContext(ctx, `
		INSERT INTO mobile_tasks (id, user_id, kind)
		VALUES ('00000000-0000-0000-0000-000000000002', 7, 'video');
		INSERT INTO mobile_video_jobs (
			task_id, user_id, group_id, execution_api_key_id, adapter, model,
			prompt, resolution, ratio, duration_seconds, reference_asset_ids
		) VALUES (
			'00000000-0000-0000-0000-000000000002', 7, 19, 29, 'grok_video', 'grok-imagine-video',
			'private prompt', '720p', '16:9', 8, '[]'::jsonb
		);
	`)
	require.NoError(t, err)
	require.NoError(t, applyMobileVideoJobsMigration(ctx, db, migrationSQL), "replay must preserve existing video rows")

	billingRaw, err := dbmigrations.FS.ReadFile("261_mobile_video_execution_billing.sql")
	require.NoError(t, err)
	billingMigrationSQL := string(billingRaw)
	require.NoError(t, applyMobileVideoJobsMigration(ctx, db, billingMigrationSQL))
	require.NoError(t, applyMobileVideoJobsMigration(ctx, db, billingMigrationSQL), "replay must preserve existing video rows and constraints")

	var videoJobs int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mobile_video_jobs`).Scan(&videoJobs))
	require.Equal(t, 1, videoJobs)

	var billingState string
	var snapshot sql.NullString
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT billing_state, execution_snapshot::text
		FROM mobile_video_jobs
		WHERE task_id = '00000000-0000-0000-0000-000000000002'
	`).Scan(&billingState, &snapshot))
	require.Equal(t, "funding", billingState)
	require.False(t, snapshot.Valid, "legacy rows must not receive a fabricated execution snapshot")

	// The migration must keep the exact legacy all-NULL funding shape so it
	// can upgrade a database that already has queued video jobs. It must still
	// reject a partial pricing row, which could never be settled safely.
	_, err = db.ExecContext(ctx, `
		INSERT INTO mobile_tasks (id, user_id, kind)
		VALUES ('00000000-0000-0000-0000-000000000003', 7, 'video')
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO mobile_video_jobs (
			task_id, user_id, group_id, execution_api_key_id, adapter, model,
			prompt, resolution, duration_seconds, reference_asset_ids, unit_price_usd
		) VALUES (
			'00000000-0000-0000-0000-000000000003', 7, 19, 29, 'grok_video', 'grok-imagine-video',
			'private prompt', '720p', 8, '[]'::jsonb, 0.12
		)
	`)
	require.Error(t, err, "partial funding rows must not bypass the immutable snapshot constraint")

	var definition string
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid = 'mobile_tasks'::regclass AND conname = 'mobile_tasks_kind_check'
	`).Scan(&definition))
	require.Contains(t, definition, "'video'")

	for _, indexName := range []string{"idx_mobile_video_jobs_claim", "idx_mobile_video_jobs_user_created", "idx_mobile_video_jobs_funding_recovery", "idx_mobile_video_jobs_releasing_recovery", "idx_mobile_video_jobs_user_active"} {
		var count int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM pg_indexes
			WHERE schemaname = 'public' AND tablename = 'mobile_video_jobs' AND indexname = $1
		`, indexName).Scan(&count))
		require.Equal(t, 1, count, indexName)
	}
}

func applyMobileVideoJobsMigration(ctx context.Context, db *sql.DB, migrationSQL string) error {
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
