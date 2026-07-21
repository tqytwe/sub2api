package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *imageStudioRepository) ListJobReferencesByID(
	ctx context.Context,
	jobID string,
	ids []string,
) (result []service.ImageStudioJobReference, err error) {
	if len(ids) == 0 {
		return []service.ImageStudioJobReference{}, nil
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT r.id::text, r.job_id::text, r.storage_key, r.content_type,
		       r.byte_size, r.sort_order, r.created_at
		FROM unnest($2::uuid[]) WITH ORDINALITY requested(id, ord)
		JOIN image_studio_job_references r ON r.id = requested.id
		WHERE r.job_id = $1::uuid
		ORDER BY requested.ord`, jobID, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("list image studio job references: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.ImageStudioJobReference, 0, len(ids))
	for rows.Next() {
		var reference service.ImageStudioJobReference
		if err := rows.Scan(
			&reference.ID,
			&reference.JobID,
			&reference.StorageKey,
			&reference.ContentType,
			&reference.ByteSize,
			&reference.SortOrder,
			&reference.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, reference)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *imageStudioRepository) ListJobReferenceStorageKeysForJob(
	ctx context.Context,
	jobID string,
) (result []string, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT storage_key
		FROM image_studio_job_references
		WHERE job_id = $1::uuid
		  AND storage_key <> ''
		ORDER BY sort_order`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list image studio job reference storage keys: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]string, 0)
	for rows.Next() {
		var storageKey string
		if err := rows.Scan(&storageKey); err != nil {
			return nil, err
		}
		out = append(out, storageKey)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) ListExpiredReferences(
	ctx context.Context,
	before time.Time,
) (result []service.ImageStudioReference, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id::text, user_id, storage_key, original_filename,
		       content_type, byte_size, created_at, expires_at
		FROM image_studio_references
		WHERE expires_at < $1
		ORDER BY expires_at, id
		LIMIT 100`, before)
	if err != nil {
		return nil, fmt.Errorf("list expired image studio references: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.ImageStudioReference, 0)
	for rows.Next() {
		var reference service.ImageStudioReference
		var expiresAt time.Time
		if err := rows.Scan(
			&reference.ID,
			&reference.UserID,
			&reference.StorageKey,
			&reference.OriginalFilename,
			&reference.ContentType,
			&reference.ByteSize,
			&reference.CreatedAt,
			&expiresAt,
		); err != nil {
			return nil, err
		}
		reference.ExpiresAt = &expiresAt
		out = append(out, reference)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) DeleteReference(ctx context.Context, referenceID string) error {
	if _, err := r.sqlExec(ctx).ExecContext(ctx, `
		DELETE FROM image_studio_references
		WHERE id = $1::uuid`, referenceID); err != nil {
		return fmt.Errorf("delete image studio reference: %w", err)
	}
	return nil
}

func (r *imageStudioRepository) GetReferenceForDelete(
	ctx context.Context,
	userID int64,
	referenceID string,
) (*service.ImageStudioReference, error) {
	var reference service.ImageStudioReference
	var expiresAt time.Time
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id::text, user_id, storage_key, original_filename,
		       content_type, byte_size, created_at, expires_at
		FROM image_studio_references
		WHERE id = $1::uuid AND user_id = $2`,
		[]any{referenceID, userID},
		&reference.ID,
		&reference.UserID,
		&reference.StorageKey,
		&reference.OriginalFilename,
		&reference.ContentType,
		&reference.ByteSize,
		&reference.CreatedAt,
		&expiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrImageStudioReferenceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get image studio reference for delete: %w", err)
	}
	reference.ExpiresAt = &expiresAt
	return &reference, nil
}

func (r *imageStudioRepository) DeleteReferenceForUser(
	ctx context.Context,
	userID int64,
	referenceID string,
) error {
	result, err := r.sqlExec(ctx).ExecContext(ctx, `
		DELETE FROM image_studio_references
		WHERE id = $1::uuid AND user_id = $2`, referenceID, userID)
	if err != nil {
		return fmt.Errorf("delete owned image studio reference: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageStudioReferenceNotFound
	}
	return nil
}

var (
	_ service.ImageStudioJobReferenceReader            = (*imageStudioRepository)(nil)
	_ service.ImageStudioJobReferenceStorageRepository = (*imageStudioRepository)(nil)
	_ service.ImageStudioReferenceCleanupRepository    = (*imageStudioRepository)(nil)
	_ service.ImageStudioReferenceLifecycleRepository  = (*imageStudioRepository)(nil)
	_ service.ImageStudioAssetPurgeRepository          = (*imageStudioRepository)(nil)
)
