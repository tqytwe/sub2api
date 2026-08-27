//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"
	"time"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestChannelMonitorV2RecomputeRangePostgres(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			require.NoError(t, err, "Docker must be available for repository integration tests in CI")
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
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, createChannelMonitorV2AggregationTestSchema(ctx, db))

	for _, migrationName := range []string{"194_channel_monitor_v2.sql", "199_channel_monitor_v2_fixed_rollups.sql"} {
		migration, err := dbmigrations.FS.ReadFile(migrationName)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}

	start := time.Now().UTC().Truncate(time.Minute).Add(-2 * time.Minute)
	end := start.Add(time.Minute)
	_, err = db.ExecContext(ctx, `
		INSERT INTO groups (id, platform) VALUES (7, 'composite');
		INSERT INTO accounts (id, platform) VALUES (9, 'openai');
		INSERT INTO usage_logs (
			id, created_at, group_id, account_id, requested_model, model, request_id,
			request_type, actual_cost, input_tokens, output_tokens, cache_creation_tokens,
			cache_read_tokens, first_token_ms, duration_ms, user_id
		) VALUES (
			1, $1, 7, 9, 'gpt-5', 'gpt-5', 'success-1', 1, 0.01, 10, 20, 0, 0, 100, 300, 42
		)
	`, start.Add(10*time.Second))
	require.NoError(t, err)

	repo := &channelMonitorV2Repository{db: db}
	// There are no candidate error IDs yet. This executes the affected CTE and
	// proves an empty error window commits instead of rolling back the aggregate.
	require.NoError(t, repo.RecomputeRange(ctx, start, end))

	var lastSuccessfulAt sql.NullTime
	var metrics, histograms int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT last_successful_at FROM channel_monitor_v2_watermarks WHERE id = 1`).Scan(&lastSuccessfulAt))
	require.True(t, lastSuccessfulAt.Valid)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_monitor_v2_metrics_1m WHERE platform = 'openai'`).Scan(&metrics))
	require.Equal(t, 1, metrics)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_monitor_v2_latency_histograms_1m WHERE platform = 'openai'`).Scan(&histograms))
	require.Greater(t, histograms, 0)

	_, err = db.ExecContext(ctx, `
		INSERT INTO ops_error_logs (
			id, created_at, request_id, group_id, account_id, requested_model, model, user_id,
			error_type, error_owner, status_code, upstream_status_code, platform, error_source,
			error_message, upstream_error_message, upstream_error_detail, error_body, upstream_errors, is_count_tokens
		) VALUES (
			1, $1, 'error-1', 7, 9, 'gpt-5', 'gpt-5', 42,
			'upstream', 'provider', 502, 502, 'composite', 'provider',
			'upstream request failed', '', '', '', '[]'::jsonb, FALSE
		)
	`, start.Add(20*time.Second))
	require.NoError(t, err)

	require.NoError(t, repo.RecomputeRange(ctx, start, end))
	var errors int
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT error_requests
		FROM channel_monitor_v2_error_metrics_1m
		WHERE platform = 'openai' AND group_id = 7 AND model = 'gpt-5'
	`).Scan(&errors))
	require.Equal(t, 1, errors)
}

func createChannelMonitorV2AggregationTestSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE groups (id BIGINT PRIMARY KEY, platform TEXT);
		CREATE TABLE accounts (id BIGINT PRIMARY KEY, platform TEXT);
		CREATE TABLE usage_logs (
			id BIGINT PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL, group_id BIGINT,
			account_id BIGINT, requested_model TEXT, model TEXT, request_id TEXT,
			request_type INTEGER, actual_cost NUMERIC NOT NULL DEFAULT 0,
			input_tokens BIGINT, output_tokens BIGINT, cache_creation_tokens BIGINT,
			cache_read_tokens BIGINT, first_token_ms BIGINT, duration_ms BIGINT, user_id BIGINT
		);
		CREATE TABLE ops_error_logs (
			id BIGINT PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL, request_id TEXT,
			group_id BIGINT, account_id BIGINT, requested_model TEXT, model TEXT, user_id BIGINT,
			error_type TEXT NOT NULL, error_owner TEXT, status_code INTEGER, upstream_status_code INTEGER,
			platform TEXT, error_source TEXT, error_message TEXT, upstream_error_message TEXT,
			upstream_error_detail TEXT, error_body TEXT, upstream_errors JSONB,
			is_count_tokens BOOLEAN NOT NULL DEFAULT FALSE
		);
	`)
	return err
}
