package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) ListArenaPaidLeaderboard(ctx context.Context, periodID int64, limit int) ([]service.PlayArenaScoreRow, error) {
	rows, err := r.sqlExec(ctx).QueryContext(ctx, `
 WITH payments AS (
  SELECT l.*,COALESCE(s.rank,CASE WHEN l.detail->>'rank' ~ '^[0-9]{1,9}$' THEN (l.detail->>'rank')::int END) AS paid_rank,
   COALESCE(s.token_sum,CASE WHEN COALESCE(l.detail->>'token_sum',l.detail->>'token') ~ '^[0-9]{1,18}$' THEN COALESCE(l.detail->>'token_sum',l.detail->>'token')::bigint END) AS paid_tokens
  FROM play_reward_ledger l
  LEFT JOIN play_arena_season_snapshots s ON s.period_id=$1 AND s.user_id=l.user_id
  WHERE l.source IN ('arena_settlement','arena_daily_settlement')
   AND (l.detail->>'period_id'=$1::text OR l.idempotency_key IN ('arena_settlement:'||$1::text||':'||l.user_id::text,'arena_daily_settlement:'||$1::text||':'||l.user_id::text))
 ) SELECT p.user_id,COALESCE(u.email,''),COALESCE(NULLIF(TRIM(a.url),''),''),
    COALESCE(p.paid_rank,0),COALESCE(p.paid_tokens,0),p.amount::float8,
    CASE WHEN p.paid_rank>0 AND p.paid_tokens IS NOT NULL THEN 'payout_recorded' ELSE 'historical_evidence_missing' END
 FROM payments p LEFT JOIN users u ON u.id=p.user_id LEFT JOIN user_avatars a ON a.user_id=p.user_id
 ORDER BY p.paid_rank NULLS LAST,p.created_at,p.id LIMIT $2`, periodID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.PlayArenaScoreRow, 0)
	for rows.Next() {
		var row service.PlayArenaScoreRow
		if err := rows.Scan(&row.UserID, &row.Email, &row.AvatarURL, &row.Rank, &row.TokenSum, &row.PaidReward, &row.EvidenceStatus); err != nil {
			return nil, err
		}
		row.DisplayName, row.Anonymous = service.PublicPlayLeaderboardIdentity(row.Email)
		out = append(out, row)
	}
	return out, rows.Err()
}

var _ service.PlayArenaPaidLeaderboardRepository = (*playRepository)(nil)
