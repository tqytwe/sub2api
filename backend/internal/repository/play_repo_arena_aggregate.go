package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) GetArenaAggregateScore(ctx context.Context, userID int64, periodType string, periodStart time.Time) (int64, int, int64, int, int, error) {
	var score int64
	var rank, newcomerRank, participants int
	var gap int64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		WITH ranked AS (
			SELECT a.subject_id, a.score,
			       ROW_NUMBER() OVER (ORDER BY a.score DESC, a.subject_id ASC)::int AS rank,
			       CASE WHEN u.created_at >= $4 - INTERVAL '30 days'
			            THEN ROW_NUMBER() OVER (
			                PARTITION BY (u.created_at >= $4 - INTERVAL '30 days')
			                ORDER BY a.score DESC, a.subject_id ASC
			            )::int ELSE 0 END AS newcomer_rank
			FROM play_usage_aggregates a
			JOIN users u ON u.id = a.subject_id
			WHERE a.aggregate_type = 'user' AND a.period_type = $1 AND a.period_start = $2::date
		), mine AS (
			SELECT * FROM ranked WHERE subject_id = $3
		), prev AS (
			SELECT r.score FROM ranked r JOIN mine m ON r.rank = m.rank - 1
		)
		SELECT COALESCE((SELECT score FROM mine), 0),
		       COALESCE((SELECT rank FROM mine), 0),
		       GREATEST(COALESCE((SELECT score FROM prev), 0) - COALESCE((SELECT score FROM mine), 0), 0),
		       COALESCE((SELECT newcomer_rank FROM mine), 0),
		       (SELECT COUNT(*)::int FROM ranked)`, []any{periodType, periodStart, userID, time.Now()},
		&score, &rank, &gap, &newcomerRank, &participants)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, 0, 0, 0, fmt.Errorf("get arena aggregate score: %w", err)
	}
	return score, rank, gap, newcomerRank, participants, nil
}

func (r *playRepository) ListArenaAggregateLeaderboard(ctx context.Context, periodType string, periodStart time.Time, limit int) ([]service.PlayArenaScoreRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT a.subject_id,
		       COALESCE(NULLIF(TRIM(u.username), ''), CONCAT('user-', a.subject_id::text)),
		       COALESCE(NULLIF(TRIM(ua.url), ''), ''),
		       a.score
		FROM play_usage_aggregates a
		JOIN users u ON u.id = a.subject_id
		LEFT JOIN user_avatars ua ON ua.user_id = a.subject_id
		WHERE a.aggregate_type = 'user' AND a.period_type = $1 AND a.period_start = $2::date
		ORDER BY a.score DESC, a.subject_id ASC
		LIMIT $3`, periodType, periodStart, limit)
	if err != nil {
		return nil, fmt.Errorf("list arena aggregate leaderboard: %w", err)
	}
	defer rows.Close()
	out := make([]service.PlayArenaScoreRow, 0, limit)
	for rows.Next() {
		var row service.PlayArenaScoreRow
		if err := rows.Scan(&row.UserID, &row.DisplayName, &row.AvatarURL, &row.TokenSum); err != nil {
			return nil, fmt.Errorf("scan arena aggregate leaderboard: %w", err)
		}
		row.Rank = len(out) + 1
		out = append(out, row)
	}
	return out, rows.Err()
}
