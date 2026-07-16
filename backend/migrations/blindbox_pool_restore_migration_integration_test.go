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
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestBlindboxPoolRestoreMigrationRunsTwiceWithoutLosingData(t *testing.T) {
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
		CREATE TABLE settings (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE play_blindbox_opens (
			id BIGSERIAL PRIMARY KEY,
			cost_amount DECIMAL(20, 8) NOT NULL,
			reward_amount DECIMAL(20, 8) NOT NULL,
			pool_version VARCHAR(64),
			open_source VARCHAR(24)
		);
		INSERT INTO settings (key, value)
		VALUES ('play_blindbox_pool_json', '{"version":"operator-custom"}');
		INSERT INTO play_blindbox_opens (cost_amount, reward_amount, pool_version, open_source)
		VALUES
			(0.5, 0.2, NULL, NULL),
			(0.5, 20, '   ', ' '),
			(1, 3, 'custom-v2', 'bonus');
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("190_restore_configurable_blindbox_pool.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)
	for range 2 {
		_, err = db.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)
	}

	var settingValue string
	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT value FROM settings WHERE key = 'play_blindbox_pool_json'`,
	).Scan(&settingValue))
	require.Equal(t, `{"version":"operator-custom"}`, settingValue)

	rows, err := db.QueryContext(ctx, `
		SELECT cost_amount::text, reward_amount::text, pool_version, open_source
		FROM play_blindbox_opens
		ORDER BY id`)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()

	type auditRow struct {
		cost, reward, poolVersion, openSource string
	}
	var got []auditRow
	for rows.Next() {
		var row auditRow
		require.NoError(t, rows.Scan(&row.cost, &row.reward, &row.poolVersion, &row.openSource))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []auditRow{
		{cost: "0.50000000", reward: "0.20000000", poolVersion: "legacy-v1", openSource: "paid"},
		{cost: "0.50000000", reward: "20.00000000", poolVersion: "legacy-v1", openSource: "paid"},
		{cost: "1.00000000", reward: "3.00000000", poolVersion: "custom-v2", openSource: "bonus"},
	}, got)

	for _, column := range []struct {
		name        string
		wantDefault string
	}{
		{name: "pool_version", wantDefault: "legacy-v1"},
		{name: "open_source", wantDefault: "paid"},
	} {
		var nullable, defaultValue string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT is_nullable, column_default
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'play_blindbox_opens'
			  AND column_name = $1`, column.name,
		).Scan(&nullable, &defaultValue))
		require.Equal(t, "NO", nullable)
		require.Contains(t, defaultValue, column.wantDefault)
	}

	_, err = db.ExecContext(ctx, `
		INSERT INTO play_blindbox_opens (cost_amount, reward_amount)
		VALUES (0.5, 0.05)`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO play_blindbox_opens (cost_amount, reward_amount, pool_version, open_source)
		VALUES (0.5, 0.05, NULL, NULL)`)
	require.Error(t, err)

	_, err = db.ExecContext(ctx, `DELETE FROM settings WHERE key = 'play_blindbox_pool_json'`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT value FROM settings WHERE key = 'play_blindbox_pool_json'`,
	).Scan(&settingValue))
	require.JSONEq(t, approvedBlindboxPoolJSON, strings.TrimSpace(settingValue))
}
