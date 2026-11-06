package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListAdminArenaPeriods(ctx context.Context, kind string, page, size int) (*service.PlayAdminArenaPeriods, error) {
	var raw []byte
	err := scanSingleRow(ctx, r.sqlExec(ctx), `WITH periods AS MATERIALIZED (
 SELECT * FROM play_arena_periods WHERE period_type=$1
 ), selected AS (SELECT * FROM periods ORDER BY start_at DESC,id DESC LIMIT $2 OFFSET $3), details AS (
 SELECT p.id,p.name,p.period_type,p.status,p.start_at,p.end_at,p.settled_at,
 p.token_accounting_version AS accounting_version,
 CASE WHEN p.reward_rules_frozen_at IS NOT NULL THEN 'frozen' WHEN p.period_type='daily' THEN 'legacy_live' ELSE 'missing' END AS reward_policy_source,
 money.paid_users,money.paid_amount
 FROM selected p CROSS JOIN LATERAL (
  SELECT COUNT(DISTINCT l.user_id)::int AS paid_users,COALESCE(SUM(l.amount),0)::text AS paid_amount
  FROM play_reward_ledger l WHERE l.source IN ('arena_settlement','arena_daily_settlement')
  AND (l.detail->>'period_id'=p.id::text OR l.idempotency_key IN ('arena_settlement:'||p.id::text||':'||l.user_id::text,'arena_daily_settlement:'||p.id::text||':'||l.user_id::text))
 ) money
 ) SELECT jsonb_build_object('total',(SELECT COUNT(*) FROM periods),'items',COALESCE((SELECT jsonb_agg(to_jsonb(details) ORDER BY start_at DESC,id DESC) FROM details),'[]'::jsonb))`, []any{kind, size, (page - 1) * size}, &raw)
	if err != nil {
		return nil, err
	}
	var out service.PlayAdminArenaPeriods
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

var _ service.PlayAdminArenaPeriodsRepository = (*playRepository)(nil)
