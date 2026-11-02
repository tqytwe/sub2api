package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *channelMonitorV2Repository) ValidateCandyGroups(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ANY($1) AND deleted_at IS NULL`, pq.Array(ids)).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("%w: one or more candy groups are unavailable", service.ErrChannelMonitorV2InvalidConfig)
	}
	return nil
}

func (r *channelMonitorV2Repository) ClaimCandyProbe(ctx context.Context, probe service.ChannelMonitorV2CandyProbe, slot time.Time, configVersion int) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO channel_monitor_v2_candy_results (group_id, config_key, schedule_slot, config_version)
		SELECT $1, $2, $3, version FROM channel_monitor_v2_config WHERE id = 1 AND version = $4
		ON CONFLICT (group_id, config_key, schedule_slot) DO NOTHING RETURNING id`,
		probe.GroupID, probe.ConfigKey(), slot, configVersion).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func (r *channelMonitorV2Repository) FinishCandyProbe(ctx context.Context, result service.ChannelMonitorV2CandyResult) error {
	_, err := r.db.ExecContext(ctx, `UPDATE channel_monitor_v2_candy_results
		SET status = 'finished', finished_at = NOW(), verdict = $2, latency_ms = $3,
		    answer_preview = $4, reason = $5
		WHERE id = $1 AND status = 'running'`, result.ID, result.Verdict, result.LatencyMs, result.AnswerPreview, result.Reason)
	return err
}

func (r *channelMonitorV2Repository) CandyHistory(ctx context.Context, scopes []service.ChannelMonitorV2CandyScope, since time.Time, limit int) ([]service.ChannelMonitorV2CandyResult, error) {
	if len(scopes) == 0 {
		return []service.ChannelMonitorV2CandyResult{}, nil
	}
	if limit < service.ChannelMonitorV2CandyHistoryMin || limit > service.ChannelMonitorV2CandyHistoryMax {
		limit = service.ChannelMonitorV2CandyHistoryDefault
	}
	args := make([]any, 0, len(scopes)*2+2)
	args = append(args, since, limit)
	pairs := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		args = append(args, scope.GroupID, scope.ConfigKey)
		pairs = append(pairs, fmt.Sprintf("($%d, $%d)", len(args)-1, len(args)))
	}
	query := `WITH ranked AS (
		SELECT id, group_id, config_key, checked_at, verdict, latency_ms, answer_preview, reason,
		       ROW_NUMBER() OVER (PARTITION BY group_id, config_key ORDER BY checked_at DESC, id DESC) AS position
		FROM channel_monitor_v2_candy_results
		WHERE status = 'finished' AND checked_at >= $1 AND (group_id, config_key) IN (` + strings.Join(pairs, ",") + `)
	) SELECT id, group_id, config_key, checked_at, verdict, latency_ms, answer_preview, reason
	  FROM ranked WHERE position <= $2 ORDER BY group_id, config_key, checked_at DESC, id DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	results := []service.ChannelMonitorV2CandyResult{}
	for rows.Next() {
		var result service.ChannelMonitorV2CandyResult
		if err := rows.Scan(&result.ID, &result.GroupID, &result.ConfigKey, &result.CheckedAt, &result.Verdict, &result.LatencyMs, &result.AnswerPreview, &result.Reason); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (r *channelMonitorV2Repository) PruneCandyHistory(ctx context.Context, before time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE channel_monitor_v2_candy_results
		SET status='finished', finished_at=NOW(), verdict='error', reason='timeout'
		WHERE status='running' AND checked_at < $1`, before.Add(-2*time.Minute))
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM channel_monitor_v2_candy_results WHERE status='finished' AND checked_at < $1`, before.Add(-service.ChannelMonitorV2CandyRetention))
	return err
}
