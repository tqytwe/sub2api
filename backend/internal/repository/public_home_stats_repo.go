package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const publicHomeStatsQuery = `WITH usage AS (
	SELECT COUNT(*)::bigint AS total_requests
	FROM usage_logs
),
ops AS (
	SELECT
		COALESCE(SUM(success_count) FILTER (
			WHERE bucket_start >= $1 AND bucket_start < $3
		), 0)::bigint AS success_30d,
		COALESCE(SUM(error_count_sla) FILTER (
			WHERE bucket_start >= $1 AND bucket_start < $3
		), 0)::bigint AS error_sla_30d,
		COALESCE(SUM(ttft_avg_ms * ttft_sample_count) FILTER (
			WHERE bucket_start >= $2 AND bucket_start < $3
				AND ttft_avg_ms IS NOT NULL
				AND ttft_sample_count > 0
		), 0)::double precision AS ttft_weighted_sum_24h,
		COALESCE(SUM(ttft_sample_count) FILTER (
			WHERE bucket_start >= $2 AND bucket_start < $3
				AND ttft_avg_ms IS NOT NULL
				AND ttft_sample_count > 0
		), 0)::bigint AS ttft_samples_24h,
		CASE
			WHEN MAX(bucket_start) FILTER (
				WHERE bucket_start >= $1 AND bucket_start < $3
			) IS NULL THEN NULL
			ELSE LEAST(
				MAX(bucket_start) FILTER (
					WHERE bucket_start >= $1 AND bucket_start < $3
				) + INTERVAL '1 hour',
				$3
			)
		END AS ops_data_through
	FROM ops_metrics_hourly
	WHERE platform IS NULL
		AND group_id IS NULL
		AND bucket_start >= $1
		AND bucket_start < $3
)
SELECT
	usage.total_requests,
	ops.success_30d,
	ops.error_sla_30d,
	ops.ttft_weighted_sum_24h,
	ops.ttft_samples_24h,
	ops.ops_data_through
FROM usage
CROSS JOIN ops`

type publicHomeStatsRepository struct {
	db *sql.DB
}

func NewPublicHomeStatsRepository(db *sql.DB) *publicHomeStatsRepository {
	return &publicHomeStatsRepository{db: db}
}

func (r *publicHomeStatsRepository) GetPublicHomeStats(ctx context.Context, now time.Time) (service.PublicHomeStatsRaw, error) {
	var raw service.PublicHomeStatsRaw
	var opsDataThrough sql.NullTime
	err := r.db.QueryRowContext(
		ctx,
		publicHomeStatsQuery,
		now.Add(-30*24*time.Hour),
		now.Add(-24*time.Hour),
		now,
	).Scan(
		&raw.TotalRequests,
		&raw.Success30d,
		&raw.ErrorSLA30d,
		&raw.TTFTWeightedSum24h,
		&raw.TTFTSamples24h,
		&opsDataThrough,
	)
	if err != nil {
		return service.PublicHomeStatsRaw{}, err
	}
	if opsDataThrough.Valid {
		value := opsDataThrough.Time.UTC()
		raw.OpsDataThrough = &value
	}
	return raw, nil
}

const publicStatusSummaryQuery = `SELECT
	total_requests,
	availability_pct,
	availability_sample_count,
	ttft_p50_ms,
	ttft_p95_ms,
	ttft_sample_count,
	data_through,
	computed_at,
	window_end
FROM public_status_snapshots
ORDER BY window_end DESC
LIMIT 1`

const publicStatusSnapshotWindowEndsQuery = `SELECT window_end
FROM public_status_snapshots
WHERE window_end >= $1
  AND window_end <= $2
ORDER BY window_end DESC`

const publicStatusSnapshotReadyQuery = `SELECT EXISTS (
	SELECT 1
	FROM ops_aggregation_watermarks
	WHERE job_name = $1
	  AND completed_through >= $2
)`

const refreshPublicStatusSnapshotQuery = `WITH bounds AS (
	SELECT $1::timestamptz AS window_end
),
watermark AS (
	SELECT completed_through
	FROM ops_aggregation_watermarks
	WHERE job_name = $2
),
ready AS (
	SELECT EXISTS (
		SELECT 1
		FROM ops_aggregation_watermarks
		WHERE job_name = $2
		  AND completed_through >= (SELECT window_end FROM bounds)
	) AS ready
),
request_total AS (
	SELECT COUNT(*)::bigint AS total_requests
	FROM usage_logs
	WHERE created_at < (SELECT window_end FROM bounds)
),
availability AS (
	SELECT
		COALESCE(SUM(success_count), 0)::bigint AS success_count,
		COALESCE(SUM(error_count_sla), 0)::bigint AS error_sla_count
	FROM ops_metrics_hourly
	WHERE platform IS NULL
		AND group_id IS NULL
		AND bucket_start >= (SELECT window_end FROM bounds) - INTERVAL '30 days'
		AND bucket_start < (SELECT window_end FROM bounds)
),
ttft AS (
	SELECT
		COUNT(*)::bigint AS sample_count,
		percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms)::double precision AS p50_ms,
		percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms)::double precision AS p95_ms
	FROM usage_logs
	WHERE created_at >= (SELECT window_end FROM bounds) - INTERVAL '24 hours'
		AND created_at < (SELECT window_end FROM bounds)
		AND first_token_ms IS NOT NULL
)
INSERT INTO public_status_snapshots (
	window_end,
	total_requests,
	availability_pct,
	availability_sample_count,
	ttft_p50_ms,
	ttft_p95_ms,
	ttft_sample_count,
	data_through,
	computed_at
)
SELECT
	b.window_end,
	r.total_requests,
	CASE
		WHEN a.success_count + a.error_sla_count > 0
		THEN a.success_count::double precision / (a.success_count + a.error_sla_count)::double precision * 100
		ELSE NULL
	END,
	a.success_count + a.error_sla_count,
	t.p50_ms,
	t.p95_ms,
	t.sample_count,
	CASE
		WHEN w.completed_through IS NULL THEN NULL
		ELSE LEAST(w.completed_through, b.window_end)
	END,
	NOW()
FROM bounds b
CROSS JOIN request_total r
CROSS JOIN availability a
CROSS JOIN ttft t
CROSS JOIN ready
LEFT JOIN watermark w ON TRUE
WHERE ready.ready
ON CONFLICT (window_end) DO NOTHING`

func (r *publicHomeStatsRepository) GetPublicStatusSummary(ctx context.Context) (service.PublicStatusSummaryRaw, error) {
	var raw service.PublicStatusSummaryRaw
	var availabilityPct sql.NullFloat64
	var ttftP50 sql.NullFloat64
	var ttftP95 sql.NullFloat64
	var dataThrough sql.NullTime

	err := r.db.QueryRowContext(ctx, publicStatusSummaryQuery).Scan(
		&raw.TotalRequests,
		&availabilityPct,
		&raw.AvailabilitySampleCount,
		&ttftP50,
		&ttftP95,
		&raw.TTFTSampleCount,
		&dataThrough,
		&raw.ComputedAt,
		&raw.WindowEnd,
	)
	if err == sql.ErrNoRows {
		return raw, nil
	}
	if err != nil {
		return service.PublicStatusSummaryRaw{}, err
	}

	raw.HasSnapshot = true
	if availabilityPct.Valid {
		value := availabilityPct.Float64
		raw.AvailabilityPct = &value
	}
	if ttftP50.Valid {
		value := ttftP50.Float64
		raw.TTFTP50Ms = &value
	}
	if ttftP95.Valid {
		value := ttftP95.Float64
		raw.TTFTP95Ms = &value
	}
	if dataThrough.Valid {
		value := dataThrough.Time.UTC()
		raw.DataThrough = &value
	}
	raw.ComputedAt = raw.ComputedAt.UTC()
	return raw, nil
}

func (r *publicHomeStatsRepository) ListPublicStatusSnapshotWindowEnds(ctx context.Context, from, through time.Time) ([]time.Time, error) {
	if from.IsZero() || through.IsZero() || from.After(through) {
		return []time.Time{}, nil
	}
	rows, err := r.db.QueryContext(ctx, publicStatusSnapshotWindowEndsQuery, from.UTC(), through.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]time.Time, 0, 8)
	for rows.Next() {
		var windowEnd time.Time
		if err := rows.Scan(&windowEnd); err != nil {
			return nil, err
		}
		out = append(out, windowEnd.UTC())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *publicHomeStatsRepository) IsPublicStatusSnapshotWindowReady(ctx context.Context, windowEnd time.Time) (bool, error) {
	if windowEnd.IsZero() {
		return false, nil
	}
	var ready bool
	if err := r.db.QueryRowContext(
		ctx,
		publicStatusSnapshotReadyQuery,
		service.OpsHourlyAggregationJobName,
		windowEnd.UTC(),
	).Scan(&ready); err != nil {
		return false, err
	}
	return ready, nil
}

func (r *publicHomeStatsRepository) RefreshPublicStatusSnapshot(ctx context.Context, windowEnd time.Time) error {
	if windowEnd.IsZero() {
		return nil
	}
	_, err := r.db.ExecContext(
		ctx,
		refreshPublicStatusSnapshotQuery,
		windowEnd.UTC(),
		service.OpsHourlyAggregationJobName,
	)
	return err
}
