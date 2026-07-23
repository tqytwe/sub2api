package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *ipRiskRepository) GetIPRiskOverview(ctx context.Context) (*service.IPRiskOverview, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	item := &service.IPRiskOverview{}
	err := r.db.QueryRowContext(ctx, `
SELECT
    COUNT(*) FILTER (WHERE status IN ('open', 'observing', 'processing')),
    COUNT(*) FILTER (WHERE status IN ('open', 'observing', 'processing') AND score >= 80),
    (SELECT COUNT(*) FROM ip_risk_policies
      WHERE mode = 'block_registration' AND enabled = TRUE
        AND (expires_at IS NULL OR expires_at > NOW())),
    (SELECT COUNT(DISTINCT user_id) FROM ip_risk_case_users cu
      JOIN ip_risk_cases c ON c.id = cu.case_id
      WHERE c.status IN ('open', 'observing', 'processing')
        AND cu.recommended_selected = TRUE),
    MAX(last_detected_at)
FROM ip_risk_cases`).Scan(
		&item.OpenCases,
		&item.CriticalCases,
		&item.BlockedPolicies,
		&item.ReviewUsers,
		&item.LastDetectedAt,
	)
	if err != nil {
		return nil, err
	}
	var status sql.NullString
	if err := r.db.QueryRowContext(ctx, `
SELECT status FROM ip_risk_scans ORDER BY created_at DESC, id DESC LIMIT 1`).Scan(&status); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if status.Valid {
		item.LatestScanStatus = status.String
	}
	return item, nil
}

func (r *ipRiskRepository) ListIPRiskCases(
	ctx context.Context,
	filter service.IPRiskCaseFilter,
) ([]service.IPRiskCaseSummary, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("nil ip risk repository")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	where := []string{"1=1"}
	args := make([]any, 0, 8)
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if value := strings.TrimSpace(filter.Level); value != "" {
		where = append(where, "c.level = "+add(value))
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		where = append(where, "c.status = "+add(value))
	}
	if value := strings.TrimSpace(filter.Signal); value != "" {
		where = append(where, "c.evidence_snapshot->'signals' @> "+add(`[{"code":"`+strings.ReplaceAll(value, `"`, ``)+`"}]`)+"::jsonb")
	}
	if value := strings.TrimSpace(filter.Search); value != "" {
		p := add("%" + value + "%")
		where = append(where, `(host(c.primary_ip) ILIKE `+p+` OR c.primary_network::text ILIKE `+p+`
			OR EXISTS (
				SELECT 1 FROM ip_risk_case_users cu
				JOIN users u ON u.id = cu.user_id
				WHERE cu.case_id = c.id
				  AND (u.email ILIKE `+p+` OR u.username ILIKE `+p+` OR u.id::text = `+add(value)+`)
			))`)
	}
	if filter.RangeStart != nil {
		where = append(where, "c.last_detected_at >= "+add(filter.RangeStart.UTC()))
	}
	if filter.RangeEnd != nil {
		where = append(where, "c.last_detected_at <= "+add(filter.RangeEnd.UTC()))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ip_risk_cases c WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (filter.Page - 1) * filter.PageSize
	args = append(args, filter.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
SELECT
    c.id, host(c.primary_ip), c.primary_network::text, c.score, c.level, c.status,
    c.evidence_confidence, c.evidence_snapshot->'signals',
    COUNT(cu.user_id), COUNT(cu.user_id) FILTER (WHERE cu.recommended_selected),
    c.auto_block_eligible, c.first_detected_at, c.last_detected_at, c.version
FROM ip_risk_cases c
LEFT JOIN ip_risk_case_users cu ON cu.case_id = c.id
WHERE `+whereSQL+`
GROUP BY c.id
ORDER BY c.score DESC, c.last_detected_at DESC, c.id DESC
LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskCaseSummary, 0, filter.PageSize)
	for rows.Next() {
		var item service.IPRiskCaseSummary
		var signalsJSON []byte
		if err := rows.Scan(
			&item.ID,
			&item.PrimaryIP,
			&item.PrimaryNetwork,
			&item.Score,
			&item.Level,
			&item.Status,
			&item.EvidenceConfidence,
			&signalsJSON,
			&item.RelatedUserCount,
			&item.SelectedUserCount,
			&item.AutoBlockEligible,
			&item.FirstDetectedAt,
			&item.LastDetectedAt,
			&item.Version,
		); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(signalsJSON, &item.Signals)
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *ipRiskRepository) GetIPRiskCaseDetail(ctx context.Context, caseID int64) (*service.IPRiskCaseDetail, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	detail := &service.IPRiskCaseDetail{}
	var signalsJSON, evidenceJSON, recommendationsJSON []byte
	err := r.db.QueryRowContext(ctx, `
SELECT
    id, host(primary_ip), primary_network::text, score, level, status,
    evidence_confidence, evidence_snapshot->'signals', evidence_snapshot->'evidence',
    recommended_actions, auto_block_eligible, first_detected_at, last_detected_at, version,
    (SELECT COUNT(*) FROM ip_risk_case_users WHERE case_id = ip_risk_cases.id),
    (SELECT COUNT(*) FROM ip_risk_case_users WHERE case_id = ip_risk_cases.id AND recommended_selected)
FROM ip_risk_cases
WHERE id = $1`, caseID).Scan(
		&detail.Case.ID,
		&detail.Case.PrimaryIP,
		&detail.Case.PrimaryNetwork,
		&detail.Case.Score,
		&detail.Case.Level,
		&detail.Case.Status,
		&detail.Case.EvidenceConfidence,
		&signalsJSON,
		&evidenceJSON,
		&recommendationsJSON,
		&detail.Case.AutoBlockEligible,
		&detail.Case.FirstDetectedAt,
		&detail.Case.LastDetectedAt,
		&detail.Case.Version,
		&detail.Case.RelatedUserCount,
		&detail.Case.SelectedUserCount,
	)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(signalsJSON, &detail.Case.Signals)
	_ = json.Unmarshal(evidenceJSON, &detail.Evidence)
	_ = json.Unmarshal(recommendationsJSON, &detail.RecommendedActions)
	users, err := r.listIPRiskRelatedUsers(ctx, caseID)
	if err != nil {
		return nil, err
	}
	detail.Users = users
	timeline, err := r.listIPRiskTimeline(ctx, detail.Case.PrimaryNetwork, detail.Case.FirstDetectedAt.Add(-24*time.Hour), detail.Case.LastDetectedAt)
	if err != nil {
		return nil, err
	}
	detail.Timeline = timeline
	actions, _, err := r.listIPRiskActions(ctx, 1, 100, &caseID)
	if err != nil {
		return nil, err
	}
	detail.Actions = actions
	return detail, nil
}

func (r *ipRiskRepository) listIPRiskRelatedUsers(ctx context.Context, caseID int64) ([]service.IPRiskRelatedUser, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT
    cu.user_id, u.email, COALESCE(u.username, ''), u.role, u.status,
    COALESCE(u.signup_source, 'email'), cu.relation_type, cu.evidence_confidence,
    cu.recommended_selected, cu.first_seen_at, cu.last_seen_at, u.created_at,
    COALESCE(u.total_recharged, 0)::double precision,
    COALESCE(u.balance, 0)::double precision,
    COALESCE(NULLIF(cu.evidence_snapshot->>'primary_ip_registration_count', '')::int, 0),
    COALESCE(NULLIF(cu.evidence_snapshot->>'shared_ua_account_count', '')::int, 0),
    COALESCE(gift.granted, 0)::double precision,
    COALESCE(gift.consumed, 0)::double precision,
    COALESCE(gift.remaining, 0)::double precision,
    cu.evidence_snapshot
FROM ip_risk_case_users cu
JOIN users u ON u.id = cu.user_id
LEFT JOIN LATERAL (
    SELECT
        SUM(original_amount) AS granted,
        SUM(consumed_amount) AS consumed,
        SUM(remaining_amount) AS remaining
    FROM balance_fund_batches
    WHERE user_id = cu.user_id
      AND source_kind IN (
          'signup_gift', 'ops_gift', 'compensation',
          'redeem_gift', 'promotion_gift', 'unknown'
      )
) gift ON TRUE
WHERE cu.case_id = $1
ORDER BY cu.recommended_selected DESC, cu.last_seen_at DESC, cu.user_id`, caseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskRelatedUser, 0)
	for rows.Next() {
		var item service.IPRiskRelatedUser
		var evidenceJSON []byte
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.Username,
			&item.Role,
			&item.Status,
			&item.SignupSource,
			&item.RelationType,
			&item.EvidenceConfidence,
			&item.RecommendedSelected,
			&item.FirstSeenAt,
			&item.LastSeenAt,
			&item.CreatedAt,
			&item.TotalRecharged,
			&item.Balance,
			&item.PrimaryIPRegistrations,
			&item.SharedUAAccountCount,
			&item.GiftGranted,
			&item.GiftConsumed,
			&item.GiftRemaining,
			&evidenceJSON,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(evidenceJSON, &item.Evidence)
		keys, err := r.listIPRiskRelatedKeys(ctx, item.UserID)
		if err != nil {
			return nil, err
		}
		item.APIKeys = keys
		item.APIKeyCount = len(keys)
		for _, key := range keys {
			if key.Status == service.StatusAPIKeyActive {
				item.ActiveAPIKeyCount++
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ipRiskRepository) listIPRiskRelatedKeys(ctx context.Context, userID int64) ([]service.IPRiskRelatedKey, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, name, status, created_at, last_used_at
FROM api_keys
WHERE user_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskRelatedKey, 0)
	for rows.Next() {
		var item service.IPRiskRelatedKey
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.CreatedAt, &item.LastUsedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ipRiskRepository) listIPRiskTimeline(ctx context.Context, network string, start, end time.Time) ([]service.IPRiskTimelineEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, event_type, user_id, host(ip_address), evidence_confidence, request_id, occurred_at
FROM auth_risk_events
WHERE ip_network = $1::cidr
  AND occurred_at >= $2 AND occurred_at <= $3
ORDER BY occurred_at DESC, id DESC
LIMIT 500`, network, start.UTC(), end.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskTimelineEvent, 0)
	for rows.Next() {
		var item service.IPRiskTimelineEvent
		var userID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.EventType, &userID, &item.IPAddress, &item.Confidence, &item.RequestID, &item.OccurredAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			value := userID.Int64
			item.UserID = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ipRiskRepository) GetIPRiskManagedConfig(ctx context.Context) (*service.IPRiskManagedConfig, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil ip risk repository")
	}
	var raw []byte
	if err := r.db.QueryRowContext(ctx, `SELECT config FROM ip_risk_config WHERE id = 1`).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrIPRiskConfigNotFound
		}
		return nil, err
	}
	item := service.DefaultIPRiskManagedConfig()
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ipRiskRepository) UpdateIPRiskManagedConfig(ctx context.Context, config service.IPRiskManagedConfig, actorID int64) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO ip_risk_config (id, config, updated_by, updated_at)
VALUES (1, $1::jsonb, NULLIF($2, 0), NOW())
ON CONFLICT (id) DO UPDATE SET
    config = EXCLUDED.config,
    updated_by = EXCLUDED.updated_by,
    updated_at = NOW()`, string(raw), actorID)
	return err
}

func (r *ipRiskRepository) ListIPRiskPolicies(ctx context.Context) ([]service.IPRiskPolicy, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, mode, COALESCE(ip_network::text, ''), COALESCE(host(exact_ip), ''),
       reason, enabled, expires_at, created_by, source_action_id, created_at, updated_at
FROM ip_risk_policies
ORDER BY enabled DESC, created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskPolicy, 0)
	for rows.Next() {
		item, err := scanIPRiskPolicy(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

type ipRiskRowScanner interface {
	Scan(dest ...any) error
}

func scanIPRiskPolicy(scanner ipRiskRowScanner) (*service.IPRiskPolicy, error) {
	item := &service.IPRiskPolicy{}
	var createdBy, sourceActionID sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Mode, &item.IPNetwork, &item.ExactIP, &item.Reason,
		&item.Enabled, &item.ExpiresAt, &createdBy, &sourceActionID, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if createdBy.Valid {
		value := createdBy.Int64
		item.CreatedBy = &value
	}
	if sourceActionID.Valid {
		value := sourceActionID.Int64
		item.SourceActionID = &value
	}
	return item, nil
}

func (r *ipRiskRepository) CreateIPRiskPolicy(ctx context.Context, input service.IPRiskPolicyInput, sourceActionID *int64) (*service.IPRiskPolicy, error) {
	return scanIPRiskPolicy(r.db.QueryRowContext(ctx, `
INSERT INTO ip_risk_policies (
    mode, ip_network, exact_ip, reason, enabled, expires_at, created_by, source_action_id
) VALUES (
    $1, NULLIF($2, '')::cidr, NULLIF($3, '')::inet, $4, $5, $6, $7, $8
)
RETURNING id, mode, COALESCE(ip_network::text, ''), COALESCE(host(exact_ip), ''),
          reason, enabled, expires_at, created_by, source_action_id, created_at, updated_at`,
		string(input.Mode),
		strings.TrimSpace(input.IPNetwork),
		strings.TrimSpace(input.ExactIP),
		strings.TrimSpace(input.Reason),
		input.Enabled,
		input.ExpiresAt,
		input.CreatedBy,
		sourceActionID,
	))
}

func (r *ipRiskRepository) UpdateIPRiskPolicy(ctx context.Context, id int64, input service.IPRiskPolicyInput) (*service.IPRiskPolicy, error) {
	return scanIPRiskPolicy(r.db.QueryRowContext(ctx, `
UPDATE ip_risk_policies
SET mode = $2, ip_network = NULLIF($3, '')::cidr, exact_ip = NULLIF($4, '')::inet,
    reason = $5, enabled = $6, expires_at = $7, updated_at = NOW()
WHERE id = $1
RETURNING id, mode, COALESCE(ip_network::text, ''), COALESCE(host(exact_ip), ''),
          reason, enabled, expires_at, created_by, source_action_id, created_at, updated_at`,
		id,
		string(input.Mode),
		strings.TrimSpace(input.IPNetwork),
		strings.TrimSpace(input.ExactIP),
		strings.TrimSpace(input.Reason),
		input.Enabled,
		input.ExpiresAt,
	))
}

func (r *ipRiskRepository) DeleteIPRiskPolicy(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM ip_risk_policies WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *ipRiskRepository) GetIPRiskScan(ctx context.Context, id int64) (*service.IPRiskScan, error) {
	item := &service.IPRiskScan{}
	var requestedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
SELECT id, scan_type, status, requested_by, range_start, range_end, progress,
       candidate_count, case_count, inferred_event_count, error_message,
       started_at, completed_at, created_at, updated_at
FROM ip_risk_scans WHERE id = $1`, id).Scan(
		&item.ID, &item.ScanType, &item.Status, &requestedBy,
		&item.RangeStart, &item.RangeEnd, &item.Progress,
		&item.CandidateCount, &item.CaseCount, &item.InferredEventCount,
		&item.ErrorMessage, &item.StartedAt, &item.CompletedAt,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if requestedBy.Valid {
		value := requestedBy.Int64
		item.RequestedBy = &value
	}
	return item, err
}

func (r *ipRiskRepository) ListIPRiskActions(ctx context.Context, page, pageSize int) ([]service.IPRiskActionRecord, int64, error) {
	return r.listIPRiskActions(ctx, page, pageSize, nil)
}

func (r *ipRiskRepository) listIPRiskActions(ctx context.Context, page, pageSize int, caseID *int64) ([]service.IPRiskActionRecord, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	where := ""
	args := make([]any, 0, 3)
	if caseID != nil {
		args = append(args, *caseID)
		where = " WHERE case_id = $1"
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ip_risk_actions"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	limitArg := len(args) - 1
	offsetArg := len(args)
	rows, err := r.db.QueryContext(ctx, `
SELECT id, case_id, action_type, status, actor_type, actor_user_id, reason,
       rollback_status, rollback_of_action_id, result_snapshot, created_at, completed_at
FROM ip_risk_actions`+where+`
ORDER BY created_at DESC, id DESC
LIMIT $`+fmt.Sprint(limitArg)+` OFFSET $`+fmt.Sprint(offsetArg), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.IPRiskActionRecord, 0, pageSize)
	for rows.Next() {
		item, err := scanIPRiskActionRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}

func scanIPRiskActionRecord(scanner ipRiskRowScanner) (*service.IPRiskActionRecord, error) {
	item := &service.IPRiskActionRecord{}
	var caseID, actorID, rollbackOf sql.NullInt64
	var resultJSON []byte
	if err := scanner.Scan(
		&item.ID, &caseID, &item.ActionType, &item.Status, &item.ActorType,
		&actorID, &item.Reason, &item.RollbackStatus, &rollbackOf,
		&resultJSON, &item.CreatedAt, &item.CompletedAt,
	); err != nil {
		return nil, err
	}
	if caseID.Valid {
		value := caseID.Int64
		item.CaseID = &value
	}
	if actorID.Valid {
		value := actorID.Int64
		item.ActorUserID = &value
	}
	if rollbackOf.Valid {
		value := rollbackOf.Int64
		item.RollbackOfActionID = &value
	}
	_ = json.Unmarshal(resultJSON, &item.Result)
	return item, nil
}

func (r *ipRiskRepository) GetIPRiskAction(ctx context.Context, id int64) (*service.IPRiskActionRecord, error) {
	item, err := scanIPRiskActionRecord(r.db.QueryRowContext(ctx, `
SELECT id, case_id, action_type, status, actor_type, actor_user_id, reason,
       rollback_status, rollback_of_action_id, result_snapshot, created_at, completed_at
FROM ip_risk_actions WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, target_type, target_id, COALESCE(host(target_ip), ''), before_state,
       after_state, status, error_message, rollback_status
FROM ip_risk_action_items
WHERE action_id = $1
ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var actionItem service.IPRiskActionItem
		var targetID sql.NullInt64
		var beforeJSON, afterJSON []byte
		if err := rows.Scan(
			&actionItem.ID, &actionItem.TargetType, &targetID, &actionItem.TargetIP,
			&beforeJSON, &afterJSON, &actionItem.Status, &actionItem.ErrorMessage,
			&actionItem.RollbackStatus,
		); err != nil {
			return nil, err
		}
		if targetID.Valid {
			value := targetID.Int64
			actionItem.TargetID = &value
		}
		_ = json.Unmarshal(beforeJSON, &actionItem.BeforeState)
		_ = json.Unmarshal(afterJSON, &actionItem.AfterState)
		item.Items = append(item.Items, actionItem)
	}
	return item, rows.Err()
}

func (r *ipRiskRepository) CreateIPRiskAction(ctx context.Context, input service.IPRiskActionCreate) (*service.IPRiskActionRecord, error) {
	raw, err := json.Marshal(input.ActionSnapshot)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if len(input.PreviewTokenHash) > 0 {
		if input.CaseID == nil || input.CaseVersion <= 0 {
			return nil, service.ErrIPRiskActionPreviewStale
		}
		result, err := tx.ExecContext(ctx, `
UPDATE ip_risk_cases
SET version = version + 1,
    updated_at = NOW()
WHERE id = $1
  AND version = $2`,
			*input.CaseID,
			input.CaseVersion,
		)
		if err != nil {
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, service.ErrIPRiskActionPreviewStale
		}
	}
	item, err := scanIPRiskActionRecord(tx.QueryRowContext(ctx, `
INSERT INTO ip_risk_actions (
    case_id, case_version, action_type, status, actor_type, actor_user_id,
    reason, preview_token_hash, preview_expires_at, action_snapshot, rollback_of_action_id
) VALUES ($1, $2, $3, 'running', $4, $5, $6, $7, $8, $9::jsonb, $10)
ON CONFLICT (preview_token_hash) WHERE preview_token_hash IS NOT NULL DO NOTHING
RETURNING id, case_id, action_type, status, actor_type, actor_user_id, reason,
          rollback_status, rollback_of_action_id, result_snapshot, created_at, completed_at`,
		input.CaseID,
		input.CaseVersion,
		string(input.ActionType),
		input.ActorType,
		input.ActorUserID,
		input.Reason,
		nullableBytes(input.PreviewTokenHash),
		input.PreviewExpiresAt,
		string(raw),
		input.RollbackOfActionID,
	))
	if errors.Is(err, sql.ErrNoRows) && len(input.PreviewTokenHash) > 0 {
		return nil, service.ErrIPRiskActionPreviewStale
	}
	if input.RollbackOfActionID != nil && isUniqueViolation(err) {
		return nil, service.ErrIPRiskActionNotRollbackEligible
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *ipRiskRepository) AddIPRiskActionItem(ctx context.Context, input service.IPRiskActionItemCreate) error {
	before, err := json.Marshal(input.BeforeState)
	if err != nil {
		return err
	}
	after, err := json.Marshal(input.AfterState)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO ip_risk_action_items (
    action_id, target_type, target_id, target_ip, before_state, after_state,
    status, error_message, rollback_status
) VALUES (
    $1, $2, $3, NULLIF($4, '')::inet, $5::jsonb, $6::jsonb, $7, $8, $9
)`,
		input.ActionID,
		input.TargetType,
		input.TargetID,
		strings.TrimSpace(input.TargetIP),
		string(before),
		string(after),
		input.Status,
		input.ErrorMessage,
		input.RollbackStatus,
	)
	return err
}

func (r *ipRiskRepository) ReserveIPRiskActionItem(ctx context.Context, input service.IPRiskActionItemCreate) (int64, error) {
	before, err := json.Marshal(input.BeforeState)
	if err != nil {
		return 0, err
	}
	after, err := json.Marshal(input.AfterState)
	if err != nil {
		return 0, err
	}
	var id int64
	err = r.db.QueryRowContext(ctx, `
INSERT INTO ip_risk_action_items (
    action_id, target_type, target_id, target_ip, before_state, after_state,
    status, error_message, rollback_status
) VALUES (
    $1, $2, $3, NULLIF($4, '')::inet, $5::jsonb, $6::jsonb,
    'pending', '', 'not_requested'
)
RETURNING id`,
		input.ActionID,
		input.TargetType,
		input.TargetID,
		strings.TrimSpace(input.TargetIP),
		string(before),
		string(after),
	).Scan(&id)
	return id, err
}

func (r *ipRiskRepository) FinalizeIPRiskActionItem(
	ctx context.Context,
	itemID int64,
	targetID *int64,
	status,
	errorMessage,
	rollbackStatus string,
) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE ip_risk_action_items
SET target_id = COALESCE($2, target_id),
    status = $3,
    error_message = $4,
    rollback_status = $5,
    updated_at = NOW()
WHERE id = $1
  AND status = 'pending'`,
		itemID,
		targetID,
		status,
		errorMessage,
		rollbackStatus,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (r *ipRiskRepository) CompleteIPRiskAction(ctx context.Context, id int64, status string, result map[string]any, rollbackEligible bool) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	rollbackStatus := "not_requested"
	if rollbackEligible {
		rollbackStatus = "eligible"
	}
	_, err = r.db.ExecContext(ctx, `
UPDATE ip_risk_actions
SET status = $2, result_snapshot = $3::jsonb, rollback_status = $4,
    completed_at = NOW()
WHERE id = $1`, id, status, string(raw), rollbackStatus)
	return err
}

func (r *ipRiskRepository) MarkIPRiskActionRolledBack(ctx context.Context, id int64, status string) error {
	rollbackStatus := "completed"
	switch status {
	case "partial":
		rollbackStatus = "partial"
	case "failed":
		rollbackStatus = "conflict"
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE ip_risk_actions
SET rollback_status = $2,
    status = CASE WHEN $2 = 'completed' THEN 'rolled_back' ELSE status END
WHERE id = $1`, id, rollbackStatus)
	return err
}

func (r *ipRiskRepository) UpdateIPRiskCaseStatus(ctx context.Context, id int64, status service.RiskCaseStatus) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE ip_risk_cases
SET status = $2,
    resolved_at = CASE WHEN $2 IN ('resolved', 'ignored') THEN NOW() ELSE NULL END,
    version = version + 1,
    updated_at = NOW()
WHERE id = $1`, id, string(status))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return sql.ErrNoRows
	}
	return err
}

func (r *ipRiskRepository) ClaimIPRiskNotification(
	ctx context.Context,
	caseID int64,
	level service.RiskLevel,
) (service.RiskLevel, bool, error) {
	if r == nil || r.db == nil {
		return "", false, errors.New("nil ip risk repository")
	}
	var previous sql.NullString
	err := r.db.QueryRowContext(ctx, `
WITH candidate AS (
    SELECT id, last_notified_level
    FROM ip_risk_cases
    WHERE id = $1
      AND CASE COALESCE(last_notified_level, '')
            WHEN 'critical' THEN 5
            WHEN 'severe' THEN 4
            WHEN 'high' THEN 3
            WHEN 'medium' THEN 2
            WHEN 'low' THEN 1
            ELSE 0
          END
          < CASE $2
              WHEN 'critical' THEN 5
              WHEN 'severe' THEN 4
              WHEN 'high' THEN 3
              WHEN 'medium' THEN 2
              WHEN 'low' THEN 1
              ELSE 0
            END
    FOR UPDATE
),
updated AS (
    UPDATE ip_risk_cases c
    SET last_notified_level = $2,
        last_notified_at = NOW(),
        updated_at = NOW()
    FROM candidate
    WHERE c.id = candidate.id
    RETURNING candidate.last_notified_level
)
SELECT last_notified_level FROM updated`, caseID, string(level)).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return service.RiskLevel(previous.String), true, nil
}

func (r *ipRiskRepository) RestoreIPRiskNotification(
	ctx context.Context,
	caseID int64,
	claimedLevel,
	previousLevel service.RiskLevel,
) error {
	if r == nil || r.db == nil {
		return errors.New("nil ip risk repository")
	}
	var previous any
	if previousLevel != "" {
		previous = string(previousLevel)
	}
	_, err := r.db.ExecContext(ctx, `
UPDATE ip_risk_cases
SET last_notified_level = $3,
    last_notified_at = CASE WHEN $3::text IS NULL THEN NULL ELSE last_notified_at END,
    updated_at = NOW()
WHERE id = $1
  AND last_notified_level = $2`,
		caseID,
		string(claimedLevel),
		previous,
	)
	return err
}
