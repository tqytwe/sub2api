package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) FreezeArenaRewardRules(ctx context.Context, periodID int64, rulesJSON string) error {
	if rulesJSON == "" {
		rulesJSON = "[]"
	}
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		UPDATE play_arena_periods
		SET reward_rules_json = $2::jsonb,
		    reward_rules_frozen_at = COALESCE(reward_rules_frozen_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1
		  AND (reward_rules_json IS NULL OR reward_rules_json = '[]'::jsonb)`, periodID, rulesJSON)
	if err != nil {
		return fmt.Errorf("freeze arena reward rules: %w", err)
	}
	return nil
}

func (r *playRepository) GetArenaRewardRulesSnapshot(ctx context.Context, periodID int64) (string, error) {
	var raw []byte
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT COALESCE(reward_rules_json, '[]'::jsonb)::text
		FROM play_arena_periods WHERE id = $1`, []any{periodID}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get arena reward rules snapshot: %w", err)
	}
	return string(raw), nil
}

func (r *playRepository) ListExpiredActiveMonthlyArenaPeriods(ctx context.Context, now time.Time) (result []service.PlayArenaPeriod, err error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id, name, start_at, end_at, status, COALESCE(period_type, 'monthly'), settled_at
		FROM play_arena_periods
		WHERE period_type = 'monthly' AND status = 'active' AND end_at <= $1
		ORDER BY end_at ASC, id ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list expired monthly arena periods: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.PlayArenaPeriod, 0)
	for rows.Next() {
		var period service.PlayArenaPeriod
		var periodType sql.NullString
		var settledAt sql.NullTime
		if err := rows.Scan(&period.ID, &period.Name, &period.StartAt, &period.EndAt, &period.Status, &periodType, &settledAt); err != nil {
			return nil, fmt.Errorf("scan expired monthly arena period: %w", err)
		}
		applyPlayArenaPeriodOptionalFields(&period, periodType, settledAt)
		out = append(out, period)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired monthly arena periods: %w", err)
	}
	return out, nil
}

func (r *playRepository) CreateArenaSeasonSnapshot(ctx context.Context, snapshot service.PlayArenaSeasonSnapshot) error {
	if snapshot.PayoutStatus == "" {
		snapshot.PayoutStatus = "paid"
	}
	paidAt := snapshot.PaidAt
	if paidAt == nil {
		now := time.Now()
		paidAt = &now
	}
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_arena_season_snapshots (
			period_id, rank, user_id, token_sum, reward_amount, payout_status, paid_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (period_id, user_id) DO NOTHING`,
		snapshot.PeriodID,
		snapshot.Rank,
		snapshot.UserID,
		snapshot.TokenSum,
		snapshot.RewardAmount,
		snapshot.PayoutStatus,
		paidAt,
	)
	if err != nil {
		return fmt.Errorf("create arena season snapshot: %w", err)
	}
	return nil
}

func (r *playRepository) ListArenaSeasonHistory(ctx context.Context, periodType string, limit int) (result []service.PlayArenaSeasonHistory, err error) {
	if limit < 1 || limit > 24 {
		limit = 6
	}
	if periodType != "daily" {
		periodType = "monthly"
	}
	periodRows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT p.id, p.name, p.start_at, p.end_at, p.status, COALESCE(p.period_type, 'monthly'), p.settled_at,
		       COUNT(s.id)::int, COALESCE(SUM(s.reward_amount), 0)::float8
		FROM play_arena_periods p
		JOIN play_arena_season_snapshots s ON s.period_id = p.id
		WHERE p.period_type = $1 AND p.status = 'settled'
		GROUP BY p.id
		ORDER BY p.end_at DESC, p.id DESC
		LIMIT $2`, periodType, limit)
	if err != nil {
		return nil, fmt.Errorf("list arena season history: %w", err)
	}
	defer func() { _ = periodRows.Close() }()
	history := make([]service.PlayArenaSeasonHistory, 0, limit)
	periodIDs := make([]int64, 0, limit)
	byPeriod := make(map[int64]int, limit)
	for periodRows.Next() {
		var item service.PlayArenaSeasonHistory
		var periodTypeValue sql.NullString
		var settledAt sql.NullTime
		if err := periodRows.Scan(&item.Period.ID, &item.Period.Name, &item.Period.StartAt, &item.Period.EndAt, &item.Period.Status, &periodTypeValue, &settledAt, &item.WinnersCount, &item.TotalAmount); err != nil {
			return nil, fmt.Errorf("scan arena season history: %w", err)
		}
		applyPlayArenaPeriodOptionalFields(&item.Period, periodTypeValue, settledAt)
		item.Winners = []service.PlayArenaSeasonSnapshot{}
		byPeriod[item.Period.ID] = len(history)
		periodIDs = append(periodIDs, item.Period.ID)
		history = append(history, item)
	}
	if err := periodRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate arena season history: %w", err)
	}
	if len(periodIDs) == 0 {
		return history, nil
	}

	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT ranked.period_id, ranked.rank, ranked.user_id, COALESCE(u.email, ''),
		       COALESCE(NULLIF(TRIM(ua.url), ''), ''), ranked.token_sum, ranked.reward_amount,
		       ranked.payout_status, ranked.paid_at
		FROM (
			SELECT s.*, ROW_NUMBER() OVER (PARTITION BY s.period_id ORDER BY s.rank ASC) AS row_num
			FROM play_arena_season_snapshots s
			WHERE s.period_id = ANY($1)
		) ranked
		JOIN users u ON u.id = ranked.user_id
		LEFT JOIN user_avatars ua ON ua.user_id = ranked.user_id
		WHERE ranked.row_num <= 10
		ORDER BY ranked.period_id DESC, ranked.rank ASC`, periodIDs)
	if err != nil {
		return nil, fmt.Errorf("list arena season winners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var periodID int64
		var item service.PlayArenaSeasonSnapshot
		var email string
		if err := rows.Scan(&periodID, &item.Rank, &item.UserID, &email, &item.AvatarURL, &item.TokenSum, &item.RewardAmount, &item.PayoutStatus, &item.PaidAt); err != nil {
			return nil, fmt.Errorf("scan arena season winner: %w", err)
		}
		item.DisplayName, item.Anonymous = service.PublicPlayLeaderboardIdentity(email)
		item.UserID = 0
		if index, ok := byPeriod[periodID]; ok {
			history[index].Winners = append(history[index].Winners, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate arena season winners: %w", err)
	}
	return history, nil
}
