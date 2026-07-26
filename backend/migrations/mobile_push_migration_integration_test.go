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

func TestMobilePushMigrationUpgradesAndReplays(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for migration integration tests in CI")
		}
		t.Skip("Docker is unavailable")
	}
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("sub2api_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.Exec(`
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE mobile_push_outbox (
			id BIGSERIAL PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, dedupe_key_hash VARCHAR(64) NOT NULL UNIQUE,
			event_type VARCHAR(64) NOT NULL, source_type VARCHAR(64) NOT NULL DEFAULT '', source_id VARCHAR(128) NOT NULL DEFAULT '',
			title_zh VARCHAR(120) NOT NULL, body_zh VARCHAR(300) NOT NULL, data JSONB NOT NULL DEFAULT '{}'::jsonb,
			status VARCHAR(16) NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0, last_error_code VARCHAR(64),
			available_at TIMESTAMPTZ NOT NULL, sent_at TIMESTAMPTZ,
			CONSTRAINT mobile_push_outbox_status_check CHECK (status IN ('pending', 'sent', 'failed', 'skipped')),
			CONSTRAINT mobile_push_outbox_attempts_check CHECK (attempts >= 0)
		)
	`)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("221_mobile_push.sql")
	require.NoError(t, err)
	require.NoError(t, execMobilePushMigrationTwice(db, string(migration)))

	for _, column := range []string{"claim_token", "lease_expires_at"} {
		var count int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = 'mobile_push_outbox' AND column_name = $1`, column).Scan(&count))
		require.Equal(t, 1, count)
	}
	var deliveriesTable string
	require.NoError(t, db.QueryRow(`SELECT to_regclass('public.mobile_push_deliveries')::text`).Scan(&deliveriesTable))
	require.Equal(t, "mobile_push_deliveries", deliveriesTable)
}

func execMobilePushMigrationTwice(db *sql.DB, migration string) error {
	if _, err := db.Exec(migration); err != nil {
		return err
	}
	_, err := db.Exec(migration)
	return err
}
