//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAgentTeamSharedRewardsMigrationPostgreSQLContract(t *testing.T) {
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
	db.SetMaxOpenConns(8)

	bootstrapAgentTeamSharedRewardsSchema(t, ctx, db)

	raw, err := dbmigrations.FS.ReadFile("191_agent_team_shared_rewards.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)
	for range 2 {
		_, err = db.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)
	}

	t.Run("preserves memberships and operator settings", func(t *testing.T) {
		var activeMembers int
		require.NoError(t, db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM play_team_members WHERE user_id = 1 AND left_at IS NULL`,
		).Scan(&activeMembers))
		require.Equal(t, 1, activeMembers)

		expected := map[string]string{
			"play_team_shared_reward_enabled": "false",
			"play_team_shared_reward_tiers":   `[{"threshold":"50","rate":"0.10"}]`,
			"play_team_shared_reward_cap":     "999",
		}
		for key, want := range expected {
			var got string
			require.NoError(t, db.QueryRowContext(
				ctx,
				`SELECT value FROM settings WHERE key = $1`,
				key,
			).Scan(&got))
			require.Equal(t, want, got)
		}

		var startMonth, shanghaiMonth string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT
				(SELECT value FROM settings
				 WHERE key = 'play_team_shared_reward_start_month'),
				TO_CHAR(CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM')`,
		).Scan(&startMonth, &shanghaiMonth))
		require.Equal(t, shanghaiMonth, startMonth)
	})

	t.Run("enforces one active team and retains membership history", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, teamID := range []int64{1, 2} {
			wg.Add(1)
			go func(teamID int64) {
				defer wg.Done()
				<-start
				_, execErr := db.ExecContext(ctx, `
					INSERT INTO play_team_members (team_id, user_id)
					VALUES ($1, 2)`,
					teamID,
				)
				results <- execErr
			}(teamID)
		}
		close(start)
		wg.Wait()
		close(results)

		successes := 0
		uniqueFailures := 0
		for execErr := range results {
			if execErr == nil {
				successes++
				continue
			}
			requirePostgresCode(t, execErr, "23505")
			uniqueFailures++
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, uniqueFailures)

		var firstTeamID int64
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT team_id
			FROM play_team_members
			WHERE user_id = 2 AND left_at IS NULL`,
		).Scan(&firstTeamID))
		secondTeamID := int64(1)
		if firstTeamID == secondTeamID {
			secondTeamID = 2
		}

		_, err = db.ExecContext(ctx, `
			UPDATE play_team_members
			SET left_at = joined_at
			WHERE user_id = 2 AND left_at IS NULL`)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `
			INSERT INTO play_team_members (team_id, user_id)
			VALUES ($1, 2)`,
			secondTeamID,
		)
		require.NoError(t, err)

		var historyRows, activeRows int
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (WHERE left_at IS NULL)
			FROM play_team_members
			WHERE user_id = 2`,
		).Scan(&historyRows, &activeRows))
		require.Equal(t, 2, historyRows)
		require.Equal(t, 1, activeRows)

		_, err = db.ExecContext(ctx, `
			UPDATE play_team_members
			SET left_at = joined_at - INTERVAL '1 second'
			WHERE user_id = 1`)
		requirePostgresCode(t, err, "23514")
	})

	t.Run("rejects invalid settlement periods windows states and amounts", func(t *testing.T) {
		validSettlementID := insertValidTeamSettlement(t, ctx, db)

		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
			)
			VALUES (
				1, DATE '2026-06-02',
				TIMESTAMPTZ '2026-06-02 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-07-02 00:00:00 Asia/Shanghai',
				30, 20, 0.02, 0.6, 250, 'pending'
			)`,
			"23514",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
			)
			VALUES (
				2, DATE '2026-05-01',
				TIMESTAMPTZ '2026-05-01 00:00:00 UTC',
				TIMESTAMPTZ '2026-06-01 00:00:00 UTC',
				30, 20, 0.02, 0.6, 250, 'pending'
			)`,
			"23514",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
			)
			VALUES (
				2, DATE '2026-04-01',
				TIMESTAMPTZ '2026-04-01 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-05-01 00:00:00 Asia/Shanghai',
				30, 20, 0.02, 0.6, 250, 'unknown'
			)`,
			"23514",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
			)
			VALUES (
				2, DATE '2026-03-01',
				TIMESTAMPTZ '2026-03-01 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-04-01 00:00:00 Asia/Shanghai',
				1000000000000, 20, 0.02, 0.6, 250, 'pending'
			)`,
			"22003",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount,
				status, processing_started_at
			)
			VALUES (
				2, DATE '2026-02-01',
				TIMESTAMPTZ '2026-02-01 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-03-01 00:00:00 Asia/Shanghai',
				30, 20, 0.02, 0.6, 250, 'completed', NOW()
			)`,
			"23514",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount,
				status, completed_at
			)
			VALUES (
				2, DATE '2026-01-01',
				TIMESTAMPTZ '2026-01-01 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-02-01 00:00:00 Asia/Shanghai',
				30, 20, 0.02, 0.6, 250, 'pending', NOW()
			)`,
			"23514",
		)
		requireSettlementRejected(t, ctx, db, `
			INSERT INTO play_team_settlements (
				team_id, period_start, window_start, window_end,
				team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
			)
			VALUES (
				2, DATE '2025-12-01',
				TIMESTAMPTZ '2025-12-01 00:00:00 Asia/Shanghai',
				TIMESTAMPTZ '2026-01-01 00:00:00 Asia/Shanghai',
				30, 20, 0.02, 0.6, 250, 'processing'
			)`,
			"23514",
		)

		_, err := db.ExecContext(ctx, `
			INSERT INTO play_team_reward_allocations (
				settlement_id, user_id, contribution, ratio, reward_amount,
				payout_status, idempotency_key
			)
			VALUES ($1, 1, 30, 1, 0.6, 'unknown', 'team-reward:invalid-status')`,
			validSettlementID,
		)
		requirePostgresCode(t, err, "23514")
		_, err = db.ExecContext(ctx, `
			INSERT INTO play_team_reward_allocations (
				settlement_id, user_id, contribution, ratio, reward_amount,
				payout_status, idempotency_key
			)
			VALUES ($1, 1, 30, 1, 0.6, 'paid', 'team-reward:paid-without-time')`,
			validSettlementID,
		)
		requirePostgresCode(t, err, "23514")
		_, err = db.ExecContext(ctx, `
			INSERT INTO play_team_reward_allocations (
				settlement_id, user_id, contribution, ratio, reward_amount,
				payout_status, idempotency_key, paid_at
			)
			VALUES ($1, 1, 30, 1, 0.6, 'pending', 'team-reward:pending-with-time', NOW())`,
			validSettlementID,
		)
		requirePostgresCode(t, err, "23514")
	})

	t.Run("supports indexed user reward history", func(t *testing.T) {
		var indexDefinition string
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT indexdef
			FROM pg_indexes
			WHERE schemaname = 'public'
			  AND indexname = 'idx_play_team_reward_allocations_user_settlement'`,
		).Scan(&indexDefinition))
		require.Contains(t, indexDefinition, "(user_id, settlement_id DESC)")

		conn, err := db.Conn(ctx)
		require.NoError(t, err)
		defer func() { require.NoError(t, conn.Close()) }()
		_, err = conn.ExecContext(ctx, `SET enable_seqscan = off`)
		require.NoError(t, err)
		rows, err := conn.QueryContext(ctx, `
			EXPLAIN (COSTS OFF)
			SELECT settlement_id, reward_amount, payout_status
			FROM play_team_reward_allocations
			WHERE user_id = 1
			ORDER BY settlement_id DESC`)
		require.NoError(t, err)
		defer func() { require.NoError(t, rows.Close()) }()

		var planLines []string
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			planLines = append(planLines, line)
		}
		require.NoError(t, rows.Err())
		require.Contains(
			t,
			strings.Join(planLines, "\n"),
			"idx_play_team_reward_allocations_user_settlement",
		)
	})
}

func bootstrapAgentTeamSharedRewardsSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	_, err := db.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY
		);
		CREATE TABLE settings (
			key VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL
		);
		CREATE TABLE play_teams (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(64) NOT NULL,
			captain_user_id BIGINT NOT NULL REFERENCES users(id),
			invite_code VARCHAR(16) NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE play_team_members (
			id BIGSERIAL PRIMARY KEY,
			team_id BIGINT NOT NULL REFERENCES play_teams(id) ON DELETE CASCADE,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_play_team_members_user UNIQUE (user_id),
			CONSTRAINT uq_play_team_members_team_user UNIQUE (team_id, user_id)
		);
		INSERT INTO users (id) VALUES (1), (2), (3), (4);
		SELECT SETVAL(pg_get_serial_sequence('users', 'id'), 4);
		INSERT INTO play_teams (id, name, captain_user_id, invite_code)
		VALUES
			(1, 'existing-team', 1, 'EXISTING'),
			(2, 'second-team', 3, 'SECOND');
		SELECT SETVAL(pg_get_serial_sequence('play_teams', 'id'), 2);
		INSERT INTO play_team_members (team_id, user_id)
		VALUES (1, 1);
		INSERT INTO settings (key, value)
		VALUES
			('play_team_shared_reward_enabled', 'false'),
			('play_team_shared_reward_tiers', '[{"threshold":"50","rate":"0.10"}]'),
			('play_team_shared_reward_cap', '999');
	`)
	require.NoError(t, err)
}

func insertValidTeamSettlement(t *testing.T, ctx context.Context, db *sql.DB) int64 {
	t.Helper()

	var settlementID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO play_team_settlements (
			team_id, period_start, window_start, window_end,
			team_spend, reached_threshold, reward_rate, pool_amount, cap_amount, status
		)
		VALUES (
			1, DATE '2026-06-01',
			TIMESTAMPTZ '2026-06-01 00:00:00 Asia/Shanghai',
			TIMESTAMPTZ '2026-07-01 00:00:00 Asia/Shanghai',
			30, 20, 0.02, 0.6, 250, 'pending'
		)
		RETURNING id`,
	).Scan(&settlementID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
		INSERT INTO play_team_reward_allocations (
			settlement_id, user_id, contribution, ratio, reward_amount,
			payout_status, idempotency_key
		)
		VALUES ($1, 1, 30, 1, 0.6, 'pending', 'team-reward:1:2026-06:1')`,
		settlementID,
	)
	require.NoError(t, err)
	return settlementID
}

func requireSettlementRejected(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	query string,
	code pq.ErrorCode,
) {
	t.Helper()

	_, err := db.ExecContext(ctx, query)
	requirePostgresCode(t, err, code)
}

func requirePostgresCode(t *testing.T, err error, code pq.ErrorCode) {
	t.Helper()

	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, code, pqErr.Code)
}
