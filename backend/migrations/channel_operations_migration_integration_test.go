//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestChannelOperationsMigrationPreservesRoutingAndBillingRelations(t *testing.T) {
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
		CREATE TABLE channels (id BIGINT PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE accounts (id BIGINT PRIMARY KEY, deleted_at TIMESTAMPTZ);
		CREATE TABLE groups (id BIGINT PRIMARY KEY);
		CREATE TABLE channel_groups (
			channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			PRIMARY KEY (channel_id, group_id)
		);
		CREATE TABLE account_groups (
			account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			PRIMARY KEY (account_id, group_id)
		);
		INSERT INTO channels (id, name) VALUES (1, 'fixture-channel'), (2, 'other-channel');
		INSERT INTO accounts (id) VALUES (10), (11);
		INSERT INTO groups (id) VALUES (20);
		INSERT INTO channel_groups (channel_id, group_id) VALUES (1, 20);
		INSERT INTO account_groups (account_id, group_id) VALUES (10, 20);
	`)
	require.NoError(t, err)

	raw, err := dbmigrations.FS.ReadFile("282_channel_operations.sql")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(raw))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, string(raw))
	require.NoError(t, err, "replay must be idempotent")

	var revision int64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT operations_revision FROM channels WHERE id=1").Scan(&revision))
	require.Zero(t, revision)

	_, err = db.ExecContext(ctx, `
		INSERT INTO channel_operation_upstreams (id, channel_id, name) VALUES (30, 1, 'fixture-upstream');
		INSERT INTO channel_operation_members (channel_id, account_id, upstream_id) VALUES (1, 10, 30);
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "INSERT INTO channel_operation_upstreams (channel_id, name) VALUES (1, '   ')")
	require.Error(t, err, "upstream names must not be blank")
	_, err = db.ExecContext(ctx, "INSERT INTO channel_operation_upstreams (channel_id, name) VALUES (1, 'fixture-upstream')")
	require.Error(t, err, "upstream names must be unique within a channel")
	_, err = db.ExecContext(ctx, "INSERT INTO channel_operation_members (channel_id, account_id, upstream_id) VALUES (1, 10, 30)")
	require.Error(t, err, "an account can have only one operational classification per channel")
	_, err = db.ExecContext(ctx, "INSERT INTO channel_operation_members (channel_id, account_id, upstream_id) VALUES (1, 99, 30)")
	require.Error(t, err, "classification members must reference an existing account")

	var originalMemberships, operationalMemberships int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM account_groups WHERE account_id=10 AND group_id=20").Scan(&originalMemberships))
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM channel_operation_members WHERE channel_id=1 AND account_id=10").Scan(&operationalMemberships))
	require.Equal(t, 1, originalMemberships)
	require.Equal(t, 1, operationalMemberships)

	_, err = db.ExecContext(ctx, "INSERT INTO channel_operation_members (channel_id, account_id, upstream_id) VALUES (2, 10, 30)")
	require.Error(t, err, "composite FK must prevent assigning an upstream to another channel")

	_, err = db.ExecContext(ctx, "DELETE FROM channels WHERE id=1")
	require.NoError(t, err)
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM channel_operation_upstreams").Scan(&operationalMemberships))
	require.Zero(t, operationalMemberships, "operational classifications must cascade with their channel")
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM account_groups WHERE account_id=10 AND group_id=20").Scan(&originalMemberships))
	require.Equal(t, 1, originalMemberships, "channel operations must not mutate scheduler membership")

}
