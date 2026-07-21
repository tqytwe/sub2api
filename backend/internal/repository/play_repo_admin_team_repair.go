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
	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

func (r *playRepository) ListAdminTeamMemberCandidates(
	ctx context.Context,
	targetTeamID int64,
	query string,
	limit int,
) (result []service.PlayAdminTeamMemberCandidate, err error) {
	_ = targetTeamID
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	search := strings.ToLower(strings.TrimSpace(query))
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT u.id,
		       COALESCE(u.username, ''),
		       COALESCE(u.email, ''),
		       CASE
		         WHEN u.deleted_at IS NOT NULL THEN 'deleted'
		         ELSE u.status
		       END,
		       u.created_at,
		       m.id,
		       m.joined_at,
		       t.id,
		       t.name,
		       t.captain_user_id,
		       t.archived_at,
		       ua.inviter_id,
		       COALESCE(inviter.username, ''),
		       COALESCE(inviter.email, '')
		FROM users u
		LEFT JOIN play_team_members m
		  ON m.user_id = u.id
		 AND m.left_at IS NULL
		LEFT JOIN play_teams t ON t.id = m.team_id
		LEFT JOIN user_affiliates ua ON ua.user_id = u.id
		LEFT JOIN users inviter ON inviter.id = ua.inviter_id
		WHERE (
			u.id::text = $1
			OR LOWER(COALESCE(u.email, '')) LIKE '%' || $1 || '%'
			OR LOWER(COALESCE(u.username, '')) LIKE '%' || $1 || '%'
		  )
		ORDER BY
		  CASE WHEN u.id::text = $1 THEN 0 ELSE 1 END,
		  CASE WHEN LOWER(COALESCE(u.email, '')) = $1 THEN 0 ELSE 1 END,
		  u.id ASC
		LIMIT $2`, search, limit)
	if err != nil {
		return nil, fmt.Errorf("list admin team member candidates: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]service.PlayAdminTeamMemberCandidate, 0, limit)
	for rows.Next() {
		candidate, scanErr := scanAdminTeamMemberCandidate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, *candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin team member candidates: %w", err)
	}
	return out, nil
}

func (r *playRepository) LockAdminTeamCandidateUser(
	ctx context.Context,
	userID int64,
) (*service.PlayAdminTeamMemberCandidate, error) {
	exec := r.sqlExec(ctx)
	var candidate service.PlayAdminTeamMemberCandidate
	var username, email string
	err := scanSingleRow(ctx, exec, `
		SELECT id, COALESCE(username, ''), COALESCE(email, ''), status, created_at
		FROM users
		WHERE id = $1
		  AND deleted_at IS NULL
		FOR UPDATE`,
		[]any{userID},
		&candidate.UserID,
		&username,
		&email,
		&candidate.Status,
		&candidate.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock admin team candidate user: %w", err)
	}
	candidate.Username = strings.TrimSpace(username)
	candidate.Email = strings.TrimSpace(email)
	candidate.DisplayName = service.AdminPlayDisplayName(username, email, candidate.UserID)
	return &candidate, nil
}

func (r *playRepository) GetActiveTeamMembership(
	ctx context.Context,
	userID int64,
) (*service.PlayTeamMembershipDB, error) {
	exec := r.sqlExec(ctx)
	var membership service.PlayTeamMembershipDB
	err := scanSingleRow(ctx, exec, `
		SELECT id, team_id, user_id, joined_at
		FROM play_team_members
		WHERE user_id = $1
		  AND left_at IS NULL`,
		[]any{userID},
		&membership.ID,
		&membership.TeamID,
		&membership.UserID,
		&membership.JoinedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active team membership: %w", err)
	}
	return &membership, nil
}

func scanAdminTeamMemberCandidate(scan rowScanner) (*service.PlayAdminTeamMemberCandidate, error) {
	var candidate service.PlayAdminTeamMemberCandidate
	var username, email string
	var membershipID sql.NullInt64
	var joinedAt sql.NullTime
	var teamID sql.NullInt64
	var teamName sql.NullString
	var captainID sql.NullInt64
	var archivedAt sql.NullTime
	var inviterID sql.NullInt64
	var inviterUsername, inviterEmail string
	if err := scan.Scan(
		&candidate.UserID,
		&username,
		&email,
		&candidate.Status,
		&candidate.CreatedAt,
		&membershipID,
		&joinedAt,
		&teamID,
		&teamName,
		&captainID,
		&archivedAt,
		&inviterID,
		&inviterUsername,
		&inviterEmail,
	); err != nil {
		return nil, fmt.Errorf("scan admin team member candidate: %w", err)
	}
	candidate.Username = strings.TrimSpace(username)
	candidate.Email = strings.TrimSpace(email)
	candidate.DisplayName = service.AdminPlayDisplayName(username, email, candidate.UserID)
	if membershipID.Valid && teamID.Valid {
		candidate.CurrentMembershipID = membershipID.Int64
		candidate.CurrentTeam = &service.PlayAdminTeamReference{
			ID:   teamID.Int64,
			Name: teamName.String,
		}
		if archivedAt.Valid {
			archivedCopy := archivedAt.Time
			candidate.CurrentTeam.ArchivedAt = &archivedCopy
		}
		if joinedAt.Valid {
			joinedCopy := joinedAt.Time
			candidate.CurrentJoinedAt = &joinedCopy
		}
		candidate.IsCaptain = captainID.Valid && captainID.Int64 == candidate.UserID
	}
	if inviterID.Valid {
		candidate.Affiliate = &service.PlayAdminAffiliateReference{
			InviterUserID:      inviterID.Int64,
			InviterDisplayName: service.AdminPlayDisplayName(inviterUsername, inviterEmail, inviterID.Int64),
		}
	}
	return &candidate, nil
}

func (r *playRepository) LockTeamForAdmin(ctx context.Context, teamID int64) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	var archivedAt sql.NullTime
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, captain_user_id, invite_code, created_at, archived_at
		FROM play_teams
		WHERE id = $1
		FOR UPDATE`,
		[]any{teamID},
		&team.ID,
		&team.Name,
		&team.CaptainUserID,
		&team.InviteCode,
		&team.CreatedAt,
		&archivedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock admin team: %w", err)
	}
	if archivedAt.Valid {
		archivedCopy := archivedAt.Time
		team.ArchivedAt = &archivedCopy
	}
	return &team, nil
}

func (r *playRepository) JoinTeamAt(ctx context.Context, teamID, userID int64, joinedAt time.Time) error {
	exec := r.sqlExec(ctx)
	_, err := exec.ExecContext(ctx, `
		INSERT INTO play_team_members (team_id, user_id, joined_at)
		VALUES ($1, $2, $3)`, teamID, userID, joinedAt)
	if err != nil {
		if isPlayTeamUniqueViolation(err) {
			return service.ErrPlayTeamAlreadyJoined
		}
		return fmt.Errorf("join team at effective time: %w", err)
	}
	return nil
}

func (r *playRepository) CloseTeamMembershipAt(
	ctx context.Context,
	membershipID int64,
	leftAt time.Time,
) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_team_members
		SET left_at = $2
		WHERE id = $1
		  AND left_at IS NULL`, membershipID, leftAt)
	if err != nil {
		return fmt.Errorf("close team membership at effective time: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamMemberNotFound, "close team membership")
}

func (r *playRepository) ArchiveTeamAt(ctx context.Context, teamID int64, archivedAt time.Time) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_teams
		SET archived_at = $2
		WHERE id = $1
		  AND archived_at IS NULL`, teamID, archivedAt)
	if err != nil {
		return fmt.Errorf("archive team at effective time: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamNotFound, "archive team at effective time")
}

func (r *playRepository) HasTeamRewardSnapshotAt(
	ctx context.Context,
	teamIDs []int64,
	effectiveAt time.Time,
) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}
	exec := r.sqlExec(ctx)
	var exists bool
	if err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1
			FROM play_team_settlements
			WHERE team_id = ANY($1)
			  AND window_start <= $2
			  AND window_end > $2
		)`, []any{pq.Array(teamIDs), effectiveAt}, &exists); err != nil {
		return false, fmt.Errorf("check team reward snapshot at effective time: %w", err)
	}
	return exists, nil
}

func (r *playRepository) HasTeamMembershipOverlap(
	ctx context.Context,
	userID int64,
	effectiveAt time.Time,
	excludeMembershipID int64,
) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	if err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1
			FROM play_team_members
			WHERE user_id = $1
			  AND left_at IS NOT NULL
			  AND left_at > $2
			  AND id <> $3
		)`, []any{userID, effectiveAt, excludeMembershipID}, &exists); err != nil {
		return false, fmt.Errorf("check team membership overlap: %w", err)
	}
	return exists, nil
}

func (r *playRepository) HasTeamCaptainChangeAfter(
	ctx context.Context,
	teamID int64,
	effectiveAt time.Time,
) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	if err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1
			FROM play_team_events
			WHERE team_id = $1
			  AND event_type = $2
			  AND created_at > $3
		)`, []any{teamID, service.PlayTeamEventCaptainTransferred, effectiveAt}, &exists); err != nil {
		return false, fmt.Errorf("check team captain changes after effective time: %w", err)
	}
	return exists, nil
}

func (r *playRepository) HasOtherTeamMembershipAfter(
	ctx context.Context,
	teamID int64,
	excludeMembershipID int64,
	effectiveAt time.Time,
) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	if err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1
			FROM play_team_members
			WHERE team_id = $1
			  AND id <> $2
			  AND (left_at IS NULL OR left_at > $3)
		)`, []any{teamID, excludeMembershipID, effectiveAt}, &exists); err != nil {
		return false, fmt.Errorf("check other team memberships after effective time: %w", err)
	}
	return exists, nil
}

func (r *playRepository) GetAdminTeamSpend(
	ctx context.Context,
	teamID int64,
	start time.Time,
	end time.Time,
) (decimal.Decimal, error) {
	exec := r.sqlExec(ctx)
	var raw string
	if err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(SUM(ul.actual_cost), 0)::text
		FROM play_team_members m
		JOIN usage_logs ul
		  ON ul.user_id = m.user_id
		 AND ul.created_at >= $2
		 AND ul.created_at < $3
		 AND ul.created_at >= m.joined_at
		 AND (m.left_at IS NULL OR ul.created_at < m.left_at)
		WHERE m.team_id = $1`, []any{teamID, start, end}, &raw); err != nil {
		return decimal.Zero, fmt.Errorf("get admin team spend: %w", err)
	}
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse admin team spend: %w", err)
	}
	return value.Round(8), nil
}

func (r *playRepository) GetUserActualCost(
	ctx context.Context,
	userID int64,
	start time.Time,
	end time.Time,
) (decimal.Decimal, error) {
	exec := r.sqlExec(ctx)
	var raw string
	if err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(SUM(actual_cost), 0)::text
		FROM usage_logs
		WHERE user_id = $1
		  AND created_at >= $2
		  AND created_at < $3`, []any{userID, start, end}, &raw); err != nil {
		return decimal.Zero, fmt.Errorf("get user actual cost: %w", err)
	}
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse user actual cost: %w", err)
	}
	return value.Round(8), nil
}

func (r *playRepository) ListAdminTeamEvents(
	ctx context.Context,
	teamID int64,
	limit int,
) (result []service.PlayAdminTeamEventRecord, err error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT e.id,
		       e.team_id,
		       e.actor_user_id,
		       COALESCE(actor.username, ''),
		       COALESCE(actor.email, ''),
		       e.subject_user_id,
		       COALESCE(subject.username, ''),
		       COALESCE(subject.email, ''),
		       e.event_type,
		       e.detail,
		       e.created_at
		FROM play_team_events e
		JOIN users actor ON actor.id = e.actor_user_id
		LEFT JOIN users subject ON subject.id = e.subject_user_id
		WHERE e.team_id = $1
		ORDER BY e.created_at DESC, e.id DESC
		LIMIT $2`, teamID, limit)
	if err != nil {
		return nil, fmt.Errorf("list admin team events: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]service.PlayAdminTeamEventRecord, 0, limit)
	for rows.Next() {
		var event service.PlayAdminTeamEventRecord
		var actorUsername, actorEmail string
		var subjectID sql.NullInt64
		var subjectUsername, subjectEmail string
		var detail []byte
		if err := rows.Scan(
			&event.ID,
			&event.TeamID,
			&event.ActorUserID,
			&actorUsername,
			&actorEmail,
			&subjectID,
			&subjectUsername,
			&subjectEmail,
			&event.Type,
			&detail,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin team event: %w", err)
		}
		event.ActorDisplayName = service.AdminPlayDisplayName(actorUsername, actorEmail, event.ActorUserID)
		if subjectID.Valid {
			subjectCopy := subjectID.Int64
			event.SubjectUserID = &subjectCopy
			event.SubjectDisplayName = service.AdminPlayDisplayName(subjectUsername, subjectEmail, subjectCopy)
		}
		event.Detail = map[string]any{}
		if len(detail) > 0 {
			if err := json.Unmarshal(detail, &event.Detail); err != nil {
				return nil, fmt.Errorf("unmarshal admin team event detail: %w", err)
			}
			event.Detail = sanitizeAdminTeamEventDetail(event.Detail)
		}
		out = append(out, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin team events: %w", err)
	}
	return out, nil
}

func sanitizeAdminTeamEventDetail(detail map[string]any) map[string]any {
	safe := make(map[string]any, len(detail))
	for key, value := range detail {
		switch key {
		case "operation", "side", "effective_at", "reason_code":
			if text, ok := value.(string); ok {
				safe[key] = text
			}
		case "reason":
			if text, ok := value.(string); ok {
				if redacted := service.RedactAdminTeamRepairReason(text); redacted != "" {
					safe[key] = redacted
				}
			}
		case "source_team_id", "target_team_id", "previous_captain_user_id":
			if id, ok := positiveJSONInteger(value); ok {
				safe[key] = id
			}
		}
	}
	return safe
}

func positiveJSONInteger(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || number <= 0 {
		return 0, false
	}
	id := int64(number)
	if float64(id) != number {
		return 0, false
	}
	return id, true
}
