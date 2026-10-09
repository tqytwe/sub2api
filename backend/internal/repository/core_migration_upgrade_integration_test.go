//go:build integration

package repository

import (
	"context"
	"database/sql"
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

// Exercise the deployed schema plus populated legacy rows before applying the
// two additive core migrations. This database belongs to the integration harness,
// never to application configuration, and is removed when the test finishes.
func TestCoreMigrationUpgradePreservesDeployedDataAndOldWriter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	database := "sub2api_test_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quotedDatabase := pq.QuoteIdentifier(database)
	_, err := integrationDB.ExecContext(ctx, "CREATE DATABASE "+quotedDatabase+" TEMPLATE template0")
	require.NoError(t, err)
	var db *sql.DB
	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
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
	db, err = sql.Open("postgres", endpoint.String())
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))

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
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	var oldColumns int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.columns
 WHERE table_schema='public' AND ((table_name='account_groups' AND column_name='allowed_models')
 OR (table_name='usage_logs' AND column_name IN ('billing_request_fingerprint','billing_settled')))`).Scan(&oldColumns))
	require.Zero(t, oldColumns, "upgrade fixture must start at the actual deployed schema")

	var userID, accountID, groupID, keyID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance) VALUES('upgrade@example.test','test-hash',42) RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type) VALUES('upgrade-account','openai','apikey') RETURNING id`).Scan(&accountID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups(name) VALUES('upgrade-group') RETURNING id`).Scan(&groupID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name) VALUES($1,'upgrade-test-key','upgrade-key') RETURNING id`, userID).Scan(&keyID))
	_, err = db.ExecContext(ctx, `INSERT INTO account_groups(account_id,group_id,priority) VALUES($1,$2,17)`, accountID, groupID)
	require.NoError(t, err)
	oldWriter := func(requestID string) {
		_, writeErr := db.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,total_cost,actual_cost,billed_cost,billing_surcharge_cost)
 VALUES($1,$2,$3,$4,'upgrade-model',9,3,2,2.2,0.2)`, userID, keyID, accountID, requestID)
		require.NoError(t, writeErr)
	}
	oldWriter("before-upgrade")
	for range 2 {
		require.NoError(t, ApplyMigrations(ctx, db), "additive upgrade must be repeatable")
	}
	oldWriter("after-upgrade")
	var unchangedRows int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_logs WHERE model='upgrade-model'
 AND input_tokens=9 AND total_cost=3 AND actual_cost=2 AND billed_cost=2.2 AND billing_surcharge_cost=0.2
 AND billing_request_fingerprint IS NULL AND NOT billing_settled`).Scan(&unchangedRows))
	require.Equal(t, 2, unchangedRows, "legacy logs and old-binary writes remain unbound with their original charges")
	var unrestricted bool
	var priority int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT allowed_models IS NULL,priority FROM account_groups WHERE account_id=$1 AND group_id=$2`, accountID, groupID).Scan(&unrestricted, &priority))
	require.True(t, unrestricted)
	require.Equal(t, 17, priority)
	var balance float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, userID).Scan(&balance))
	require.Equal(t, 42.0, balance, "schema migration must never apply historical charges")
}
