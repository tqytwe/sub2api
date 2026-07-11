package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

func (r *imageStudioRepository) InsertAssets(ctx context.Context, jobID string, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	exec := r.sqlExec(ctx)
	for i, url := range urls {
		if _, err := exec.ExecContext(ctx, `
			INSERT INTO image_studio_assets (job_id, url, sort_order)
			VALUES ($1::uuid, $2, $3)`, jobID, url, i); err != nil {
			return fmt.Errorf("insert image studio asset: %w", err)
		}
	}
	return nil
}

func (r *imageStudioRepository) GetJob(ctx context.Context, userID int64, jobID string) (*service.ImageStudioJob, error) {
	exec := r.sqlExec(ctx)
	var job service.ImageStudioJob
	var actualCost sql.NullFloat64
	var errMsg sql.NullString
	var apiKeyID sql.NullInt64
	var expiresAt sql.NullTime
	err := scanSingleRow(ctx, exec, `
		SELECT id::text, user_id, template_id, prompt_hash, size, count, status,
		       estimated_cost, actual_cost, api_key_id, error_message, created_at, expires_at
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2`,
		[]any{jobID, userID},
		&job.ID, &job.UserID, &job.TemplateID, &job.PromptHash, &job.Size, &job.Count, &job.Status,
		&job.EstimatedCost, &actualCost, &apiKeyID, &errMsg, &job.CreatedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrImageStudioJobNotFound
		}
		return nil, fmt.Errorf("get image studio job: %w", err)
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
	assets, err := r.listAssets(ctx, jobID)
	if err != nil {
		return nil, err
	}
	job.Assets = assets
	return &job, nil
}

func (r *imageStudioRepository) listAssets(ctx context.Context, jobID string) ([]service.ImageStudioAsset, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id::text, url, sort_order
		FROM image_studio_assets
		WHERE job_id = $1::uuid
		ORDER BY sort_order ASC`, jobID)
	if err != nil {
		return nil, fmt.Errorf("list image studio assets: %w", err)
	}
	defer rows.Close()
	out := make([]service.ImageStudioAsset, 0)
	for rows.Next() {
		var a service.ImageStudioAsset
		if err := rows.Scan(&a.ID, &a.URL, &a.SortOrder); err != nil {
			return nil, fmt.Errorf("scan image studio asset: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) ListJobs(ctx context.Context, userID int64, limit int) ([]service.ImageStudioJob, error) {
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
	defer rows.Close()
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
