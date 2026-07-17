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
	"github.com/lib/pq"
)

type imageStudioRepository struct {
	client *dbent.Client
	sql    sqlExecutor
	db     *sql.DB
}

func NewImageStudioRepository(client *dbent.Client, db *sql.DB) service.ImageStudioRepository {
	return &imageStudioRepository{client: client, sql: db, db: db}
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

func (r *imageStudioRepository) CreatePendingJob(
	ctx context.Context,
	job *service.ImageStudioJob,
	items []service.ImageStudioItem,
	reserve func(context.Context) error,
) error {
	_, _, err := r.createPendingJob(ctx, job, items, reserve, false)
	return err
}

func (r *imageStudioRepository) CreatePendingJobIdempotent(
	ctx context.Context,
	job *service.ImageStudioJob,
	items []service.ImageStudioItem,
	reserve func(context.Context) error,
) (existingJobID string, created bool, err error) {
	if job == nil ||
		strings.TrimSpace(job.IdempotencyKeyHash) == "" ||
		strings.TrimSpace(job.IdempotencyFingerprint) == "" {
		return "", false, service.ErrIdempotencyKeyInvalid
	}
	return r.createPendingJob(ctx, job, items, reserve, true)
}

func (r *imageStudioRepository) FindJobByIdempotency(
	ctx context.Context,
	userID int64,
	keyHash string,
	fingerprint string,
) (string, bool, error) {
	var jobID, storedFingerprint string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id::text, idempotency_fingerprint
		FROM image_studio_jobs
		WHERE user_id = $1
		  AND idempotency_key_hash = $2`,
		[]any{userID, keyHash},
		&jobID,
		&storedFingerprint,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find idempotent image studio job: %w", err)
	}
	if storedFingerprint != fingerprint {
		return "", false, service.ErrIdempotencyKeyConflict
	}
	return jobID, true, nil
}

func (r *imageStudioRepository) createPendingJob(
	ctx context.Context,
	job *service.ImageStudioJob,
	items []service.ImageStudioItem,
	reserve func(context.Context) error,
	idempotent bool,
) (existingJobID string, created bool, err error) {
	if r.db == nil {
		return "", false, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, fmt.Errorf("begin create image studio job: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, job.UserID); err != nil {
		return "", false, fmt.Errorf("lock image studio user: %w", err)
	}
	if idempotent {
		var existingFingerprint string
		err := tx.QueryRowContext(ctx, `
			SELECT id::text, idempotency_fingerprint
			FROM image_studio_jobs
			WHERE user_id = $1
			  AND idempotency_key_hash = $2`,
			job.UserID,
			job.IdempotencyKeyHash,
		).Scan(&existingJobID, &existingFingerprint)
		switch {
		case err == nil:
			if existingFingerprint != job.IdempotencyFingerprint {
				return "", false, service.ErrIdempotencyKeyConflict
			}
			return existingJobID, false, nil
		case errors.Is(err, sql.ErrNoRows):
		default:
			return "", false, fmt.Errorf("lookup idempotent image studio job: %w", err)
		}
	}
	var active int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1 AND status IN ('pending', 'running')`, job.UserID).Scan(&active); err != nil {
		return "", false, fmt.Errorf("count active image studio jobs: %w", err)
	}
	if active >= 2 {
		return "", false, service.ErrImageStudioConcurrentJobLimit
	}
	if reserve != nil {
		if err := reserve(withUsageBillingTransaction(ctx, tx)); err != nil {
			return "", false, err
		}
	}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO image_studio_jobs (
			id, user_id, template_id, prompt_hash, request_payload_encrypted,
			model, quality, size, count, status, estimated_cost, actual_cost,
			api_key_id, hold_amount, hold_id, success_count, fail_count, expires_at,
			idempotency_key_hash, idempotency_fingerprint
		)
		VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, NULLIF($19, ''), NULLIF($20, '')
		)
		RETURNING created_at`,
		job.ID, job.UserID, job.TemplateID, job.PromptHash, job.RequestPayloadEncrypted,
		job.Model, job.Quality, job.Size, job.Count, job.Status, job.EstimatedCost, job.ActualCost,
		job.APIKeyID, job.HoldAmount, job.HoldID, job.SuccessCount, job.FailCount, job.ExpiresAt,
		job.IdempotencyKeyHash, job.IdempotencyFingerprint,
	).Scan(&job.CreatedAt); err != nil {
		return "", false, fmt.Errorf("insert durable image studio job: %w", err)
	}
	if len(job.JobReferences) > 4 {
		return "", false, service.ErrImageStudioReferenceLimit
	}
	for _, reference := range job.JobReferences {
		if strings.TrimSpace(reference.ID) == "" ||
			(reference.JobID != "" && reference.JobID != job.ID) ||
			strings.TrimSpace(reference.StorageKey) == "" ||
			strings.TrimSpace(reference.ContentType) == "" ||
			reference.ByteSize <= 0 ||
			reference.SortOrder < 0 {
			return "", false, errors.New("invalid image studio job reference")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO image_studio_job_references (
				id, job_id, storage_key, content_type, byte_size, sort_order
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`,
			reference.ID,
			job.ID,
			reference.StorageKey,
			reference.ContentType,
			reference.ByteSize,
			reference.SortOrder,
		); err != nil {
			return "", false, fmt.Errorf("insert image studio job reference: %w", err)
		}
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO image_studio_items (id, job_id, sort_order, status)
			VALUES ($1::uuid, $2::uuid, $3, $4)`,
			item.ID, job.ID, item.SortOrder, item.Status); err != nil {
			return "", false, fmt.Errorf("insert image studio item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", false, fmt.Errorf("commit create image studio job: %w", err)
	}
	return "", true, nil
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
			INSERT INTO image_studio_assets (
				id, job_id, url, sort_order, storage_key, content_type, byte_size,
				width, height, thumbnail_storage_key, thumbnail_content_type, thumbnail_byte_size
			)
			VALUES (
				$1::uuid, $2::uuid, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), $7,
				NULLIF($8, 0), NULLIF($9, 0), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, 0)
			)`,
			id, jobID, url, asset.SortOrder, storageKey, asset.ContentType, asset.ByteSize,
			asset.Width, asset.Height, asset.ThumbnailStorageKey, asset.ThumbnailContentType,
			asset.ThumbnailByteSize); err != nil {
			return fmt.Errorf("insert image studio asset: %w", err)
		}
	}
	return nil
}

func (r *imageStudioRepository) GetJob(ctx context.Context, userID int64, jobID string) (*service.ImageStudioJob, error) {
	job, err := queryFullImageStudioJob(ctx, r.sqlExec(ctx), fullImageStudioJobSelect+`
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
	items, err := r.listItems(ctx, jobID)
	if err != nil {
		return nil, err
	}
	job.Items = items
	return job, nil
}

func (r *imageStudioRepository) GetActiveJob(ctx context.Context, userID int64) (*service.ImageStudioJob, error) {
	job, err := queryFullImageStudioJob(ctx, r.sqlExec(ctx), fullImageStudioJobSelect+`
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

func (r *imageStudioRepository) ListActiveJobs(ctx context.Context, userID int64) ([]service.ImageStudioJob, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, fullImageStudioJobSelect+`
		WHERE user_id = $1 AND status IN ('pending', 'running')
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list active image studio jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.ImageStudioJob, 0, 2)
	ids := make([]string, 0, 2)
	for rows.Next() {
		job, err := scanFullImageStudioJob(ctx, rows)
		if err != nil {
			return nil, fmt.Errorf("scan active image studio job: %w", err)
		}
		ids = append(ids, job.ID)
		out = append(out, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	assetsByJob, err := r.listAssetsForJobIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	itemsByJob, err := r.listItemsForJobIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Assets = assetsByJob[out[i].ID]
		out[i].Items = itemsByJob[out[i].ID]
	}
	return out, nil
}

func (r *imageStudioRepository) GetAsset(ctx context.Context, userID int64, assetID string) (*service.ImageStudioAsset, error) {
	exec := r.sqlExec(ctx)
	var asset service.ImageStudioAsset
	var url sql.NullString
	var storageKey sql.NullString
	var contentType sql.NullString
	var thumbnailStorageKey sql.NullString
	var thumbnailContentType sql.NullString
	err := scanSingleRow(ctx, exec, `
		SELECT a.id::text, COALESCE(a.url, ''), a.sort_order,
		       COALESCE(a.storage_key, ''), COALESCE(a.content_type, ''), COALESCE(a.byte_size, 0),
		       COALESCE(a.width, 0), COALESCE(a.height, 0),
		       COALESCE(a.thumbnail_storage_key, ''), COALESCE(a.thumbnail_content_type, ''),
		       COALESCE(a.thumbnail_byte_size, 0)
		FROM image_studio_assets a
		JOIN image_studio_jobs j ON j.id = a.job_id
		WHERE a.id = $1::uuid
		  AND j.user_id = $2
		  AND j.status IN ('completed', 'partial')`,
		[]any{assetID, userID},
		&asset.ID, &url, &asset.SortOrder, &storageKey, &contentType, &asset.ByteSize,
		&asset.Width, &asset.Height, &thumbnailStorageKey, &thumbnailContentType,
		&asset.ThumbnailByteSize)
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
	if thumbnailStorageKey.Valid {
		asset.ThumbnailStorageKey = thumbnailStorageKey.String
	}
	if thumbnailContentType.Valid {
		asset.ThumbnailContentType = thumbnailContentType.String
	}
	asset.AspectRatio = imageStudioAssetAspectRatio(asset.Width, asset.Height)
	return &asset, nil
}

func (r *imageStudioRepository) listAssets(ctx context.Context, jobID string) (result []service.ImageStudioAsset, err error) {
	byJob, err := r.listAssetsForJobIDs(ctx, []string{jobID})
	if err != nil {
		return nil, err
	}
	return byJob[jobID], nil
}

func (r *imageStudioRepository) listAssetsForJobIDs(
	ctx context.Context,
	jobIDs []string,
) (result map[string][]service.ImageStudioAsset, err error) {
	out := make(map[string][]service.ImageStudioAsset, len(jobIDs))
	if len(jobIDs) == 0 {
		return out, nil
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT a.job_id::text, a.id::text, COALESCE(a.url, ''), a.sort_order,
		       COALESCE(a.storage_key, ''), COALESCE(a.content_type, ''), COALESCE(a.byte_size, 0),
		       COALESCE(a.width, 0), COALESCE(a.height, 0),
		       COALESCE(a.thumbnail_storage_key, ''), COALESCE(a.thumbnail_content_type, ''),
		       COALESCE(a.thumbnail_byte_size, 0)
		FROM image_studio_assets a
		JOIN image_studio_jobs j ON j.id = a.job_id
		WHERE a.job_id = ANY($1::uuid[])
		  AND j.status IN ('completed', 'partial')
		ORDER BY a.job_id, a.sort_order ASC`, pq.Array(jobIDs))
	if err != nil {
		return nil, fmt.Errorf("list image studio assets: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var jobID string
		var a service.ImageStudioAsset
		if err := rows.Scan(
			&jobID, &a.ID, &a.URL, &a.SortOrder, &a.StorageKey, &a.ContentType, &a.ByteSize,
			&a.Width, &a.Height, &a.ThumbnailStorageKey, &a.ThumbnailContentType,
			&a.ThumbnailByteSize,
		); err != nil {
			return nil, fmt.Errorf("scan image studio asset: %w", err)
		}
		a.AspectRatio = imageStudioAssetAspectRatio(a.Width, a.Height)
		out[jobID] = append(out[jobID], a)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) listItems(ctx context.Context, jobID string) (result []service.ImageStudioItem, err error) {
	byJob, err := r.listItemsForJobIDs(ctx, []string{jobID})
	if err != nil {
		return nil, err
	}
	return byJob[jobID], nil
}

func (r *imageStudioRepository) listItemsForJobIDs(
	ctx context.Context,
	jobIDs []string,
) (result map[string][]service.ImageStudioItem, err error) {
	out := make(map[string][]service.ImageStudioItem, len(jobIDs))
	if len(jobIDs) == 0 {
		return out, nil
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id::text, job_id::text, sort_order, status, actual_cost, error,
		       asset_id::text, attempt_count, started_at, finished_at
		FROM image_studio_items
		WHERE job_id = ANY($1::uuid[])
		ORDER BY job_id, sort_order ASC`, pq.Array(jobIDs))
	if err != nil {
		return nil, fmt.Errorf("list image studio items: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var item service.ImageStudioItem
		var actual sql.NullFloat64
		var itemErr, assetID sql.NullString
		var startedAt, finishedAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.JobID, &item.SortOrder, &item.Status, &actual, &itemErr,
			&assetID, &item.AttemptCount, &startedAt, &finishedAt,
		); err != nil {
			return nil, err
		}
		if actual.Valid {
			item.ActualCost = &actual.Float64
		}
		if itemErr.Valid {
			item.Error = itemErr.String
		}
		if assetID.Valid {
			item.AssetID = &assetID.String
		}
		if startedAt.Valid {
			item.StartedAt = &startedAt.Time
		}
		if finishedAt.Valid {
			item.FinishedAt = &finishedAt.Time
		}
		out[item.JobID] = append(out[item.JobID], item)
	}
	return out, rows.Err()
}

func (r *imageStudioRepository) CreateReference(ctx context.Context, reference *service.ImageStudioReference) error {
	if reference == nil {
		return errors.New("image studio reference is nil")
	}
	if r.db == nil {
		return errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create image studio reference: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, reference.UserID); err != nil {
		return fmt.Errorf("lock image studio reference owner: %w", err)
	}
	var pendingCount int
	var pendingBytes int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)::int, COALESCE(SUM(byte_size), 0)::bigint
		FROM image_studio_references
		WHERE user_id = $1
		  AND expires_at > NOW()`, reference.UserID).Scan(&pendingCount, &pendingBytes); err != nil {
		return fmt.Errorf("count image studio references: %w", err)
	}
	if pendingCount >= service.ImageStudioReferenceMaxPendingCount ||
		pendingBytes+reference.ByteSize > service.ImageStudioReferenceMaxPendingBytes {
		return service.ErrImageStudioReferenceQuota
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO image_studio_references (
			id, user_id, storage_key, original_filename, content_type, byte_size, expires_at
		)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		RETURNING created_at`,
		reference.ID,
		reference.UserID,
		reference.StorageKey,
		reference.OriginalFilename,
		reference.ContentType,
		reference.ByteSize,
		reference.ExpiresAt,
	).Scan(&reference.CreatedAt)
	if err != nil {
		return fmt.Errorf("create image studio reference: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit image studio reference: %w", err)
	}
	return nil
}

func (r *imageStudioRepository) ListReferencesByID(ctx context.Context, userID int64, ids []string) (result []service.ImageStudioReference, err error) {
	if len(ids) == 0 {
		return []service.ImageStudioReference{}, nil
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT r.id::text, r.user_id, r.storage_key, r.original_filename,
		       r.content_type, r.byte_size, r.created_at, r.expires_at
		FROM unnest($2::uuid[]) WITH ORDINALITY requested(id, ord)
		JOIN image_studio_references r ON r.id = requested.id
		WHERE r.user_id = $1 AND r.expires_at > NOW()
		ORDER BY requested.ord`, userID, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("list image studio references: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.ImageStudioReference, 0, len(ids))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *imageStudioRepository) ListJobs(ctx context.Context, userID int64, limit int) (result []service.ImageStudioJob, err error) {
	if limit <= 0 {
		limit = 20
	}
	jobs, _, err := r.ListJobsPage(ctx, userID, 1, limit)
	return jobs, err
}

func (r *imageStudioRepository) ListJobsPage(
	ctx context.Context,
	userID int64,
	page, pageSize int,
) (result []service.ImageStudioJob, total int, err error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	exec := r.sqlExec(ctx)
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1
		  AND status IN ('completed', 'partial', 'failed', 'cancelled')`, []any{userID}, &total); err != nil {
		return nil, 0, fmt.Errorf("count image studio jobs: %w", err)
	}
	rows, err := exec.QueryContext(ctx, fullImageStudioJobSelect+`
		WHERE user_id = $1
		  AND status IN ('completed', 'partial', 'failed', 'cancelled')
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list image studio jobs page: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
			total = 0
		}
	}()
	jobs := make([]service.ImageStudioJob, 0, pageSize)
	ids := make([]string, 0, pageSize)
	for rows.Next() {
		job, err := scanFullImageStudioJob(ctx, rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan image studio job: %w", err)
		}
		ids = append(ids, job.ID)
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, err
	}
	assetsByJob, err := r.listAssetsForJobIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	itemsByJob, err := r.listItemsForJobIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range jobs {
		jobs[i].Assets = assetsByJob[jobs[i].ID]
		jobs[i].Items = itemsByJob[jobs[i].ID]
	}
	return jobs, total, nil
}

func imageStudioAssetAspectRatio(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	a, b := width, height
	for b != 0 {
		a, b = b, a%b
	}
	if a <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", width/a, height/a)
}

func (r *imageStudioRepository) ListAssetStorageKeysForJob(ctx context.Context, jobID string) (result []string, err error) {
	return listImageStudioAssetStorageKeys(ctx, r.sqlExec(ctx), jobID)
}

func listImageStudioAssetStorageKeys(
	ctx context.Context,
	exec sqlExecutor,
	jobID string,
) (result []string, err error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT keys.storage_key
		FROM image_studio_assets a
		CROSS JOIN LATERAL (
			VALUES (a.storage_key), (a.thumbnail_storage_key)
		) AS keys(storage_key)
		WHERE a.job_id = $1::uuid
		  AND keys.storage_key IS NOT NULL
		  AND keys.storage_key <> ''`, jobID)
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
		WHERE expires_at IS NOT NULL AND expires_at < $1
		  AND status IN ('completed', 'failed', 'cancelled', 'partial')`, before)
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

func (r *imageStudioRepository) DeleteJobWithStorageKeys(
	ctx context.Context,
	userID int64,
	jobID string,
) (_ []string, err error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin delete image studio job: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()

	var lockedJobID, status string
	if err := scanSingleRow(ctx, tx, `
		SELECT id::text, status
		FROM image_studio_jobs
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`, []any{jobID, userID}, &lockedJobID, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrImageStudioJobNotFound
		}
		return nil, fmt.Errorf("lock image studio job for delete: %w", err)
	}
	if status == service.ImageStudioJobStatusPending || status == service.ImageStudioJobStatusRunning {
		return nil, service.ErrImageStudioJobRunning
	}

	keys, err := listImageStudioAssetStorageKeys(ctx, tx, lockedJobID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO image_studio_object_deletions (user_id, job_id, storage_key)
		SELECT $1, $2::uuid, pending.storage_key
		FROM (
			SELECT keys.storage_key
			FROM image_studio_assets a
			CROSS JOIN LATERAL (
				VALUES (a.storage_key), (a.thumbnail_storage_key)
			) AS keys(storage_key)
			WHERE a.job_id = $2::uuid
			  AND keys.storage_key IS NOT NULL
			  AND keys.storage_key <> ''
			UNION
			SELECT storage_key
			FROM image_studio_job_references
			WHERE job_id = $2::uuid
			  AND storage_key <> ''
		) pending
		ON CONFLICT (job_id, storage_key) DO NOTHING`,
		userID,
		jobID,
	); err != nil {
		return nil, fmt.Errorf("enqueue image studio object deletions: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM image_studio_jobs WHERE id = $1::uuid AND user_id = $2`, jobID, userID)
	if err != nil {
		return nil, fmt.Errorf("delete image studio job: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("read deleted image studio job count: %w", err)
	}
	if n == 0 {
		return nil, service.ErrImageStudioJobNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit delete image studio job: %w", err)
	}
	return keys, nil
}

func (r *imageStudioRepository) ListPendingObjectDeletions(
	ctx context.Context,
	limit int,
) (result []string, err error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT storage_key
		FROM image_studio_object_deletions
		ORDER BY updated_at, id
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		result = append(result, key)
	}
	return result, rows.Err()
}

func (r *imageStudioRepository) AcknowledgeObjectDeletion(ctx context.Context, storageKey string) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		DELETE FROM image_studio_object_deletions
		WHERE storage_key = $1`, storageKey)
	return err
}

func (r *imageStudioRepository) RecordObjectDeletionFailure(
	ctx context.Context,
	storageKey string,
	deleteErr error,
) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE image_studio_object_deletions
		SET attempts = attempts + 1,
		    last_error = $2,
		    updated_at = NOW()
		WHERE storage_key = $1`,
		storageKey,
		sanitizeImageStudioObjectDeletionError(deleteErr),
	)
	return err
}

func (r *imageStudioRepository) FilterTrackedObjectStorageKeys(
	ctx context.Context,
	storageKeys []string,
) (result map[string]struct{}, err error) {
	tracked := make(map[string]struct{}, len(storageKeys))
	if len(storageKeys) == 0 {
		return tracked, nil
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT storage_key
		FROM image_studio_references
		WHERE storage_key = ANY($1::text[])
		UNION
		SELECT storage_key
		FROM image_studio_job_references
		WHERE storage_key = ANY($1::text[])
		UNION
		SELECT keys.storage_key
		FROM image_studio_assets a
		CROSS JOIN LATERAL (
			VALUES (a.storage_key), (a.thumbnail_storage_key)
		) AS keys(storage_key)
		WHERE keys.storage_key = ANY($1::text[])`,
		pq.Array(storageKeys),
	)
	if err != nil {
		return nil, fmt.Errorf("filter tracked image studio storage keys: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		tracked[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tracked, nil
}

func sanitizeImageStudioObjectDeletionError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(strings.ReplaceAll(err.Error(), "\x00", ""))
	if len(message) > 1024 {
		message = message[:1024]
	}
	return message
}

func (r *imageStudioRepository) ClaimNextJob(
	ctx context.Context,
	leaseOwner string,
	now time.Time,
	leaseDuration time.Duration,
) (_ *service.ImageStudioJob, err error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	var jobID string
	var userID int64
	err = tx.QueryRowContext(ctx, `
		SELECT j.id::text, j.user_id
		FROM image_studio_jobs j
		WHERE (
			(j.status = 'pending' AND j.cancel_requested_at IS NULL)
			OR (j.status = 'running' AND (j.lease_expires_at IS NULL OR j.lease_expires_at <= $1))
		)
		  AND (
			SELECT COUNT(*)
			FROM image_studio_jobs active
			WHERE active.user_id = j.user_id
			  AND active.status = 'running'
			  AND active.id <> j.id
			  AND active.lease_expires_at > $1
		  ) < 2
		ORDER BY j.created_at ASC
		FOR UPDATE OF j SKIP LOCKED
		LIMIT 1`, now).Scan(&jobID, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim image studio job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		return nil, fmt.Errorf("lock image studio claim user: %w", err)
	}
	var running int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1
		  AND status = 'running'
		  AND id <> $2::uuid
		  AND lease_expires_at > $3`, userID, jobID, now).Scan(&running); err != nil {
		return nil, fmt.Errorf("count running image studio claims: %w", err)
	}
	if running >= 2 {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE image_studio_items
		SET status = CASE
				WHEN status = 'persisting' AND attempt_count >= 3 THEN 'failed'
				WHEN status = 'persisting' THEN 'persisting'
				WHEN EXISTS (
					SELECT 1 FROM image_studio_jobs
					WHERE id = $1::uuid AND cancel_requested_at IS NOT NULL
				) THEN 'cancelled'
				WHEN attempt_count >= 3 THEN 'failed'
				ELSE 'pending'
			END,
		    error = CASE
				WHEN status = 'persisting' AND attempt_count >= 3
					THEN COALESCE(error, 'image studio item persistence retry limit exceeded')
				WHEN status = 'persisting' THEN error
				WHEN attempt_count >= 3 THEN COALESCE(error, 'image studio item retry limit exceeded')
				ELSE error
			END,
		    started_at = CASE
				WHEN status = 'persisting' THEN started_at
				WHEN EXISTS (
					SELECT 1 FROM image_studio_jobs
					WHERE id = $1::uuid AND cancel_requested_at IS NOT NULL
				) OR attempt_count >= 3 THEN started_at
				ELSE NULL
			END,
		    finished_at = CASE
				WHEN status = 'persisting' AND attempt_count >= 3 THEN $2::timestamptz
				WHEN status = 'persisting' THEN NULL::timestamptz
				WHEN EXISTS (
					SELECT 1 FROM image_studio_jobs
					WHERE id = $1::uuid AND cancel_requested_at IS NOT NULL
				) OR attempt_count >= 3 THEN $2::timestamptz
				ELSE NULL::timestamptz
			END,
			    actual_cost = CASE
					WHEN status = 'persisting' AND attempt_count >= 3
						THEN COALESCE(actual_cost, checkpoint_actual_cost)
					ELSE actual_cost
				END,
		    checkpoint_data = CASE
				WHEN status = 'persisting' AND attempt_count >= 3 THEN NULL
				ELSE checkpoint_data
			END,
		    checkpoint_content_type = CASE
				WHEN status = 'persisting' AND attempt_count >= 3 THEN NULL
				ELSE checkpoint_content_type
			END,
		    checkpoint_actual_cost = CASE
				WHEN status = 'persisting' AND attempt_count >= 3 THEN NULL
				ELSE checkpoint_actual_cost
			END
		WHERE job_id = $1::uuid AND status IN ('running', 'persisting')`, jobID, now); err != nil {
		return nil, err
	}
	leaseExpiresAt := now.Add(leaseDuration)
	if _, err := tx.ExecContext(ctx, `
		UPDATE image_studio_jobs
		SET status = 'running',
		    started_at = COALESCE(started_at, $2),
		    heartbeat_at = $2,
		    lease_owner = $3,
		    lease_expires_at = $4
		WHERE id = $1::uuid`, jobID, now, leaseOwner, leaseExpiresAt); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	job, err := r.getJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *imageStudioRepository) HeartbeatJob(ctx context.Context, jobID, leaseOwner string, now time.Time, leaseDuration time.Duration) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE image_studio_jobs
		SET heartbeat_at = $3, lease_expires_at = $4
		WHERE id = $1::uuid AND status = 'running' AND lease_owner = $2
		  AND lease_expires_at > $3`,
		jobID, leaseOwner, now, now.Add(leaseDuration))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrImageStudioLeaseLost
	}
	return nil
}

func (r *imageStudioRepository) ClaimNextItem(ctx context.Context, jobID, leaseOwner string, now time.Time) (_ *service.ImageStudioItem, err error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var item service.ImageStudioItem
	var checkpointContentType sql.NullString
	var checkpointActual sql.NullFloat64
	err = tx.QueryRowContext(ctx, `
		SELECT i.id::text, i.job_id::text, i.sort_order, i.status, i.attempt_count,
		       i.checkpoint_data, i.checkpoint_content_type, i.checkpoint_actual_cost
		FROM image_studio_items i
		JOIN image_studio_jobs j ON j.id = i.job_id
			WHERE i.job_id = $1::uuid
			  AND (
				(i.status = 'persisting' AND i.attempt_count < 3)
				OR (i.status = 'pending' AND j.cancel_requested_at IS NULL)
		  )
		  AND j.status = 'running' AND j.lease_owner = $2
		  AND j.lease_expires_at > $3
		ORDER BY CASE WHEN i.status = 'persisting' THEN 0 ELSE 1 END, i.sort_order
		FOR UPDATE OF i SKIP LOCKED
		LIMIT 1`, jobID, leaseOwner, now).
		Scan(
			&item.ID,
			&item.JobID,
			&item.SortOrder,
			&item.Status,
			&item.AttemptCount,
			&item.CheckpointData,
			&checkpointContentType,
			&checkpointActual,
		)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if checkpointContentType.Valid {
		item.CheckpointContentType = checkpointContentType.String
	}
	if checkpointActual.Valid {
		item.CheckpointActualCost = &checkpointActual.Float64
	}
	claimedStatus := item.Status
	if _, err := tx.ExecContext(ctx, `
		UPDATE image_studio_items
		SET status = CASE WHEN status = 'pending' THEN 'running' ELSE status END,
		    started_at = COALESCE(started_at, $2),
		    attempt_count = attempt_count + 1
		WHERE id = $1::uuid`, item.ID, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if claimedStatus == service.ImageStudioItemStatusPending {
		item.Status = service.ImageStudioItemStatusRunning
	}
	item.AttemptCount++
	item.StartedAt = &now
	return &item, nil
}

func (r *imageStudioRepository) CheckpointItem(
	ctx context.Context,
	jobID, itemID, leaseOwner string,
	image service.ImageStudioImagePayload,
	actualCost float64,
	now time.Time,
) (err error) {
	if len(image.Data) == 0 {
		return errors.New("image studio checkpoint data is empty")
	}
	if actualCost < 0 {
		actualCost = 0
	}
	if r.db == nil {
		return errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	var lockedJobID string
	var cancelRequestedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, cancel_requested_at
		FROM image_studio_jobs
		WHERE id = $1::uuid
		  AND status = 'running'
		  AND lease_owner = $2
		  AND lease_expires_at > $3
		FOR UPDATE`, jobID, leaseOwner, now).Scan(&lockedJobID, &cancelRequestedAt); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return service.ErrImageStudioLeaseLost
	}
	if cancelRequestedAt.Valid {
		res, err := tx.ExecContext(ctx, `
			UPDATE image_studio_items
			SET status = 'cancelled',
			    actual_cost = $3,
			    error = NULL,
			    finished_at = $4,
			    checkpoint_data = NULL,
			    checkpoint_content_type = NULL,
			    checkpoint_actual_cost = NULL
			WHERE id = $1::uuid
			  AND job_id = $2::uuid
			  AND status = 'running'`,
			itemID, jobID, actualCost, now)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return service.ErrImageStudioLeaseLost
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return service.ErrImageStudioCheckpointCancelled
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE image_studio_items
		SET status = 'persisting',
		    checkpoint_data = $3,
		    checkpoint_content_type = NULLIF($4, ''),
		    checkpoint_actual_cost = $5,
		    error = NULL,
		    finished_at = NULL
		WHERE id = $1::uuid
		  AND job_id = $2::uuid
		  AND status = 'running'`,
		itemID, jobID, image.Data, image.ContentType, actualCost)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrImageStudioLeaseLost
	}
	return tx.Commit()
}

func (r *imageStudioRepository) RetryItem(
	ctx context.Context,
	jobID, itemID, leaseOwner string,
	now time.Time,
) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE image_studio_items i
		SET status = 'pending',
		    actual_cost = NULL,
		    error = NULL,
		    asset_id = NULL,
		    finished_at = NULL,
		    checkpoint_data = NULL,
		    checkpoint_content_type = NULL,
		    checkpoint_actual_cost = NULL
		FROM image_studio_jobs j
		WHERE i.id = $1::uuid
		  AND i.job_id = $2::uuid
		  AND i.status = 'running'
		  AND i.attempt_count < $5
		  AND j.id = i.job_id
		  AND j.status = 'running'
		  AND j.cancel_requested_at IS NULL
		  AND j.lease_owner = $3
		  AND j.lease_expires_at > $4`,
		itemID, jobID, leaseOwner, now, service.ImageStudioMaxItemAttempts)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrImageStudioLeaseLost
	}
	return nil
}

func (r *imageStudioRepository) GetItem(ctx context.Context, jobID, itemID string) (*service.ImageStudioItem, error) {
	var item service.ImageStudioItem
	var actual, checkpointActual sql.NullFloat64
	var itemErr, assetID, checkpointContentType sql.NullString
	var startedAt, finishedAt sql.NullTime
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT id::text, job_id::text, sort_order, status, actual_cost, error,
		       asset_id::text, attempt_count, started_at, finished_at,
		       checkpoint_data, checkpoint_content_type, checkpoint_actual_cost
		FROM image_studio_items
		WHERE id = $1::uuid AND job_id = $2::uuid`, []any{itemID, jobID},
		&item.ID, &item.JobID, &item.SortOrder, &item.Status, &actual, &itemErr,
		&assetID, &item.AttemptCount, &startedAt, &finishedAt,
		&item.CheckpointData, &checkpointContentType, &checkpointActual,
	)
	if err != nil {
		return nil, err
	}
	if actual.Valid {
		item.ActualCost = &actual.Float64
	}
	if itemErr.Valid {
		item.Error = itemErr.String
	}
	if assetID.Valid {
		item.AssetID = &assetID.String
	}
	if startedAt.Valid {
		item.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		item.FinishedAt = &finishedAt.Time
	}
	if checkpointContentType.Valid {
		item.CheckpointContentType = checkpointContentType.String
	}
	if checkpointActual.Valid {
		item.CheckpointActualCost = &checkpointActual.Float64
	}
	return &item, nil
}

func (r *imageStudioRepository) CompleteItem(
	ctx context.Context,
	jobID, itemID, leaseOwner, status string,
	actualCost *float64,
	errMsg string,
	asset *service.ImageStudioAssetRecord,
	now time.Time,
) (err error) {
	if r.db == nil {
		return errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedJobID string
	var cancelRequestedAt sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text, cancel_requested_at
		FROM image_studio_jobs
			WHERE id = $1::uuid
			  AND status = 'running'
			  AND lease_owner = $2
		  AND lease_expires_at > $3
		FOR UPDATE`, jobID, leaseOwner, now).Scan(&lockedJobID, &cancelRequestedAt); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return service.ErrImageStudioLeaseLost
	}
	var lockedItemStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM image_studio_items
		WHERE id = $1::uuid
		  AND job_id = $2::uuid
		  AND status IN ('running', 'persisting')
		FOR UPDATE`, itemID, jobID).Scan(&lockedItemStatus); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return service.ErrImageStudioLeaseLost
	}
	completingPersistedCheckpoint := lockedItemStatus == service.ImageStudioItemStatusPersisting &&
		status == service.ImageStudioItemStatusSuccess
	if cancelRequestedAt.Valid && !completingPersistedCheckpoint {
		return service.ErrImageStudioLeaseLost
	}
	var assetID any
	if status == service.ImageStudioItemStatusSuccess {
		if asset == nil {
			return errors.New("successful image studio item requires asset")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO image_studio_assets (
				id, job_id, url, sort_order, storage_key, content_type, byte_size,
				width, height, thumbnail_storage_key, thumbnail_content_type, thumbnail_byte_size
			)
			VALUES (
				$1::uuid, $2::uuid, NULLIF($3, ''), $4, NULLIF($5, ''), NULLIF($6, ''), $7,
				NULLIF($8, 0), NULLIF($9, 0), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, 0)
			)`,
			asset.ID, jobID, asset.URL, asset.SortOrder, asset.StorageKey, asset.ContentType,
			asset.ByteSize, asset.Width, asset.Height, asset.ThumbnailStorageKey,
			asset.ThumbnailContentType, asset.ThumbnailByteSize); err != nil {
			return err
		}
		assetID = asset.ID
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE image_studio_items
		SET status = $3, actual_cost = $4, error = NULLIF($5, ''),
		    asset_id = $6::uuid, finished_at = $7,
		    checkpoint_data = NULL, checkpoint_content_type = NULL,
		    checkpoint_actual_cost = NULL
		WHERE id = $1::uuid AND job_id = $2::uuid
		  AND status = $8`,
		itemID, jobID, status, actualCost, errMsg, assetID, now, lockedItemStatus)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrImageStudioLeaseLost
	}
	return tx.Commit()
}

func (r *imageStudioRepository) RequestCancel(
	ctx context.Context,
	userID int64,
	jobID string,
	now time.Time,
	release func(context.Context, *service.ImageStudioJob) error,
) (_ *service.ImageStudioJob, err error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanFullImageStudioJob(ctx, tx.QueryRowContext(ctx, fullImageStudioJobSelect+`
		WHERE id = $1::uuid AND user_id = $2
		FOR UPDATE`, jobID, userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrImageStudioJobNotFound
		}
		return nil, err
	}
	switch job.Status {
	case service.ImageStudioJobStatusCompleted, service.ImageStudioJobStatusFailed,
		service.ImageStudioJobStatusPartial:
	case service.ImageStudioJobStatusCancelled:
		if release != nil {
			if err := release(withUsageBillingTransaction(ctx, tx), job); err != nil {
				return nil, err
			}
		}
	case service.ImageStudioJobStatusPending:
		if release != nil {
			if err := release(withUsageBillingTransaction(ctx, tx), job); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE image_studio_jobs
			SET status = 'cancelled', cancel_requested_at = COALESCE(cancel_requested_at, $2),
			    finished_at = COALESCE(finished_at, $2), lease_owner = NULL, lease_expires_at = NULL
			WHERE id = $1::uuid`, jobID, now); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE image_studio_items
			SET status = 'cancelled', finished_at = $2
			WHERE job_id = $1::uuid AND status IN ('pending', 'running')`, jobID, now); err != nil {
			return nil, err
		}
	default:
		if _, err := tx.ExecContext(ctx, `
			UPDATE image_studio_jobs
			SET cancel_requested_at = COALESCE(cancel_requested_at, $2)
			WHERE id = $1::uuid`, jobID, now); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE image_studio_items
			SET status = 'cancelled', finished_at = $2
			WHERE job_id = $1::uuid AND status = 'pending'`, jobID, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetJob(ctx, userID, jobID)
}

func (r *imageStudioRepository) SettleJob(
	ctx context.Context,
	jobID, leaseOwner string,
	now time.Time,
	settle func(context.Context, *service.ImageStudioJob, float64) error,
) (_ *service.ImageStudioJob, err error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	job, err := scanFullImageStudioJob(ctx, tx.QueryRowContext(ctx, fullImageStudioJobSelect+`
		WHERE id = $1::uuid FOR UPDATE`, jobID))
	if err != nil {
		return nil, err
	}
	if job.Status != service.ImageStudioJobStatusRunning || job.LeaseOwner != leaseOwner {
		return nil, service.ErrImageStudioLeaseLost
	}
	var successCount, failCount, cancelledCount, unfinishedCount int
	var actualCost float64
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'success')::int,
			COUNT(*) FILTER (WHERE status = 'failed')::int,
			COUNT(*) FILTER (WHERE status = 'cancelled')::int,
			COUNT(*) FILTER (WHERE status IN ('pending', 'running', 'persisting'))::int,
				COALESCE(SUM(actual_cost), 0)
		FROM image_studio_items WHERE job_id = $1::uuid`, jobID).
		Scan(&successCount, &failCount, &cancelledCount, &unfinishedCount, &actualCost); err != nil {
		return nil, err
	}
	if unfinishedCount > 0 {
		return job, nil
	}
	status := service.ImageStudioJobStatusFailed
	switch {
	case successCount == job.Count:
		status = service.ImageStudioJobStatusCompleted
	case successCount > 0:
		status = service.ImageStudioJobStatusPartial
	case cancelledCount > 0 && failCount == 0:
		status = service.ImageStudioJobStatusCancelled
	}
	if settle != nil {
		if err := settle(withUsageBillingTransaction(ctx, tx), job, actualCost); err != nil {
			return nil, err
		}
	}
	var errMsg any
	if status == service.ImageStudioJobStatusFailed {
		errMsg = "all image outputs failed"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE image_studio_jobs
		SET status = $2, actual_cost = $3, success_count = $4, fail_count = $5,
		    error_message = $6, finished_at = $7, heartbeat_at = $7,
		    lease_owner = NULL, lease_expires_at = NULL
		WHERE id = $1::uuid`,
		jobID, status, actualCost, successCount, failCount, errMsg, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getJobByID(ctx, jobID)
}

const fullImageStudioJobSelect = `
	SELECT id::text, user_id, template_id, prompt_hash, COALESCE(request_payload_encrypted, ''),
	       model, quality, size, count, status, estimated_cost, actual_cost, api_key_id,
	       hold_amount, COALESCE(hold_id, ''), success_count, fail_count, error_message,
	       created_at, expires_at, cancel_requested_at, started_at, finished_at,
	       heartbeat_at, COALESCE(lease_owner, ''), lease_expires_at
	FROM image_studio_jobs`

type imageStudioRowScanner interface {
	Scan(dest ...any) error
}

func scanFullImageStudioJob(_ context.Context, row imageStudioRowScanner) (*service.ImageStudioJob, error) {
	var job service.ImageStudioJob
	var actualCost, holdAmount sql.NullFloat64
	var apiKeyID sql.NullInt64
	var errMsg sql.NullString
	var expiresAt, cancelAt, startedAt, finishedAt, heartbeatAt, leaseExpiresAt sql.NullTime
	if err := row.Scan(
		&job.ID, &job.UserID, &job.TemplateID, &job.PromptHash, &job.RequestPayloadEncrypted,
		&job.Model, &job.Quality, &job.Size, &job.Count, &job.Status, &job.EstimatedCost,
		&actualCost, &apiKeyID, &holdAmount, &job.HoldID, &job.SuccessCount, &job.FailCount,
		&errMsg, &job.CreatedAt, &expiresAt, &cancelAt, &startedAt, &finishedAt,
		&heartbeatAt, &job.LeaseOwner, &leaseExpiresAt,
	); err != nil {
		return nil, err
	}
	if actualCost.Valid {
		job.ActualCost = &actualCost.Float64
	}
	if holdAmount.Valid {
		job.HoldAmount = &holdAmount.Float64
	}
	if apiKeyID.Valid {
		job.APIKeyID = &apiKeyID.Int64
	}
	if errMsg.Valid {
		job.ErrorMessage = errMsg.String
	}
	assignNullTime := func(src sql.NullTime, dest **time.Time) {
		if src.Valid {
			v := src.Time
			*dest = &v
		}
	}
	assignNullTime(expiresAt, &job.ExpiresAt)
	assignNullTime(cancelAt, &job.CancelRequestedAt)
	assignNullTime(startedAt, &job.StartedAt)
	assignNullTime(finishedAt, &job.FinishedAt)
	assignNullTime(heartbeatAt, &job.HeartbeatAt)
	assignNullTime(leaseExpiresAt, &job.LeaseExpiresAt)
	return &job, nil
}

func queryFullImageStudioJob(
	ctx context.Context,
	exec sqlExecutor,
	query string,
	args ...any,
) (_ *service.ImageStudioJob, err error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	return scanFullImageStudioJob(ctx, rows)
}

func (r *imageStudioRepository) getJobByID(ctx context.Context, jobID string) (*service.ImageStudioJob, error) {
	if r.db == nil {
		return nil, errors.New("image studio repository db is nil")
	}
	job, err := scanFullImageStudioJob(ctx, r.db.QueryRowContext(ctx, fullImageStudioJobSelect+`
		WHERE id = $1::uuid`, jobID))
	if err != nil {
		return nil, err
	}
	job.Assets, err = r.listAssets(ctx, jobID)
	if err != nil {
		return nil, err
	}
	job.Items, err = r.listItems(ctx, jobID)
	if err != nil {
		return nil, err
	}
	return job, nil
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

func (r *imageStudioRepository) DeleteExpiredJobsBefore(ctx context.Context, before time.Time) (_ int64, err error) {
	if r.db == nil {
		return 0, errors.New("image studio repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin delete expired image studio jobs: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text
		FROM image_studio_jobs
		WHERE expires_at IS NOT NULL AND expires_at < $1
		  AND status IN ('completed', 'failed', 'cancelled', 'partial')
		FOR UPDATE`, before)
	if err != nil {
		return 0, fmt.Errorf("lock expired image studio jobs: %w", err)
	}
	jobIDs := make([]string, 0)
	for rows.Next() {
		var jobID string
		if err := rows.Scan(&jobID); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan expired image studio job: %w", err)
		}
		jobIDs = append(jobIDs, jobID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("iterate expired image studio jobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired image studio jobs: %w", err)
	}
	if len(jobIDs) == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit empty expired image studio job deletion: %w", err)
		}
		return 0, nil
	}
	if _, err := tx.ExecContext(ctx, `
		WITH pending AS (
			SELECT jobs.user_id, jobs.id AS job_id, keys.storage_key
			FROM image_studio_jobs jobs
			JOIN image_studio_assets a ON a.job_id = jobs.id
			CROSS JOIN LATERAL (
				VALUES (a.storage_key), (a.thumbnail_storage_key)
			) AS keys(storage_key)
			WHERE jobs.id = ANY($1::uuid[])
			  AND keys.storage_key IS NOT NULL
			  AND keys.storage_key <> ''
			UNION
			SELECT jobs.user_id, jobs.id AS job_id, refs.storage_key
			FROM image_studio_jobs jobs
			JOIN image_studio_job_references refs ON refs.job_id = jobs.id
			WHERE jobs.id = ANY($1::uuid[])
			  AND refs.storage_key <> ''
		)
		INSERT INTO image_studio_object_deletions (user_id, job_id, storage_key)
		SELECT user_id, job_id, storage_key
		FROM pending
		ON CONFLICT (job_id, storage_key) DO NOTHING`, pq.Array(jobIDs)); err != nil {
		return 0, fmt.Errorf("enqueue expired image studio object deletions: %w", err)
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM image_studio_jobs
		WHERE id = ANY($1::uuid[])`, pq.Array(jobIDs))
	if err != nil {
		return 0, fmt.Errorf("delete expired image studio jobs: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit expired image studio job deletion: %w", err)
	}
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
