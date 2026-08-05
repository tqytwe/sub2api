package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func liveSettlementTestRecord() *service.LiveCallRecord {
	return &service.LiveCallRecord{
		CallHash: "aabbcc", AccountID: 11, APIKeyID: 22, UserID: 33, GroupID: 44,
		LeaseID: "lease", Model: "gpt-live", CreatedAt: time.Now().UTC().Add(-time.Minute),
		ExpiresAt: time.Now().UTC().Add(time.Hour), InboundEndpoint: "/v1/realtime",
		UserAgent: "test", IPAddress: "127.0.0.1", Usage: service.OpenAIUsage{
			InputTokens: 10, InputAudioTokens: 2, OutputTokens: 4, OutputAudioTokens: 1,
		},
	}
}

func TestLiveSettlementOutboxRepositoryEnqueueAndActivate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewLiveSettlementOutboxRepository(db)
	record := liveSettlementTestRecord()

	mock.ExpectExec("(?s)INSERT INTO live_usage_settlement_outbox.*ON CONFLICT.*status = 'closing'").
		WithArgs(
			record.CallHash, record.AccountID, record.APIKeyID, record.UserID, record.GroupID,
			record.SubscriptionID, record.LeaseID, record.Model, record.CreatedAt.UTC(), record.ExpiresAt.UTC(),
			record.InboundEndpoint, record.UserAgent, record.IPAddress, 10, 2, 0, 4, 1, 0, 0, 0, 0, 0,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Enqueue(context.Background(), record))

	mock.ExpectExec("UPDATE live_usage_settlement_outbox.*SET status = 'ready'.*WHERE call_hash = \\$1 AND status = 'closing'").
		WithArgs(record.CallHash).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Activate(context.Background(), record.CallHash))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiveSettlementOutboxRepositoryClaimAndRetryPreserveLeaseOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewLiveSettlementOutboxRepository(db)
	record := liveSettlementTestRecord()
	created := record.CreatedAt
	expires := record.ExpiresAt
	available := time.Now().UTC()
	columns := []string{
		"id", "call_hash", "account_id", "api_key_id", "user_id", "group_id", "subscription_id",
		"lease_id", "model", "call_created_at", "call_expires_at", "inbound_endpoint", "user_agent", "ip_address",
		"input_tokens", "input_audio_tokens", "image_input_tokens", "output_tokens", "output_audio_tokens",
		"cache_creation_tokens", "cache_creation_audio_tokens", "cache_read_tokens", "cache_read_audio_tokens",
		"image_output_tokens", "status", "attempts", "available_at", "claimed_by",
	}
	rows := sqlmock.NewRows(columns).AddRow(
		int64(1), record.CallHash, record.AccountID, record.APIKeyID, record.UserID, record.GroupID, record.SubscriptionID,
		record.LeaseID, record.Model, created, expires, record.InboundEndpoint, record.UserAgent, record.IPAddress,
		10, 2, 0, 4, 1, 0, 0, 0, 0, 0, "ready", 2, available, "worker-a",
	)
	mock.ExpectQuery("(?s)WITH candidates.*FOR UPDATE SKIP LOCKED.*RETURNING").
		WithArgs("worker-a", 32, int64(30)).WillReturnRows(rows)
	jobs, err := repo.Claim(context.Background(), "worker-a", 32, 30*time.Second)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, record.CallHash, jobs[0].CallHash)
	require.Equal(t, record.Usage.InputAudioTokens, jobs[0].Record.Usage.InputAudioTokens)

	next := time.Now().UTC().Add(time.Minute)
	mock.ExpectExec("UPDATE live_usage_settlement_outbox.*attempts = attempts \\+ 1.*WHERE id = \\$1 AND claimed_by = \\$2").
		WithArgs(int64(1), "worker-a", next, "temporary failure").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Retry(context.Background(), 1, "worker-a", next, "temporary failure"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLiveSettlementOutboxMigrationHasNoSensitivePayloadColumns(t *testing.T) {
	content, err := migrations.FS.ReadFile("249_live_usage_settlement_outbox.sql")
	require.NoError(t, err)
	sqlText := string(content)
	for _, required := range []string{
		"live_usage_settlement_outbox", "call_hash", "status", "claimed_at", "claimed_by",
		"idx_live_usage_settlement_outbox_ready",
	} {
		require.Contains(t, sqlText, required)
	}
	ddl := sqlText[strings.Index(strings.ToLower(sqlText), "create table"):]
	for _, forbidden := range []string{"transcript", "audio_frame", "access_token", "password", "response_id"} {
		require.NotContains(t, strings.ToLower(ddl), forbidden)
	}
}
