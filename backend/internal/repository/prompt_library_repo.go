package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type PromptLibraryRepository struct {
	db *sql.DB
}

func NewPromptLibraryRepository(db *sql.DB) *PromptLibraryRepository {
	return &PromptLibraryRepository{db: db}
}

func (r *PromptLibraryRepository) ListPrompts(
	ctx context.Context,
	filter service.PromptListFilter,
	userID *int64,
	publicOnly bool,
) ([]service.Prompt, *pagination.PaginationResult, error) {
	params := normalizePromptPagination(filter.Pagination)
	where, args := buildPromptWhere(filter, userID, publicOnly)

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM prompts p JOIN prompt_versions v ON v.prompt_id = p.id AND v.version = "+
		promptVersionExpression(publicOnly)+" WHERE "+where, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count prompts: %w", err)
	}

	args = append(args, params.Limit(), params.Offset())
	query := `SELECT ` + promptSelectColumns(userID) + `
		FROM prompts p
		JOIN prompt_versions v
		  ON v.prompt_id = p.id
		 AND v.version = ` + promptVersionExpression(publicOnly) + `
		WHERE ` + where + `
		ORDER BY ` + promptOrderBy(filter.Sort) + `
		LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list prompts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.Prompt, 0, params.Limit())
	for rows.Next() {
		prompt, err := scanPrompt(rows, userID != nil)
		if err != nil {
			return nil, nil, err
		}
		version := prompt.CurrentVersion
		if publicOnly {
			version = prompt.PublishedVersion
		}
		prompt.Media, err = r.listPromptMedia(ctx, prompt.ID, version)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, *prompt)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return out, paginationResultFromTotal(total, params), nil
}

func (r *PromptLibraryRepository) GetPrompt(
	ctx context.Context,
	id int64,
	userID *int64,
	publicOnly bool,
) (*service.Prompt, error) {
	where := "p.id = $1"
	if publicOnly {
		where += " AND p.status IN ('published', 'pending_review') AND p.published_version IS NOT NULL"
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+promptSelectColumns(userID)+`
		FROM prompts p
		JOIN prompt_versions v
		  ON v.prompt_id = p.id
		 AND v.version = `+promptVersionExpression(publicOnly)+`
		WHERE `+where, id)
	prompt, err := scanPrompt(row, userID != nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prompt: %w", err)
	}
	version := prompt.CurrentVersion
	if publicOnly {
		version = prompt.PublishedVersion
	}
	prompt.Media, err = r.listPromptMedia(ctx, id, version)
	if err != nil {
		return nil, err
	}
	if !publicOnly {
		prompt.Sources, err = r.ListPromptSources(ctx, id)
		if err != nil {
			return nil, err
		}
		prompt.Reviews, err = r.ListPromptReviews(ctx, id, 0)
		if err != nil {
			return nil, err
		}
		prompt.CategoryIDs, err = r.listPromptCategoryIDs(ctx, id, prompt.CurrentVersion)
		if err != nil {
			return nil, err
		}
	}
	return prompt, nil
}

func (r *PromptLibraryRepository) SavePrompt(
	ctx context.Context,
	prompt *service.Prompt,
	actorID int64,
) (*service.Prompt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	saved, err := savePromptTx(ctx, tx, prompt, actorID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPrompt(ctx, saved.ID, nil, false)
}

func savePromptTx(
	ctx context.Context,
	tx *sql.Tx,
	prompt *service.Prompt,
	actorID int64,
) (*service.Prompt, error) {
	normalizePromptForPersistence(prompt)
	if strings.TrimSpace(prompt.PublicAttributionNote) == "" {
		prompt.PublicAttributionNote = defaultPromptAttributionNote(prompt)
	}
	if prompt.ID == 0 {
		err := tx.QueryRowContext(ctx, `
			INSERT INTO prompts (
				status, brand_type, provenance_type, authorization_status,
				source_evidence_verified, title_zh, description_zh, purpose, style,
				subject, featured, current_version, public_attribution_note,
				created_by, updated_by
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,1,$12,$13,$13
			)
			RETURNING id, created_at, updated_at`,
			prompt.Status, prompt.BrandType, prompt.ProvenanceType,
			prompt.AuthorizationStatus, prompt.SourceEvidenceVerified,
			strings.TrimSpace(prompt.TitleZH), strings.TrimSpace(prompt.DescriptionZH),
			strings.TrimSpace(prompt.Purpose), strings.TrimSpace(prompt.Style),
			strings.TrimSpace(prompt.Subject), prompt.Featured,
			promptNullString(prompt.PublicAttributionNote), nullActorID(actorID),
		).Scan(&prompt.ID, &prompt.CreatedAt, &prompt.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert prompt: %w", err)
		}
		prompt.CurrentVersion = 1
	} else {
		var currentVersion int
		var currentStatus service.PromptStatus
		if err := tx.QueryRowContext(ctx,
			`SELECT current_version, status FROM prompts WHERE id = $1 FOR UPDATE`,
			prompt.ID,
		).Scan(&currentVersion, &currentStatus); err != nil {
			return nil, fmt.Errorf("lock prompt: %w", err)
		}
		if prompt.ExpectedVersion > 0 && prompt.ExpectedVersion != currentVersion {
			return nil, service.ErrPromptVersionConflict
		}
		prompt.CurrentVersion = currentVersion + 1
		if prompt.Status == "" {
			prompt.Status = currentStatus
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE prompts SET
				status = $2,
				brand_type = $3,
				provenance_type = $4,
				authorization_status = $5,
				source_evidence_verified = $6,
				title_zh = $7,
				description_zh = $8,
				purpose = $9,
				style = $10,
				subject = $11,
				featured = $12,
				current_version = $13,
				public_attribution_note = $14,
				updated_by = $15,
				updated_at = NOW()
			WHERE id = $1`,
			prompt.ID, prompt.Status, prompt.BrandType, prompt.ProvenanceType,
			prompt.AuthorizationStatus, prompt.SourceEvidenceVerified,
			strings.TrimSpace(prompt.TitleZH), strings.TrimSpace(prompt.DescriptionZH),
			strings.TrimSpace(prompt.Purpose), strings.TrimSpace(prompt.Style),
			strings.TrimSpace(prompt.Subject), prompt.Featured, prompt.CurrentVersion,
			promptNullString(prompt.PublicAttributionNote), nullActorID(actorID),
		)
		if err != nil {
			return nil, fmt.Errorf("update prompt: %w", err)
		}
	}

	variables, err := json.Marshal(nonNilMap(prompt.Variables))
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
			INSERT INTO prompt_versions (
				prompt_id, version, brand_type, provenance_type, authorization_status,
				source_evidence_verified, title_zh, description_zh, purpose, style, subject,
				featured, prompt_text, variables, models, sizes, reference_requirement,
				reference_instructions, requires_reference, public_attribution_note, created_by
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21
			)`,
		prompt.ID, prompt.CurrentVersion, prompt.BrandType, prompt.ProvenanceType,
		prompt.AuthorizationStatus, prompt.SourceEvidenceVerified,
		strings.TrimSpace(prompt.TitleZH), strings.TrimSpace(prompt.DescriptionZH),
		strings.TrimSpace(prompt.Purpose), strings.TrimSpace(prompt.Style),
		strings.TrimSpace(prompt.Subject), prompt.Featured, strings.TrimSpace(prompt.PromptText),
		variables, pq.Array(nonNilStrings(prompt.Models)), pq.Array(nonNilStrings(prompt.Sizes)),
		prompt.ReferenceRequirement, strings.TrimSpace(prompt.ReferenceInstructions),
		prompt.RequiresReference, promptNullString(prompt.PublicAttributionNote), nullActorID(actorID),
	)
	if err != nil {
		return nil, fmt.Errorf("insert prompt version: %w", err)
	}

	if err := insertPromptCategoryLinks(ctx, tx, prompt.ID, prompt.CurrentVersion, prompt.CategoryIDs); err != nil {
		return nil, err
	}
	if err := insertPromptMedia(ctx, tx, prompt.ID, prompt.CurrentVersion, prompt.Media); err != nil {
		return nil, err
	}
	if err := appendPromptSources(ctx, tx, prompt.ID, prompt.CurrentVersion, prompt.Sources, actorID); err != nil {
		return nil, err
	}
	return prompt, nil
}

func (r *PromptLibraryRepository) ListPromptSources(ctx context.Context, promptID int64) ([]service.PromptSource, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, prompt_id, version, source_key, COALESCE(external_id, ''),
		       COALESCE(source_url, ''), COALESCE(original_author, ''),
		       source_payload, evidence, authorization_status, evidence_verified, created_at
		FROM prompt_sources
		WHERE prompt_id = $1
		ORDER BY version DESC, id DESC`, promptID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.PromptSource, 0)
	for rows.Next() {
		var source service.PromptSource
		var payload, evidence []byte
		if err := rows.Scan(
			&source.ID, &source.PromptID, &source.Version, &source.SourceKey, &source.ExternalID,
			&source.SourceURL, &source.OriginalAuthor, &payload, &evidence,
			&source.AuthorizationStatus, &source.EvidenceVerified, &source.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(payload, &source.SourcePayload)
		_ = json.Unmarshal(evidence, &source.Evidence)
		out = append(out, source)
	}
	return out, rows.Err()
}

func (r *PromptLibraryRepository) ListPromptReviews(
	ctx context.Context,
	promptID int64,
	version int,
) ([]service.PromptReviewRecord, error) {
	args := []any{promptID}
	where := "prompt_id = $1"
	if version > 0 {
		args = append(args, version)
		where += " AND version = $2"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, prompt_id, version, decision, note, reviewer_id, created_at
		FROM prompt_review_records
		WHERE `+where+`
		ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptReviewRecord, 0)
	for rows.Next() {
		var review service.PromptReviewRecord
		if err := rows.Scan(
			&review.ID, &review.PromptID, &review.Version, &review.Decision,
			&review.Note, &review.ReviewerID, &review.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, review)
	}
	return out, rows.Err()
}

func (r *PromptLibraryRepository) AddPromptReview(ctx context.Context, record service.PromptReviewRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO prompt_review_records (
			prompt_id, version, decision, note, reviewer_id
		) VALUES ($1,$2,$3,$4,$5)`,
		record.PromptID, record.Version, record.Decision, record.Note, record.ReviewerID,
	)
	return err
}

func (r *PromptLibraryRepository) ApprovePromptVersion(
	ctx context.Context,
	promptID int64,
	version int,
	actorID int64,
	note string,
) (*service.Prompt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_review_records (
			prompt_id, version, decision, note, reviewer_id
		) VALUES ($1,$2,'approve',$3,$4)`,
		promptID, version, strings.TrimSpace(note), actorID,
	); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE prompts SET
			status = 'published',
			published_version = $2,
			published_at = NOW(),
			published_by = $3,
			updated_by = $3,
			updated_at = NOW()
		WHERE id = $1 AND current_version = $2`,
		promptID, version, nullActorID(actorID),
	)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPrompt(ctx, promptID, nil, false)
}

func (r *PromptLibraryRepository) SetPromptStatus(
	ctx context.Context,
	id int64,
	version int,
	status service.PromptStatus,
	actorID int64,
) (*service.Prompt, error) {
	publish := status == service.PromptStatusPublished
	res, err := r.db.ExecContext(ctx, `
		UPDATE prompts SET
			status = $2,
			published_version = CASE WHEN $5 THEN $3 ELSE published_version END,
			published_at = CASE WHEN $5 THEN NOW() ELSE published_at END,
			published_by = CASE WHEN $5 THEN $4 ELSE published_by END,
			updated_by = $4,
			updated_at = NOW()
		WHERE id = $1 AND current_version = $3`,
		id, string(status), version, nullActorID(actorID), publish,
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	return r.GetPrompt(ctx, id, nil, false)
}

func (r *PromptLibraryRepository) SubmitPromptReview(ctx context.Context, promptID int64, actorID int64) (*service.Prompt, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE prompts
		SET status = 'pending_review', updated_by = $2, updated_at = NOW()
		WHERE id = $1 AND status IN ('draft', 'offline', 'published')`,
		promptID, nullActorID(actorID),
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	return r.GetPrompt(ctx, promptID, nil, false)
}

func (r *PromptLibraryRepository) RollbackPrompt(
	ctx context.Context,
	id int64,
	version int,
	actorID int64,
) (*service.Prompt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var current int
	if err := tx.QueryRowContext(ctx,
		`SELECT current_version FROM prompts WHERE id = $1 FOR UPDATE`, id,
	).Scan(&current); err != nil {
		return nil, err
	}
	if version <= 0 || version >= current {
		return nil, service.ErrPromptRollbackVersionInvalid
	}
	next := current + 1
	res, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_versions (
			prompt_id, version, brand_type, provenance_type, authorization_status,
			source_evidence_verified, title_zh, description_zh, purpose, style, subject,
			featured, prompt_text, variables, models, sizes, reference_requirement,
			reference_instructions, requires_reference, public_attribution_note,
			change_note, created_by
		)
		SELECT prompt_id, $3, brand_type, provenance_type, authorization_status,
		       source_evidence_verified, title_zh, description_zh, purpose, style, subject,
		       featured, prompt_text, variables, models, sizes, reference_requirement,
		       reference_instructions, requires_reference, public_attribution_note, $4, $5
		FROM prompt_versions
		WHERE prompt_id = $1 AND version = $2`,
		id, version, next, fmt.Sprintf("rollback from version %d", version), nullActorID(actorID),
	)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_category_links (prompt_id, version, category_id)
		SELECT prompt_id, $3, category_id
		FROM prompt_category_links
		WHERE prompt_id = $1 AND version = $2`,
		id, version, next,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_media (
			prompt_id, version, media_type, url, alt_zh, sort_order
		)
		SELECT prompt_id, $3, media_type, url, alt_zh, sort_order
		FROM prompt_media
		WHERE prompt_id = $1 AND version = $2`,
		id, version, next,
	); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO prompt_sources (
			prompt_id, version, source_key, external_id, source_url, original_author,
			source_payload, evidence, authorization_status, evidence_verified, recorded_by
		)
		SELECT prompt_id, $3, source_key, external_id, source_url, original_author,
		       source_payload, evidence, authorization_status, evidence_verified, $4
		FROM prompt_sources
		WHERE prompt_id = $1 AND version = $2`,
		id, version, next, nullActorID(actorID),
	); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE prompts p SET
			brand_type = v.brand_type,
			provenance_type = v.provenance_type,
			authorization_status = v.authorization_status,
			source_evidence_verified = v.source_evidence_verified,
			title_zh = v.title_zh,
			description_zh = v.description_zh,
			purpose = v.purpose,
			style = v.style,
			subject = v.subject,
			featured = v.featured,
			public_attribution_note = v.public_attribution_note,
			current_version = $2,
			status = 'pending_review',
			updated_by = $3,
			updated_at = NOW()
		FROM prompt_versions v
		WHERE p.id = $1 AND v.prompt_id = p.id AND v.version = $2`,
		id, next, nullActorID(actorID),
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPrompt(ctx, id, nil, false)
}

func (r *PromptLibraryRepository) ListCategories(ctx context.Context, publicOnly bool) ([]service.PromptCategory, error) {
	where := ""
	if publicOnly {
		where = "WHERE enabled = TRUE"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, slug, name_zh, description_zh, dimension, sort_order, enabled, created_at, updated_at
		FROM prompt_categories `+where+`
		ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptCategory, 0)
	for rows.Next() {
		var category service.PromptCategory
		if err := rows.Scan(
			&category.ID, &category.Slug, &category.NameZH, &category.DescriptionZH, &category.Dimension,
			&category.SortOrder, &category.Enabled, &category.CreatedAt, &category.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, category)
	}
	return out, rows.Err()
}

func (r *PromptLibraryRepository) SaveCategory(ctx context.Context, category *service.PromptCategory) (*service.PromptCategory, error) {
	if category.ID == 0 {
		err := r.db.QueryRowContext(ctx, `
			INSERT INTO prompt_categories (slug, name_zh, description_zh, dimension, sort_order, enabled)
			VALUES ($1,$2,$3,$4,$5,$6)
			RETURNING id, created_at, updated_at`,
			strings.TrimSpace(category.Slug), strings.TrimSpace(category.NameZH),
			strings.TrimSpace(category.DescriptionZH), normalizePromptCategoryDimension(category.Dimension),
			category.SortOrder, category.Enabled,
		).Scan(&category.ID, &category.CreatedAt, &category.UpdatedAt)
		return category, err
	}
	err := r.db.QueryRowContext(ctx, `
		UPDATE prompt_categories SET
			slug = $2, name_zh = $3, description_zh = $4,
			dimension = $5, sort_order = $6, enabled = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING created_at, updated_at`,
		category.ID, strings.TrimSpace(category.Slug), strings.TrimSpace(category.NameZH),
		strings.TrimSpace(category.DescriptionZH), normalizePromptCategoryDimension(category.Dimension),
		category.SortOrder, category.Enabled,
	).Scan(&category.CreatedAt, &category.UpdatedAt)
	return category, err
}

func normalizePromptCategoryDimension(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "style", "subject", "model", "size":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "purpose"
	}
}

func (r *PromptLibraryRepository) DeleteCategory(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM prompt_categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PromptLibraryRepository) SetFavorite(
	ctx context.Context,
	promptID, userID int64,
	favorite bool,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var changed int
	if favorite {
		err = tx.QueryRowContext(ctx, `
			WITH inserted AS (
				INSERT INTO prompt_favorites (prompt_id, user_id)
				SELECT id, $2 FROM prompts
				WHERE id = $1
				  AND status IN ('published', 'pending_review')
				  AND published_version IS NOT NULL
				ON CONFLICT (prompt_id, user_id) DO NOTHING
				RETURNING 1
				)
				SELECT COUNT(*) FROM inserted`, promptID, userID,
		).Scan(&changed)
	} else {
		err = tx.QueryRowContext(ctx, `
			WITH deleted AS (
				DELETE FROM prompt_favorites
				WHERE prompt_id = $1 AND user_id = $2
				RETURNING 1
				)
				SELECT COUNT(*) FROM deleted`, promptID, userID,
		).Scan(&changed)
	}
	if err != nil {
		return false, err
	}
	var state bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM prompt_favorites WHERE prompt_id = $1 AND user_id = $2
		)`, promptID, userID,
	).Scan(&state); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return state, nil
}

func (r *PromptLibraryRepository) UsePrompt(ctx context.Context, promptID, userID int64) (*service.Prompt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var version int
	err = tx.QueryRowContext(ctx, `
		UPDATE prompts
		SET use_count = use_count + 1, updated_at = updated_at
		WHERE id = $1
		  AND status IN ('published', 'pending_review')
		  AND published_version IS NOT NULL
		RETURNING published_version`, promptID,
	).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO prompt_use_events (prompt_id, version, user_id)
		VALUES ($1,$2,$3)`, promptID, version, userID)
	if err != nil {
		return nil, err
	}
	prompt, err := getPromptWithQueryer(ctx, tx, promptID, &userID, true)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return prompt, nil
}

func (r *PromptLibraryRepository) CreateReport(
	ctx context.Context,
	report service.PromptReport,
) (*service.PromptReport, error) {
	var reporterID any
	if report.ReporterID != nil {
		reporterID = *report.ReporterID
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO prompt_reports (prompt_id, reporter_id, reason, detail)
		SELECT id, $2, $3, $4
		FROM prompts
		WHERE id = $1
		  AND status IN ('published', 'pending_review')
		  AND published_version IS NOT NULL
		RETURNING id, status, created_at`,
		report.PromptID, reporterID, strings.TrimSpace(report.Reason), strings.TrimSpace(report.Detail),
	).Scan(&report.ID, &report.Status, &report.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *PromptLibraryRepository) CreateImportJob(
	ctx context.Context,
	input service.PromptImportJobInput,
	actorID int64,
) (*service.PromptImportJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rawPayload, err := json.Marshal(nonNilMap(input.RawPayload))
	if err != nil {
		return nil, err
	}
	job := &service.PromptImportJob{
		SourceKey: strings.TrimSpace(input.SourceKey),
		Status:    service.PromptImportStatusPendingReview,
		CreatedBy: actorID,
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO prompt_import_jobs (source_key, status, raw_payload, created_by)
		VALUES ($1,'pending_review',$2,$3)
		RETURNING id, created_at, updated_at`,
		job.SourceKey, rawPayload, nullActorID(actorID),
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		return nil, err
	}

	for i := range input.Items {
		item := input.Items[i]
		normalized, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		sourcePayload, err := json.Marshal(nonNilMap(item.SourcePayload))
		if err != nil {
			return nil, err
		}
		evidence, err := json.Marshal(nonNilMap(item.Evidence))
		if err != nil {
			return nil, err
		}
		res, err := tx.ExecContext(ctx, `
			INSERT INTO prompt_import_items (
				job_id, source_key, external_id, normalized_hash, status,
				normalized_payload, source_payload, evidence, authorization_status
			) VALUES ($1,$2,$3,$4,'pending_review',$5,$6,$7,$8)
			ON CONFLICT DO NOTHING`,
			job.ID, job.SourceKey, strings.TrimSpace(item.ExternalID),
			strings.TrimSpace(item.NormalizedHash), normalized, sourcePayload,
			evidence, normalizeAuthorization(item.AuthorizationStatus),
		)
		if err != nil {
			return nil, err
		}
		n, _ := res.RowsAffected()
		job.ItemCount += int(n)
	}
	if job.ItemCount == 0 {
		job.Status = service.PromptImportStatusCompleted
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE prompt_import_jobs
		SET item_count = $2, status = $3, updated_at = NOW()
		WHERE id = $1`,
		job.ID, job.ItemCount, job.Status)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return job, nil
}

func (r *PromptLibraryRepository) GetImportJob(ctx context.Context, id int64) (*service.PromptImportJob, error) {
	var job service.PromptImportJob
	var raw []byte
	var createdBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT id, source_key, status, raw_payload, item_count, created_by, created_at, updated_at
		FROM prompt_import_jobs WHERE id = $1`, id,
	).Scan(
		&job.ID, &job.SourceKey, &job.Status, &raw, &job.ItemCount,
		&createdBy, &job.CreatedAt, &job.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if createdBy.Valid {
		job.CreatedBy = createdBy.Int64
	}
	_ = json.Unmarshal(raw, &job.RawPayload)
	return &job, nil
}

func (r *PromptLibraryRepository) ListImportJobs(
	ctx context.Context,
	params pagination.PaginationParams,
) ([]service.PromptImportJob, *pagination.PaginationResult, error) {
	params = normalizePromptPagination(params)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_import_jobs`).Scan(&total); err != nil {
		return nil, nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_key, status, item_count, created_by, created_at, updated_at
		FROM prompt_import_jobs
		ORDER BY created_at DESC, id DESC
		LIMIT $1 OFFSET $2`, params.Limit(), params.Offset())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptImportJob, 0, params.Limit())
	for rows.Next() {
		var job service.PromptImportJob
		var createdBy sql.NullInt64
		if err := rows.Scan(
			&job.ID, &job.SourceKey, &job.Status, &job.ItemCount,
			&createdBy, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, nil, err
		}
		if createdBy.Valid {
			job.CreatedBy = createdBy.Int64
		}
		out = append(out, job)
	}
	return out, paginationResultFromTotal(total, params), rows.Err()
}

func (r *PromptLibraryRepository) ListImportItems(
	ctx context.Context,
	filter service.PromptImportItemListFilter,
) ([]service.PromptImportItem, *pagination.PaginationResult, error) {
	params := normalizePromptPagination(filter.Pagination)
	where := "1=1"
	args := make([]any, 0, 4)
	if filter.JobID > 0 {
		args = append(args, filter.JobID)
		where += fmt.Sprintf(" AND job_id = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM prompt_import_items WHERE "+where, args...,
	).Scan(&total); err != nil {
		return nil, nil, err
	}
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, source_key, external_id, normalized_hash, status,
		       normalized_payload, authorization_status, prompt_id,
		       COALESCE(rejection_reason, ''), created_at
		FROM prompt_import_items
		WHERE `+where+`
		ORDER BY id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptImportItem, 0)
	for rows.Next() {
		var item service.PromptImportItem
		var normalized []byte
		var promptID sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.JobID, &item.SourceKey, &item.ExternalID,
			&item.NormalizedHash, &item.Status, &normalized,
			&item.AuthorizationStatus, &promptID, &item.RejectionReason, &item.CreatedAt,
		); err != nil {
			return nil, nil, err
		}
		_ = json.Unmarshal(normalized, &item.NormalizedPayload)
		if promptID.Valid {
			id := promptID.Int64
			item.PromptID = &id
		}
		out = append(out, item)
	}
	return out, paginationResultFromTotal(total, params), rows.Err()
}

func (r *PromptLibraryRepository) ReviewImportItem(
	ctx context.Context,
	id, actorID int64,
	approve bool,
	reason string,
) (*service.PromptImportItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var item service.PromptImportItem
	var normalized, sourcePayload, evidence []byte
	err = tx.QueryRowContext(ctx, `
		SELECT id, job_id, source_key, external_id, normalized_hash, status,
		       normalized_payload, source_payload, evidence, authorization_status
		FROM prompt_import_items
		WHERE id = $1
		FOR UPDATE`, id,
	).Scan(
		&item.ID, &item.JobID, &item.SourceKey, &item.ExternalID, &item.NormalizedHash,
		&item.Status, &normalized, &sourcePayload, &evidence, &item.AuthorizationStatus,
	)
	if err != nil {
		return nil, err
	}
	if item.Status != service.PromptImportItemPendingReview {
		return nil, fmt.Errorf("import item already reviewed")
	}
	var lockedJobID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id FROM prompt_import_jobs WHERE id = $1 FOR UPDATE`,
		item.JobID,
	).Scan(&lockedJobID); err != nil {
		return nil, err
	}
	if !approve {
		item.Status = service.PromptImportItemRejected
		item.RejectionReason = strings.TrimSpace(reason)
		if _, err := tx.ExecContext(ctx, `
			UPDATE prompt_import_items SET
				status = 'rejected', rejection_reason = $2,
				reviewed_by = $3, reviewed_at = NOW(), updated_at = NOW()
			WHERE id = $1`, id, item.RejectionReason, nullActorID(actorID)); err != nil {
			return nil, err
		}
	} else {
		var input service.PromptImportItemInput
		if err := json.Unmarshal(normalized, &input); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(sourcePayload, &input.SourcePayload)
		_ = json.Unmarshal(evidence, &input.Evidence)
		input.BrandType = service.NormalizeImportedBrand(
			input.BrandType, input.AuthorizationStatus, input.EvidenceVerified,
		)
		prompt, err := savePromptTx(ctx, tx, &service.Prompt{
			Status:                 service.PromptStatusPendingReview,
			BrandType:              input.BrandType,
			ProvenanceType:         service.PromptProvenanceExternal,
			AuthorizationStatus:    input.AuthorizationStatus,
			SourceEvidenceVerified: input.EvidenceVerified,
			TitleZH:                input.TitleZH,
			DescriptionZH:          input.DescriptionZH,
			Purpose:                input.Purpose,
			Style:                  input.Style,
			Subject:                input.Subject,
			PromptText:             input.PromptText,
			Variables:              input.Variables,
			Models:                 input.Models,
			Sizes:                  input.Sizes,
			ReferenceRequirement:   input.ReferenceRequirement,
			ReferenceInstructions:  input.ReferenceInstructions,
			RequiresReference:      input.RequiresReference,
			Media:                  input.Media,
			PublicAttributionNote:  "",
			Sources: []service.PromptSource{{
				SourceKey:           item.SourceKey,
				ExternalID:          item.ExternalID,
				SourceURL:           input.SourceURL,
				OriginalAuthor:      input.OriginalAuthor,
				SourcePayload:       input.SourcePayload,
				Evidence:            input.Evidence,
				AuthorizationStatus: input.AuthorizationStatus,
				EvidenceVerified:    input.EvidenceVerified,
			}},
		}, actorID)
		if err != nil {
			return nil, err
		}
		item.Status = service.PromptImportItemApproved
		item.PromptID = &prompt.ID
		if _, err := tx.ExecContext(ctx, `
			UPDATE prompt_import_items SET
				status = 'approved', prompt_id = $2,
				reviewed_by = $3, reviewed_at = NOW(), updated_at = NOW()
			WHERE id = $1`, id, prompt.ID, nullActorID(actorID)); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE prompt_import_jobs
		SET status = CASE
				WHEN EXISTS (
					SELECT 1 FROM prompt_import_items
					WHERE job_id = $1 AND status = 'pending_review'
				) THEN 'pending_review'
				ELSE 'completed'
			END,
			updated_at = NOW()
		WHERE id = $1`, item.JobID,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *PromptLibraryRepository) ListReports(
	ctx context.Context,
	filter service.PromptReportListFilter,
) ([]service.PromptReport, *pagination.PaginationResult, error) {
	params := normalizePromptPagination(filter.Pagination)
	where := "1=1"
	args := make([]any, 0, 3)
	if strings.TrimSpace(filter.Status) != "" {
		args = append(args, strings.TrimSpace(filter.Status))
		where += " AND pr.status = $1"
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM prompt_reports pr WHERE "+where, args...).Scan(&total); err != nil {
		return nil, nil, err
	}
	args = append(args, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.id, pr.prompt_id, pr.reporter_id, pr.reason, pr.detail, pr.status, pr.resolution,
		       pr.resolved_by, pr.resolved_at, pr.created_at,
		       v.title_zh,
		       COALESCE(NULLIF(BTRIM(u.username), ''), u.email, '')
		FROM prompt_reports pr
		JOIN prompts p ON p.id = pr.prompt_id
		JOIN prompt_versions v
		  ON v.prompt_id = p.id
		 AND v.version = COALESCE(p.published_version, p.current_version)
		LEFT JOIN users u ON u.id = pr.reporter_id
		WHERE `+where+`
		ORDER BY pr.created_at DESC, pr.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptReport, 0)
	for rows.Next() {
		var report service.PromptReport
		var reporterID, resolvedBy sql.NullInt64
		var resolvedAt sql.NullTime
		if err := rows.Scan(
			&report.ID, &report.PromptID, &reporterID, &report.Reason, &report.Detail,
			&report.Status, &report.Resolution, &resolvedBy, &resolvedAt, &report.CreatedAt,
			&report.PromptTitle, &report.ReporterName,
		); err != nil {
			return nil, nil, err
		}
		if reporterID.Valid {
			id := reporterID.Int64
			report.ReporterID = &id
		}
		if resolvedBy.Valid {
			id := resolvedBy.Int64
			report.ResolvedBy = &id
		}
		if resolvedAt.Valid {
			at := resolvedAt.Time
			report.ResolvedAt = &at
		}
		out = append(out, report)
	}
	return out, paginationResultFromTotal(total, params), rows.Err()
}

func (r *PromptLibraryRepository) ResolveReport(
	ctx context.Context,
	id, actorID int64,
	status, resolution string,
) (*service.PromptReport, error) {
	var report service.PromptReport
	var reporterID sql.NullInt64
	var resolvedBy sql.NullInt64
	var resolvedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		UPDATE prompt_reports SET
			status = $2, resolution = $3, resolved_by = $4,
			resolved_at = NOW(), updated_at = NOW()
		WHERE id = $1
		RETURNING id, prompt_id, reporter_id, reason, detail, status, resolution,
		          resolved_by, resolved_at, created_at`,
		id, status, strings.TrimSpace(resolution), nullActorID(actorID),
	).Scan(
		&report.ID, &report.PromptID, &reporterID, &report.Reason, &report.Detail,
		&report.Status, &report.Resolution, &resolvedBy, &resolvedAt, &report.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if reporterID.Valid {
		value := reporterID.Int64
		report.ReporterID = &value
	}
	if resolvedBy.Valid {
		value := resolvedBy.Int64
		report.ResolvedBy = &value
	}
	if resolvedAt.Valid {
		value := resolvedAt.Time
		report.ResolvedAt = &value
	}
	return &report, nil
}

type promptScanner interface {
	Scan(dest ...any) error
}

type promptQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getPromptWithQueryer(
	ctx context.Context,
	queryer promptQueryer,
	id int64,
	userID *int64,
	publicOnly bool,
) (*service.Prompt, error) {
	where := "p.id = $1"
	if publicOnly {
		where += " AND p.status IN ('published', 'pending_review') AND p.published_version IS NOT NULL"
	}
	prompt, err := scanPrompt(queryer.QueryRowContext(ctx, `SELECT `+promptSelectColumns(userID)+`
		FROM prompts p
		JOIN prompt_versions v
		  ON v.prompt_id = p.id
		 AND v.version = `+promptVersionExpression(publicOnly)+`
		WHERE `+where, id), userID != nil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return prompt, err
}

func promptSelectColumns(userID *int64) string {
	favorited := "FALSE"
	if userID != nil {
		favorited = fmt.Sprintf(
			"EXISTS (SELECT 1 FROM prompt_favorites f WHERE f.prompt_id = p.id AND f.user_id = %d)",
			*userID,
		)
	}
	return `p.id, p.status, v.brand_type, v.provenance_type,
		v.authorization_status, v.source_evidence_verified,
		v.title_zh, v.description_zh, v.purpose, v.style, v.subject,
		v.featured, p.current_version, COALESCE(p.published_version, 0),
		v.prompt_text, v.variables, v.models, v.sizes,
		v.reference_requirement, v.reference_instructions, v.requires_reference,
		COALESCE(v.public_attribution_note, ''), p.use_count, p.favorite_count,
		` + favorited + `, p.published_at, p.created_at, p.updated_at`
}

func scanPrompt(scanner promptScanner, _ bool) (*service.Prompt, error) {
	var prompt service.Prompt
	var variables []byte
	var models, sizes pq.StringArray
	var publishedAt sql.NullTime
	if err := scanner.Scan(
		&prompt.ID, &prompt.Status, &prompt.BrandType, &prompt.ProvenanceType,
		&prompt.AuthorizationStatus, &prompt.SourceEvidenceVerified,
		&prompt.TitleZH, &prompt.DescriptionZH, &prompt.Purpose, &prompt.Style,
		&prompt.Subject, &prompt.Featured, &prompt.CurrentVersion, &prompt.PublishedVersion,
		&prompt.PromptText, &variables, &models, &sizes,
		&prompt.ReferenceRequirement, &prompt.ReferenceInstructions, &prompt.RequiresReference,
		&prompt.PublicAttributionNote, &prompt.UseCount, &prompt.FavoriteCount,
		&prompt.Favorited, &publishedAt, &prompt.CreatedAt, &prompt.UpdatedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(variables, &prompt.Variables)
	prompt.Models = []string(models)
	prompt.Sizes = []string(sizes)
	if publishedAt.Valid {
		at := publishedAt.Time
		prompt.PublishedAt = &at
	}
	return &prompt, nil
}

func (r *PromptLibraryRepository) listPromptMedia(
	ctx context.Context,
	promptID int64,
	version int,
) ([]service.PromptMedia, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, media_type, url, alt_zh, sort_order
		FROM prompt_media
		WHERE prompt_id = $1 AND version = $2
		ORDER BY sort_order, id`, promptID, version)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PromptMedia, 0)
	for rows.Next() {
		var media service.PromptMedia
		if err := rows.Scan(&media.ID, &media.MediaType, &media.URL, &media.AltZH, &media.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, media)
	}
	return out, rows.Err()
}

func (r *PromptLibraryRepository) listPromptCategoryIDs(
	ctx context.Context,
	promptID int64,
	version int,
) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_id FROM prompt_category_links
		WHERE prompt_id = $1 AND version = $2
		ORDER BY category_id`, promptID, version)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func insertPromptCategoryLinks(
	ctx context.Context,
	tx *sql.Tx,
	promptID int64,
	version int,
	categoryIDs []int64,
) error {
	if categoryIDs == nil {
		return nil
	}
	for _, categoryID := range categoryIDs {
		if categoryID <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO prompt_category_links (prompt_id, version, category_id)
			VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
			promptID, version, categoryID,
		); err != nil {
			return err
		}
	}
	return nil
}

func insertPromptMedia(
	ctx context.Context,
	tx *sql.Tx,
	promptID int64,
	version int,
	media []service.PromptMedia,
) error {
	if media == nil {
		return nil
	}
	for _, item := range media {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO prompt_media (prompt_id, version, media_type, url, alt_zh, sort_order)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			promptID, version, defaultString(item.MediaType, "image"),
			strings.TrimSpace(item.URL), strings.TrimSpace(item.AltZH), item.SortOrder,
		); err != nil {
			return err
		}
	}
	return nil
}

func appendPromptSources(
	ctx context.Context,
	tx *sql.Tx,
	promptID int64,
	version int,
	sources []service.PromptSource,
	actorID int64,
) error {
	for _, source := range sources {
		source.Version = version
		payload, err := json.Marshal(nonNilMap(source.SourcePayload))
		if err != nil {
			return err
		}
		evidence, err := json.Marshal(nonNilMap(source.Evidence))
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO prompt_sources (
				prompt_id, version, source_key, external_id, source_url, original_author,
				source_payload, evidence, authorization_status, evidence_verified, recorded_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			promptID, source.Version, strings.TrimSpace(source.SourceKey), promptNullString(source.ExternalID),
			promptNullString(source.SourceURL), promptNullString(source.OriginalAuthor), payload,
			evidence, normalizeAuthorization(source.AuthorizationStatus),
			source.EvidenceVerified, nullActorID(actorID),
		); err != nil {
			return err
		}
	}
	return nil
}

func buildPromptWhere(
	filter service.PromptListFilter,
	userID *int64,
	publicOnly bool,
) (string, []any) {
	parts := []string{"1=1"}
	args := make([]any, 0, 10)
	if publicOnly {
		parts = append(parts, "p.status IN ('published', 'pending_review')", "p.published_version IS NOT NULL")
	} else if filter.Status != "" {
		args = append(args, filter.Status)
		parts = append(parts, fmt.Sprintf("p.status = $%d", len(args)))
	}
	if value := strings.TrimSpace(filter.Query); value != "" {
		args = append(args, "%"+strings.ToLower(value)+"%")
		n := len(args)
		parts = append(parts, fmt.Sprintf(
			"(LOWER(v.title_zh) LIKE $%d OR LOWER(v.description_zh) LIKE $%d OR LOWER(v.prompt_text) LIKE $%d)",
			n, n, n,
		))
	}
	for column, value := range map[string]string{
		"v.purpose": filter.Purpose,
		"v.style":   filter.Style,
		"v.subject": filter.Subject,
	} {
		if value = strings.TrimSpace(value); value != "" {
			args = append(args, value)
			parts = append(parts, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	if value := strings.TrimSpace(filter.Model); value != "" {
		args = append(args, value)
		parts = append(parts, fmt.Sprintf("$%d = ANY(v.models)", len(args)))
	}
	if value := strings.TrimSpace(filter.Size); value != "" {
		args = append(args, value)
		parts = append(parts, fmt.Sprintf("$%d = ANY(v.sizes)", len(args)))
	}
	if filter.ReferenceRequirement != "" {
		args = append(args, filter.ReferenceRequirement)
		parts = append(parts, fmt.Sprintf("v.reference_requirement = $%d", len(args)))
	}
	if filter.FavoritedOnly {
		if userID == nil {
			parts = append(parts, "FALSE")
		} else {
			args = append(args, *userID)
			parts = append(parts, fmt.Sprintf(
				"EXISTS (SELECT 1 FROM prompt_favorites pf WHERE pf.prompt_id = p.id AND pf.user_id = $%d)",
				len(args),
			))
		}
	}
	if filter.Featured != nil {
		args = append(args, *filter.Featured)
		parts = append(parts, fmt.Sprintf("v.featured = $%d", len(args)))
	}
	return strings.Join(parts, " AND "), args
}

func promptVersionExpression(publicOnly bool) string {
	if publicOnly {
		return "p.published_version"
	}
	return "p.current_version"
}

func promptOrderBy(sortBy string) string {
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "latest":
		return "p.published_at DESC NULLS LAST, p.id DESC"
	case "popular":
		return "p.use_count DESC, p.published_at DESC NULLS LAST, p.id DESC"
	default:
		return "v.featured DESC, p.published_at DESC NULLS LAST, p.id DESC"
	}
}

func normalizePromptPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	return params
}

func normalizePromptForPersistence(prompt *service.Prompt) {
	if prompt.ID == 0 && prompt.Status == "" {
		prompt.Status = service.PromptStatusDraft
	}
	if prompt.BrandType == "" {
		prompt.BrandType = service.PromptBrandCurated
	}
	if prompt.ProvenanceType == "" {
		prompt.ProvenanceType = service.PromptProvenanceInternal
	}
	if prompt.AuthorizationStatus == "" {
		prompt.AuthorizationStatus = service.PromptAuthorizationUnknown
	}
	if prompt.ReferenceRequirement == "" {
		if prompt.RequiresReference {
			prompt.ReferenceRequirement = service.PromptReferenceRequired
		} else {
			prompt.ReferenceRequirement = service.PromptReferenceNone
		}
	}
	prompt.RequiresReference = prompt.ReferenceRequirement == service.PromptReferenceRequired
}

func defaultPromptAttributionNote(prompt *service.Prompt) string {
	if prompt.BrandType == service.PromptBrandOriginal || len(prompt.Sources) == 0 {
		return ""
	}
	source := prompt.Sources[0]
	parts := []string{"本内容由极速蹬整理、翻译并完成模型适配。"}
	if author := strings.TrimSpace(source.OriginalAuthor); author != "" {
		parts = append(parts, "原作者或权利人："+author+"。")
	}
	if sourceURL := strings.TrimSpace(source.SourceURL); sourceURL != "" {
		parts = append(parts, "原始出处："+sourceURL+"。")
	}
	return strings.Join(parts, "")
}

func normalizeAuthorization(value service.PromptAuthorization) service.PromptAuthorization {
	if value == "" {
		return service.PromptAuthorizationUnknown
	}
	return value
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func nonNilStrings(value []string) []string {
	if value == nil {
		return []string{}
	}
	return value
}

func promptNullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullActorID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

var _ service.PromptLibraryRepository = (*PromptLibraryRepository)(nil)
