package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ReadPrioritySchedulingSignals is deliberately limited to persisted usage
// aggregates. It never treats absent quality history as a passing result.
func (r *usageLogRepository) ReadPrioritySchedulingSignals(ctx context.Context, q service.PrioritySchedulingQuery) (map[int64]service.PrioritySchedulingSignal, error) {
	if q.LatencyMeasurement == "attributed_v1" {
		return r.readAttributedPrioritySchedulingSignals(ctx, q)
	}
	percentile := priorityLatencyPercentile(q.LatencyPercentile)
	rows, err := r.sql.QueryContext(ctx, fmt.Sprintf(`
SELECT account_id,
       COUNT(*) FILTER (WHERE stream AND first_token_ms > 0) AS samples,
       COALESCE(percentile_cont(%0.2f) WITHIN GROUP (ORDER BY first_token_ms) FILTER (WHERE stream AND first_token_ms > 0), 0),
       MAX(created_at) FILTER (WHERE stream AND first_token_ms > $6),
       COALESCE(SUM(COALESCE(actual_cost, 0)), 0),
       COALESCE(SUM(COALESCE(account_stats_cost, total_cost, 0) * COALESCE(account_rate_multiplier, 1)), 0)
FROM usage_logs
WHERE account_id = ANY($1::bigint[])
  AND created_at >= $2 AND created_at < NOW()
  AND COALESCE(NULLIF(BTRIM(requested_model), ''), model) = $3
  AND ($4::bigint IS NULL OR group_id = $4)
  AND ($5 = '' OR COALESCE(NULLIF(BTRIM(inbound_endpoint), ''), '') = $5)
GROUP BY account_id`, percentile), pq.Array(q.AccountIDs), q.Since, q.Model, nullablePriorityGroup(q.GroupID), q.InboundEndpoint, q.TargetTTFTMs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]service.PrioritySchedulingSignal, len(q.AccountIDs))
	for rows.Next() {
		var id int64
		var signal service.PrioritySchedulingSignal
		var latestBad sql.NullTime
		if err := rows.Scan(&id, &signal.Samples, &signal.P90TTFTMs, &latestBad, &signal.Revenue, &signal.TheoreticalCost); err != nil {
			return nil, err
		}
		if latestBad.Valid {
			signal.LatestBadLatencyAt = &latestBad.Time
		}
		out[id] = signal
	}
	return out, rows.Err()
}

func (r *usageLogRepository) readAttributedPrioritySchedulingSignals(ctx context.Context, q service.PrioritySchedulingQuery) (map[int64]service.PrioritySchedulingSignal, error) {
	percentile := priorityLatencyPercentile(q.LatencyPercentile)
	rows, err := r.sql.QueryContext(ctx, fmt.Sprintf(`
SELECT a.account_id,
       COUNT(*) AS samples,
       COALESCE(percentile_cont(%0.2f) WITHIN GROUP (ORDER BY o.first_semantic_ms), 0),
       MAX(o.created_at) FILTER (WHERE o.first_semantic_ms > $6),
       0::double precision, 0::double precision
FROM gateway_request_priority_observations o
JOIN gateway_request_attempt_evidence e ON e.attempt_id=o.attempt_id
JOIN gateway_request_attempts a ON a.request_id=e.request_id AND a.attempt_no=e.attempt_no
JOIN gateway_requests r ON r.id=a.request_id
WHERE a.account_id = ANY($1::bigint[])
  AND o.created_at >= $2 AND o.created_at <= NOW()
  AND o.model = $3
  AND o.inbound_endpoint = $4
  AND o.group_id IS NOT DISTINCT FROM $5::bigint
GROUP BY a.account_id`, percentile), pq.Array(q.AccountIDs), q.Since, q.Model, q.InboundEndpoint, nullablePriorityGroup(q.GroupID), q.TargetTTFTMs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int64]service.PrioritySchedulingSignal, len(q.AccountIDs))
	for rows.Next() {
		var id int64
		var signal service.PrioritySchedulingSignal
		var latestBad sql.NullTime
		if err := rows.Scan(&id, &signal.Samples, &signal.P90TTFTMs, &latestBad, &signal.Revenue, &signal.TheoreticalCost); err != nil {
			return nil, err
		}
		if latestBad.Valid {
			signal.LatestBadLatencyAt = &latestBad.Time
		}
		out[id] = signal
	}
	return out, rows.Err()
}

func priorityLatencyPercentile(value int) float64 {
	switch value {
	case 50:
		return 0.50
	case 70:
		return 0.70
	case 95:
		return 0.95
	default:
		return 0.90
	}
}

func nullablePriorityGroup(id *int64) any {
	if id == nil || *id <= 0 {
		return nil
	}
	return *id
}

var _ service.PrioritySchedulingSignalReader = (*usageLogRepository)(nil)
