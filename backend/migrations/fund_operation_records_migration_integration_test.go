//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestFundOperationRecordsMigrationBackfillsAndReplaysPostgres(t *testing.T) {
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

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	_, err = db.ExecContext(ctx, `
		CREATE TABLE users (id BIGINT PRIMARY KEY);
		CREATE TABLE balance_transactions (
			id BIGINT PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id),
			actor_user_id BIGINT REFERENCES users(id),
			balance_delta NUMERIC(20,8) NOT NULL,
			source_type TEXT NOT NULL,
			metadata JSONB,
			description TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO users (id) VALUES (1), (2);
		INSERT INTO balance_transactions (id, user_id, actor_user_id, balance_delta, source_type, metadata, description) VALUES
			(1, 1, 2, 50.50, 'offline_recharge', '{"external_ref":"gateway-1001","reason":"verified wire"}', 'offline credit'),
			(2, 1, 2, 10.00, 'ops_gift', '{"reason":"campaign"}', 'gift'),
			(3, 1, 2, 5.00, 'compensation', '{"reason":"incident"}', 'compensation'),
			(4, 1, 2, 30.00, 'legacy_signup_gift', '{"reason":"legacy"}', 'excluded legacy gift'),
			(5, 1, 2, -1.00, 'ops_gift', '{"reason":"reversal"}', 'negative reversal');
	`)
	require.NoError(t, err)

	migration, err := dbmigrations.FS.ReadFile("270_fund_operation_records.sql")
	require.NoError(t, err)
	require.NoError(t, execFundOperationRecordsMigrationTwice(ctx, db, string(migration)))

	var count int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fund_operation_records").Scan(&count))
	require.Equal(t, 3, count, "only positive credits from the three operational source types are projected")

	var offlineKind, offlineRef string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT operation_kind, root_external_ref FROM fund_operation_records WHERE balance_transaction_id = 1`).Scan(&offlineKind, &offlineRef))
	require.Equal(t, "offline_recharge", offlineKind)
	require.Equal(t, "gateway-1001", offlineRef)

	_, err = db.ExecContext(ctx, `
		INSERT INTO fund_operation_records (operation_no, operation_kind, status, target_user_id, amount, root_external_ref)
		VALUES ('FM-DUPLICATE-REF', 'offline_recharge', 'completed', 1, 1, 'gateway-1001')
	`)
	requirePostgresErrorCode(t, err, "23505")

	var originalID int64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT id FROM fund_operation_records WHERE balance_transaction_id = 2").Scan(&originalID))
	_, err = db.ExecContext(ctx, `
		INSERT INTO fund_operation_corrections (correction_no, original_operation_id, corrected_user_id, amount, reason)
		VALUES ('FC-PENDING-001', $1, 2, 10, 'insufficient source balance')
	`, originalID)
	require.NoError(t, err)

	var correctionStatus string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT status FROM fund_operation_corrections WHERE correction_no = 'FC-PENDING-001'").Scan(&correctionStatus))
	require.Equal(t, "pending_insufficient_balance", correctionStatus)

	_, err = db.ExecContext(ctx, `
		INSERT INTO fund_operation_corrections (correction_no, original_operation_id, corrected_user_id, amount, status, reason)
		VALUES ('FC-INVALID-001', $1, 2, 1, 'pending', 'invalid status')
	`, originalID)
	requirePostgresErrorCode(t, err, "23514")
}

func execFundOperationRecordsMigrationTwice(ctx context.Context, db *sql.DB, migration string) error {
	if _, err := db.ExecContext(ctx, migration); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, migration)
	return err
}

func requirePostgresErrorCode(t *testing.T, err error, code pq.ErrorCode) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, code, pqErr.Code)
}
