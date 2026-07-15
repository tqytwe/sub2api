package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) SetTeamMaxMembers(ctx context.Context, teamID int64, maxMembers int) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `UPDATE play_teams SET max_members = $2 WHERE id = $1`, teamID, maxMembers)
	return err
}

func (r *playRepository) GetTeamMemberCount(ctx context.Context, teamID int64) (int, error) {
	var count int
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT COUNT(*)::int FROM play_team_members WHERE team_id = $1`, []any{teamID}, &count)
	return count, err
}

func (r *playRepository) GetTeamEngagement(ctx context.Context, teamID int64, monthStart, weekStart time.Time) (*service.PlayTeamEngagement, error) {
	var out service.PlayTeamEngagement
	var tokenTarget, requestTarget, weeklyTokens, weeklyRequests int64
	var weeklyDays int
	var completedAt sql.NullTime
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT t.level, t.max_members, t.is_public,
		       COALESCE(a.request_count, 0), COALESCE(a.token_sum, 0), COALESCE(a.active_days, 0),
		       COALESCE(w.token_target, 0), COALESCE(w.request_target, 0),
		       COALESCE(w.token_sum, 0), COALESCE(w.request_count, 0), COALESCE(w.active_days, 0), w.completed_at
		FROM play_teams t
		LEFT JOIN play_usage_aggregates a ON a.aggregate_type = 'team' AND a.subject_id = t.id
		  AND a.period_type = 'monthly' AND a.period_start = $2::date
		LEFT JOIN play_team_weekly_progress w ON w.team_id = t.id AND w.week_start = $3::date
		WHERE t.id = $1`, []any{teamID, monthStart, weekStart},
		&out.Level, &out.MaxMembers, &out.IsPublic, &out.RequestCount, &out.TokenSum, &out.ActiveDays,
		&tokenTarget, &requestTarget, &weeklyTokens, &weeklyRequests, &weeklyDays, &completedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team engagement: %w", err)
	}
	out.Weekly = &service.PlayTeamWeeklyProgress{
		WeekStart: weekStart.Format("2006-01-02"), TokenTarget: tokenTarget,
		RequestTarget: requestTarget, TokenSum: weeklyTokens, RequestCount: weeklyRequests,
		ActiveDays: weeklyDays, Completed: completedAt.Valid,
	}
	return &out, nil
}

func (r *playRepository) ListTeamMemberEngagement(ctx context.Context, userIDs []int64, start, end time.Time) (map[int64]service.PlayMemberEngagement, error) {
	out := make(map[int64]service.PlayMemberEngagement, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(userIDs))
	args := make([]any, 0, len(userIDs)+2)
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	args = append(args, start, end)
	rows, err := r.sqlExec(ctx).QueryContext(ctx, fmt.Sprintf(`
		SELECT user_id, COUNT(*)::bigint,
		       COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens), 0)::bigint,
		       COUNT(DISTINCT DATE(created_at))::int
		FROM usage_logs
		WHERE user_id IN (%s) AND created_at >= $%d AND created_at < $%d AND request_type <> 4
		GROUP BY user_id`, strings.Join(placeholders, ","), len(userIDs)+1, len(userIDs)+2), args...)
	if err != nil {
		return nil, fmt.Errorf("list team member engagement: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var userID int64
		var item service.PlayMemberEngagement
		if err := rows.Scan(&userID, &item.RequestCount, &item.TokenSum, &item.ActiveDays); err != nil {
			return nil, err
		}
		out[userID] = item
	}
	return out, rows.Err()
}

func (r *playRepository) ListDiscoverableTeams(ctx context.Context, monthStart time.Time, limit int) ([]service.PlayTeamDiscovery, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT t.id, t.name, COUNT(m.user_id)::int, t.max_members, t.level,
		       COALESCE(a.token_sum, 0), COALESCE(a.request_count, 0)
		FROM play_teams t
		LEFT JOIN play_team_members m ON m.team_id = t.id
		LEFT JOIN play_usage_aggregates a ON a.aggregate_type = 'team' AND a.subject_id = t.id
		  AND a.period_type = 'monthly' AND a.period_start = $1::date
		WHERE t.is_public = TRUE
		GROUP BY t.id, a.token_sum, a.request_count, a.score
		HAVING COUNT(m.user_id) < t.max_members
		ORDER BY COALESCE(a.score, 0) DESC, t.created_at ASC
		LIMIT $2`, monthStart, limit)
	if err != nil {
		return nil, fmt.Errorf("list discoverable teams: %w", err)
	}
	defer rows.Close()
	out := make([]service.PlayTeamDiscovery, 0, limit)
	for rows.Next() {
		var item service.PlayTeamDiscovery
		if err := rows.Scan(&item.ID, &item.Name, &item.MemberCount, &item.MaxMembers, &item.Level, &item.TokenSum, &item.RequestCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *playRepository) CreateTeamJoinRequest(ctx context.Context, teamID, userID int64) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_team_join_requests (team_id, user_id, status)
		VALUES ($1, $2, 'pending')
		ON CONFLICT (team_id, user_id) DO UPDATE SET status = 'pending', reviewed_by = NULL, reviewed_at = NULL, updated_at = NOW()`, teamID, userID)
	return err
}

func (r *playRepository) ListTeamJoinRequests(ctx context.Context, teamID int64) ([]service.PlayTeamJoinRequest, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT r.id, r.team_id, r.user_id,
		       COALESCE(NULLIF(TRIM(u.username), ''), CONCAT('user-', r.user_id::text)), r.status, r.created_at
		FROM play_team_join_requests r JOIN users u ON u.id = r.user_id
		WHERE r.team_id = $1 AND r.status = 'pending' ORDER BY r.created_at ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PlayTeamJoinRequest{}
	for rows.Next() {
		var item service.PlayTeamJoinRequest
		if err := rows.Scan(&item.ID, &item.TeamID, &item.UserID, &item.DisplayName, &item.Status, &item.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *playRepository) ApproveTeamJoinRequest(ctx context.Context, requestID, captainID int64) error {
	if dbent.TxFromContext(ctx) != nil {
		return r.approveTeamJoinRequestTx(ctx, requestID, captainID)
	}
	if r.client == nil {
		return errors.New("approve team join request: ent client is unavailable")
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin approve team join request transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := r.approveTeamJoinRequestTx(txCtx, requestID, captainID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit approve team join request transaction: %w", err)
	}
	return nil
}

func (r *playRepository) approveTeamJoinRequestTx(ctx context.Context, requestID, captainID int64) error {
	var teamID, userID int64
	var maxMembers int
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
				SELECT r.team_id, r.user_id, t.max_members FROM play_team_join_requests r
			JOIN play_teams t ON t.id = r.team_id
			WHERE r.id = $1 AND r.status = 'pending' AND t.captain_user_id = $2
			FOR UPDATE OF r, t`, []any{requestID, captainID}, &teamID, &userID, &maxMembers)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPlayTeamJoinRequestNotFound
	}
	if err != nil {
		return err
	}
	memberCount, err := r.GetTeamMemberCount(ctx, teamID)
	if err != nil {
		return err
	}
	if memberCount >= maxMembers {
		return service.ErrPlayTeamFull
	}
	if err := r.JoinTeam(ctx, teamID, userID); err != nil {
		return err
	}
	res, err := r.sqlExec(ctx).ExecContext(ctx, `UPDATE play_team_join_requests SET status = 'approved', reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW() WHERE id = $1 AND status = 'pending'`, requestID, captainID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrPlayTeamJoinRequestNotFound
	}
	return nil
}

func (r *playRepository) RejectTeamJoinRequest(ctx context.Context, requestID, captainID int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_team_join_requests r SET status = 'rejected', reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW()
		FROM play_teams t WHERE r.id = $1 AND r.team_id = t.id AND t.captain_user_id = $2 AND r.status = 'pending'`, requestID, captainID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrPlayTeamJoinRequestNotFound
	}
	return nil
}

func (r *playRepository) LeaveTeam(ctx context.Context, teamID, userID int64, deleteTeam bool) error {
	if deleteTeam {
		_, err := r.sqlExec(ctx).ExecContext(ctx, `DELETE FROM play_teams WHERE id = $1 AND captain_user_id = $2`, teamID, userID)
		return err
	}
	_, err := r.sqlExec(ctx).ExecContext(ctx, `DELETE FROM play_team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	return err
}

func (r *playRepository) TransferTeamCaptain(ctx context.Context, teamID, captainID, nextCaptainID int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_teams SET captain_user_id = $3, invite_version = invite_version + 1
		WHERE id = $1 AND captain_user_id = $2
		  AND EXISTS (SELECT 1 FROM play_team_members WHERE team_id = $1 AND user_id = $3)`, teamID, captainID, nextCaptainID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrPlayTeamNotCaptain
	}
	return nil
}

func (r *playRepository) RemoveTeamMember(ctx context.Context, teamID, captainID, memberID int64) error {
	res, err := r.sqlExec(ctx).ExecContext(ctx, `
		DELETE FROM play_team_members m USING play_teams t
		WHERE m.team_id = $1 AND m.user_id = $3 AND t.id = m.team_id AND t.captain_user_id = $2 AND $2 <> $3`, teamID, captainID, memberID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return service.ErrPlayTeamNotCaptain
	}
	return nil
}

func (r *playRepository) ListTeamActivity(ctx context.Context, teamID int64, limit int) ([]service.PlayPublicActivity, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id, event_type, CONCAT('#', actor_hash), payload, created_at
		FROM play_activity_events
		WHERE is_public = TRUE AND subject_type = 'team' AND subject_id = $1
		ORDER BY created_at DESC LIMIT $2`, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.PlayPublicActivity{}
	for rows.Next() {
		var item service.PlayPublicActivity
		var raw []byte
		if err := rows.Scan(&item.ID, &item.EventType, &item.Actor, &raw, &item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.Payload)
		out = append(out, item)
	}
	return out, rows.Err()
}
