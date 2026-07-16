package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type imageStudioRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewImageStudioRepository(client *dbent.Client, db *sql.DB) service.ImageStudioRepository {
	return &imageStudioRepository{client: client, sql: db}
}

func (r *imageStudioRepository) sqlExec(ctx context.Context) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if exec := sqlExecutorFromEntClient(tx.Client()); exec != nil {
			return exec
		}
	}
	return r.sql
}

func (r *imageStudioRepository) InsertJob(ctx context.Context, job *service.ImageStudioJob) error {
	exec := r.sqlExec(ctx)
	err := scanSingleRow(ctx, exec, `
		INSERT INTO image_studio_jobs
			(id, user_id, template_id, prompt_hash, size, count, status, estimated_cost, api_key_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at`,
		[]any{job.ID, job.UserID, job.TemplateID, job.PromptHash, job.Size, job.Count, job.Status, job.EstimatedCost, job.APIKeyID, job.ExpiresAt},
		&job.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert image studio job: %w", err)
	}
	return nil
}

func (r *imageStudioRepository) UpdateJobResult(ctx context.Context, jobID string, status string, actualCost *float64, errMsg string) error {
	exec := r.sqlExec(ctx)
	_, err := exec.ExecContext(ctx, `
		UPDATE image_studio_jobs
		SET status = $2, actual_cost = $3, error_message = NULLIF($4, '')
		WHERE id = $1::uuid`, jobID, status, actualCost, errMsg)
	if err != nil {
		return fmt.Errorf("update image studio job: %w", err)
	}
	return nil
}

func (r *imageStudioRepository) InsertAssets(ctx context.Context, jobID string, assets []service.ImageStudioAssetRecord) error {
	if len(assets) == 0 {
		return nil
	}
	exec := r.sqlExec(ctx)
	for _, asset := range assets {
		id := asset.ID
		if id == "" {
			id = uuid.NewString()
		}
		url := strings.TrimSpace(asset.URL)
		storageKey := strings.TrimSpace(asset.StorageKey)
		if url == "" && storageKey == "" {
			return fmt.Errorf("insert image studio asset: either url or storage_key is required")
		}
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO image_studio_assets (id, job_id, url, sort_order, storage_key, content_type, byte_size)
			VALUES ($1::uuid, $2::uuid, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), $7)`,
			id, jobID, url, asset.SortOrder, storageKey, asset.ContentType, asset.ByteSize); err != nil {
			return fmt.Errorf("insert image studio asset: %w", err)
		}
	}
	return nil
}

func (r *imageStudioRepository) GetJob(ctx context.Context, userID int64, jobID string) (*service.ImageStudioJob, error) {
	job, err := r.scanJob(ctx, `
		SELECT id::text, user_id, template_id, prompt_hash, size, count, status,
		       estimated_cost, actual_cost, api_key_id, error_message, created_at, expires_at
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2`, jobID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrImageStudioJobNotFound
		}
		return nil, err
	}
	assets, err := r.listAssets(ctx, jobID)
	if err != nil {
		return nil, err
	}
	job.Assets = assets
	return job, nil
}

func (r *imageStudioRepository) GetActiveJob(ctx context.Context, userID int64) (*service.ImageStudioJob, error) {
	job, err := r.scanJob(ctx, `
		SELECT id::text, user_id, template_id, prompt_hash, size, count, status,
		       estimated_cost, actual_cost, api_key_id, error_message, created_at, expires_at
		FROM image_studio_jobs
		WHERE user_id = $1 AND status IN ('pending', 'running')
		ORDER BY created_at DESC
		LIMIT 1`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	assets, err := r.listAssets(ctx, job.ID)
	if err != nil {
		return nil, err
	}
	job.Assets = assets
	return job, nil
}

func (r *imageStudioRepository) GetAsset(ctx context.Context, userID int64, assetID string) (*service.ImageStudioAsset, error) {
	exec := r.sqlExec(ctx)
	var asset service.ImageStudioAsset
	var url sql.NullString
	var storageKey sql.NullString
	var contentType sql.NullString
	err := scanSingleRow(ctx, exec, `
		SELECT a.id::text, COALESCE(a.url, ''), a.sort_order,
		       COALESCE(a.storage_key, ''), COALESCE(a.content_type, ''), COALESCE(a.byte_size, 0)
		FROM image_studio_assets a
		JOIN image_studio_jobs j ON j.id = a.job_id
		WHERE a.id = $1::uuid AND j.user_id = $2`,
		[]any{assetID, userID},
		&asset.ID, &url, &asset.SortOrder, &storageKey, &contentType, &asset.ByteSize)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrImageStudioAssetNotFound
		}
		return nil, fmt.Errorf("get image studio asset: %w", err)
	}
	if url.Valid {
		asset.URL = url.String
	}
	if storageKey.Valid {
		asset.StorageKey = storageKey.String
	}
	if contentType.Valid {
		asset.ContentType = contentType.String
	}
	return &asset, nil
}

func (r *imageStudioRepository) scanJob(ctx context.Context, query string, args ...any) (*service.ImageStudioJob, error) {
	exec := r.sqlExec(ctx)
	var job service.ImageStudioJob
	var actualCost sql.NullFloat64
	var errMsg sql.NullString
	var apiKeyID sql.NullInt64
	var expiresAt sql.NullTime
	err := scanSingleRow(ctx, exec, query, args,
		&job.ID, &job.UserID, &job.TemplateID, &job.PromptHash, &job.Size, &job.Count, &job.Status,
		&job.EstimatedCost, &actualCost, &apiKeyID, &errMsg, &job.CreatedAt, &expiresAt)
	if err != nil {
		return nil, err
	}
	if actualCost.Valid {
		job.ActualCost = &actualCost.Float64
	}
	if apiKeyID.Valid {
		job.APIKeyID = &apiKeyID.Int64
	}
	if errMsg.Valid {
		job.ErrorMessage = errMsg.String
	}
	if expiresAt.Valid {
		job.ExpiresAt = &expiresAt.Time
	}
	return &job, nil
}

func (r *imageStudioRepository) listAssets(ctx context.Context, jobID string) (result []service.ImageStudioAsset, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id::text, COALESCE(url, ''), sort_order,
		       COALESCE(storage_key, ''), COALESCE(content_type, ''), COALESCE(byte_size, 0)
		FROM image_studio_assets
		WHERE job_id = $1::uuid
		ORDER BY sort_order ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list image studio assets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.ImageStudioAsset, 0)
	for rows.Next() {
		var a service.ImageStudioAsset
		if err := rows.Scan(&a.ID, &a.URL, &a.SortOrder, &a.StorageKey, &a.ContentType, &a.ByteSize); err != nil {
			return nil, fmt.Errorf("scan image studio asset: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) ListJobs(ctx context.Context, userID int64, limit int) (result []service.ImageStudioJob, err error) {
	if limit <= 0 {
		limit = 20
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id::text, user_id, template_id, prompt_hash, size, count, status,
		       estimated_cost, actual_cost, api_key_id, error_message, created_at, expires_at
		FROM image_studio_jobs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list image studio jobs: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.ImageStudioJob, 0)
	for rows.Next() {
		var job service.ImageStudioJob
		var actualCost sql.NullFloat64
		var apiKeyID sql.NullInt64
		var errMsg sql.NullString
		var expiresAt sql.NullTime
		if err := rows.Scan(
			&job.ID, &job.UserID, &job.TemplateID, &job.PromptHash, &job.Size, &job.Count, &job.Status,
			&job.EstimatedCost, &actualCost, &apiKeyID, &errMsg, &job.CreatedAt, &expiresAt,
		); err != nil {
			return nil, fmt.Errorf("scan image studio job: %w", err)
		}
		if actualCost.Valid {
			job.ActualCost = &actualCost.Float64
		}
		if apiKeyID.Valid {
			job.APIKeyID = &apiKeyID.Int64
		}
		if errMsg.Valid {
			job.ErrorMessage = errMsg.String
		}
		if expiresAt.Valid {
			job.ExpiresAt = &expiresAt.Time
		}
		assets, err := r.listAssets(ctx, job.ID)
		if err != nil {
			return nil, err
		}
		job.Assets = assets
		out = append(out, job)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) ListAssetStorageKeysForJob(ctx context.Context, jobID string) (result []string, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT COALESCE(storage_key, '')
		FROM image_studio_assets
		WHERE job_id = $1::uuid AND storage_key IS NOT NULL AND storage_key <> ''`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list image studio asset storage keys: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		if key != "" {
			out = append(out, key)
		}
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) ListExpiredJobIDs(ctx context.Context, before time.Time) (result []string, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id::text
		FROM image_studio_jobs
		WHERE expires_at IS NOT NULL AND expires_at < $1`, before)
	if err != nil {
		return nil, fmt.Errorf("list expired image studio jobs: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) DeleteJob(ctx context.Context, userID int64, jobID string) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		DELETE FROM image_studio_jobs WHERE id = $1::uuid AND user_id = $2`, jobID, userID)
	if err != nil {
		return fmt.Errorf("delete image studio job: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrImageStudioJobNotFound
	}
	return nil
}

func (r *imageStudioRepository) CountCompletedToday(ctx context.Context, userID int64, dayStart time.Time) (int, error) {
	exec := r.sqlExec(ctx)
	var count int
	err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1 AND status = 'completed'
		  AND created_at >= $2 AND created_at < $3`,
		[]any{userID, dayStart, dayStart.AddDate(0, 0, 1)}, &count)
	if err != nil {
		return 0, fmt.Errorf("count image studio jobs today: %w", err)
	}
	return count, nil
}

func (r *imageStudioRepository) UpdateJobStatus(ctx context.Context, jobID string, status string) error {
	exec := r.sqlExec(ctx)
	_, err := exec.ExecContext(ctx, `UPDATE image_studio_jobs SET status = $2 WHERE id = $1::uuid`, jobID, status)
	if err != nil {
		return fmt.Errorf("update image studio job status: %w", err)
	}
	return nil
}

func (r *imageStudioRepository) DeleteExpiredJobsBefore(ctx context.Context, before time.Time) (int64, error) {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		DELETE FROM image_studio_jobs
		WHERE expires_at IS NOT NULL AND expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired image studio jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *imageStudioRepository) HasCompletedJob(ctx context.Context, userID int64) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1 FROM image_studio_jobs WHERE user_id = $1 AND status = 'completed' LIMIT 1
		)`, []any{userID}, &exists)
	if err != nil {
		return false, fmt.Errorf("has completed image studio job: %w", err)
	}
	return exists, nil
}
