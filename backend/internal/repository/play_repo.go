package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type playRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewPlayRepository(client *dbent.Client, db *sql.DB) service.PlayRepository {
	return &playRepository{client: client, sql: db}
}

func (r *playRepository) sqlExec(ctx context.Context) sqlExecutor {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		if exec := sqlExecutorFromEntClient(tx.Client()); exec != nil {
			return exec
		}
	}
	return r.sql
}

func (r *playRepository) HasCheckin(ctx context.Context, userID int64, date time.Time) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1 FROM play_checkins
			WHERE user_id = $1 AND checkin_date = $2
		)`, []any{userID, date.Format("2006-01-02")}, &exists)
	if err != nil {
		return false, fmt.Errorf("check play checkin: %w", err)
	}
	return exists, nil
}

func (r *playRepository) InsertCheckin(ctx context.Context, userID int64, date time.Time, reward float64, streakCount int) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		INSERT INTO play_checkins (user_id, checkin_date, reward_amount, streak_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, checkin_date) DO NOTHING`,
		userID, date.Format("2006-01-02"), reward, streakCount,
	)
	if err != nil {
		return fmt.Errorf("insert play checkin: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert play checkin rows affected: %w", err)
	}
	if n == 0 {
		return service.ErrPlayCheckinAlreadyDone
	}
	return nil
}

func (r *playRepository) InsertRewardLedger(ctx context.Context, entry service.PlayRewardLedgerEntry) error {
	exec := r.sqlExec(ctx)
	detail, err := json.Marshal(entry.Detail)
	if err != nil {
		return fmt.Errorf("marshal reward detail: %w", err)
	}
	res, err := exec.ExecContext(ctx, `
		INSERT INTO play_reward_ledger (user_id, source, amount, idempotency_key, detail)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (idempotency_key) DO NOTHING`,
		entry.UserID, entry.Source, entry.Amount, entry.IdempotencyKey, detail,
	)
	if err != nil {
		return fmt.Errorf("insert play reward ledger: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert play reward ledger rows affected: %w", err)
	}
	if n == 0 {
		return service.ErrPlayRewardDuplicate
	}
	return nil
}

func (r *playRepository) GetActiveArenaPeriod(ctx context.Context, now time.Time) (*service.PlayArenaPeriod, error) {
	exec := r.sqlExec(ctx)
	var p service.PlayArenaPeriod
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, start_at, end_at, status
		FROM play_arena_periods
		WHERE status = 'active' AND period_type = 'monthly'
		  AND start_at <= $1 AND end_at > $1
		ORDER BY start_at DESC
		LIMIT 1`, []any{now}, &p.ID, &p.Name, &p.StartAt, &p.EndAt, &p.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active arena period: %w", err)
	}
	return &p, nil
}

func (r *playRepository) EnsureMonthlyArenaPeriod(ctx context.Context, now time.Time) (*service.PlayArenaPeriod, error) {
	if existing, err := r.GetActiveArenaPeriod(ctx, now); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	name := start.Format("2006-01")

	exec := r.sqlExec(ctx)
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO play_arena_periods (name, start_at, end_at, status, period_type)
		VALUES ($1, $2, $3, 'active', 'monthly')`, name, start, end); err != nil {
		return nil, fmt.Errorf("insert arena period: %w", err)
	}
	return r.GetActiveArenaPeriod(ctx, now)
}

func (r *playRepository) ListArenaLeaderboard(ctx context.Context, start, end time.Time, limit int) (result []service.PlayArenaScoreRow, err error) {
	if limit <= 0 {
		limit = 50
	}
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT s.user_id,
		       COALESCE(u.username, '') AS username,
		       COALESCE(u.email, '') AS email,
		       COALESCE(NULLIF(TRIM(ua.url), ''), '') AS avatar_url,
		       s.token_sum
		FROM (
			SELECT user_id,
			       SUM(input_tokens + output_tokens + cache_creation_tokens)::bigint AS token_sum,
			       MIN(created_at) AS first_at
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY user_id
			HAVING SUM(input_tokens + output_tokens + cache_creation_tokens) > 0
		) s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = s.user_id
		ORDER BY s.token_sum DESC, s.first_at ASC
		LIMIT $3`, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("list arena leaderboard: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()

	out := make([]service.PlayArenaScoreRow, 0, limit)
	rank := 0
	for rows.Next() {
		var row service.PlayArenaScoreRow
		var username, email string
		if err := rows.Scan(&row.UserID, &username, &email, &row.AvatarURL, &row.TokenSum); err != nil {
			return nil, fmt.Errorf("scan arena leaderboard: %w", err)
		}
		row.DisplayName = service.PublicPlayDisplayName(username, email, row.UserID)
		row.Email = email
		rank++
		row.Rank = rank
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate arena leaderboard: %w", err)
	}
	return out, nil
}

func (r *playRepository) GetUserArenaScore(ctx context.Context, userID int64, start, end time.Time) (int64, int, error) {
	exec := r.sqlExec(ctx)
	var tokenSum int64
	err := scanSingleRow(ctx, exec, `
		SELECT COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens), 0)::bigint
		FROM usage_logs
		WHERE user_id = $1 AND created_at >= $2 AND created_at < $3`,
		[]any{userID, start, end}, &tokenSum)
	if err != nil {
		return 0, 0, fmt.Errorf("get user arena score: %w", err)
	}
	if tokenSum <= 0 {
		return 0, 0, nil
	}

	var rank int
	err = scanSingleRow(ctx, exec, `
		WITH scores AS (
			SELECT user_id,
			       SUM(input_tokens + output_tokens + cache_creation_tokens)::bigint AS token_sum,
			       MIN(created_at) AS first_at
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY user_id
			HAVING SUM(input_tokens + output_tokens + cache_creation_tokens) > 0
		), mine AS (
			SELECT token_sum, first_at FROM scores WHERE user_id = $3
		)
		SELECT 1 + COUNT(*)::int
		FROM scores s, mine m
		WHERE s.token_sum > m.token_sum
		   OR (s.token_sum = m.token_sum AND s.first_at < m.first_at)`,
		[]any{start, end, userID}, &rank)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return tokenSum, 0, nil
		}
		return tokenSum, 0, fmt.Errorf("get user arena rank: %w", err)
	}
	return tokenSum, rank, nil
}

func (r *playRepository) GetArenaTokensToPrevRank(ctx context.Context, userID int64, start, end time.Time, rank int, tokenSum int64) (int64, error) {
	if rank <= 1 {
		return 0, nil
	}
	exec := r.sqlExec(ctx)
	var prevTokens int64
	err := scanSingleRow(ctx, exec, `
		WITH scores AS (
			SELECT user_id,
			       SUM(input_tokens + output_tokens + cache_creation_tokens)::bigint AS token_sum,
			       MIN(created_at) AS first_at
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY user_id
			HAVING SUM(input_tokens + output_tokens + cache_creation_tokens) > 0
		), ranked AS (
			SELECT user_id, token_sum,
			       ROW_NUMBER() OVER (ORDER BY token_sum DESC, first_at ASC) AS rn
			FROM scores
		)
		SELECT token_sum FROM ranked WHERE rn = $3`,
		[]any{start, end, rank - 1}, &prevTokens)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get arena tokens to prev rank: %w", err)
	}
	gap := prevTokens - tokenSum
	if gap < 0 {
		return 0, nil
	}
	return gap, nil
}
