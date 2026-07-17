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

func (r *playRepository) LockBlindboxOpenUser(ctx context.Context, userID int64) (float64, error) {
	exec := r.sqlExec(ctx)
	var balance float64
	err := scanSingleRow(ctx, exec, `
		SELECT balance
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE`, []any{userID}, &balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, service.ErrUserNotFound
		}
		return 0, fmt.Errorf("lock blindbox user: %w", err)
	}
	return balance, nil
}

func (r *playRepository) UpdatePlayBalance(ctx context.Context, userID int64, amount float64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
		    updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL`,
		amount, userID,
	)
	if err != nil {
		return fmt.Errorf("update play balance: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update play balance rows affected: %w", err)
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

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
	return r.InsertBlindboxOpenRecord(ctx, service.PlayBlindboxOpenRecord{
		UserID:         userID,
		Date:           date,
		Cost:           cost,
		Reward:         reward,
		IdempotencyKey: idempotencyKey,
		PoolVersion:    "legacy-v1",
		OpenSource:     "paid",
	})
}

func (r *playRepository) InsertBlindboxOpenRecord(ctx context.Context, record service.PlayBlindboxOpenRecord) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		INSERT INTO play_blindbox_opens (
			user_id, open_date, cost_amount, reward_amount, idempotency_key, pool_version, open_source
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		record.UserID,
		record.Date.Format("2006-01-02"),
		record.Cost,
		record.Reward,
		record.IdempotencyKey,
		record.PoolVersion,
		record.OpenSource,
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

func (r *playRepository) ListRecentBlindboxWins(ctx context.Context, limit int) (result []service.PlayBlindboxRecentWin, err error) {
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

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

func (r *playRepository) ListQuizQuestions(ctx context.Context, language string) (result []service.PlayQuizQuestionDB, err error) {
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

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
		  AND m.left_at IS NULL
		  AND t.archived_at IS NULL
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
		FROM play_teams
		WHERE id = $1
		  AND archived_at IS NULL`, []any{teamID}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
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
		FROM play_teams
		WHERE invite_code = $1
		  AND archived_at IS NULL`, []any{inviteCode}, &team.ID, &team.Name, &team.CaptainUserID, &team.InviteCode)
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
		if isPlayTeamUniqueViolation(err) {
			return service.ErrPlayTeamAlreadyJoined
		}
		return fmt.Errorf("join team: %w", err)
	}
	return nil
}

func isPlayTeamUniqueViolation(err error) bool {
	type sqlStateError interface {
		SQLState() string
	}
	var stateErr sqlStateError
	if errors.As(err, &stateErr) && stateErr.SQLState() == "23505" {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

func (r *playRepository) LockActiveTeamMembership(ctx context.Context, userID int64) (*service.PlayTeamMembershipDB, error) {
	exec := r.sqlExec(ctx)
	var membership service.PlayTeamMembershipDB
	err := scanSingleRow(ctx, exec, `
		SELECT id, team_id, user_id, joined_at
		FROM play_team_members
		WHERE user_id = $1
		  AND left_at IS NULL
		FOR UPDATE`,
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
		return nil, fmt.Errorf("lock active team membership: %w", err)
	}
	return &membership, nil
}

func (r *playRepository) LockTeam(ctx context.Context, teamID int64) (*service.PlayTeamDB, error) {
	exec := r.sqlExec(ctx)
	var team service.PlayTeamDB
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, captain_user_id, invite_code
		FROM play_teams
		WHERE id = $1
		  AND archived_at IS NULL
		FOR UPDATE`,
		[]any{teamID},
		&team.ID,
		&team.Name,
		&team.CaptainUserID,
		&team.InviteCode,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("lock team: %w", err)
	}
	return &team, nil
}

func (r *playRepository) CountActiveTeamMembers(ctx context.Context, teamID int64) (int, error) {
	exec := r.sqlExec(ctx)
	var count int
	if err := scanSingleRow(ctx, exec, `
		SELECT COUNT(*)::int
		FROM play_team_members
		WHERE team_id = $1
		  AND left_at IS NULL`, []any{teamID}, &count); err != nil {
		return 0, fmt.Errorf("count active team members: %w", err)
	}
	return count, nil
}

func (r *playRepository) LeaveTeam(ctx context.Context, teamID, userID int64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_team_members
		SET left_at = NOW()
		WHERE team_id = $1
		  AND user_id = $2
		  AND left_at IS NULL`, teamID, userID)
	if err != nil {
		return fmt.Errorf("leave team: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamMemberNotFound, "leave team")
}

func (r *playRepository) TransferTeamCaptain(ctx context.Context, teamID, captainUserID int64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_teams
		SET captain_user_id = $2
		WHERE id = $1
		  AND archived_at IS NULL`, teamID, captainUserID)
	if err != nil {
		return fmt.Errorf("transfer team captain: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamNotFound, "transfer team captain")
}

func (r *playRepository) RemoveTeamMember(ctx context.Context, teamID, userID int64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_team_members
		SET left_at = NOW()
		WHERE team_id = $1
		  AND user_id = $2
		  AND left_at IS NULL`, teamID, userID)
	if err != nil {
		return fmt.Errorf("remove team member: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamMemberNotFound, "remove team member")
}

func (r *playRepository) ArchiveTeam(ctx context.Context, teamID int64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_teams
		SET archived_at = NOW()
		WHERE id = $1
		  AND archived_at IS NULL`, teamID)
	if err != nil {
		return fmt.Errorf("archive team: %w", err)
	}
	return requireTeamMutationRow(res, service.ErrPlayTeamNotFound, "archive team")
}

func requireTeamMutationRow(result sql.Result, notFound error, operation string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rows affected: %w", operation, err)
	}
	if rows == 0 {
		return notFound
	}
	return nil
}

func (r *playRepository) InsertTeamEvent(ctx context.Context, event service.PlayTeamEvent) error {
	detail, err := json.Marshal(event.Detail)
	if err != nil {
		return fmt.Errorf("marshal team event detail: %w", err)
	}
	var subjectUserID any
	if event.SubjectUserID > 0 {
		subjectUserID = event.SubjectUserID
	}
	exec := r.sqlExec(ctx)
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO play_team_events (
			team_id, actor_user_id, subject_user_id, event_type, detail
		)
		VALUES ($1, $2, $3, $4, $5)`,
		event.TeamID,
		event.ActorUserID,
		subjectUserID,
		event.Type,
		detail,
	); err != nil {
		return fmt.Errorf("insert team event: %w", err)
	}
	return nil
}

func (r *playRepository) ListTeamMembers(ctx context.Context, teamID int64) (result []service.PlayTeamMember, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT m.user_id,
		       COALESCE(u.username, '') AS username,
		       COALESCE(u.email, '') AS email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       m.joined_at
		FROM play_team_members m
		JOIN play_teams t ON t.id = m.team_id
		JOIN users u ON u.id = m.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = m.user_id
		WHERE m.team_id = $1
		  AND m.left_at IS NULL
		  AND t.archived_at IS NULL
		ORDER BY m.joined_at ASC`, teamID)
	if err != nil {
		return nil, fmt.Errorf("list team members: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]service.PlayTeamMember, 0, 8)
	for rows.Next() {
		var m service.PlayTeamMember
		var username, email string
		if err := rows.Scan(&m.UserID, &username, &email, &m.AvatarURL, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		m.DisplayName = service.PublicPlayDisplayName(username, email, m.UserID)
		m.Email = email
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

func (r *playRepository) ListTeamMemberTokenUsage(ctx context.Context, userIDs []int64, start, end time.Time) (result map[int64]int64, err error) {
	out := make(map[int64]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
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
		SELECT user_id,
		       COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens), 0)::bigint
		FROM usage_logs
		WHERE user_id IN (%s) AND created_at >= $%d AND created_at < $%d
		GROUP BY user_id`,
		strings.Join(placeholders, ","), len(userIDs)+1, len(userIDs)+2)
	rows, err := exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list team member token usage: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	for rows.Next() {
		var userID, tokenSum int64
		if err := rows.Scan(&userID, &tokenSum); err != nil {
			return nil, fmt.Errorf("scan team member token usage: %w", err)
		}
		out[userID] = tokenSum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team member token usage: %w", err)
	}
	return out, nil
}
