package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type mobileFeedbackScanner interface {
	Scan(dest ...any) error
}

func (r *playRepository) EnsureMobileFeedbackWorkItem(ctx context.Context, feedbackID int64, item service.MobileFeedbackWorkItem) (*service.MobileFeedbackWorkItem, error) {
	exec := r.sqlExec(ctx)
	source := strings.TrimSpace(item.Source)
	if source == "" {
		source = "auto"
	}
	return scanMobileFeedbackWorkItemFromQuery(ctx, exec, `
		INSERT INTO mobile_feedback_work_items (
			feedback_id,
			type,
			priority,
			status,
			title,
			summary,
			acceptance_criteria,
			target_version,
			released_version,
			owner,
			source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (feedback_id, source) WHERE source = 'auto'
		DO UPDATE SET
			title = EXCLUDED.title,
			summary = EXCLUDED.summary,
			acceptance_criteria = EXCLUDED.acceptance_criteria,
			updated_at = NOW()
		RETURNING id, feedback_id, type, priority, status, title, summary, acceptance_criteria,
		          target_version, released_version, owner, source, created_at, updated_at`,
		[]any{
			feedbackID,
			item.Type,
			item.Priority,
			item.Status,
			item.Title,
			item.Summary,
			item.AcceptanceCriteria,
			item.TargetVersion,
			item.ReleasedVersion,
			item.Owner,
			source,
		})
}

func (r *playRepository) ListMobileFeedbackWorkItems(ctx context.Context, feedbackID int64) ([]service.MobileFeedbackWorkItem, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id, feedback_id, type, priority, status, title, summary, acceptance_criteria,
		       target_version, released_version, owner, source, created_at, updated_at
		FROM mobile_feedback_work_items
		WHERE feedback_id = $1
		ORDER BY created_at ASC, id ASC`, feedbackID)
	if err != nil {
		return nil, fmt.Errorf("list mobile feedback work items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MobileFeedbackWorkItem, 0)
	for rows.Next() {
		item, scanErr := scanMobileFeedbackWorkItem(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mobile feedback work items: %w", err)
	}
	return items, nil
}

func (r *playRepository) UpdateMobileFeedbackWorkItem(ctx context.Context, feedbackID int64, input service.MobileFeedbackWorkItemUpdate) (*service.MobileFeedbackWorkItem, error) {
	exec := r.sqlExec(ctx)
	return scanMobileFeedbackWorkItemFromQuery(ctx, exec, `
		WITH target AS (
			SELECT id
			FROM mobile_feedback_work_items
			WHERE feedback_id = $1 AND ($2::bigint = 0 OR id = $2)
			ORDER BY CASE WHEN source = 'auto' THEN 0 ELSE 1 END, id ASC
			LIMIT 1
		)
		UPDATE mobile_feedback_work_items wi
		SET status = CASE WHEN $3 = '' THEN wi.status ELSE $3 END,
		    priority = CASE WHEN $4 = '' THEN wi.priority ELSE $4 END,
		    target_version = CASE WHEN $5 = '' THEN wi.target_version ELSE $5 END,
		    released_version = CASE WHEN $6 = '' THEN wi.released_version ELSE $6 END,
		    owner = CASE WHEN $7 = '' THEN wi.owner ELSE $7 END,
		    updated_at = NOW()
		FROM target
		WHERE wi.id = target.id
		RETURNING wi.id, wi.feedback_id, wi.type, wi.priority, wi.status, wi.title, wi.summary, wi.acceptance_criteria,
		          wi.target_version, wi.released_version, wi.owner, wi.source, wi.created_at, wi.updated_at`,
		[]any{
			feedbackID,
			input.ID,
			strings.TrimSpace(input.Status),
			strings.TrimSpace(input.Priority),
			strings.TrimSpace(input.TargetVersion),
			strings.TrimSpace(input.ReleasedVersion),
			strings.TrimSpace(input.Owner),
		})
}

func (r *playRepository) CreateMobileFeedback(ctx context.Context, record service.MobileFeedbackRecord) (*service.MobileFeedbackRecord, error) {
	exec := r.sqlExec(ctx)
	deviceInfo, err := json.Marshal(nonNilJSONMap(record.DeviceInfo))
	if err != nil {
		return nil, fmt.Errorf("marshal mobile feedback device info: %w", err)
	}
	screenshots, err := json.Marshal(nonNilScreenshots(record.Screenshots))
	if err != nil {
		return nil, fmt.Errorf("marshal mobile feedback screenshots: %w", err)
	}
	groupID := sql.NullInt64{}
	if record.GroupID != nil && *record.GroupID > 0 {
		groupID = sql.NullInt64{Int64: *record.GroupID, Valid: true}
	}
	var created service.MobileFeedbackRecord
	var deviceInfoRaw, screenshotsRaw []byte
	err = scanSingleRow(ctx, exec, `
		INSERT INTO mobile_feedback (
			user_id,
			title,
			category,
			content,
			status,
			app_version,
			platform,
			device_model,
			android_version,
			system_version,
			group_name,
			group_id,
			backend_url,
			last_error,
			crash_log,
			device_info,
			screenshots
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16::jsonb, $17::jsonb)
		RETURNING
			id,
			user_id,
			'' AS user_email,
			'' AS user_name,
			title,
			category,
			content,
			status,
			app_version,
			platform,
			device_model,
			android_version,
			system_version,
			group_name,
			group_id,
			backend_url,
			last_error,
			crash_log,
			device_info,
			screenshots,
			admin_note,
			created_at,
			updated_at`,
		[]any{
			record.UserID,
			record.Title,
			record.Category,
			record.Content,
			record.Status,
			record.AppVersion,
			record.Platform,
			record.DeviceModel,
			record.AndroidVersion,
			record.SystemVersion,
			record.GroupName,
			groupID,
			record.BackendURL,
			record.LastError,
			record.CrashLog,
			string(deviceInfo),
			string(screenshots),
		},
		&created.ID,
		&created.UserID,
		&created.UserEmail,
		&created.UserName,
		&created.Title,
		&created.Category,
		&created.Content,
		&created.Status,
		&created.AppVersion,
		&created.Platform,
		&created.DeviceModel,
		&created.AndroidVersion,
		&created.SystemVersion,
		&created.GroupName,
		&groupID,
		&created.BackendURL,
		&created.LastError,
		&created.CrashLog,
		&deviceInfoRaw,
		&screenshotsRaw,
		&created.AdminNote,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert mobile feedback: %w", err)
	}
	if groupID.Valid {
		created.GroupID = &groupID.Int64
	}
	created.DeviceInfo = decodeMobileFeedbackDeviceInfo(deviceInfoRaw)
	created.Screenshots = decodeMobileFeedbackScreenshots(screenshotsRaw)
	return &created, nil
}

func (r *playRepository) ListAdminMobileFeedback(ctx context.Context, filter service.MobileFeedbackListFilter) ([]service.MobileFeedbackRecord, int64, error) {
	exec := r.sqlExec(ctx)
	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize
	args := []any{filter.Status, filter.Query}

	var total int64
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)
		FROM mobile_feedback f
		JOIN users u ON u.id = f.user_id
		WHERE ($1 = '' OR f.status = $1)
		  AND (
			$2 = ''
			OR f.title ILIKE '%' || $2 || '%'
			OR f.content ILIKE '%' || $2 || '%'
			OR f.device_model ILIKE '%' || $2 || '%'
			OR f.app_version ILIKE '%' || $2 || '%'
			OR u.email ILIKE '%' || $2 || '%'
		  )`,
		args,
		&total,
	); err != nil {
		return nil, 0, fmt.Errorf("count admin mobile feedback: %w", err)
	}

	rows, err := exec.QueryContext(ctx, `
		SELECT
			f.id,
			f.user_id,
			COALESCE(u.email, '') AS user_email,
			COALESCE(u.username, '') AS user_name,
			f.title,
			f.category,
			f.content,
			f.status,
			f.app_version,
			f.platform,
			f.device_model,
			f.android_version,
			f.system_version,
			f.group_name,
			f.group_id,
			f.backend_url,
			f.last_error,
			f.crash_log,
			f.device_info,
			f.screenshots,
			f.admin_note,
			f.created_at,
			f.updated_at
		FROM mobile_feedback f
		JOIN users u ON u.id = f.user_id
		WHERE ($1 = '' OR f.status = $1)
		  AND (
			$2 = ''
			OR f.title ILIKE '%' || $2 || '%'
			OR f.content ILIKE '%' || $2 || '%'
			OR f.device_model ILIKE '%' || $2 || '%'
			OR f.app_version ILIKE '%' || $2 || '%'
			OR u.email ILIKE '%' || $2 || '%'
		  )
		ORDER BY f.created_at DESC, f.id DESC
		LIMIT $3 OFFSET $4`,
		filter.Status,
		filter.Query,
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin mobile feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.MobileFeedbackRecord, 0, limit)
	for rows.Next() {
		item, err := scanMobileFeedbackRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate admin mobile feedback: %w", err)
	}
	return items, total, nil
}

func (r *playRepository) GetAdminMobileFeedback(ctx context.Context, id int64) (*service.MobileFeedbackRecord, error) {
	exec := r.sqlExec(ctx)
	record, err := scanMobileFeedbackRecordFromQuery(ctx, exec, `
		SELECT
			f.id,
			f.user_id,
			COALESCE(u.email, '') AS user_email,
			COALESCE(u.username, '') AS user_name,
			f.title,
			f.category,
			f.content,
			f.status,
			f.app_version,
			f.platform,
			f.device_model,
			f.android_version,
			f.system_version,
			f.group_name,
			f.group_id,
			f.backend_url,
			f.last_error,
			f.crash_log,
			f.device_info,
			f.screenshots,
			f.admin_note,
			f.created_at,
			f.updated_at
		FROM mobile_feedback f
		JOIN users u ON u.id = f.user_id
		WHERE f.id = $1`,
		[]any{id},
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *playRepository) UpdateAdminMobileFeedback(ctx context.Context, id int64, status, adminNote string) (*service.MobileFeedbackRecord, error) {
	exec := r.sqlExec(ctx)
	record, err := scanMobileFeedbackRecordFromQuery(ctx, exec, `
		UPDATE mobile_feedback
		SET status = $2, admin_note = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			user_id,
			'' AS user_email,
			'' AS user_name,
			title,
			category,
			content,
			status,
			app_version,
			platform,
			device_model,
			android_version,
			system_version,
			group_name,
			group_id,
			backend_url,
			last_error,
			crash_log,
			device_info,
			screenshots,
			admin_note,
			created_at,
			updated_at`,
		[]any{id, status, adminNote},
	)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(adminNote) != "" && record != nil && strings.TrimSpace(record.AdminNote) != "" {
		_, _ = exec.ExecContext(ctx, `
			INSERT INTO mobile_feedback_messages (feedback_id, sender_type, content)
			SELECT $1, 'support', $2
			WHERE NOT EXISTS (
				SELECT 1
				FROM mobile_feedback_messages
				WHERE feedback_id = $1 AND sender_type = 'support' AND content = $2
			)`,
			id,
			record.AdminNote,
		)
	}
	return record, nil
}

func (r *playRepository) ListUserMobileFeedback(ctx context.Context, userID int64, filter service.MobileFeedbackListFilter) ([]service.MobileFeedbackRecord, int64, error) {
	exec := r.sqlExec(ctx)
	limit := filter.PageSize
	offset := (filter.Page - 1) * filter.PageSize
	var total int64
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)
		FROM mobile_feedback
		WHERE user_id = $1 AND ($2 = '' OR status = $2)`,
		[]any{userID, filter.Status}, &total); err != nil {
		return nil, 0, fmt.Errorf("count user mobile feedback: %w", err)
	}
	rows, err := exec.QueryContext(ctx, `
		SELECT id, user_id, '' AS user_email, '' AS user_name,
		       title, category, content, status, app_version, platform,
		       device_model, android_version, system_version, group_name,
		       group_id, backend_url, last_error, crash_log, device_info,
		       screenshots, admin_note, created_at, updated_at
		FROM mobile_feedback
		WHERE user_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY updated_at DESC, id DESC
		LIMIT $3 OFFSET $4`, userID, filter.Status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list user mobile feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.MobileFeedbackRecord, 0, limit)
	for rows.Next() {
		item, scanErr := scanMobileFeedbackRecord(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate user mobile feedback: %w", err)
	}
	return items, total, nil
}

func (r *playRepository) GetUserMobileFeedback(ctx context.Context, userID, id int64) (*service.MobileFeedbackRecord, error) {
	return scanMobileFeedbackRecordFromQuery(ctx, r.sqlExec(ctx), `
		SELECT id, user_id, '' AS user_email, '' AS user_name,
		       title, category, content, status, app_version, platform,
		       device_model, android_version, system_version, group_name,
		       group_id, backend_url, last_error, crash_log, device_info,
		       screenshots, admin_note, created_at, updated_at
		FROM mobile_feedback
		WHERE id = $1 AND user_id = $2`, []any{id, userID})
}

func (r *playRepository) ListMobileFeedbackMessages(ctx context.Context, userID, feedbackID int64) ([]service.MobileFeedbackMessage, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT m.id, m.feedback_id, m.sender_type, m.content, m.created_at
		FROM mobile_feedback_messages m
		JOIN mobile_feedback f ON f.id = m.feedback_id
		WHERE m.feedback_id = $1 AND f.user_id = $2
		ORDER BY m.created_at ASC, m.id ASC`, feedbackID, userID)
	if err != nil {
		return nil, fmt.Errorf("list mobile feedback messages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	messages := make([]service.MobileFeedbackMessage, 0)
	for rows.Next() {
		var message service.MobileFeedbackMessage
		if err := rows.Scan(&message.ID, &message.FeedbackID, &message.SenderType, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan mobile feedback message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mobile feedback messages: %w", err)
	}
	return messages, nil
}

func (r *playRepository) CreateUserMobileFeedbackMessage(ctx context.Context, userID, feedbackID int64, content string) (*service.MobileFeedbackMessage, error) {
	var message service.MobileFeedbackMessage
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		WITH owned AS (
			UPDATE mobile_feedback
			SET status = CASE
				WHEN status = 'handled' THEN 'new'
				WHEN status = 'deferred' THEN 'viewed'
				ELSE status
			END,
			updated_at = NOW()
			WHERE id = $1 AND user_id = $2 AND status <> 'ignored'
			RETURNING id
		)
		INSERT INTO mobile_feedback_messages (feedback_id, sender_type, content)
		SELECT id, 'user', $3 FROM owned
		RETURNING id, feedback_id, sender_type, content, created_at`,
		[]any{feedbackID, userID, content},
		&message.ID, &message.FeedbackID, &message.SenderType, &message.Content, &message.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrMobileFeedbackClosed
		}
		return nil, fmt.Errorf("create mobile feedback message: %w", err)
	}
	return &message, nil
}

func (r *playRepository) CloseUserMobileFeedback(ctx context.Context, userID, id int64) (*service.MobileFeedbackRecord, error) {
	record, err := scanMobileFeedbackRecordFromQuery(ctx, r.sqlExec(ctx), `
		UPDATE mobile_feedback
		SET status = 'ignored', updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND status <> 'ignored'
		RETURNING id, user_id, '' AS user_email, '' AS user_name,
		          title, category, content, status, app_version, platform,
		          device_model, android_version, system_version, group_name,
		          group_id, backend_url, last_error, crash_log, device_info,
		          screenshots, admin_note, created_at, updated_at`, []any{id, userID})
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, service.ErrMobileFeedbackNotFound) {
		return nil, err
	}
	// Closing an already closed ticket is intentionally idempotent.
	return r.GetUserMobileFeedback(ctx, userID, id)
}

func scanMobileFeedbackRecordFromQuery(ctx context.Context, exec sqlExecutor, query string, args []any) (*service.MobileFeedbackRecord, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mobile feedback: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query mobile feedback: %w", err)
		}
		return nil, service.ErrMobileFeedbackNotFound
	}
	record, err := scanMobileFeedbackRecord(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, fmt.Errorf("query mobile feedback: unexpected duplicate rows")
	}
	return &record, nil
}

func scanMobileFeedbackRecord(scanner mobileFeedbackScanner) (service.MobileFeedbackRecord, error) {
	var record service.MobileFeedbackRecord
	var groupID sql.NullInt64
	var deviceInfoRaw, screenshotsRaw []byte
	if err := scanner.Scan(
		&record.ID,
		&record.UserID,
		&record.UserEmail,
		&record.UserName,
		&record.Title,
		&record.Category,
		&record.Content,
		&record.Status,
		&record.AppVersion,
		&record.Platform,
		&record.DeviceModel,
		&record.AndroidVersion,
		&record.SystemVersion,
		&record.GroupName,
		&groupID,
		&record.BackendURL,
		&record.LastError,
		&record.CrashLog,
		&deviceInfoRaw,
		&screenshotsRaw,
		&record.AdminNote,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return record, service.ErrMobileFeedbackNotFound
		}
		return record, fmt.Errorf("scan mobile feedback: %w", err)
	}
	if groupID.Valid {
		record.GroupID = &groupID.Int64
	}
	record.DeviceInfo = decodeMobileFeedbackDeviceInfo(deviceInfoRaw)
	record.Screenshots = decodeMobileFeedbackScreenshots(screenshotsRaw)
	return record, nil
}

func scanMobileFeedbackWorkItemFromQuery(ctx context.Context, exec sqlExecutor, query string, args []any) (*service.MobileFeedbackWorkItem, error) {
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query mobile feedback work item: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query mobile feedback work item: %w", err)
		}
		return nil, service.ErrMobileFeedbackNotFound
	}
	item, err := scanMobileFeedbackWorkItem(rows)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, fmt.Errorf("query mobile feedback work item: unexpected duplicate rows")
	}
	return &item, nil
}

func scanMobileFeedbackWorkItem(scanner mobileFeedbackScanner) (service.MobileFeedbackWorkItem, error) {
	var item service.MobileFeedbackWorkItem
	err := scanner.Scan(
		&item.ID,
		&item.FeedbackID,
		&item.Type,
		&item.Priority,
		&item.Status,
		&item.Title,
		&item.Summary,
		&item.AcceptanceCriteria,
		&item.TargetVersion,
		&item.ReleasedVersion,
		&item.Owner,
		&item.Source,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return service.MobileFeedbackWorkItem{}, fmt.Errorf("scan mobile feedback work item: %w", err)
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	return item, nil
}

func decodeMobileFeedbackDeviceInfo(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func decodeMobileFeedbackScreenshots(raw []byte) []service.MobileFeedbackScreenshot {
	out := []service.MobileFeedbackScreenshot{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []service.MobileFeedbackScreenshot{}
	}
	return out
}

func nonNilJSONMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func nonNilScreenshots(value []service.MobileFeedbackScreenshot) []service.MobileFeedbackScreenshot {
	if value == nil {
		return []service.MobileFeedbackScreenshot{}
	}
	return value
}
