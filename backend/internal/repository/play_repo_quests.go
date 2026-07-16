package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) UpsertQuestProgress(ctx context.Context, userID int64, questDate time.Time, questKey string, completed bool) error {
	exec := r.sqlExec(ctx)
	var completedAt any
	if completed {
		completedAt = time.Now()
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO play_quest_progress (user_id, quest_date, quest_key, completed, completed_at)
		VALUES ($1, $2::date, $3, $4, $5)
		ON CONFLICT (user_id, quest_date, quest_key) DO UPDATE
		SET completed = EXCLUDED.completed,
		    completed_at = CASE WHEN EXCLUDED.completed THEN COALESCE(play_quest_progress.completed_at, EXCLUDED.completed_at) ELSE play_quest_progress.completed_at END`,
		userID, questDate.Format("2006-01-02"), questKey, completed, completedAt)
	if err != nil {
		return fmt.Errorf("upsert quest progress: %w", err)
	}
	return nil
}

func (r *playRepository) ListQuestProgress(ctx context.Context, userID int64, questDate time.Time) (result []service.PlayQuestProgressRow, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT quest_key, completed, completed_at, reward_claimed
		FROM play_quest_progress
		WHERE user_id = $1 AND quest_date = $2::date`,
		userID, questDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("list quest progress: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.PlayQuestProgressRow, 0)
	for rows.Next() {
		var row service.PlayQuestProgressRow
		var completedAt sql.NullTime
		if err := rows.Scan(&row.QuestKey, &row.Completed, &completedAt, &row.RewardClaimed); err != nil {
			return nil, fmt.Errorf("scan quest progress: %w", err)
		}
		if completedAt.Valid {
			row.CompletedAt = &completedAt.Time
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *playRepository) GetUserDailyTokenSum(ctx context.Context, userID int64, start, end time.Time) (int64, error) {
	exec := r.sqlExec(ctx)
	var tokenSum int64
	err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens), 0)::bigint
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		[]any{userID, start, end}, &tokenSum)
	if err != nil {
		return 0, fmt.Errorf("get user daily token sum: %w", err)
	}
	return tokenSum, nil
}

func (r *playRepository) GetActiveArenaPeriodByType(ctx context.Context, now time.Time, periodType string) (*service.PlayArenaPeriod, error) {
	exec := r.sqlExec(ctx)
	var p service.PlayArenaPeriod
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, start_at, end_at, status
		FROM play_arena_periods
		WHERE status = 'active' AND period_type = $2
		  AND start_at <= $1 AND end_at > $1
		ORDER BY start_at DESC
		LIMIT 1`, []any{now, periodType}, &p.ID, &p.Name, &p.StartAt, &p.EndAt, &p.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active arena period by type: %w", err)
	}
	return &p, nil
}

func (r *playRepository) EnsureDailyArenaPeriod(ctx context.Context, now time.Time) (*service.PlayArenaPeriod, error) {
	if existing, err := r.GetActiveArenaPeriodByType(ctx, now, "daily"); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	name := start.Format("2006-01-02")
	exec := r.sqlExec(ctx)
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO play_arena_periods (name, start_at, end_at, status, period_type)
		VALUES ($1, $2, $3, 'active', 'daily')`, name, start, end); err != nil {
		return nil, fmt.Errorf("insert daily arena period: %w", err)
	}
	return r.GetActiveArenaPeriodByType(ctx, now, "daily")
}

func (r *playRepository) CountImageStudioJobsToday(ctx context.Context, userID int64, dayStart time.Time) (int, error) {
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

func (r *playRepository) HasCompletedImageStudioJob(ctx context.Context, userID int64) (bool, error) {
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
