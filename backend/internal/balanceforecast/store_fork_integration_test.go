//go:build integration

package balanceforecast

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T) (*Store, time.Time) {
	t.Helper()
	dsn := os.Getenv("SUB2API_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SUB2API_TEST_POSTGRES_DSN to the isolated test database")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", u.Hostname())
	require.Regexp(t, regexp.MustCompile(`^/sub2api_test_[a-z0-9_]+$`), u.Path)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	schema := fmt.Sprintf("balance_forecast_%d", time.Now().UnixNano())
	_, err = db.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	private, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, private.Close())
		_, cleanupErr := db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, cleanupErr)
		require.NoError(t, db.Close())
	})

	now := time.Now().UTC().Truncate(time.Second)
	_, err = private.Exec(`
		CREATE TABLE users (id bigint PRIMARY KEY, balance numeric(20,8) NOT NULL, created_at timestamptz NOT NULL,
		 deleted_at timestamptz, status text DEFAULT 'active', balance_notify_enabled boolean DEFAULT true);
		CREATE TABLE balance_forecast_coverage (id bool PRIMARY KEY, started_at timestamptz NOT NULL);
		CREATE TABLE balance_transactions (user_id bigint, source_type text, source_id text, metadata jsonb,
		 balance_delta numeric(20,8), frozen_delta numeric(20,8), is_backfilled bool DEFAULT false,
		 confidence text DEFAULT 'high', created_at timestamptz);
		CREATE TABLE usage_logs (user_id bigint, request_id text, api_key_id bigint, billing_type int,
		 actual_cost numeric(20,8), created_at timestamptz);`)
	require.NoError(t, err)
	_, err = private.Exec(`INSERT INTO balance_forecast_coverage VALUES (true, $1)`, now.Add(-4*time.Hour))
	require.NoError(t, err)
	_, err = private.Exec(`INSERT INTO users (id,balance,created_at) VALUES (1,30,$1)`, now.Add(-24*time.Hour))
	require.NoError(t, err)
	return &Store{DB: private}, now
}

func TestCandidatesUseSettledForkLedger(t *testing.T) {
	store, now := fixture(t)
	db := store.DB
	_, err := db.Exec(`INSERT INTO users (id,balance,created_at) VALUES (2,30,$1),(3,0,$1)`, now.Add(-24*time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO balance_transactions
		(user_id,source_type,source_id,metadata,balance_delta,frozen_delta,is_backfilled,created_at) VALUES
		(1,'usage_charge','regular','{"request_id":"regular","api_key_id":1}',-2,0,false,$1),
		(1,'image_balance_capture','batch-a','{"request_id":"batch-result","api_key_id":1}',3,-8,false,$2),
		(1,'video_balance_capture','video-a','{"request_id":"video-result","api_key_id":1}',-1,-4,false,$3),
		(1,'image_balance_hold','batch-only','{}',-20,20,false,$3),
		(1,'image_balance_release','batch-released','{}',10,-10,false,$3),
		(1,'manual_adjustment','manual','{}',-20,0,false,$3),
		(1,'usage_charge','historical-estimate','{}',-10,0,true,$3),
		(1,'usage_charge','before-coverage','{}',-40,0,false,$4),
		(1,'usage_charge','after-cutoff','{}',-40,0,false,$5),
		(2,'usage_charge','other-user','{}',-50,0,false,$2)`,
		now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour), now.Add(-5*time.Hour), now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs VALUES
		(1,'regular',1,0,2,$1),(1,'batch-result',1,0,5,$2),(1,'video-result',1,0,5,$3),
		(1,'subscription',1,1,99,$3)`, now.Add(-3*time.Hour), now.Add(-2*time.Hour), now.Add(-time.Hour))
	require.NoError(t, err)

	rows, err := store.Candidates(t.Context(), now, 0, 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.True(t, rows[0].Reliable())
	require.Equal(t, 3, rows[0].Samples)
	require.InDelta(t, 12, rows[0].Spent, 0.00000001)
	require.InDelta(t, 10, rows[0].Hours, 0.00000001)
	require.False(t, rows[1].Reliable())

	for _, requestID := range []string{"unsettled", "regular"} {
		t.Run(fmt.Sprintf("unproven charge %s", requestID), func(t *testing.T) {
			_, err := db.Exec(`INSERT INTO usage_logs VALUES (1,$1,2,0,500,$2)`, requestID, now.Add(-time.Hour))
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := db.Exec(`DELETE FROM usage_logs WHERE user_id=1 AND api_key_id=2 AND request_id=$1`, requestID)
				require.NoError(t, err)
			})
			rows, err := store.Candidates(t.Context(), now, 0, 10)
			require.NoError(t, err)
			require.False(t, rows[0].Reliable())
			require.Zero(t, rows[0].Hours)
		})
	}
}
