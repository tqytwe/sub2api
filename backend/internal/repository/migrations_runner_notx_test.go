package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestValidateMigrationExecutionMode(t *testing.T) {
	t.Run("事务迁移包含CONCURRENTLY会被拒绝", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx.sql", "CREATE INDEX CONCURRENTLY idx_a ON t(a);")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移要求CREATE使用IF NOT EXISTS", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "CREATE INDEX CONCURRENTLY idx_a ON t(a);")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移要求DROP使用IF EXISTS", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_drop_idx_notx.sql", "DROP INDEX CONCURRENTLY idx_a;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移禁止事务控制语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "BEGIN; CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a); COMMIT;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移禁止混用非CONCURRENTLY语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a); UPDATE t SET a = 1;")
		require.False(t, nonTx)
		require.Error(t, err)
	})

	t.Run("notx迁移允许幂等并发索引语句", func(t *testing.T) {
		nonTx, err := validateMigrationExecutionMode("001_add_idx_notx.sql", `
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_a ON t(a);
DROP INDEX CONCURRENTLY IF EXISTS idx_b;
`)
		require.True(t, nonTx)
		require.NoError(t, err)
	})
}

func TestSplitSQLStatementsPreservesQuotedAndCommentedSemicolons(t *testing.T) {
	statements := splitSQLStatements(`
-- a line comment with a semicolon;
COMMENT ON COLUMN image_studio_jobs.request_payload_encrypted IS
	'ciphertext; never plaintext';

SELECT "identifier;with:semicolon";

DO $migration$
BEGIN
	PERFORM 1;
	PERFORM 'value;inside';
END;
$migration$;

SELECT 2 /* outer; comment /* nested; comment */ still comment */;
`)

	require.Len(t, statements, 4)
	require.Contains(t, statements[0], "'ciphertext; never plaintext'")
	require.Contains(t, statements[1], `"identifier;with:semicolon"`)
	require.Contains(t, statements[2], "PERFORM 1;")
	require.Contains(t, statements[2], "'value;inside'")
	require.Contains(t, statements[3], "nested; comment")
}

func TestApplyMigrationsFS_NonTransactionalMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_idx_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t\\(a\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_idx_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"001_add_idx_notx.sql": &fstest.MapFile{
			Data: []byte("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t(a);"),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_NonTransactionalMigration_MultiStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_multi_idx_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t\\(a\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_b ON t\\(b\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_multi_idx_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"001_add_multi_idx_notx.sql": &fstest.MapFile{
			Data: []byte(`
-- first
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_a ON t(a);
-- second
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_t_b ON t(b);
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_NonTransactionalMigration_LatestAPIKeyIPIndexDropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(latestAPIKeyIPIndexMigration).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs(latestAPIKeyIPIndex).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_usage_logs_api_key_latest_ip").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_api_key_latest_ip").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs(latestAPIKeyIPIndexMigration, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		latestAPIKeyIPIndexMigration: &fstest.MapFile{
			Data: []byte(`
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_usage_logs_api_key_latest_ip
    ON usage_logs (api_key_id, created_at DESC, id DESC)
    INCLUDE (ip_address)
    WHERE ip_address IS NOT NULL AND ip_address <> '';
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_PaymentOrdersOutTradeNoUniqueMigration_FailsFastOnDuplicatePrecheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT out_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"out_trade_no", "duplicate_count"}).AddRow("dup-out-trade-no", 2))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"120_enforce_payment_orders_out_trade_no_unique_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique
    ON payment_orders (out_trade_no)
    WHERE out_trade_no <> '';

DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate out_trade_no")
	require.Contains(t, err.Error(), "dup-out-trade-no")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_PaymentOrdersOutTradeNoUniqueMigration_DropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT out_trade_no, COUNT\\(\\*\\) AS duplicate_count FROM payment_orders").
		WillReturnRows(sqlmock.NewRows([]string{"out_trade_no", "duplicate_count"}))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("paymentorder_out_trade_no_unique").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("120_enforce_payment_orders_out_trade_no_unique_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"120_enforce_payment_orders_out_trade_no_unique_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS paymentorder_out_trade_no_unique
    ON payment_orders (out_trade_no)
    WHERE out_trade_no <> '';

DROP INDEX CONCURRENTLY IF EXISTS paymentorder_out_trade_no;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_SchedulerOutboxPendingDedupKeyMigration_DropsInvalidIndexBeforeRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("153_scheduler_outbox_pending_dedup_key_index_notx.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("idx_scheduler_outbox_pending_dedup_key").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_scheduler_outbox_pending_dedup_key").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_pending_dedup_key").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("153_scheduler_outbox_pending_dedup_key_index_notx.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"153_scheduler_outbox_pending_dedup_key_index_notx.sql": &fstest.MapFile{
			Data: []byte(`
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_scheduler_outbox_pending_dedup_key
    ON scheduler_outbox (dedup_key)
    WHERE dedup_key IS NOT NULL;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrepareNonTransactionalMigration_ImageStudioIndexes(t *testing.T) {
	const (
		persistentJobsMigration   = "192_image_studio_persistent_jobs_indexes_notx.sql"
		assetDerivativesMigration = "194_image_studio_asset_derivatives_indexes_notx.sql"
	)

	t.Run("keeps healthy persistent job indexes", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		for _, indexName := range []string{
			"idx_image_studio_jobs_claim",
			"idx_image_studio_jobs_user_active",
			"uq_image_studio_jobs_user_idempotency",
			"idx_image_studio_items_job_status",
		} {
			mock.ExpectQuery("SELECT EXISTS \\(").
				WithArgs(indexName).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		}

		require.NoError(t, prepareNonTransactionalMigration(
			context.Background(),
			db,
			persistentJobsMigration,
		))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("drops only invalid persistent job indexes", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("idx_image_studio_jobs_claim").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("idx_image_studio_jobs_user_active").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_image_studio_jobs_user_active").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("uq_image_studio_jobs_user_idempotency").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("idx_image_studio_items_job_status").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_image_studio_items_job_status").
			WillReturnResult(sqlmock.NewResult(0, 0))

		require.NoError(t, prepareNonTransactionalMigration(
			context.Background(),
			db,
			persistentJobsMigration,
		))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("keeps healthy asset derivatives index", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("idx_image_studio_jobs_user_created_id").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		require.NoError(t, prepareNonTransactionalMigration(
			context.Background(),
			db,
			assetDerivativesMigration,
		))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("drops invalid asset derivatives index", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		mock.ExpectQuery("SELECT EXISTS \\(").
			WithArgs("idx_image_studio_jobs_user_created_id").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS idx_image_studio_jobs_user_created_id").
			WillReturnResult(sqlmock.NewResult(0, 0))

		require.NoError(t, prepareNonTransactionalMigration(
			context.Background(),
			db,
			assetDerivativesMigration,
		))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestApplyMigrationsFS_TransactionalMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	// The advisory lock and all migration work must share one session. This also
	// proves startup cannot self-deadlock when deployments cap the pool at one.
	db.SetMaxOpenConns(1)

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("001_add_col.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("ALTER TABLE t ADD COLUMN name TEXT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("001_add_col.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		"001_add_col.sql": &fstest.MapFile{
			Data: []byte("ALTER TABLE t ADD COLUMN name TEXT;"),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_RecoverablePhasedMigration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(imageStudioPersistentJobsMigration).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL lock_timeout = '5s'").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE image_studio_jobs ADD COLUMN IF NOT EXISTS lease_owner TEXT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL lock_timeout = '5s'").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE image_studio_jobs SET status = 'failed' WHERE status = 'running'").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs(imageStudioPersistentJobsMigration, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		imageStudioPersistentJobsMigration: &fstest.MapFile{
			Data: []byte(`
ALTER TABLE image_studio_jobs
	ADD COLUMN IF NOT EXISTS lease_owner TEXT;

UPDATE image_studio_jobs
SET status = 'failed'
WHERE status = 'running';
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyMigrationsFS_RecoverablePhasedMigrationDoesNotRecordPartialFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs(imageStudioAssetDerivativesMigration).
		WillReturnError(sql.ErrNoRows)

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL lock_timeout = '5s'").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE image_studio_assets ADD COLUMN IF NOT EXISTS width INT").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL lock_timeout = '5s'").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("ALTER TABLE image_studio_assets VALIDATE CONSTRAINT image_studio_assets_width_chk_upgrade").
		WillReturnError(errors.New("validation failed"))
	mock.ExpectRollback()

	expectMigrationsUnlock(mock)

	fsys := fstest.MapFS{
		imageStudioAssetDerivativesMigration: &fstest.MapFile{
			Data: []byte(`
ALTER TABLE image_studio_assets
	ADD COLUMN IF NOT EXISTS width INT;

ALTER TABLE image_studio_assets
	VALIDATE CONSTRAINT image_studio_assets_width_chk_upgrade;
`),
		},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.Error(t, err)
	require.Contains(t, err.Error(), "phase 2")
	require.Contains(t, err.Error(), "validation failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func prepareMigrationsBootstrapExpectations(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT pg_try_advisory_lock\\(\\$1\\)").
		WithArgs(migrationsAdvisoryLockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT EXISTS \\(").
		WithArgs("atlas_schema_revisions").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM atlas_schema_revisions").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
}

func expectMigrationsUnlock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT pg_advisory_unlock\\(\\$1\\)").
		WithArgs(migrationsAdvisoryLockID).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_unlock"}).AddRow(true))
}
