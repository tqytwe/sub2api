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

func (r *playRepository) CountBlindboxOpens(ctx context.Context, userID int64, date time.Time) (int, error) {
	exec := r.sqlExec(ctx)
	var count int
	err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int FROM play_blindbox_opens
		WHERE user_id = $1 AND open_date = $2`, []any{userID, date.Format("2006-01-02")}, &count)
	if err != nil {
		return 0, fmt.Errorf("count blindbox opens: %w", err)
	}
	return count, nil
}

func (r *playRepository) InsertBlindboxOpen(ctx context.Context, userID int64, date time.Time, cost, reward float64, idempotencyKey string) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		INSERT INTO play_blindbox_opens (user_id, open_date, cost_amount, reward_amount, idempotency_key)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		userID, date.Format("2006-01-02"), cost, reward, idempotencyKey,
	)
	if err != nil {
		return fmt.Errorf("insert blindbox open: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert blindbox open rows affected: %w", err)
	}
	if n == 0 {
		return service.ErrPlayRewardDuplicate
	}
	return nil
}

func (r *playRepository) ListRecentBlindboxWins(ctx context.Context, limit int) ([]service.PlayBlindboxRecentWin, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(TRIM(u.username), ''), NULLIF(TRIM(u.email), ''), CONCAT('user-', b.user_id::text)),
		       b.reward_amount,
		       b.created_at
		FROM play_blindbox_opens b
		JOIN users u ON u.id = b.user_id
		ORDER BY b.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent blindbox wins: %w", err)
	}
	defer rows.Close()

	out := make([]service.PlayBlindboxRecentWin, 0, limit)
	for rows.Next() {
		var win service.PlayBlindboxRecentWin
		if err := rows.Scan(&win.UserLabel, &win.RewardAmount, &win.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent blindbox win: %w", err)
		}
		out = append(out, win)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent blindbox wins: %w", err)
	}
	return out, nil
}

func (r *playRepository) ListQuizQuestions(ctx context.Context, language string) ([]service.PlayQuizQuestionDB, error) {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		language = "en"
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, language, prompt, options, correct_index
		FROM play_quiz_questions
		WHERE active = TRUE AND language = $1
		ORDER BY sort_order ASC, id ASC
	`, language)
	if err != nil {
		return nil, fmt.Errorf("list quiz questions: %w", err)
	}
	defer rows.Close()

	out := make([]service.PlayQuizQuestionDB, 0, 32)
	for rows.Next() {
		var q service.PlayQuizQuestionDB
		var options []byte
		if err := rows.Scan(&q.ID, &q.Language, &q.Prompt, &options, &q.CorrectIndex); err != nil {
			return nil, fmt.Errorf("scan quiz question: %w", err)
		}
		q.OptionsJSON = string(options)
		out = append(out, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quiz questions: %w", err)
	}
	return out, nil
}

func (r *playRepository) GetQuizAttempt(ctx context.Context, userID int64, date time.Time) (*service.PlayQuizAttemptDB, error) {
	exec := r.sqlExec(ctx)
	var attempt service.PlayQuizAttemptDB
	err := scanSingleRow(ctx, exec, `
		SELECT score, total, reward_amount
		FROM play_quiz_attempts
		WHERE user_id = $1 AND attempt_date = $2`,
		[]any{userID, date.Format("2006-01-02")}, &attempt.Score, &attempt.Total, &attempt.RewardAmount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get quiz attempt: %w", err)
	}
	return &attempt, nil
}

func (r *playRepository) InsertQuizAttempt(ctx context.Context, userID int64, date time.Time, score, total int, reward float64, answers map[string]any) error {
	exec := r.sqlExec(ctx)
	detail, err := json.Marshal(answers)
	if err != nil {
		return fmt.Errorf("marshal quiz answers: %w", err)
	}
	res, err := exec.ExecContext(ctx, `
		INSERT INTO play_quiz_attempts (user_id, attempt_date, score, total, reward_amount, answers)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, attempt_date) DO NOTHING`,
		userID, date.Format("2006-01-02"), score, total, reward, detail,
	)
	if err != nil {
		return fmt.Errorf("insert quiz attempt: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert quiz attempt rows affected: %w", err)
	}
	if n == 0 {
		return service.ErrPlayQuizAlreadyDone
	}
	return nil
}

func (r *playRepository) GetUserTeam(ctx context.Context, userID int64) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, exec, `
		SELECT t.id, t.name, t.captain_user_id, t.invite_code
		FROM play_teams t
		JOIN play_team_members m ON m.team_id = t.id
		WHERE m.user_id = $1
		LIMIT 1`, []any{userID}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user team: %w", err)
	}
	return &team, nil
}

func (r *playRepository) GetTeamByID(ctx context.Context, teamID int64) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, captain_user_id, invite_code
		FROM play_teams WHERE id = $1`, []any{teamID}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team by id: %w", err)
	}
	return &team, nil
}

func (r *playRepository) GetTeamByInviteCode(ctx context.Context, inviteCode string) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, captain_user_id, invite_code
		FROM play_teams WHERE invite_code = $1`, []any{inviteCode}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get team by invite code: %w", err)
	}
	return &team, nil
}

func (r *playRepository) CreateTeam(ctx context.Context, name string, captainUserID int64, inviteCode string) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, exec, `
		INSERT INTO play_teams (name, captain_user_id, invite_code)
		VALUES ($1, $2, $3)
		RETURNING id, name, captain_user_id, invite_code`,
		[]any{name, captainUserID, inviteCode}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
	if err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}
	return &team, nil
}

func (r *playRepository) JoinTeam(ctx context.Context, teamID, userID int64) error {
	exec := r.sqlExec(ctx)
	_, err := exec.ExecContext(ctx, `
		INSERT INTO play_team_members (team_id, user_id)
		VALUES ($1, $2)`, teamID, userID)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint") {
			return service.ErrPlayTeamAlreadyJoined
		}
		return fmt.Errorf("join team: %w", err)
	}
	return nil
}

func (r *playRepository) ListTeamMembers(ctx context.Context, teamID int64) ([]service.PlayTeamMember, error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT m.user_id,
		       COALESCE(NULLIF(TRIM(u.username), ''), CONCAT('user-', m.user_id::text)) AS display_name,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       m.joined_at
		FROM play_team_members m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = m.user_id
		WHERE m.team_id = $1
		ORDER BY m.joined_at ASC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer rows.Close()

	out := make([]service.PlayTeamMember, 0, 8)
	for rows.Next() {
		var m service.PlayTeamMember
		if err := rows.Scan(&m.UserID, &m.DisplayName, &m.AvatarURL, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team members: %w", err)
	}
	return out, nil
}

func (r *playRepository) SumTeamTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (int64, error) {
	if len(userIDs) == 0 {
		return 0, nil
	}
	exec := r.sqlExec(ctx)
	placeholders := make([]string, len(userIDs))
	args := make([]any, 0, len(userIDs)+2)
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, id)
	}
	args = append(args, start, end)
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens), 0)::bigint
		FROM usage_logs
		WHERE user_id IN (%s) AND created_at >= $%d AND created_at < $%d`,
		strings.Join(placeholders, ","), len(userIDs)+1, len(userIDs)+2)
	var sum int64
	err := scanSingleRow(ctx, exec, query, args, &sum)
	if err != nil {
		return 0, fmt.Errorf("sum team token usage: %w", err)
	}
	return sum, nil
}
