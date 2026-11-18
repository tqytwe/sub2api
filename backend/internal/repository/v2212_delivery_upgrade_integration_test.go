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

func TestV2212DeliveryUpgradePreservesLegacyAccountsAndBilling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database := "sub2api_test_v2212_upgrade_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	endpoint.Path = "/" + database
	db, err = sql.Open("postgres", endpoint.String())
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))

	// Reproduce the deployed #434 schema, including its higher-numbered migrations.
	candidateMigrations := map[string]bool{
		"363_openai_auth_connections.sql":              true,
		"369_account_performance_monitor.sql":          true,
		"370_quality_shared_batches.sql":               true,
		"371_quality_oauth_401_streaks.sql":            true,
		"372_quality_oauth_401_trusted_revocation.sql": true,
	}
	files, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	baseline := fstest.MapFS{}
	for _, name := range files {
		if candidateMigrations[name] {
			continue
		}
		content, readErr := migrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		baseline[name] = &fstest.MapFile{Data: content}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, baseline))
	var userID, accountID, keyID, planID int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,balance)
 VALUES('v2212-upgrade@example.test','fixture-hash',42) RETURNING id`).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type,credentials,extra,schedulable)
 VALUES('v2212-upgrade','openai','oauth','{"access_token":"fixture-only-token","model_mapping":{"public":"upstream"}}',
 '{"custom_label":"preserved","openai_excel_bps":false}',false) RETURNING id`).Scan(&accountID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name)
 VALUES($1,'fixture-v2212-upgrade','legacy') RETURNING id`, userID).Scan(&keyID))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO scheduled_test_plans(account_id,model_id,enabled,pelican_config)
 VALUES($1,'fixture-model',false,'{"question_kind":"text","prompt":"fixture prompt","quality":{"action":"observe_only","expected_answer":"21"}}')
 RETURNING id`, accountID).Scan(&planID))
	_, err = db.ExecContext(ctx, `INSERT INTO scheduled_test_results(plan_id,status,response_text)
 VALUES($1,'success','fixture answer')`, planID)
	require.NoError(t, err)

	oldWriter := func(requestID string) {
		_, writeErr := db.ExecContext(ctx, `INSERT INTO usage_logs
 (user_id,api_key_id,account_id,request_id,model,input_tokens,total_cost,actual_cost,billed_cost,billing_surcharge_cost)
 VALUES($1,$2,$3,$4,'fixture-model',9,3,2,2.2,0.2)`, userID, keyID, accountID, requestID)
		require.NoError(t, writeErr)
	}
	legacySnapshot := func() string {
		var snapshot string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT jsonb_build_object(
 'balance',(SELECT balance FROM users WHERE id=$1),
 'credentials',(SELECT credentials FROM accounts WHERE id=$2),
 'extra',(SELECT extra FROM accounts WHERE id=$2),
 'schedulable',(SELECT schedulable FROM accounts WHERE id=$2),
 'plan',(SELECT to_jsonb(p) FROM scheduled_test_plans p WHERE id=$3),
 'result',(SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM scheduled_test_results r WHERE plan_id=$3)
 )::text`, userID, accountID, planID).Scan(&snapshot))
		return snapshot
	}
	oldWriter("v2212-before")
	before := legacySnapshot()
	for range 2 {
		require.NoError(t, ApplyMigrations(ctx, db), "upgrade and application restart must both succeed")
	}
	require.JSONEq(t, before, legacySnapshot(), "schema upgrades must preserve operational and billing state")
	oldWriter("v2212-after")
	var unchangedRows int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs
 WHERE account_id=$1 AND model='fixture-model' AND input_tokens=9 AND total_cost=3
 AND actual_cost=2 AND billed_cost=2.2 AND billing_surcharge_cost=0.2`, accountID).Scan(&unchangedRows))
	require.Equal(t, 2, unchangedRows, "old application writes retain the same charges")

	for _, table := range []string{
		"openai_auth_connections", "openai_auth_connection_policies",
		"openai_auth_connection_usage", "openai_auth_connection_affinity",
		"account_performance_monitors", "account_performance_events",
		"quality_probe_batches", "quality_probe_batch_runs", "quality_oauth_401_streaks",
	} {
		var exists bool
		require.NoError(t, db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists))
		require.True(t, exists, "missing delivered schema: %s", table)
		var rows int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM "+pq.QuoteIdentifier(table)).Scan(&rows))
		require.Zero(t, rows, "migration must not auto-enable policies, trigger calls or fabricate history: %s", table)
	}
}
