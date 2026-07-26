package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MobilePushRepository struct {
	db          *sql.DB
	claimLease  time.Duration
	maxAttempts int
}

func NewMobilePushRepository(db *sql.DB, claimLease time.Duration, maxAttempts int) service.MobilePushRepository {
	if claimLease <= 0 {
		claimLease = 2 * time.Minute
	}
	if maxAttempts <= 0 {
		maxAttempts = 8
	}
	return &MobilePushRepository{db: db, claimLease: claimLease, maxAttempts: maxAttempts}
}

func (r *MobilePushRepository) UpsertDevice(ctx context.Context, write service.MobileDeviceWrite) (*service.MobileDevice, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	lockKeys := []string{"installation:" + write.InstallationID, "token:" + write.TokenHash}
	sort.Strings(lockKeys)
	for _, key := range lockKeys {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
			return nil, err
		}
	}

	// A token moving to another account must stop receiving the previous user's
	// notifications before the new ownership becomes visible.
	if _, err := tx.ExecContext(ctx, `
		UPDATE mobile_devices
		SET enabled = FALSE, revoked_at = $1, updated_at = $1
		WHERE token_hash = $2
		  AND enabled = TRUE
		  AND NOT (user_id = $3 AND installation_id = $4::uuid)
	`, write.LastSeenAt, write.TokenHash, write.UserID, write.InstallationID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE mobile_devices
		SET enabled = FALSE, revoked_at = $1, updated_at = $1
		WHERE installation_id = $2::uuid
		  AND enabled = TRUE
		  AND user_id <> $3
	`, write.LastSeenAt, write.InstallationID, write.UserID); err != nil {
		return nil, err
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO mobile_devices (
			id, user_id, installation_id, platform, push_provider,
			token_ciphertext, token_hash, app_version, locale,
			enabled, last_seen_at, revoked_at, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2::uuid, $3, $4,
			$5, $6, $7, $8, TRUE, $9, NULL, $9, $9
		)
		ON CONFLICT (user_id, installation_id) DO UPDATE SET
			platform = EXCLUDED.platform,
			push_provider = EXCLUDED.push_provider,
			token_ciphertext = EXCLUDED.token_ciphertext,
			token_hash = EXCLUDED.token_hash,
			app_version = EXCLUDED.app_version,
			locale = EXCLUDED.locale,
			enabled = TRUE,
			last_seen_at = EXCLUDED.last_seen_at,
			revoked_at = NULL,
			updated_at = EXCLUDED.updated_at
		RETURNING id::text, user_id, installation_id::text, platform, push_provider,
			token_ciphertext, token_hash, app_version, locale, enabled,
			last_seen_at, revoked_at, created_at, updated_at
	`, write.UserID, write.InstallationID, write.Platform, write.PushProvider,
		write.TokenCiphertext, write.TokenHash, write.AppVersion, write.Locale, write.LastSeenAt)
	device, err := scanMobileDevice(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return device, nil
}

func (r *MobilePushRepository) RevokeDevice(ctx context.Context, userID int64, installationID string, now time.Time) (bool, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mobile_devices
		SET enabled = FALSE, revoked_at = $1, updated_at = $1
		WHERE user_id = $2 AND installation_id = $3::uuid AND enabled = TRUE
	`, now, userID, installationID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *MobilePushRepository) EnqueuePush(ctx context.Context, item *service.MobilePushOutboxItem) (*service.MobilePushOutboxItem, bool, error) {
	data, err := json.Marshal(item.Data)
	if err != nil {
		return nil, false, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO mobile_push_outbox (
			user_id, dedupe_key_hash, event_type, source_type, source_id,
			title_zh, body_zh, data, status, attempts, available_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, 'pending', 0, $9, NOW(), NOW())
		ON CONFLICT (dedupe_key_hash) DO NOTHING
		RETURNING `+mobilePushOutboxColumns,
		item.UserID, item.DedupeKeyHash, item.EventType, item.SourceType, item.SourceID,
		item.TitleZh, item.BodyZh, data, item.AvailableAt)
	createdItem, err := scanMobilePushOutbox(row)
	if err == nil {
		return createdItem, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	existing, err := scanMobilePushOutbox(r.db.QueryRowContext(ctx, `SELECT `+mobilePushOutboxColumns+` FROM mobile_push_outbox WHERE dedupe_key_hash = $1`, item.DedupeKeyHash))
	return existing, false, err
}

func (r *MobilePushRepository) ClaimPendingPush(ctx context.Context, now time.Time) (*service.MobilePushOutboxItem, error) {
	leaseUntil := now.Add(r.claimLease)
	row := r.db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id
			FROM mobile_push_outbox
			WHERE (status = 'pending' AND available_at <= $1)
			   OR (status = 'processing' AND lease_expires_at <= $1)
			ORDER BY available_at ASC, id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE mobile_push_outbox AS outbox
		SET status = 'processing', claim_token = gen_random_uuid(),
		    lease_expires_at = $2, updated_at = $1
		FROM candidate
		WHERE outbox.id = candidate.id
		RETURNING `+mobilePushOutboxReturningColumns("outbox"), now, leaseUntil)
	item, err := scanMobilePushOutbox(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *MobilePushRepository) PreparePushDeliveries(ctx context.Context, outboxID, userID int64, claimToken string, now time.Time) ([]service.MobilePushDeviceDelivery, error) {
	claimGuard := `EXISTS (
		SELECT 1 FROM mobile_push_outbox o
		WHERE o.id = $1 AND o.user_id = $2 AND o.status = 'processing' AND o.claim_token = $3::uuid
	)`
	if _, err := r.db.ExecContext(ctx, `
		UPDATE mobile_push_deliveries AS delivery
		SET status = 'skipped', last_error_code = 'DEVICE_REVOKED', updated_at = $4
		FROM mobile_devices AS device
		WHERE delivery.outbox_id = $1 AND delivery.device_id = device.id
		  AND delivery.status = 'pending'
		  AND (device.enabled = FALSE OR device.revoked_at IS NOT NULL)
		  AND `+claimGuard, outboxID, userID, claimToken, now); err != nil {
		return nil, err
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO mobile_push_deliveries (
			outbox_id, device_id, status, attempts, available_at, created_at, updated_at
		)
		SELECT $1, device.id, 'pending', 0, $4, $4, $4
		FROM mobile_devices AS device
		WHERE device.user_id = $2 AND device.enabled = TRUE AND device.revoked_at IS NULL
		  AND `+claimGuard+`
		  AND NOT EXISTS (SELECT 1 FROM mobile_push_deliveries existing WHERE existing.outbox_id = $1)
		ON CONFLICT (outbox_id, device_id) DO NOTHING
	`, outboxID, userID, claimToken, now); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT delivery.id, delivery.outbox_id, delivery.status, delivery.attempts, delivery.available_at,
		       device.id::text, device.user_id, device.installation_id::text, device.platform, device.push_provider,
		       device.token_ciphertext, device.token_hash, device.app_version, device.locale, device.enabled,
		       device.last_seen_at, device.revoked_at, device.created_at, device.updated_at
		FROM mobile_push_deliveries AS delivery
		JOIN mobile_devices AS device ON device.id = delivery.device_id
		WHERE delivery.outbox_id = $1 AND delivery.status = 'pending' AND delivery.available_at <= $4
		  AND device.user_id = $2 AND device.enabled = TRUE AND device.revoked_at IS NULL
		  AND `+claimGuard+`
		ORDER BY delivery.id ASC
	`, outboxID, userID, claimToken, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	deliveries := make([]service.MobilePushDeviceDelivery, 0)
	for rows.Next() {
		var target service.MobilePushDeviceDelivery
		if err := rows.Scan(
			&target.ID, &target.OutboxID, &target.Status, &target.Attempts, &target.AvailableAt,
			&target.Device.ID, &target.Device.UserID, &target.Device.InstallationID, &target.Device.Platform,
			&target.Device.PushProvider, &target.Device.TokenCiphertext, &target.Device.TokenHash,
			&target.Device.AppVersion, &target.Device.Locale, &target.Device.Enabled, &target.Device.LastSeenAt,
			&target.Device.RevokedAt, &target.Device.CreatedAt, &target.Device.UpdatedAt,
		); err != nil {
			return nil, err
		}
		deliveries = append(deliveries, target)
	}
	return deliveries, rows.Err()
}

func (r *MobilePushRepository) MarkPushDeliverySent(ctx context.Context, id, outboxID int64, claimToken string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mobile_push_deliveries AS delivery
		SET status = 'sent', sent_at = $4, last_error_code = NULL, updated_at = $4
		WHERE delivery.id = $1 AND delivery.outbox_id = $2 AND delivery.status = 'pending'
		  AND EXISTS (SELECT 1 FROM mobile_push_outbox o WHERE o.id = $2 AND o.status = 'processing' AND o.claim_token = $3::uuid)
	`, id, outboxID, claimToken, now)
	return requireMobilePushClaim(result, err)
}

func (r *MobilePushRepository) FailPushDelivery(ctx context.Context, id, outboxID int64, claimToken, code string, availableAt time.Time, terminal bool) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mobile_push_deliveries AS delivery
		SET attempts = attempts + 1,
		    status = CASE WHEN $6 OR attempts + 1 >= $7 THEN CASE WHEN $6 THEN 'skipped' ELSE 'failed' END ELSE 'pending' END,
		    last_error_code = $4, available_at = $5, updated_at = NOW()
		WHERE delivery.id = $1 AND delivery.outbox_id = $2 AND delivery.status = 'pending'
		  AND EXISTS (SELECT 1 FROM mobile_push_outbox o WHERE o.id = $2 AND o.status = 'processing' AND o.claim_token = $3::uuid)
	`, id, outboxID, claimToken, code, availableAt, terminal, r.maxAttempts)
	return requireMobilePushClaim(result, err)
}

func (r *MobilePushRepository) RevokeDeviceByID(ctx context.Context, deviceID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE mobile_devices SET enabled = FALSE, revoked_at = $2, updated_at = $2
		WHERE id = $1::uuid AND enabled = TRUE
	`, deviceID, now)
	return err
}

func (r *MobilePushRepository) FinalizePush(ctx context.Context, id int64, claimToken string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		WITH summary AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE status = 'pending') AS pending_count,
			       COUNT(*) FILTER (WHERE status = 'sent') AS sent_count,
			       COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
			       MIN(available_at) FILTER (WHERE status = 'pending') AS next_available_at
			FROM mobile_push_deliveries WHERE outbox_id = $1
		)
		UPDATE mobile_push_outbox AS outbox
		SET status = CASE
				WHEN summary.total = 0 THEN 'skipped'
				WHEN summary.pending_count > 0 THEN 'pending'
				WHEN summary.sent_count > 0 THEN 'sent'
				WHEN summary.failed_count > 0 THEN 'failed'
				ELSE 'skipped'
			END,
			available_at = COALESCE(summary.next_available_at, outbox.available_at),
			sent_at = CASE WHEN summary.pending_count = 0 AND summary.sent_count > 0 THEN $3 ELSE outbox.sent_at END,
			last_error_code = CASE WHEN summary.pending_count > 0 THEN 'DELIVERY_PENDING' WHEN summary.failed_count > 0 THEN 'DELIVERY_FAILED' ELSE NULL END,
			claim_token = NULL, lease_expires_at = NULL, updated_at = $3
		FROM summary
		WHERE outbox.id = $1 AND outbox.status = 'processing' AND outbox.claim_token = $2::uuid
	`, id, claimToken, now)
	return requireMobilePushClaim(result, err)
}

func (r *MobilePushRepository) MarkPushSkipped(ctx context.Context, id int64, claimToken, code string, now time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE mobile_push_outbox
		SET status = 'skipped', last_error_code = $3, claim_token = NULL, lease_expires_at = NULL, updated_at = $4
		WHERE id = $1 AND status = 'processing' AND claim_token = $2::uuid
	`, id, claimToken, code, now)
	return requireMobilePushClaim(result, err)
}

func (r *MobilePushRepository) RequeuePush(ctx context.Context, id int64, claimToken, code string, availableAt time.Time, countAttempt bool) error {
	increment := 0
	if countAttempt {
		increment = 1
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE mobile_push_outbox
		SET attempts = attempts + $2,
		    status = CASE WHEN attempts + $2 >= $3 THEN 'failed' ELSE 'pending' END,
		    last_error_code = $4,
		    available_at = $5,
		    claim_token = NULL,
		    lease_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'processing' AND claim_token = $6::uuid
	`, id, increment, r.maxAttempts, code, availableAt, claimToken)
	return requireMobilePushClaim(result, err)
}

func requireMobilePushClaim(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrMobilePushClaimLost
	}
	return nil
}

const mobilePushOutboxColumns = `id, user_id, dedupe_key_hash, event_type, source_type, source_id, title_zh, body_zh, data, status, attempts, last_error_code, available_at, sent_at, claim_token::text, lease_expires_at, created_at, updated_at`

func mobilePushOutboxReturningColumns(alias string) string {
	return alias + `.id, ` + alias + `.user_id, ` + alias + `.dedupe_key_hash, ` + alias + `.event_type, ` +
		alias + `.source_type, ` + alias + `.source_id, ` + alias + `.title_zh, ` + alias + `.body_zh, ` +
		alias + `.data, ` + alias + `.status, ` + alias + `.attempts, ` + alias + `.last_error_code, ` +
		alias + `.available_at, ` + alias + `.sent_at, ` + alias + `.claim_token::text, ` + alias + `.lease_expires_at, ` + alias + `.created_at, ` + alias + `.updated_at`
}

type mobilePushScanner interface {
	Scan(...any) error
}

func scanMobileDevice(scanner mobilePushScanner) (*service.MobileDevice, error) {
	var device service.MobileDevice
	err := scanner.Scan(
		&device.ID, &device.UserID, &device.InstallationID, &device.Platform, &device.PushProvider,
		&device.TokenCiphertext, &device.TokenHash, &device.AppVersion, &device.Locale, &device.Enabled,
		&device.LastSeenAt, &device.RevokedAt, &device.CreatedAt, &device.UpdatedAt,
	)
	return &device, err
}

func scanMobilePushOutbox(scanner mobilePushScanner) (*service.MobilePushOutboxItem, error) {
	var item service.MobilePushOutboxItem
	var data []byte
	var claimToken sql.NullString
	err := scanner.Scan(
		&item.ID, &item.UserID, &item.DedupeKeyHash, &item.EventType, &item.SourceType, &item.SourceID,
		&item.TitleZh, &item.BodyZh, &data, &item.Status, &item.Attempts, &item.LastErrorCode,
		&item.AvailableAt, &item.SentAt, &claimToken, &item.LeaseExpiresAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	item.ClaimToken = claimToken.String
	item.Data = map[string]string{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &item.Data); err != nil {
			return nil, err
		}
	}
	return &item, nil
}

var _ service.MobilePushRepository = (*MobilePushRepository)(nil)
