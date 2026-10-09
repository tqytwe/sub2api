//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestCoreMigrationsBoundLockWaitAndRollback(t *testing.T) {
	for _, migration := range []struct {
		name    string
		table   string
		columns int
	}{
		{"273_account_group_allowed_models.sql", "account_groups", 1},
		{"274_usage_log_billing_reconciliation.sql", "usage_logs", 2},
	} {
		t.Run(migration.name, func(t *testing.T) {
			content, err := migrations.FS.ReadFile(migration.name)
			require.NoError(t, err)
			fsys := fstest.MapFS{migration.name: &fstest.MapFile{Data: content}}
			for _, stage := range []struct {
				name       string
				lock       string
				errorStage string
			}{
				{"target_table", "LOCK TABLE " + pq.QuoteIdentifier(migration.table) + " IN ACCESS SHARE MODE", "apply migration "},
				{"migration_history", "LOCK TABLE schema_migrations IN SHARE MODE", "record migration "},
			} {
				t.Run(stage.name, func(t *testing.T) {
					runner, blocker := newCoreMigrationLockTestDB(t)
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					seedCoreMigrationLockLegacyRows(t, ctx, runner)
					legacy := coreMigrationLockLegacySnapshot(t, ctx, runner)
					assertCoreMigrationLockColumnCount(t, ctx, runner, 0)
					assertCoreMigrationLockHistoryCount(t, ctx, runner, 0)

					// Distinct session values expose leaks after both rollback and commit.
					// The runner pool retains exactly one physical connection throughout.
					_, err := runner.ExecContext(ctx, "SET lock_timeout = '17s'; SET statement_timeout = '19s'")
					require.NoError(t, err)
					before := coreMigrationLockSession(t, ctx, runner)
					require.Equal(t, "17s", before.lockTimeout)
					require.Equal(t, "19s", before.statementTimeout)
					var blockerPID int
					require.NoError(t, blocker.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
					require.NotEqual(t, before.pid, blockerPID)

					tx, err := blocker.BeginTx(ctx, nil)
					require.NoError(t, err)
					defer func() { _ = tx.Rollback() }()
					_, err = tx.ExecContext(ctx, stage.lock)
					require.NoError(t, err)

					// The outer deadline is only a safety net: PostgreSQL must return
					// lock_not_available well before it, preserving the same session.
					attemptCtx, attemptCancel := context.WithTimeout(context.Background(), 10*time.Second)
					started := time.Now()
					migrationErr := applyMigrationsFS(attemptCtx, runner, fsys)
					elapsed := time.Since(started)
					attemptCancel()
					// Release locks before any result assertion can terminate the test.
					releaseErr := tx.Rollback()
					require.NoError(t, releaseErr)
					require.Error(t, migrationErr)
					require.Contains(t, migrationErr.Error(), stage.errorStage+migration.name)
					var pgErr *pq.Error
					require.ErrorAs(t, migrationErr, &pgErr)
					require.Equal(t, pq.ErrorCode("55P03"), pgErr.Code, "must fail on the migration's lock_timeout, not the safety deadline")
					require.GreaterOrEqual(t, elapsed, 1500*time.Millisecond)
					require.Less(t, elapsed, 6*time.Second)
					t.Logf("%s timed out with SQLSTATE %s after %s", stage.errorStage+migration.name, pgErr.Code, elapsed)
					require.Equal(t, before, coreMigrationLockSession(t, ctx, runner), "rollback must restore session timeouts without replacing the connection")
					assertCoreMigrationLockColumnCount(t, ctx, runner, 0)
					assertCoreMigrationLockHistoryCount(t, ctx, runner, 0)
					require.Equal(t, legacy, coreMigrationLockLegacySnapshot(t, ctx, runner))

					// In the history case, reaching 'record migration' proves the DDL
					// succeeded before the INSERT blocked; the zero columns above prove
					// that both operations rolled back as one transaction.
					for attempt := range 2 {
						retryCtx, retryCancel := context.WithTimeout(context.Background(), 10*time.Second)
						retryErr := applyMigrationsFS(retryCtx, runner, fsys)
						retryCancel()
						require.NoError(t, retryErr, "retry/replay attempt %d", attempt+1)
						require.Equal(t, before, coreMigrationLockSession(t, ctx, runner), "commit/replay must preserve session timeouts and connection identity")
						assertCoreMigrationLockColumnCount(t, ctx, runner, migration.columns)
						assertCoreMigrationLockHistoryCount(t, ctx, runner, 1)
						var checksum string
						require.NoError(t, runner.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename=$1", migration.name).Scan(&checksum))
						sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
						require.Equal(t, hex.EncodeToString(sum[:]), checksum)
						require.Equal(t, legacy, coreMigrationLockLegacySnapshot(t, ctx, runner), "migration and replay must preserve all legacy row values")
					}
					var unchangedDefaults int
					if migration.table == "account_groups" {
						err = runner.QueryRowContext(ctx, "SELECT COUNT(*) FROM account_groups WHERE allowed_models IS NULL").Scan(&unchangedDefaults)
					} else {
						err = runner.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE billing_request_fingerprint IS NULL AND NOT billing_settled").Scan(&unchangedDefaults)
					}
					require.NoError(t, err)
					require.Equal(t, 1, unchangedDefaults)
				})
			}
		})
	}
}

// Each case starts from the actual embedded pre-273 history in its own database.
// The integration harness supplies the disposable server, never application DSNs.
func newCoreMigrationLockTestDB(t *testing.T) (*sql.DB, *sql.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	database := "sub2api_test_core_lock_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedDatabase := pq.QuoteIdentifier(database)
	_, err := integrationDB.ExecContext(ctx, "CREATE DATABASE "+quotedDatabase+" TEMPLATE template0")
	require.NoError(t, err)
	var runner, blocker *sql.DB
	t.Cleanup(func() {
		if blocker != nil {
			_ = blocker.Close()
		}
		if runner != nil {
			_ = runner.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, "DROP DATABASE "+quotedDatabase)
		require.NoError(t, cleanupErr)
	})
	endpoint, err := url.Parse(integrationDSN)
	require.NoError(t, err)
	require.Equal(t, "postgres", endpoint.Scheme)
	endpoint.Path = "/" + database
	runner, err = sql.Open("postgres", endpoint.String())
	require.NoError(t, err)
	runner.SetMaxOpenConns(1)
	runner.SetMaxIdleConns(1)
	blocker, err = sql.Open("postgres", endpoint.String())
	require.NoError(t, err)
	blocker.SetMaxOpenConns(1)
	blocker.SetMaxIdleConns(1)

	files, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	baseline := fstest.MapFS{}
	for _, name := range files {
		if name >= "273_" {
			continue
		}
		content, readErr := migrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		baseline[name] = &fstest.MapFile{Data: content}
	}
	require.NoError(t, applyMigrationsFS(ctx, runner, baseline))
	return runner, blocker
}

func seedCoreMigrationLockLegacyRows(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var userID, accountID, groupID, keyID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES('core-lock@example.test','test-hash',42) RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type) VALUES('core-lock-account','openai','apikey') RETURNING id`).Scan(&accountID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups(name) VALUES('core-lock-group') RETURNING id`).Scan(&groupID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name) VALUES($1,'core-lock-test-key','core-lock-key') RETURNING id`, userID).Scan(&keyID))
	_, err := db.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,17)`, accountID, groupID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,total_cost,actual_cost,billed_cost,billing_surcharge_cost)
 VALUES($1,$2,$3,'before-core-lock-upgrade','core-lock-model',9,3,2,2.2,0.2)`, userID, keyID, accountID)
	require.NoError(t, err)
}

func coreMigrationLockLegacySnapshot(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()
	var snapshot string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT jsonb_build_object(
 'users', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM users r),
 'accounts', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM accounts r),
 'groups', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM groups r),
 'api_keys', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM api_keys r),
 'account_groups', (SELECT jsonb_agg(to_jsonb(r) - 'allowed_models' ORDER BY account_id,group_id) FROM account_groups r),
 'usage_logs', (SELECT jsonb_agg(to_jsonb(r) - 'billing_request_fingerprint' - 'billing_settled' ORDER BY id) FROM usage_logs r)
)::text`).Scan(&snapshot))
	return snapshot
}

type coreMigrationLockSessionState struct {
	pid              int
	lockTimeout      string
	statementTimeout string
}

func coreMigrationLockSession(t *testing.T, ctx context.Context, db *sql.DB) coreMigrationLockSessionState {
	t.Helper()
	var state coreMigrationLockSessionState
	require.NoError(t, db.QueryRowContext(ctx, `SELECT pg_backend_pid(), current_setting('lock_timeout'), current_setting('statement_timeout')`).Scan(&state.pid, &state.lockTimeout, &state.statementTimeout))
	return state
}

func assertCoreMigrationLockColumnCount(t *testing.T, ctx context.Context, db *sql.DB, want int) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
 WHERE table_schema='public' AND ((table_name='account_groups' AND column_name='allowed_models')
 OR (table_name='usage_logs' AND column_name IN ('billing_request_fingerprint','billing_settled')))`).Scan(&count))
	require.Equal(t, want, count)
}

func assertCoreMigrationLockHistoryCount(t *testing.T, ctx context.Context, db *sql.DB, want int) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations
 WHERE filename IN ('273_account_group_allowed_models.sql','274_usage_log_billing_reconciliation.sql')`).Scan(&count))
	require.Equal(t, want, count)
}
