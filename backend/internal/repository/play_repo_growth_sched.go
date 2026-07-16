package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListExpiredActiveDailyArenaPeriods(ctx context.Context, now time.Time) (result []service.PlayArenaPeriod, err error) {
	exec := r.sqlExec(ctx)
	rows, err := exec.QueryContext(ctx, `
		SELECT id, name, start_at, end_at, status
		FROM play_arena_periods
		WHERE period_type = 'daily' AND status = 'active' AND end_at <= $1
		ORDER BY end_at ASC`, now)
	if err != nil {
		return nil, fmt.Errorf("list expired daily arena periods: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			result = nil
		}
	}()
	out := make([]service.PlayArenaPeriod, 0)
	for rows.Next() {
		var p service.PlayArenaPeriod
		if err := rows.Scan(&p.ID, &p.Name, &p.StartAt, &p.EndAt, &p.Status); err != nil {
			return nil, fmt.Errorf("scan expired daily arena period: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
