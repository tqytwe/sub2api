package repository

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *channelMonitorV2Repository) attachRecentSampleBuckets(ctx context.Context, matrix *service.ChannelMonitorV2Matrix, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, admin bool) error {
	pairs := make(map[[2]string]bool)
	for _, row := range matrix.Items {
		if row.GroupID != nil {
			pairs[[2]string{row.Platform, fmt.Sprint(*row.GroupID)}] = true
		}
	}
	platforms, ids := []string{}, []int64{}
	for pair := range pairs {
		var id int64
		_, _ = fmt.Sscan(pair[1], &id)
		platforms = append(platforms, pair[0])
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	start, end := time.Now().UTC().Add(-7*24*time.Hour), time.Now().UTC().Add(time.Minute)
	modelSQL, modelArgs := channelMonitorV2SampleModelClause(filter, cfg, "m", 6)
	modelFilter := ""
	if modelSQL != "" {
		modelFilter = " AND " + modelSQL
	}
	query := `SELECT s.bucket_start, s.platform, s.group_id, s.success_requests, s.error_requests, s.input_tokens, s.output_tokens, s.cache_creation_tokens, s.cache_read_tokens, s.ttft_sum_ms, s.ttft_count, s.duration_sum_ms, s.duration_count
	FROM unnest($1::text[], $2::bigint[]) AS keys(platform, group_id)
	CROSS JOIN LATERAL (
	 SELECT m.bucket_start, m.platform, m.group_id,
	        SUM(m.success_requests)::bigint AS success_requests, SUM(m.error_requests)::bigint AS error_requests,
	        SUM(m.input_tokens)::bigint AS input_tokens, SUM(m.output_tokens)::bigint AS output_tokens,
	        SUM(m.cache_creation_tokens)::bigint AS cache_creation_tokens, SUM(m.cache_read_tokens)::bigint AS cache_read_tokens,
	        SUM(m.ttft_sum_ms)::bigint AS ttft_sum_ms, SUM(m.ttft_count)::bigint AS ttft_count,
	        SUM(m.duration_sum_ms)::bigint AS duration_sum_ms, SUM(m.duration_count)::bigint AS duration_count
	 FROM channel_monitor_v2_metrics_rollup m
	 WHERE m.bucket_seconds=300 AND m.platform=keys.platform AND m.group_id=keys.group_id AND m.bucket_start >= $3 AND m.bucket_start < $4` + modelFilter + `
	 GROUP BY m.bucket_start,m.platform,m.group_id
	 HAVING SUM(m.success_requests + m.error_requests) >= $5
	 ORDER BY m.bucket_start DESC LIMIT 18
	) s ORDER BY s.bucket_start`
	minimum := cfg.HealthThresholds.MinimumSample
	if minimum <= 0 {
		minimum = 50
	}
	args := []any{pq.Array(platforms), pq.Array(ids), start, end, minimum}
	args = append(args, modelArgs...)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("load recent monitor samples: %w", err)
	}
	defer rows.Close()
	loaded := map[[2]string][]service.ChannelMonitorV2TrendPoint{}
	accumulators := map[sampleBucketKey]*metricAccumulator{}
	for rows.Next() {
		var point service.ChannelMonitorV2TrendPoint
		var platform string
		var groupID int64
		var success, errors, input, output, cacheCreate, cacheRead, ttftSum, ttftCount, durationSum, durationCount int64
		if err := rows.Scan(&point.BucketStart, &platform, &groupID, &success, &errors, &input, &output, &cacheCreate, &cacheRead, &ttftSum, &ttftCount, &durationSum, &durationCount); err != nil {
			return err
		}
		count := success + errors
		fact := channelMonitorV2Fact{BucketStart: point.BucketStart.UTC().Format(time.RFC3339Nano), Platform: platform, GroupID: groupID, Success: success, Errors: errors, Input: input, Output: output, CacheCreation: cacheCreate, CacheRead: cacheRead, TTFTSum: ttftSum, TTFTCount: ttftCount, DurationSum: durationSum, DurationCount: durationCount}
		acc := newMetricAccumulator()
		acc.addFact(fact)
		key := sampleBucketKey{platform: platform, groupID: groupID, bucketUnix: point.BucketStart.Unix()}
		accumulators[key] = acc
		m := acc.metric(service.ChannelMonitorV2RecentSampleBucket.Minutes(), admin)
		m.HasSamples = count > 0
		point.Metrics = m
		point.Health = service.ChannelMonitorV2HealthForWithThresholds(m, cfg.HealthThresholds)
		loaded[[2]string{platform, fmt.Sprint(groupID)}] = append(loaded[[2]string{platform, fmt.Sprint(groupID)}], point)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := r.attachRecentSampleHistograms(ctx, platforms, ids, start, end, filter, cfg, modelArgs, accumulators); err != nil {
		return err
	}
	for i := range matrix.Items {
		if matrix.Items[i].GroupID != nil {
			platform, groupID := matrix.Items[i].Platform, *matrix.Items[i].GroupID
			buckets := loaded[[2]string{platform, fmt.Sprint(groupID)}]
			sort.Slice(buckets, func(a, b int) bool { return buckets[a].BucketStart.Before(buckets[b].BucketStart) })
			for j := range buckets {
				acc := accumulators[sampleBucketKey{platform: platform, groupID: groupID, bucketUnix: buckets[j].BucketStart.Unix()}]
				if acc != nil {
					buckets[j].Metrics.TTFT = latencyMetric(acc.ttftSum, acc.ttftCount, acc.hist["ttft"])
					buckets[j].Metrics.Duration = latencyMetric(acc.durationSum, acc.durationCount, acc.hist["duration"])
				}
				buckets[j].Health = service.ChannelMonitorV2HealthForWithThresholds(buckets[j].Metrics, cfg.HealthThresholds)
			}
			matrix.Items[i].SampleBuckets = &buckets
		}
	}
	return nil
}

type sampleBucketKey struct {
	platform   string
	groupID    int64
	bucketUnix int64
}

func (r *channelMonitorV2Repository) attachRecentSampleHistograms(ctx context.Context, platforms []string, ids []int64, start, end time.Time, filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, modelArgs []any, accumulators map[sampleBucketKey]*metricAccumulator) error {
	modelSQL, _ := channelMonitorV2SampleModelClause(filter, cfg, "h", 5)
	modelFilter := ""
	if modelSQL != "" {
		modelFilter = " AND " + modelSQL
	}
	args := []any{pq.Array(platforms), pq.Array(ids), start, end}
	args = append(args, modelArgs...)
	query := `SELECT h.bucket_start, h.platform, h.group_id, h.metric, h.upper_bound_ms, SUM(h.sample_count)
	FROM unnest($1::text[], $2::bigint[]) AS keys(platform, group_id)
	JOIN channel_monitor_v2_latency_histograms_rollup h
	  ON h.platform=keys.platform AND h.group_id=keys.group_id
	WHERE h.bucket_seconds=300 AND h.user_id=0 AND h.bucket_start >= $3 AND h.bucket_start < $4` + modelFilter + `
	GROUP BY h.bucket_start,h.platform,h.group_id,h.metric,h.upper_bound_ms`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("load recent monitor sample latency: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bucket time.Time
		var platform, metric string
		var groupID, upperBound, count int64
		if err := rows.Scan(&bucket, &platform, &groupID, &metric, &upperBound, &count); err != nil {
			return err
		}
		if acc := accumulators[sampleBucketKey{platform: platform, groupID: groupID, bucketUnix: bucket.Unix()}]; acc != nil {
			acc.addHistogram(channelMonitorV2Histogram{Metric: metric, UpperBound: upperBound, Count: count})
		}
	}
	return rows.Err()
}

func channelMonitorV2SampleModelClause(filter service.ChannelMonitorV2Filter, cfg service.ChannelMonitorV2Config, alias string, firstArg int) (string, []any) {
	if len(filter.Models) == 0 {
		return "", nil
	}
	selected := make(map[string]bool)
	other := false
	for _, model := range filter.Models {
		if model == service.ChannelMonitorV2OtherModel {
			other = true
		} else if strings.TrimSpace(model) != "" {
			selected[strings.TrimSpace(model)] = true
		}
	}
	platforms := channelMonitorV2EnabledPlatforms(cfg)
	if len(filter.Platforms) > 0 {
		platforms = intersectStrings(platforms, filter.Platforms)
	}
	args := []any{}
	parts := []string{}
	next := firstArg
	for _, platform := range platforms {
		allow := channelMonitorV2ConfiguredModels(cfg, platform)
		exact := []string{}
		if len(allow) == 0 {
			for model := range selected {
				exact = append(exact, model)
			}
		} else {
			for model := range selected {
				if containsString(allow, model) {
					exact = append(exact, model)
				}
			}
		}
		sort.Strings(exact)
		sub := []string{}
		if len(exact) > 0 {
			args = append(args, platform, pq.Array(exact))
			sub = append(sub, fmt.Sprintf("(%s.platform=$%d AND %s.model=ANY($%d))", alias, next, alias, next+1))
			next += 2
		}
		if other && len(allow) > 0 {
			args = append(args, platform, pq.Array(allow))
			sub = append(sub, fmt.Sprintf("(%s.platform=$%d AND NOT (%s.model=ANY($%d)))", alias, next, alias, next+1))
			next += 2
		}
		if len(sub) > 0 {
			parts = append(parts, "("+strings.Join(sub, " OR ")+")")
		}
	}
	if len(parts) == 0 {
		return "FALSE", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}

func channelMonitorV2ConfiguredModels(cfg service.ChannelMonitorV2Config, platform string) []string {
	for _, configured := range cfg.Platforms {
		if configured.Platform == platform {
			return configured.Models
		}
	}
	return nil
}
