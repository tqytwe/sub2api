package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type liveSettlementOutboxRepository struct {
	db *sql.DB
}

func NewLiveSettlementOutboxRepository(db *sql.DB) service.LiveSettlementOutboxRepository {
	return &liveSettlementOutboxRepository{db: db}
}

const liveSettlementSelectColumns = `
	id, call_hash, account_id, api_key_id, user_id, group_id, subscription_id,
	lease_id, model, call_created_at, call_expires_at, inbound_endpoint,
	user_agent, ip_address, input_tokens, input_audio_tokens, image_input_tokens,
	output_tokens, output_audio_tokens, cache_creation_tokens,
	cache_creation_audio_tokens, cache_read_tokens, cache_read_audio_tokens,
	image_output_tokens, status, attempts, available_at, claimed_by`

const liveSettlementReturningColumns = `
	o.id, o.call_hash, o.account_id, o.api_key_id, o.user_id, o.group_id, o.subscription_id,
	o.lease_id, o.model, o.call_created_at, o.call_expires_at, o.inbound_endpoint,
	o.user_agent, o.ip_address, o.input_tokens, o.input_audio_tokens, o.image_input_tokens,
	o.output_tokens, o.output_audio_tokens, o.cache_creation_tokens,
	o.cache_creation_audio_tokens, o.cache_read_tokens, o.cache_read_audio_tokens,
	o.image_output_tokens, o.status, o.attempts, o.available_at, o.claimed_by`

func (r *liveSettlementOutboxRepository) Enqueue(ctx context.Context, record *service.LiveCallRecord) error {
	if r == nil || r.db == nil {
		return errors.New("nil live settlement outbox database")
	}
	if record == nil || strings.TrimSpace(record.CallHash) == "" || record.AccountID <= 0 ||
		record.APIKeyID <= 0 || record.UserID <= 0 || strings.TrimSpace(record.LeaseID) == "" ||
		strings.TrimSpace(record.Model) == "" || record.CreatedAt.IsZero() || record.ExpiresAt.IsZero() {
		return errors.New("invalid live settlement record")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO live_usage_settlement_outbox (
			call_hash, account_id, api_key_id, user_id, group_id, subscription_id,
			lease_id, model, call_created_at, call_expires_at, inbound_endpoint,
			user_agent, ip_address, input_tokens, input_audio_tokens, image_input_tokens,
			output_tokens, output_audio_tokens, cache_creation_tokens,
			cache_creation_audio_tokens, cache_read_tokens, cache_read_audio_tokens,
			image_output_tokens, status, available_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, 'closing', NOW()
		)
		ON CONFLICT (call_hash) DO UPDATE SET
			account_id = EXCLUDED.account_id,
			api_key_id = EXCLUDED.api_key_id,
			user_id = EXCLUDED.user_id,
			group_id = EXCLUDED.group_id,
			subscription_id = EXCLUDED.subscription_id,
			lease_id = EXCLUDED.lease_id,
			model = EXCLUDED.model,
			call_created_at = EXCLUDED.call_created_at,
			call_expires_at = EXCLUDED.call_expires_at,
			inbound_endpoint = EXCLUDED.inbound_endpoint,
			user_agent = EXCLUDED.user_agent,
			ip_address = EXCLUDED.ip_address,
			input_tokens = EXCLUDED.input_tokens,
			input_audio_tokens = EXCLUDED.input_audio_tokens,
			image_input_tokens = EXCLUDED.image_input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			output_audio_tokens = EXCLUDED.output_audio_tokens,
			cache_creation_tokens = EXCLUDED.cache_creation_tokens,
			cache_creation_audio_tokens = EXCLUDED.cache_creation_audio_tokens,
			cache_read_tokens = EXCLUDED.cache_read_tokens,
			cache_read_audio_tokens = EXCLUDED.cache_read_audio_tokens,
			image_output_tokens = EXCLUDED.image_output_tokens,
			updated_at = NOW()
		WHERE live_usage_settlement_outbox.status = 'closing'
	`,
		record.CallHash,
		record.AccountID,
		record.APIKeyID,
		record.UserID,
		record.GroupID,
		record.SubscriptionID,
		record.LeaseID,
		record.Model,
		record.CreatedAt.UTC(),
		record.ExpiresAt.UTC(),
		record.InboundEndpoint,
		record.UserAgent,
		record.IPAddress,
		record.Usage.InputTokens,
		record.Usage.InputAudioTokens,
		record.Usage.ImageInputTokens,
		record.Usage.OutputTokens,
		record.Usage.OutputAudioTokens,
		record.Usage.CacheCreationInputTokens,
		record.Usage.CacheCreationInputAudioTokens,
		record.Usage.CacheReadInputTokens,
		record.Usage.CacheReadInputAudioTokens,
		record.Usage.ImageOutputTokens,
	)
	return err
}

func (r *liveSettlementOutboxRepository) Activate(ctx context.Context, callHash string) error {
	if r == nil || r.db == nil {
		return errors.New("nil live settlement outbox database")
	}
	if strings.TrimSpace(callHash) == "" {
		return errors.New("live settlement call hash is required")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE live_usage_settlement_outbox
		SET status = 'ready', available_at = NOW(), claimed_at = NULL,
			claimed_by = NULL, last_error = NULL, updated_at = NOW()
		WHERE call_hash = $1 AND status = 'closing'
	`, callHash)
	return err
}

func (r *liveSettlementOutboxRepository) ListClosing(ctx context.Context, limit int) ([]service.LiveSettlementJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil live settlement outbox database")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+liveSettlementSelectColumns+`
		FROM live_usage_settlement_outbox
		WHERE status = 'closing'
		ORDER BY id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanLiveSettlementJobs(rows)
}

func (r *liveSettlementOutboxRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]service.LiveSettlementJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil live settlement outbox database")
	}
	if strings.TrimSpace(workerID) == "" {
		return nil, errors.New("live settlement worker id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	leaseSeconds := int64(lease / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id
			FROM live_usage_settlement_outbox
			WHERE status = 'ready'
			  AND available_at <= NOW()
			  AND (claimed_at IS NULL OR claimed_at < NOW() - ($3 * INTERVAL '1 second'))
			ORDER BY id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE live_usage_settlement_outbox AS o
		SET claimed_at = NOW(), claimed_by = $1, updated_at = NOW()
		FROM candidates AS c
		WHERE o.id = c.id
		RETURNING `+liveSettlementReturningColumns+`
	`, workerID, limit, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanLiveSettlementJobs(rows)
}

func (r *liveSettlementOutboxRepository) Ack(ctx context.Context, id int64, workerID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM live_usage_settlement_outbox WHERE id = $1 AND claimed_by = $2
	`, id, workerID)
	if err != nil {
		return err
	}
	return requireLiveSettlementClaim(result, id, workerID)
}

func (r *liveSettlementOutboxRepository) Retry(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE live_usage_settlement_outbox
		SET attempts = attempts + 1, available_at = $3, last_error = $4,
			claimed_at = NULL, claimed_by = NULL, updated_at = NOW()
		WHERE id = $1 AND claimed_by = $2
	`, id, workerID, availableAt.UTC(), boundedLiveSettlementRepositoryError(lastError))
	if err != nil {
		return err
	}
	return requireLiveSettlementClaim(result, id, workerID)
}

func requireLiveSettlementClaim(result sql.Result, id int64, workerID string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("live settlement claim %d is no longer owned by %s", id, workerID)
	}
	return nil
}

func boundedLiveSettlementRepositoryError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 1024 {
		return message[:1024]
	}
	return message
}

func scanLiveSettlementJobs(rows *sql.Rows) ([]service.LiveSettlementJob, error) {
	jobs := make([]service.LiveSettlementJob, 0)
	for rows.Next() {
		job, err := scanLiveSettlementJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

type liveSettlementRowScanner interface {
	Scan(dest ...any) error
}

func scanLiveSettlementJob(row liveSettlementRowScanner) (service.LiveSettlementJob, error) {
	var (
		job       service.LiveSettlementJob
		record    service.LiveCallRecord
		claimedBy sql.NullString
	)
	err := row.Scan(
		&job.ID, &job.CallHash, &record.AccountID, &record.APIKeyID, &record.UserID,
		&record.GroupID, &record.SubscriptionID, &record.LeaseID, &record.Model,
		&record.CreatedAt, &record.ExpiresAt, &record.InboundEndpoint, &record.UserAgent,
		&record.IPAddress, &record.Usage.InputTokens, &record.Usage.InputAudioTokens,
		&record.Usage.ImageInputTokens, &record.Usage.OutputTokens,
		&record.Usage.OutputAudioTokens, &record.Usage.CacheCreationInputTokens,
		&record.Usage.CacheCreationInputAudioTokens, &record.Usage.CacheReadInputTokens,
		&record.Usage.CacheReadInputAudioTokens, &record.Usage.ImageOutputTokens,
		&job.Status, &job.Attempts, &job.AvailableAt, &claimedBy,
	)
	if err != nil {
		return service.LiveSettlementJob{}, err
	}
	record.CallHash = job.CallHash
	job.Record = &record
	job.ClaimedBy = claimedBy.String
	return job, nil
}
