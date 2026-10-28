package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ReadPrioritySchedulingSignals is deliberately limited to persisted usage
// aggregates. It never treats absent quality history as a passing result.
func (r *usageLogRepository) ReadPrioritySchedulingSignals(ctx context.Context, q service.PrioritySchedulingQuery) (map[int64]service.PrioritySchedulingSignal, error) {
	rows, err := r.sql.QueryContext(ctx, `
SELECT account_id,
       COUNT(*) FILTER (WHERE stream AND first_token_ms > 0) AS samples,
       COALESCE(percentile_cont(0.90) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms > 0), 0),
       COALESCE(SUM(COALESCE(actual_cost, 0)), 0),
       COALESCE(SUM(COALESCE(account_stats_cost, total_cost, 0) * COALESCE(account_rate_multiplier, 1)), 0)
FROM usage_logs
WHERE account_id = ANY($1::bigint[])
  AND created_at >= $2 AND created_at <= NOW()
  AND COALESCE(NULLIF(BTRIM(requested_model), ''), model) = $3
  AND ($4::bigint IS NULL OR group_id = $4)
GROUP BY account_id`, pq.Array(q.AccountIDs), q.Since, q.Model, nullablePriorityGroup(q.GroupID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]service.PrioritySchedulingSignal, len(q.AccountIDs))
	for rows.Next() {
		var id int64
		var signal service.PrioritySchedulingSignal
		if err := rows.Scan(&id, &signal.Samples, &signal.P90TTFTMs, &signal.Revenue, &signal.TheoreticalCost); err != nil {
			return nil, err
		}
		out[id] = signal
	}
	return out, rows.Err()
}

func nullablePriorityGroup(id *int64) any {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

var _ service.PrioritySchedulingSignalReader = (*usageLogRepository)(nil)
