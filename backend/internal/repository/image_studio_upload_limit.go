package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

func (r *imageStudioRepository) AcquireImageStudioUploadSlot(
	ctx context.Context,
	userID int64,
	now time.Time,
	leaseDuration time.Duration,
	concurrency int,
	rate int,
	window time.Duration,
) (token string, acquired bool, err error) {
	if r.db == nil {
		return "", false, errors.New("image studio repository db is nil")
	}
	if leaseDuration <= 0 {
		leaseDuration = 10 * time.Minute
	}
	if concurrency <= 0 || rate <= 0 || window <= 0 {
		return "", false, errors.New("image studio upload limit is invalid")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return "", false, fmt.Errorf("lock image studio upload user: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE image_studio_upload_slots
		SET released_at = COALESCE(released_at, $2)
		WHERE user_id = $1
		  AND released_at IS NULL
		  AND lease_expires_at <= $2`, userID, now); err != nil {
		return "", false, err
	}
	cutoff := now.Add(-window)
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM image_studio_upload_slots
		WHERE user_id = $1
		  AND started_at < $2
		  AND released_at IS NOT NULL`, userID, cutoff); err != nil {
		return "", false, err
	}
	var active, attempts int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (
				WHERE released_at IS NULL AND lease_expires_at > $2
			)::int,
			COUNT(*) FILTER (WHERE started_at > $3)::int
		FROM image_studio_upload_slots
		WHERE user_id = $1`, userID, now, cutoff).Scan(&active, &attempts); err != nil {
		return "", false, err
	}
	if active >= concurrency || attempts >= rate {
		if err := tx.Commit(); err != nil {
			return "", false, err
		}
		return "", false, nil
	}
	token = uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO image_studio_upload_slots (
			id, user_id, started_at, lease_expires_at
		)
		VALUES ($1::uuid, $2, $3, $4)`,
		token, userID, now, now.Add(leaseDuration),
	); err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return token, true, nil
}

func (r *imageStudioRepository) ReleaseImageStudioUploadSlot(
	ctx context.Context,
	userID int64,
	token string,
	now time.Time,
) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE image_studio_upload_slots
		SET released_at = COALESCE(released_at, $3)
		WHERE id = $1::uuid AND user_id = $2`,
		token, userID, now,
	)
	return err
}

var _ service.ImageStudioUploadLimitRepository = (*imageStudioRepository)(nil)
