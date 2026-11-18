package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Trends reuse the completed, uniquely instrumented sample population of the
// account performance reader. Missing or unsuccessful timings are not zeroes.
func (r *usageLogRepository) ReadPriorityLatencySeries(ctx context.Context, q service.PriorityLatencySeriesQuery) ([]service.PriorityLatencyBucket, error) {
	if len(q.AccountIDs) == 0 {
		return []service.PriorityLatencyBucket{}, nil
	}
	base, _, ok := strings.Cut(performanceMetricsSQL, ", rounds AS (")
	if !ok {
		return nil, fmt.Errorf("account performance query has no rounds boundary")
	}
	rows, err := r.sql.QueryContext(ctx, base+`
 SELECT account_id,
 date_bin(($4::text || ' minutes')::interval,event_at,TIMESTAMPTZ 'epoch') AS bucket,
 COUNT(*), percentile_cont(.5) WITHIN GROUP (ORDER BY first_token_ms),
 percentile_cont(.9) WITHIN GROUP (ORDER BY first_token_ms),
 COUNT(*) FILTER(WHERE first_token_ms>$5),NULL::bigint
 FROM successful
 WHERE stream AND first_token_ms>=0 AND first_token_ms<=duration_ms AND output_tokens>=2
 AND ($6='' OR COALESCE(NULLIF(requested_model,''),model)=$6)
	AND ($7::bigint IS NULL OR group_id=$7)
 GROUP BY account_id,bucket ORDER BY account_id,bucket`, pq.Array(q.AccountIDs), q.Since, time.Now(), q.BucketMinutes, q.SlowTTFTMs, q.Model, nullablePriorityGroup(q.GroupID))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PriorityLatencyBucket, 0)
	for rows.Next() {
		var row service.PriorityLatencyBucket
		if err := rows.Scan(&row.AccountID, &row.Bucket, &row.Samples, &row.P50TTFTMs, &row.P90TTFTMs, &row.SlowSamples, &row.ErrorSamples); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
