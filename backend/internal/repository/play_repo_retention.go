package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) GetCheckinStreakOnDate(ctx context.Context, userID int64, date time.Time) (int, bool, error) {
	exec := r.sqlExec(ctx)
	var streak int
	err := scanSingleRow(ctx, exec, `
		SELECT streak_count FROM play_checkins
		WHERE user_id = $1 AND checkin_date = $2`,
		[]any{userID, date.Format("2006-01-02")}, &streak)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("get checkin streak: %w", err)
	}
	return streak, true, nil
}

func (r *playRepository) GetArenaPeriodByID(ctx context.Context, periodID int64) (*service.PlayArenaPeriod, error) {
	exec := r.sqlExec(ctx)
	var p service.PlayArenaPeriod
	var periodType sql.NullString
	var settledAt sql.NullTime
	err := scanSingleRow(ctx, exec, `
		SELECT id, name, start_at, end_at, status, COALESCE(period_type, 'monthly') AS period_type, settled_at
		FROM play_arena_periods WHERE id = $1`, []any{periodID},
		&p.ID, &p.Name, &p.StartAt, &p.EndAt, &p.Status, &periodType, &settledAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get arena period: %w", err)
	}
	applyPlayArenaPeriodOptionalFields(&p, periodType, settledAt)
	return &p, nil
}

func (r *playRepository) MarkArenaPeriodSettled(ctx context.Context, periodID int64) error {
	exec := r.sqlExec(ctx)
	res, err := exec.ExecContext(ctx, `
		UPDATE play_arena_periods
		SET status = 'settled',
		    settled_at = COALESCE(settled_at, NOW()),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'active'`, periodID)
	if err != nil {
		return fmt.Errorf("mark arena period settled: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark arena period settled rows: %w", err)
	}
	if n == 0 {
		return service.ErrPlayArenaPeriodNotSettleable
	}
	return nil
}

func (r *playRepository) UpsertRechargeBoost(ctx context.Context, userID int64, expiresAt time.Time) error {
	exec := r.sqlExec(ctx)
	_, err := exec.ExecContext(ctx, `
		INSERT INTO play_recharge_boosts (user_id, expires_at)
		VALUES ($1, $2)`, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("insert recharge boost: %w", err)
	}
	return nil
}

func (r *playRepository) GetActiveRechargeBoost(ctx context.Context, userID int64, now time.Time) (*time.Time, error) {
	exec := r.sqlExec(ctx)
	var expiresAt time.Time
	err := scanSingleRow(ctx, exec, `
		SELECT expires_at FROM play_recharge_boosts
		WHERE user_id = $1 AND expires_at > $2
		ORDER BY expires_at DESC
		LIMIT 1`, []any{userID, now}, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active recharge boost: %w", err)
	}
	return &expiresAt, nil
}

func (r *playRepository) HasCompletedBalanceRechargeSince(ctx context.Context, userID int64, since time.Time) (bool, error) {
	exec := r.sqlExec(ctx)
	var exists bool
	err := scanSingleRow(ctx, exec, `
		SELECT EXISTS(
			SELECT 1 FROM payment_orders
			WHERE user_id = $1
			  AND order_type = $2
			  AND status = $3
			  AND completed_at >= $4
		)`, []any{userID, payment.OrderTypeBalance, payment.OrderStatusCompleted, since}, &exists)
	if err != nil {
		return false, fmt.Errorf("check completed balance recharge: %w", err)
	}
	return exists, nil
}
