package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

func (r *playRepository) GetArenaUserUsageReport(ctx context.Context, userID int64, dayStart, dayEnd, monthStart, monthEnd, asOf time.Time) (*service.PlayArenaUserReport, error) {
	out := &service.PlayArenaUserReport{}
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
      SELECT
        COALESCE(SUM(input_tokens) FILTER(WHERE created_at >= $2 AND created_at < $3),0)::bigint,
        COALESCE(SUM(output_tokens) FILTER(WHERE created_at >= $2 AND created_at < $3),0)::bigint,
        COALESCE(SUM(cache_creation_tokens) FILTER(WHERE created_at >= $2 AND created_at < $3),0)::bigint,
        COALESCE(SUM(cache_read_tokens) FILTER(WHERE created_at >= $2 AND created_at < $3),0)::bigint,
        COALESCE(SUM(input_tokens),0)::bigint,COALESCE(SUM(output_tokens),0)::bigint,
        COALESCE(SUM(cache_creation_tokens),0)::bigint,COALESCE(SUM(cache_read_tokens),0)::bigint,
        (SELECT COALESCE(SUM(amount),0)::bigint FROM play_growth_energy_ledger WHERE user_id=$1 AND created_at <= $6)
      FROM usage_logs WHERE user_id=$1 AND created_at >= $4 AND created_at < $5 AND created_at <= $6`,
		[]any{userID, dayStart, dayEnd, monthStart, monthEnd, asOf},
		&out.Daily.Input, &out.Daily.Output, &out.Daily.CacheCreation, &out.Daily.CacheRead,
		&out.Monthly.Input, &out.Monthly.Output, &out.Monthly.CacheCreation, &out.Monthly.CacheRead, &out.QualificationGrowthEnergy)
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _ service.PlayArenaAccountingRepository = (*playRepository)(nil)

func (r *playRepository) LockArenaSettlement(ctx context.Context, id int64) error {
	var found int64
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT id FROM play_arena_periods WHERE id=$1 FOR UPDATE`, []any{id}, &found)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrPlayArenaNoPeriod
	}
	return err
}

func (r *playRepository) HasArenaRewardLedger(ctx context.Context, id int64) (bool, error) {
	var found bool
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT EXISTS(SELECT 1 FROM play_reward_ledger WHERE source IN ('arena_settlement','arena_daily_settlement') AND (detail->>'period_id'=$1::text OR idempotency_key LIKE 'arena_settlement:'||$1::text||':%' OR idempotency_key LIKE 'arena_daily_settlement:'||$1::text||':%'))`, []any{id}, &found)
	return found, err
}

func (r *playRepository) PreviewArenaAccounting(ctx context.Context, id int64) (*service.PlayArenaAccountingPreview, error) {
	var raw []byte
	err := scanSingleRow(ctx, r.sqlExec(ctx), `
      WITH p AS (SELECT * FROM play_arena_periods WHERE id=$1),
      scores AS (
        SELECT user_id,SUM(input_tokens::bigint+output_tokens+cache_creation_tokens)::bigint AS old_tokens,
          SUM(input_tokens::bigint+output_tokens+cache_creation_tokens+cache_read_tokens)::bigint AS new_tokens,
          MIN(u.created_at) AS first_at
        FROM usage_logs u CROSS JOIN p WHERE u.created_at >= p.start_at AND u.created_at < p.end_at
        GROUP BY user_id HAVING SUM(input_tokens::bigint+output_tokens+cache_creation_tokens+cache_read_tokens)>0
      ), ranked AS (
        SELECT user_id,old_tokens,new_tokens,
          CASE WHEN old_tokens>0 THEN ROW_NUMBER() OVER(ORDER BY old_tokens DESC,first_at,user_id) ELSE 0 END AS old_rank,
          ROW_NUMBER() OVER(ORDER BY new_tokens DESC,first_at,user_id) AS new_rank
        FROM scores
      ), limited AS(SELECT * FROM ranked ORDER BY new_rank LIMIT 100)
      SELECT jsonb_build_object(
        'reward_policy_source',CASE WHEN p.reward_rules_frozen_at IS NOT NULL THEN 'frozen' WHEN p.period_type='daily' THEN 'legacy_live' ELSE 'missing' END,
        'period_id',p.id,'version',p.token_accounting_version,'status',p.status,'start_at',p.start_at,'end_at',p.end_at,
        'has_payout',EXISTS(SELECT 1 FROM play_reward_ledger l WHERE l.source IN ('arena_settlement','arena_daily_settlement') AND
          (l.detail->>'period_id'=p.id::text OR l.idempotency_key LIKE 'arena_settlement:'||p.id::text||':%' OR l.idempotency_key LIKE 'arena_daily_settlement:'||p.id::text||':%'))
          OR EXISTS(SELECT 1 FROM play_arena_season_snapshots s WHERE s.period_id=p.id),
        'digest',md5(p.id::text||':'||p.token_accounting_version::text||':'||p.status||':'||COALESCE(p.reward_rules_json::text,'')||':'||
          COALESCE((SELECT STRING_AGG(user_id::text||':'||old_tokens::text||':'||new_tokens::text||':'||old_rank::text||':'||new_rank::text,',' ORDER BY user_id) FROM ranked),'')),
        'total_users',(SELECT COUNT(*) FROM ranked),
        'truncated',(SELECT COUNT(*)>100 FROM ranked),
        'changes',COALESCE((SELECT jsonb_agg(to_jsonb(limited) ORDER BY new_rank) FROM limited),'[]'::jsonb)
      ) FROM p`, []any{id}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrPlayArenaNoPeriod
	}
	if err != nil {
		return nil, err
	}
	var out service.PlayArenaAccountingPreview
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (r *playRepository) ActivateArenaAccounting(ctx context.Context, input service.PlayArenaAccountingActivation) error {
	if dbent.TxFromContext(ctx) == nil {
		return fmt.Errorf("arena activation requires transaction")
	}
	result, err := r.sqlExec(ctx).ExecContext(ctx, `UPDATE play_arena_periods SET token_accounting_version=2,updated_at=NOW() WHERE id=$1 AND token_accounting_version=1`, input.PeriodID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return service.ErrPlayArenaPeriodNotSettleable
	}
	_, err = r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_arena_accounting_events(period_id,actor_user_id,from_version,to_version,preview_digest,reason) VALUES($1,$2,1,2,$3,$4)`, input.PeriodID, input.ActorUserID, input.ExpectedDigest, input.Reason)
	return err
}
