package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ReadPrioritySchedulingSignals combines scoped latency history with account/model
// quality rounds. Missing or inconclusive quality history never implies a pass.
func (r *usageLogRepository) ReadPrioritySchedulingSignals(ctx context.Context, q service.PrioritySchedulingQuery) (map[int64]service.PrioritySchedulingSignal, error) {
	var out map[int64]service.PrioritySchedulingSignal
	var err error
	if q.LatencyMeasurement == "attributed_v1" {
		out, err = r.readAttributedPrioritySchedulingSignals(ctx, q)
	} else {
		out, err = r.readLegacyPrioritySchedulingSignals(ctx, q)
	}
	if err != nil {
		return nil, err
	}
	if err := r.readPriorityQualitySignals(ctx, q, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *usageLogRepository) readLegacyPrioritySchedulingSignals(ctx context.Context, q service.PrioritySchedulingQuery) (map[int64]service.PrioritySchedulingSignal, error) {
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

// Quality probes have account/model attribution; their judge group is not the
// request group. Keep this independent of the latency endpoint/group scope.
func (r *usageLogRepository) readPriorityQualitySignals(ctx context.Context, q service.PrioritySchedulingQuery, out map[int64]service.PrioritySchedulingSignal) error {
	since := q.QualitySince
	if since.IsZero() {
		since = q.Since
	}
	rows, err := r.sql.QueryContext(ctx, `
WITH quality_rounds AS (
 SELECT p.account_id, p.id AS plan_id, r.quality_round_id,
        bool_or(r.quality_judgment->>'verdict' = 'incorrect'
          AND r.status = 'failed' AND r.error_message = 'answer_mismatch') AS failed,
        bool_and(COALESCE(r.quality_judgment->>'verdict' = 'correct'
          AND r.status = 'success', false)) AS passed
 FROM scheduled_test_plans p
 JOIN accounts a ON a.id = p.account_id
 JOIN scheduled_test_results r ON r.plan_id = p.id
 CROSS JOIN LATERAL (
   SELECT CASE
     WHEN r.pelican_config->'parallel_count' IS NULL OR r.pelican_config->'parallel_count' = '0'::jsonb THEN 1
     WHEN jsonb_typeof(r.pelican_config->'parallel_count') = 'number'
       AND r.pelican_config->>'parallel_count' ~ '^([1-9]|[12][0-9]|3[0-2])$'
       THEN (r.pelican_config->>'parallel_count')::int
   END AS expected
 ) parallel
 WHERE p.account_id = ANY($1::bigint[]) AND p.enabled AND p.model_id = $2
   AND a.platform = 'openai' AND a.deleted_at IS NULL
   AND jsonb_typeof(p.pelican_config->'quality') = 'object'
   AND (p.pelican_config->'model_ids' IS NULL
     OR CASE WHEN jsonb_typeof(p.pelican_config->'model_ids') = 'array'
        THEN jsonb_array_length(p.pelican_config->'model_ids') = 0
          OR (jsonb_array_length(p.pelican_config->'model_ids') = 1
            AND p.pelican_config->'model_ids'->>0 = p.model_id) ELSE false END)
   AND BTRIM(r.quality_round_id) <> ''
 GROUP BY p.account_id, p.id, r.quality_round_id
 HAVING bool_and(COALESCE(jsonb_typeof(r.pelican_config) = 'object'
          AND COALESCE(NULLIF(r.pelican_config->>'model_id', ''), p.model_id) = p.model_id
          AND r.started_at >= p.quality_rule_updated_at
          AND r.created_at >= p.quality_rule_updated_at
          AND r.created_at >= $3 AND r.created_at <= NOW()
          AND r.started_at <= NOW()
          AND r.quality_action <> '' AND r.quality_action <> 'stale_run'
          AND (r.pelican_config->'model_ids' IS NULL
            OR CASE WHEN jsonb_typeof(r.pelican_config->'model_ids') = 'array'
               THEN jsonb_array_length(r.pelican_config->'model_ids') = 0
                 OR (jsonb_array_length(r.pelican_config->'model_ids') = 1
                   AND r.pelican_config->'model_ids'->>0 = p.model_id) ELSE false END), false))
    AND bool_and(parallel.expected IS NOT NULL)
    AND MIN(parallel.expected) = MAX(parallel.expected)
    AND COUNT(*) = MAX(parallel.expected)
)
SELECT account_id, COUNT(*) FILTER (WHERE passed OR failed),
       COUNT(*) FILTER (WHERE passed AND NOT COALESCE(failed, false))
FROM quality_rounds
GROUP BY account_id`, pq.Array(q.AccountIDs), q.Model, since)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var samples, passed int
		if err := rows.Scan(&id, &samples, &passed); err != nil {
			return err
		}
		signal := out[id]
		signal.QualitySamples, signal.QualityPassed = samples, passed
		out[id] = signal
	}
	return rows.Err()
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
