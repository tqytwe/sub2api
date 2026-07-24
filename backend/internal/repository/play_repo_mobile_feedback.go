package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type mobileFeedbackScanner interface {
	Scan(dest ...any) error
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
	return record, nil
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
