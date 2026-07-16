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

func NewPublicHomeStatsRepository(db *sql.DB) service.PublicHomeStatsRepository {
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
