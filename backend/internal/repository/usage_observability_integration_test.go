//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Exercise the real report and page enrichment against Fork schemas, including
// multiple HTTP requests that share one idempotent billed usage record.
func TestUsageObservabilityReportAndContentIsolation(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	repo := &usageLogRepository{sql: tx}
	now := time.Now().UTC()
	var userID, keyID, accountID, usageID int64
	suffix := uuid.NewString()
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, suffix+"@example.test").Scan(&userID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'diagnostic key') RETURNING id`, userID, "sk-"+suffix).Scan(&keyID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type) VALUES('diagnostic account','openai','apikey') RETURNING id`).Scan(&accountID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,input_tokens,cache_read_tokens,cache_creation_tokens,created_at) VALUES($1,$2,$3,$4,'gpt-fixture',0,900,100,$5) RETURNING id`, userID, keyID, accountID, "gateway:"+uuid.NewString(), now).Scan(&usageID))
	end := float64(200)
	snapshot := requesttiming.Snapshot{
		Version: 1, TraceID: uuid.NewString(), StartedAt: now,
		BodyBytes: 1024, TotalMS: 250, FinalResponseAttempt: 1,
		TTFTMode: "semantic", Events: map[string]float64{"first_semantic": 100},
		Spans: []requesttiming.Span{{Name: "forward_attempt", StartMS: 25, EndMS: 200}},
		Attempts: []requesttiming.Attempt{
			{Number: 1, Kind: "egress", AccountID: accountID},
			{Number: 2, Parent: 1, Kind: "http", AccountID: accountID, StartMS: 0, EndMS: &end, Events: map[string]float64{
				"headers_written": 10, "request_written": 20, "first_byte": 50,
				"first_semantic": 100, "first_visible": 120, "first_visible_text": 120,
			}},
		},
	}
	insertTrace := func(s requesttiming.Snapshot, created time.Time) {
		raw, err := json.Marshal(s)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO request_timing_details(usage_log_id,trace_id,created_at,detail) VALUES($1,$2,$3,$4::jsonb)`, usageID, s.TraceID, created, string(raw))
		require.NoError(t, err)
	}
	insertTrace(snapshot, now)
	content, err := repo.ReadUsageContentTimings(ctx, []int64{usageID})
	require.NoError(t, err)
	require.Equal(t, "observed", content[usageID].Status)
	require.NotNil(t, content[usageID].VisibleTextFirstMS)
	require.Equal(t, float64(120), *content[usageID].VisibleTextFirstMS)
	observations, err := repo.ReadUsageTimingObservations(ctx, []int64{usageID})
	require.NoError(t, err)
	require.NotNil(t, observations[usageID].Queue)
	require.Equal(t, int64(250), observations[usageID].Queue.ServerDurationMs)
	require.Equal(t, int64(175), observations[usageID].Queue.ForwardDurationMs)
	require.Equal(t, int64(75), *observations[usageID].Queue.ForwardFirstTokenMs)

	f := service.LatencyReportFilter{Start: now.Add(-time.Hour), End: now.Add(time.Hour), UserID: userID, APIKeyID: keyID}
	groups, err := repo.ReadLatencyReport(ctx, f)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(1), groups[0].Requests)
	require.Equal(t, int64(1), groups[0].Traces)
	require.Equal(t, int64(1), groups[0].Attempts, "logical egress parents must not double count physical HTTP samples")
	require.Equal(t, "80-95%", groups[0].CacheBucket)
	require.NotNil(t, groups[0].UploadAvgMS)
	require.Equal(t, float64(10), *groups[0].UploadAvgMS)
	require.Equal(t, float64(80), *groups[0].WaitAvgMS)
	require.Equal(t, float64(120), *groups[0].TextAvgMS)

	// Another real HTTP trace sharing a usage row must stay separate in the
	// report and details; its per-row summary is deliberately ambiguous.
	snapshot.TraceID = uuid.NewString()
	insertTrace(snapshot, now)
	content, err = repo.ReadUsageContentTimings(ctx, []int64{usageID})
	require.NoError(t, err)
	require.Equal(t, "not_collected", content[usageID].Status)
	require.Nil(t, content[usageID].VisibleTextFirstMS)
	observations, err = repo.ReadUsageTimingObservations(ctx, []int64{usageID})
	require.NoError(t, err)
	require.Nil(t, observations[usageID].Queue)
	groups, err = repo.ReadLatencyReport(ctx, f)
	require.NoError(t, err)
	require.Equal(t, int64(1), groups[0].Requests)
	require.Equal(t, int64(2), groups[0].Traces)
	require.Equal(t, int64(2), groups[0].Attempts)
	f.UserID++
	groups, err = repo.ReadLatencyReport(ctx, f)
	require.NoError(t, err)
	require.Empty(t, groups)

	// Retention is also applied while reading, independently of whether the
	// hourly diagnostic writer has started since this process was deployed.
	_, err = tx.ExecContext(ctx, `UPDATE request_timing_details SET created_at=$1 WHERE usage_log_id=$2`, now.Add(-31*24*time.Hour), usageID)
	require.NoError(t, err)
	traces, err := repo.RequestTimings(ctx, usageID)
	require.NoError(t, err)
	require.Empty(t, traces)
	content, err = repo.ReadUsageContentTimings(ctx, []int64{usageID})
	require.NoError(t, err)
	require.Equal(t, "not_collected", content[usageID].Status)
	observations, err = repo.ReadUsageTimingObservations(ctx, []int64{usageID})
	require.NoError(t, err)
	require.Nil(t, observations[usageID].Queue)
}

func TestUsageObservabilityMigrationsPreserveExistingTraces(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	id := uuid.NewString()
	_, err := tx.ExecContext(ctx, `INSERT INTO request_timing_details(usage_log_id,trace_id,detail) VALUES(0,$1,'{"version":1}'::jsonb)`, id)
	require.NoError(t, err)
	for _, filename := range []string{"361_request_timing_details.sql", "362_request_connection_traces.sql"} {
		raw, err := dbmigrations.FS.ReadFile(filename)
		require.NoError(t, err)
		for i := 0; i < 2; i++ {
			_, err = tx.ExecContext(ctx, string(raw))
			require.NoError(t, err, filename)
		}
	}
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM request_timing_details WHERE trace_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
	requireIndex(t, tx, "request_timing_details", "request_timing_details_trace_id_idx")
}

func TestUsageObservabilityUsesPrivateBillingIdentity(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	repo := &usageLogRepository{sql: tx}
	now := time.Now().UTC()
	var userID, keyID, accountID int64
	suffix := uuid.NewString()
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash) VALUES($1,'fixture') RETURNING id`, suffix+"@example.test").Scan(&userID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'diagnostic key') RETURNING id`, userID, "sk-"+suffix).Scan(&keyID))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type) VALUES('diagnostic account','openai','apikey') RETURNING id`).Scan(&accountID))
	billedID := "gateway:" + uuid.NewString()
	_, err := tx.ExecContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,created_at) VALUES($1,$2,$3,$4,'gpt-fixture',$5)`, userID, keyID, accountID, billedID, now)
	require.NoError(t, err)
	for _, billingID := range []string{billedID, "gateway:" + uuid.NewString()} {
		id := uuid.NewString()
		_, err = tx.ExecContext(ctx, `INSERT INTO request_connection_traces(trace_id,started_at,request_id,client_request_id,billing_request_id,user_id,api_key_id,account_id,model,path,stream,detail) VALUES($1,$2,$3,'reused-public-correlation',$4,$5,$6,$7,'gpt-fixture','/v1/responses',true,'{"version":1,"status":200,"body_bytes":1024}'::jsonb)`, id, now, "server-"+id, billingID, userID, keyID, accountID)
		require.NoError(t, err)
	}
	f := service.LatencyReportFilter{Start: now.Add(-time.Hour), End: now.Add(time.Hour), APIKeyID: keyID, IncludeUnbilled: true}
	groups, err := repo.ReadLatencyReport(ctx, f)
	require.NoError(t, err)
	var requests int64
	for _, group := range groups {
		requests += group.Requests
	}
	require.Equal(t, int64(2), requests, "a billed request is not counted twice while a separate request sharing public correlation remains visible")
	page, err := repo.ListConnectionTraces(ctx, service.ConnectionTraceFilter{Start: f.Start, End: f.End, RequestID: billedID, Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, billedID, page.Items[0].BillingRequestID)
}
