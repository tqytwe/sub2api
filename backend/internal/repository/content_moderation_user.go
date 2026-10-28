package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const moderationUserProjection = `SELECT u.id, COALESCE(s.version,0),
 COALESCE(s.generation,0),COALESCE(s.auto_ban_exempt,FALSE),COALESCE(s.keyword_exempt,FALSE),
 COALESCE(s.api_audit_exempt,FALSE),COALESCE(s.email_exempt,FALSE),
 COALESCE(u.status='disabled' AND s.owned_status_revision=u.moderation_status_revision,FALSE),
 u.status,
 (SELECT COUNT(*) FROM content_moderation_user_hits h
  WHERE h.user_id=u.id AND h.generation=COALESCE(s.generation,0)
   AND h.created_at >= $1 AND ($2::bool=FALSE OR h.action<>'cyber_policy')),
 COALESCE(s.reason,''), s.actor_id, s.updated_at
 FROM users u LEFT JOIN content_moderation_user_state s ON s.user_id=u.id `

type moderationUserScanner interface{ Scan(...any) error }

func scanModerationUser(row moderationUserScanner) (*service.ModerationUserState, error) {
	state := new(service.ModerationUserState)
	err := row.Scan(&state.UserID, &state.Version, &state.Generation, &state.AutoBanExempt,
		&state.KeywordExempt, &state.APIAuditExempt, &state.EmailExempt, &state.OwnedDisable, &state.Status,
		&state.CurrentCount, &state.Reason, &state.ActorID, &state.UpdatedAt)
	return state, err
}

func (r *contentModerationRepository) GetModerationUser(ctx context.Context, id int64, since time.Time, excludeCyber bool) (*service.ModerationUserState, error) {
	state, err := scanModerationUser(r.db.QueryRowContext(ctx, moderationUserProjection+`WHERE u.id=$3 AND u.deleted_at IS NULL`, since, excludeCyber, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrUserNotFound
	}
	return state, err
}

func (r *contentModerationRepository) ListModerationUsers(ctx context.Context, page, size int, status string, userID int64, since time.Time, excludeCyber bool) ([]service.ModerationUserState, int64, error) {
	where := `u.deleted_at IS NULL AND (s.user_id IS NOT NULL OR EXISTS (
 SELECT 1 FROM content_moderation_user_hits legacy
 WHERE legacy.user_id=u.id AND legacy.generation=0
)) AND ` + moderationExemptionFilter("$3") + ` AND ($4::bigint=0 OR u.id=$4)`
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users u LEFT JOIN content_moderation_user_state s ON s.user_id=u.id WHERE `+where, since, excludeCyber, status, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count content moderation users: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, moderationUserProjection+`WHERE `+where+` ORDER BY s.updated_at DESC NULLS LAST,u.id DESC LIMIT $5 OFFSET $6`, since, excludeCyber, status, userID, size, (page-1)*size)
	if err != nil {
		return nil, 0, fmt.Errorf("list content moderation users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.ModerationUserState, 0)
	for rows.Next() {
		state, err := scanModerationUser(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan content moderation user: %w", err)
		}
		items = append(items, *state)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate content moderation users: %w", err)
	}
	return items, total, nil
}

func moderationExemptionFilter(parameter string) string {
	allExemptions := `(COALESCE(s.auto_ban_exempt,FALSE) OR COALESCE(s.keyword_exempt,FALSE) OR COALESCE(s.api_audit_exempt,FALSE) OR COALESCE(s.email_exempt,FALSE))`
	return `(` + parameter + `='all'
 OR (` + parameter + `='exempt' AND ` + allExemptions + `)
 OR (` + parameter + `='enforced' AND NOT ` + allExemptions + `)
 OR (` + parameter + `='auto_ban_exempt' AND COALESCE(s.auto_ban_exempt,FALSE))
 OR (` + parameter + `='keyword_exempt' AND COALESCE(s.keyword_exempt,FALSE))
 OR (` + parameter + `='api_audit_exempt' AND COALESCE(s.api_audit_exempt,FALSE))
 OR (` + parameter + `='email_exempt' AND COALESCE(s.email_exempt,FALSE)))`
}

type lockedModerationUser struct {
	status, role                                              string
	revision                                                  int64
	version                                                   int64
	generation                                                int64
	autoBanExempt, keywordExempt, apiAuditExempt, emailExempt bool
	owned                                                     sql.NullInt64
}

func lockModerationUser(ctx context.Context, tx *sql.Tx, id int64, createState bool) (lockedModerationUser, error) {
	var user lockedModerationUser
	err := tx.QueryRowContext(ctx, `SELECT status, role, moderation_status_revision FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&user.status, &user.role, &user.revision)
	if errors.Is(err, sql.ErrNoRows) {
		return user, service.ErrUserNotFound
	}
	if err != nil {
		return user, err
	}
	if createState {
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_moderation_user_state(user_id) VALUES($1) ON CONFLICT(user_id) DO NOTHING`, id); err != nil {
			return user, err
		}
	}
	err = tx.QueryRowContext(ctx, `SELECT version,generation,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt,owned_status_revision FROM content_moderation_user_state WHERE user_id=$1 FOR UPDATE`, id).Scan(&user.version, &user.generation, &user.autoBanExempt, &user.keywordExempt, &user.apiAuditExempt, &user.emailExempt, &user.owned)
	if errors.Is(err, sql.ErrNoRows) {
		return user, infraerrors.Conflict("MODERATION_STATE_UNAVAILABLE", "Moderation state has not been recorded for this user; refresh after a new moderation hit")
	}
	return user, err
}

func (r *contentModerationRepository) ManageModerationUser(ctx context.Context, id int64, input service.ModerationUserAction) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	user, err := lockModerationUser(ctx, tx, id, false)
	if err != nil {
		return err
	}
	if user.version != input.Version {
		return infraerrors.Conflict("MODERATION_STATE_CONFLICT", "State changed; reload before retrying")
	}
	var owned any
	switch input.Action {
	case "unban":
		if user.status == service.StatusDisabled {
			if !user.owned.Valid || user.owned.Int64 != user.revision {
				return infraerrors.Conflict("MODERATION_DISABLE_NOT_OWNED", "Current disable is not owned by content moderation")
			}
			var status string
			err = tx.QueryRowContext(ctx, `UPDATE users SET status='active',updated_at=NOW() WHERE id=$1 AND status='disabled' AND moderation_status_revision=$2 RETURNING status`, id, user.revision).Scan(&status)
			if errors.Is(err, sql.ErrNoRows) {
				return infraerrors.Conflict("MODERATION_STATE_CONFLICT", "User status changed; reload before retrying")
			}
			if err != nil {
				return err
			}
			if status != service.StatusActive {
				return fmt.Errorf("moderation unban readback: unexpected status")
			}
		} else if user.status != service.StatusActive {
			return infraerrors.Conflict("MODERATION_DISABLE_NOT_OWNED", "Current state is not a content moderation disable")
		}
		user.owned = sql.NullInt64{}
	case "reset":
		user.generation++
	case "exemption":
		if input.Exempt != nil {
			user.autoBanExempt = *input.Exempt
		}
		if input.KeywordExempt != nil {
			user.keywordExempt = *input.KeywordExempt
		}
		if input.APIAuditExempt != nil {
			user.apiAuditExempt = *input.APIAuditExempt
		}
		if input.EmailExempt != nil {
			user.emailExempt = *input.EmailExempt
		}
	default:
		return infraerrors.BadRequest("INVALID_MODERATION_ACTION", "Invalid moderation action")
	}
	if user.owned.Valid {
		owned = user.owned.Int64
	}
	result, err := tx.ExecContext(ctx, `UPDATE content_moderation_user_state SET version=version+1,generation=$2,auto_ban_exempt=$3,owned_status_revision=$4,reason=$5,actor_id=$6,keyword_exempt=$7,api_audit_exempt=$8,email_exempt=$9,updated_at=NOW() WHERE user_id=$1 AND version=$10`, id, user.generation, user.autoBanExempt, owned, input.Reason, input.ActorID, user.keywordExempt, user.apiAuditExempt, user.emailExempt, user.version)
	if err != nil {
		return fmt.Errorf("update content moderation user state: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return infraerrors.Conflict("MODERATION_STATE_CONFLICT", "State changed; reload before retrying")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO content_moderation_user_actions(user_id,actor_id,action,reason,version,auto_ban_exempt,keyword_exempt,api_audit_exempt,email_exempt) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, input.ActorID, input.Action, input.Reason, user.version+1, user.autoBanExempt, user.keywordExempt, user.apiAuditExempt, user.emailExempt)
	if err != nil {
		return fmt.Errorf("record content moderation user action: %w", err)
	}
	return tx.Commit()
}

func (r *contentModerationRepository) ApplyModerationAutoBan(ctx context.Context, id int64) (service.ModerationAutoBanResult, error) {
	var result service.ModerationAutoBanResult
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	user, err := lockModerationUser(ctx, tx, id, false)
	if err != nil {
		return result, err
	}
	if user.role == service.RoleAdmin {
		return result, tx.Commit()
	}
	if user.status != service.StatusActive {
		result.Disabled = user.status == service.StatusDisabled
		return result, tx.Commit()
	}
	var revision int64
	if err := tx.QueryRowContext(ctx, `UPDATE users SET status='disabled',updated_at=NOW() WHERE id=$1 AND status='active' RETURNING moderation_status_revision`, id).Scan(&revision); err != nil {
		return result, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE content_moderation_user_state SET version=version+1,owned_status_revision=$2,reason='automatic moderation threshold',actor_id=NULL,updated_at=NOW() WHERE user_id=$1 AND version=$3 AND auto_ban_exempt=FALSE`, id, revision, user.version)
	if err != nil {
		return result, fmt.Errorf("record content moderation ownership: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return service.ModerationAutoBanResult{Changed: true, Disabled: true}, nil
}

func (r *contentModerationRepository) ApplyModerationHit(ctx context.Context, input service.ModerationHit) (service.ModerationHitResult, error) {
	var result service.ModerationHitResult
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	user, err := lockModerationUser(ctx, tx, input.UserID, true)
	if err != nil {
		return result, err
	}
	if input.Version < 0 || user.version != input.Version {
		return result, nil
	}
	inserted, err := tx.ExecContext(ctx, `INSERT INTO content_moderation_user_hits(user_id,generation,request_id,action) VALUES($1,$2,$3,$4) ON CONFLICT(user_id,generation,request_id) DO NOTHING`, input.UserID, user.generation, input.RequestID, input.Action)
	if err != nil {
		return result, err
	}
	insertedCount, err := inserted.RowsAffected()
	if err != nil {
		return result, err
	}
	if insertedCount == 0 {
		return result, tx.Commit()
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_moderation_user_hits WHERE user_id=$1 AND generation=$2 AND created_at >= $3 AND ($4::bool=FALSE OR action<>'cyber_policy')`, input.UserID, user.generation, input.Since, input.ExcludeCyber).Scan(&result.Count); err != nil {
		return result, err
	}
	if input.AutoBan && input.Threshold > 0 && result.Count >= input.Threshold && !user.autoBanExempt && user.role != service.RoleAdmin && user.status == service.StatusActive {
		var revision int64
		if err = tx.QueryRowContext(ctx, `UPDATE users SET status='disabled',updated_at=NOW() WHERE id=$1 AND status='active' RETURNING moderation_status_revision`, input.UserID).Scan(&revision); err != nil {
			return result, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE content_moderation_user_state SET version=version+1,owned_status_revision=$2,reason='automatic moderation threshold',actor_id=NULL,updated_at=NOW() WHERE user_id=$1 AND version=$3`, input.UserID, revision, user.version); err != nil {
			return result, err
		}
		result.BanApplied, result.OwnedDisable = true, true
	} else {
		result.OwnedDisable = user.status == service.StatusDisabled && user.owned.Valid && user.owned.Int64 == user.revision
	}
	if err = tx.Commit(); err != nil {
		return service.ModerationHitResult{}, err
	}
	result.Accepted = true
	return result, nil
}

var _ service.ContentModerationRepository = (*contentModerationRepository)(nil)
