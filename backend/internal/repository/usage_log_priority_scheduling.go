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
 SELECT p.account_id, p.id AS plan_id, r.quality_round_id, probe.model_id,
        jsonb_array_length(targets.models) AS model_count,
        COUNT(*) AS sample_count, MAX(parallel.expected) AS expected,
        bool_or(r.quality_judgment->>'verdict' = 'incorrect'
          AND r.status = 'failed' AND r.error_message = 'answer_mismatch') AS failed,
        bool_and(COALESCE(r.quality_judgment->>'verdict' = 'correct'
          AND r.status = 'success', false)) AS passed,
        bool_and(COALESCE(jsonb_typeof(r.pelican_config) = 'object'
          AND probe.model_id IS NOT NULL
          AND targets.models @> jsonb_build_array(probe.model_id)
          AND r.started_at >= p.quality_rule_updated_at
          AND r.created_at >= p.quality_rule_updated_at
          AND r.created_at >= $3 AND r.created_at <= NOW()
          AND r.started_at <= NOW()
          AND r.quality_action <> '' AND r.quality_action <> 'stale_run'
          AND parallel.expected IS NOT NULL AND parallel.expected = parallel.planned
          AND CASE WHEN jsonb_array_length(targets.models) > 1
            THEN jsonb_typeof(r.pelican_config->'model_id') = 'string'
              AND r.pelican_config->'model_ids' = targets.models
            ELSE (r.pelican_config->'model_id' IS NULL
                OR r.pelican_config->'model_id' = 'null'::jsonb
                OR jsonb_typeof(r.pelican_config->'model_id') = 'string')
              AND (r.pelican_config->'model_ids' IS NULL
                OR r.pelican_config->'model_ids' = '[]'::jsonb
                OR r.pelican_config->'model_ids' = targets.models)
            END, false)) AS valid
 FROM scheduled_test_plans p
 JOIN accounts a ON a.id = p.account_id
 JOIN scheduled_test_results r ON r.plan_id = p.id
 CROSS JOIN LATERAL (
   SELECT CASE
     WHEN p.pelican_config->'model_ids' IS NULL OR p.pelican_config->'model_ids' = '[]'::jsonb
       THEN jsonb_build_array(p.model_id)
     WHEN jsonb_typeof(p.pelican_config->'model_ids') = 'array'
       THEN p.pelican_config->'model_ids'
     ELSE '[]'::jsonb END AS models
 ) targets
 CROSS JOIN LATERAL (
   SELECT CASE
     WHEN r.pelican_config->'parallel_count' IS NULL OR r.pelican_config->'parallel_count' = '0'::jsonb THEN 1
     WHEN jsonb_typeof(r.pelican_config->'parallel_count') = 'number'
       AND r.pelican_config->>'parallel_count' ~ '^([1-9]|[12][0-9]|3[0-2])$'
       THEN (r.pelican_config->>'parallel_count')::int
   END AS expected,
   CASE
     WHEN p.pelican_config->'parallel_count' IS NULL OR p.pelican_config->'parallel_count' = '0'::jsonb THEN 1
     WHEN jsonb_typeof(p.pelican_config->'parallel_count') = 'number'
       AND p.pelican_config->>'parallel_count' ~ '^([1-9]|[12][0-9]|3[0-2])$'
       THEN (p.pelican_config->>'parallel_count')::int
   END AS planned
 ) parallel
 CROSS JOIN LATERAL (
   SELECT CASE WHEN jsonb_array_length(targets.models) = 1
     THEN COALESCE(NULLIF(r.pelican_config->>'model_id', ''), p.model_id)
     ELSE NULLIF(r.pelican_config->>'model_id', '') END AS model_id
 ) probe
 WHERE p.account_id = ANY($1::bigint[]) AND p.enabled
   AND a.platform = 'openai' AND a.deleted_at IS NULL
   AND jsonb_typeof(p.pelican_config->'quality') = 'object'
   AND targets.models @> jsonb_build_array($2::text)
   AND jsonb_array_length(targets.models) BETWEEN 1 AND 50
   AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(targets.models) model
     WHERE jsonb_typeof(model.value) <> 'string' OR BTRIM(model.value#>>'{}') = ''
       OR octet_length(model.value#>>'{}') > 100)
   AND (SELECT COUNT(DISTINCT model.value) FROM jsonb_array_elements(targets.models) model)
     = jsonb_array_length(targets.models)
   AND jsonb_array_length(targets.models) * parallel.planned <= 100
   AND BTRIM(r.quality_round_id) <> ''
 GROUP BY p.account_id, p.id, r.quality_round_id, probe.model_id, targets.models
), complete_rounds AS (
 SELECT account_id, plan_id, quality_round_id
 FROM quality_rounds
 GROUP BY account_id, plan_id, quality_round_id
 HAVING bool_and(valid) AND bool_and(sample_count = expected)
   AND MIN(expected) = MAX(expected) AND COUNT(*) = MAX(model_count)
)
SELECT q.account_id, COUNT(*) FILTER (WHERE q.passed OR q.failed),
       COUNT(*) FILTER (WHERE q.passed AND NOT COALESCE(q.failed, false))
FROM quality_rounds q
JOIN complete_rounds c USING (account_id, plan_id, quality_round_id)
WHERE q.model_id = $2
GROUP BY q.account_id`, pq.Array(q.AccountIDs), q.Model, since)
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
