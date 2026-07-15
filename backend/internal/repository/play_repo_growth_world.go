package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) GetLatestPublicMetricSnapshot(ctx context.Context) (*service.PublicMetricSnapshot, error) {
	var out service.PublicMetricSnapshot
	var successRate sql.NullFloat64
	var p50, p95 sql.NullInt64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
		SELECT snapshot_id, bucket_at, source, requests_24h, requests_total,
		       active_users_7d, tokens_total, success_rate_30d, p50_ttft_ms, p95_ttft_ms
		FROM public_metric_snapshots
		ORDER BY bucket_at DESC
		LIMIT 1`, nil,
		&out.SnapshotID, &out.UpdatedAt, &out.Source, &out.Requests24h, &out.RequestsTotal,
		&out.ActiveUsers7d, &out.TokensTotal, &successRate, &p50, &p95)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get public metric snapshot: %w", err)
	}
	assignPublicMetricNullable(&out, successRate, p50, p95)
	return &out, nil
}

func (r *playRepository) RefreshPublicMetricSnapshot(ctx context.Context, bucket time.Time) (*service.PublicMetricSnapshot, error) {
	exec := r.sqlExec(ctx)
	snapshotID := bucket.UTC().Format(time.RFC3339)
	var out service.PublicMetricSnapshot
	var successRate sql.NullFloat64
	var p50, p95 sql.NullInt64
	err := scanSingleRow(ctx, exec, `
		WITH success AS (
			SELECT COUNT(*)::bigint AS total,
			       COUNT(*) FILTER (WHERE created_at >= $2 - INTERVAL '24 hours')::bigint AS requests_24h,
			       COUNT(DISTINCT user_id) FILTER (WHERE created_at >= $2 - INTERVAL '7 days')::bigint AS active_users_7d,
			       COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0)::bigint AS tokens_total,
			       COUNT(*) FILTER (WHERE created_at >= $2 - INTERVAL '30 days')::bigint AS success_30d
			FROM usage_logs
			WHERE request_type <> 4
		), failures AS (
			SELECT COUNT(*)::bigint AS failures_30d
			FROM ops_error_logs
			WHERE created_at >= $2 - INTERVAL '30 days'
			  AND COALESCE(is_count_tokens, false) = false
		), latency AS (
			SELECT percentile_cont(0.50) WITHIN GROUP (ORDER BY first_token_ms)::bigint AS p50,
			       percentile_cont(0.95) WITHIN GROUP (ORDER BY first_token_ms)::bigint AS p95
			FROM usage_logs
			WHERE first_token_ms > 0 AND created_at >= $2 - INTERVAL '30 days'
		), metric_values AS (
			SELECT s.requests_24h, s.total, s.active_users_7d, s.tokens_total,
			       CASE WHEN s.success_30d + f.failures_30d > 0
			            THEN ROUND((s.success_30d::numeric * 100) / (s.success_30d + f.failures_30d), 4)
			            ELSE NULL END AS success_rate,
			       l.p50, l.p95
			FROM success s CROSS JOIN failures f CROSS JOIN latency l
		), upserted AS (
			INSERT INTO public_metric_snapshots (
				snapshot_id, bucket_at, source, requests_24h, requests_total,
				active_users_7d, tokens_total, success_rate_30d, p50_ttft_ms, p95_ttft_ms
			)
			SELECT $1, $2, 'live', requests_24h, total, active_users_7d, tokens_total,
			       success_rate, p50, p95
			FROM metric_values
			ON CONFLICT (snapshot_id) DO UPDATE SET
				requests_24h = EXCLUDED.requests_24h,
				requests_total = EXCLUDED.requests_total,
				active_users_7d = EXCLUDED.active_users_7d,
				tokens_total = EXCLUDED.tokens_total,
				success_rate_30d = EXCLUDED.success_rate_30d,
				p50_ttft_ms = EXCLUDED.p50_ttft_ms,
				p95_ttft_ms = EXCLUDED.p95_ttft_ms
			RETURNING snapshot_id, bucket_at, source, requests_24h, requests_total,
			          active_users_7d, tokens_total, success_rate_30d, p50_ttft_ms, p95_ttft_ms
		)
		SELECT * FROM upserted`, []any{snapshotID, bucket.UTC()},
		&out.SnapshotID, &out.UpdatedAt, &out.Source, &out.Requests24h, &out.RequestsTotal,
		&out.ActiveUsers7d, &out.TokensTotal, &successRate, &p50, &p95)
	if err != nil {
		return nil, fmt.Errorf("refresh public metric snapshot: %w", err)
	}
	assignPublicMetricNullable(&out, successRate, p50, p95)
	return &out, nil
}

func assignPublicMetricNullable(out *service.PublicMetricSnapshot, success sql.NullFloat64, p50, p95 sql.NullInt64) {
	if success.Valid {
		v := success.Float64
		out.SuccessRate30d = &v
	}
	if p50.Valid {
		v := p50.Int64
		out.P50TTFTMs = &v
	}
	if p95.Valid {
		v := p95.Int64
		out.P95TTFTMs = &v
	}
}

func (r *playRepository) ListPublicActivity(ctx context.Context, limit, minCount int) ([]service.PlayPublicActivity, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
		SELECT id, event_type, CONCAT('#', actor_hash), payload, created_at
		FROM play_activity_events
		WHERE is_public = TRUE
		  AND (event_type <> 'usage_day' OR COALESCE((payload->>'requests')::int, 0) >= $2)
		ORDER BY created_at DESC
		LIMIT $1`, limit, minCount)
	if err != nil {
		return nil, fmt.Errorf("list public activity: %w", err)
	}
	defer rows.Close()
	out := make([]service.PlayPublicActivity, 0, limit)
	for rows.Next() {
		var item service.PlayPublicActivity
		var payload []byte
		if err := rows.Scan(&item.ID, &item.EventType, &item.Actor, &payload, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan public activity: %w", err)
		}
		_ = json.Unmarshal(payload, &item.Payload)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *playRepository) InsertPlayActivity(ctx context.Context, eventKey, eventType string, userID int64, subjectType string, subjectID int64, payload map[string]any, createdAt time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal play activity: %w", err)
	}
	_, err = r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_activity_events (event_key, event_type, actor_hash, subject_type, subject_id, payload, created_at)
		VALUES ($1, $2, UPPER(SUBSTRING(MD5($3::text || ':growth-world-v1') FROM 1 FOR 4)), NULLIF($4, ''), NULLIF($5, 0), $6, $7)
		ON CONFLICT (event_key) DO UPDATE SET payload = EXCLUDED.payload, created_at = EXCLUDED.created_at`,
		eventKey, eventType, userID, subjectType, subjectID, encoded, createdAt)
	if err != nil {
		return fmt.Errorf("insert play activity: %w", err)
	}
	return nil
}

func (r *playRepository) RefreshPlayUsageAggregates(ctx context.Context, now time.Time, rechargeMultiplier, campaignMultiplier float64, weeklyTokenTarget, weeklyRequestTarget int64, firstRequestTickets, teamWeeklyTickets int) error {
	if rechargeMultiplier < 1 {
		rechargeMultiplier = 1
	}
	if campaignMultiplier < 1 {
		campaignMultiplier = 1
	}
	dayStart := startOfDay(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	weekStart := dayStart.AddDate(0, 0, -((int(dayStart.Weekday()) + 6) % 7))
	periods := []struct {
		name  string
		start time.Time
	}{
		{name: "daily", start: dayStart},
		{name: "weekly", start: weekStart},
		{name: "monthly", start: monthStart},
	}
	for _, period := range periods {
		if err := r.refreshUsageAggregatePeriod(ctx, period.name, period.start, now, rechargeMultiplier, campaignMultiplier); err != nil {
			return err
		}
	}
	if _, err := r.sqlExec(ctx).ExecContext(ctx, `
		INSERT INTO play_team_weekly_progress (
			team_id, week_start, token_target, request_target, token_sum, request_count, active_days, completed_at, updated_at
		)
		SELECT t.id, $1::date, $2, $3,
		       COALESCE(a.token_sum, 0), COALESCE(a.request_count, 0), COALESCE(a.active_days, 0),
		       CASE WHEN COALESCE(a.token_sum, 0) >= $2 AND COALESCE(a.request_count, 0) >= $3 THEN COALESCE(p.completed_at, NOW()) ELSE NULL END,
		       NOW()
		FROM play_teams t
		LEFT JOIN play_usage_aggregates a ON a.aggregate_type = 'team' AND a.subject_id = t.id
		  AND a.period_type = 'weekly' AND a.period_start = $1::date
		LEFT JOIN play_team_weekly_progress p ON p.team_id = t.id AND p.week_start = $1::date
		ON CONFLICT (team_id, week_start) DO UPDATE SET
			token_target = EXCLUDED.token_target,
			request_target = EXCLUDED.request_target,
			token_sum = EXCLUDED.token_sum,
			request_count = EXCLUDED.request_count,
			active_days = EXCLUDED.active_days,
			completed_at = COALESCE(play_team_weekly_progress.completed_at, EXCLUDED.completed_at),
			updated_at = NOW()`, weekStart, weeklyTokenTarget, weeklyRequestTarget); err != nil {
		return fmt.Errorf("refresh team weekly progress: %w", err)
	}
	if err := r.syncPublicUsageActivity(ctx, now); err != nil {
		return err
	}
	return r.grantGrowthBlindboxTickets(ctx, weekStart, firstRequestTickets, teamWeeklyTickets)
}

func (r *playRepository) refreshUsageAggregatePeriod(ctx context.Context, periodType string, start, now time.Time, rechargeMultiplier, campaignMultiplier float64) error {
	exec := r.sqlExec(ctx)
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO play_usage_aggregates (
			aggregate_type, subject_id, period_type, period_start,
			request_count, token_sum, active_days, score, updated_at
		)
		SELECT 'user', ul.user_id, $1, $2::date,
		       COUNT(*)::bigint,
		       SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens)::bigint,
		       COUNT(DISTINCT DATE(ul.created_at))::int,
		       ROUND(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens)::numeric
		         * $4
		         * CASE WHEN EXISTS (
		             SELECT 1 FROM play_recharge_boosts b WHERE b.user_id = ul.user_id AND b.expires_at > $3
		           ) THEN $5 ELSE 1 END)::bigint,
		       NOW()
		FROM usage_logs ul
		WHERE ul.created_at >= $2 AND ul.created_at < $3 AND ul.request_type <> 4
		GROUP BY ul.user_id
		HAVING SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens) > 0
		ON CONFLICT (aggregate_type, subject_id, period_type, period_start) DO UPDATE SET
			request_count = EXCLUDED.request_count,
			token_sum = EXCLUDED.token_sum,
			active_days = EXCLUDED.active_days,
			score = EXCLUDED.score,
			updated_at = NOW()`, periodType, start, now, campaignMultiplier, rechargeMultiplier); err != nil {
		return fmt.Errorf("refresh user %s aggregate: %w", periodType, err)
	}
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM play_usage_aggregates a
		WHERE a.aggregate_type = 'user' AND a.period_type = $1 AND a.period_start = $2::date
		  AND NOT EXISTS (
			SELECT 1 FROM usage_logs ul
			WHERE ul.user_id = a.subject_id AND ul.created_at >= $2 AND ul.created_at < $3
			  AND ul.request_type <> 4
			  AND ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens > 0
		  )`, periodType, start, now); err != nil {
		return fmt.Errorf("delete stale user %s aggregates: %w", periodType, err)
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO play_usage_aggregates (
			aggregate_type, subject_id, period_type, period_start,
			request_count, token_sum, active_days, score, updated_at
		)
		SELECT 'team', tm.team_id, $1, $2::date,
		       COUNT(*)::bigint,
		       SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens)::bigint,
		       COUNT(DISTINCT DATE(ul.created_at))::int,
		       ROUND(SUM((ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens)::numeric
		         * $4
		         * CASE WHEN EXISTS (
		             SELECT 1 FROM play_recharge_boosts b WHERE b.user_id = ul.user_id AND b.expires_at > $3
		           ) THEN $5 ELSE 1 END))::bigint,
		       NOW()
		FROM usage_logs ul
		JOIN play_team_members tm ON tm.user_id = ul.user_id
		WHERE ul.created_at >= $2 AND ul.created_at < $3 AND ul.request_type <> 4
		GROUP BY tm.team_id
		HAVING SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens) > 0
		ON CONFLICT (aggregate_type, subject_id, period_type, period_start) DO UPDATE SET
			request_count = EXCLUDED.request_count,
			token_sum = EXCLUDED.token_sum,
			active_days = EXCLUDED.active_days,
			score = EXCLUDED.score,
			updated_at = NOW()`, periodType, start, now, campaignMultiplier, rechargeMultiplier); err != nil {
		return fmt.Errorf("refresh team %s aggregate: %w", periodType, err)
	}
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM play_usage_aggregates a
		WHERE a.aggregate_type = 'team' AND a.period_type = $1 AND a.period_start = $2::date
		  AND NOT EXISTS (
			SELECT 1 FROM play_team_members tm
			JOIN usage_logs ul ON ul.user_id = tm.user_id
			WHERE tm.team_id = a.subject_id AND ul.created_at >= $2 AND ul.created_at < $3
			  AND ul.request_type <> 4
			  AND ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens > 0
		  )`, periodType, start, now); err != nil {
		return fmt.Errorf("delete stale team %s aggregates: %w", periodType, err)
	}
	return nil
}

func (r *playRepository) syncPublicUsageActivity(ctx context.Context, now time.Time) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `
		WITH daily AS (
			SELECT user_id, DATE(created_at) AS day, COUNT(*)::bigint AS requests,
			       SUM(input_tokens + output_tokens + cache_creation_tokens)::bigint AS tokens,
			       MAX(created_at) AS last_at
			FROM usage_logs
			WHERE created_at >= $1 - INTERVAL '24 hours' AND request_type <> 4
			GROUP BY user_id, DATE(created_at)
		)
		INSERT INTO play_activity_events (event_key, event_type, actor_hash, subject_type, subject_id, payload, created_at)
		SELECT CONCAT('usage_day:', user_id, ':', day::text), 'usage_day',
		       UPPER(SUBSTRING(MD5(user_id::text || ':growth-world-v1') FROM 1 FOR 4)),
		       'user', user_id, jsonb_build_object('requests', requests, 'tokens', tokens), last_at
		FROM daily
		ON CONFLICT (event_key) DO UPDATE SET payload = EXCLUDED.payload, created_at = EXCLUDED.created_at`, now)
	if err != nil {
		return fmt.Errorf("sync public usage activity: %w", err)
	}
	return nil
}

func (r *playRepository) grantGrowthBlindboxTickets(ctx context.Context, weekStart time.Time, firstRequestTickets, teamWeeklyTickets int) error {
	if firstRequestTickets > 0 {
		if _, err := r.sqlExec(ctx).ExecContext(ctx, `
			INSERT INTO play_blindbox_ticket_ledger (user_id, source, quantity, idempotency_key, detail)
			SELECT ul.user_id, 'first_valid_request', $1,
			       CONCAT('blindbox_ticket:first_request:', ul.user_id),
			       jsonb_build_object('reason', 'first_valid_request')
			FROM usage_logs ul
			WHERE ul.request_type <> 4
			GROUP BY ul.user_id
			ON CONFLICT (idempotency_key) DO NOTHING`, firstRequestTickets); err != nil {
			return fmt.Errorf("grant first request blindbox tickets: %w", err)
		}
		if _, err := r.sqlExec(ctx).ExecContext(ctx, `
			INSERT INTO play_activity_events (event_key, event_type, actor_hash, subject_type, subject_id, payload, created_at)
			SELECT CONCAT('first_valid_request:', ul.user_id), 'first_valid_request',
			       UPPER(SUBSTRING(MD5(ul.user_id::text || ':growth-world-v1') FROM 1 FOR 4)),
			       'user', ul.user_id, jsonb_build_object('ticket_reward', $1), MIN(ul.created_at)
			FROM usage_logs ul WHERE ul.request_type <> 4 GROUP BY ul.user_id
			ON CONFLICT (event_key) DO NOTHING`, firstRequestTickets); err != nil {
			return fmt.Errorf("sync first request activity: %w", err)
		}
	}
	if teamWeeklyTickets > 0 {
		if _, err := r.sqlExec(ctx).ExecContext(ctx, `
			INSERT INTO play_blindbox_ticket_ledger (user_id, source, quantity, idempotency_key, detail)
			SELECT tm.user_id, 'team_weekly', $2,
			       CONCAT('blindbox_ticket:team_weekly:', p.team_id, ':', p.week_start::text, ':', tm.user_id),
			       jsonb_build_object('team_id', p.team_id, 'week_start', p.week_start)
			FROM play_team_weekly_progress p
			JOIN play_team_members tm ON tm.team_id = p.team_id
			WHERE p.week_start = $1::date AND p.completed_at IS NOT NULL
			ON CONFLICT (idempotency_key) DO NOTHING`, weekStart, teamWeeklyTickets); err != nil {
			return fmt.Errorf("grant team weekly blindbox tickets: %w", err)
		}
		if _, err := r.sqlExec(ctx).ExecContext(ctx, `
			INSERT INTO play_activity_events (event_key, event_type, actor_hash, subject_type, subject_id, payload, created_at)
			SELECT CONCAT('team_weekly_completed:', p.team_id, ':', p.week_start::text), 'weekly_mission_completed',
			       UPPER(SUBSTRING(MD5(p.team_id::text || ':growth-world-v1') FROM 1 FOR 4)),
			       'team', p.team_id, jsonb_build_object('tickets_each', $2), p.completed_at
			FROM play_team_weekly_progress p
			WHERE p.week_start = $1::date AND p.completed_at IS NOT NULL
			ON CONFLICT (event_key) DO NOTHING`, weekStart, teamWeeklyTickets); err != nil {
			return fmt.Errorf("sync team weekly activity: %w", err)
		}
	}
	return nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
