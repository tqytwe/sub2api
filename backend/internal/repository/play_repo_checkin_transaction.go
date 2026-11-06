package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) LockCheckinUser(ctx context.Context, id int64) error {
	if dbent.TxFromContext(ctx) == nil {
		return fmt.Errorf("check-in lock requires transaction")
	}
	_, err := r.sqlExec(ctx).ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('play-checkin:'||$1::text,0))`, id)
	return err
}

func (r *playRepository) HasCheckinMilestoneReward(ctx context.Context, id int64, date time.Time) (bool, error) {
	var paid bool
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT EXISTS(
		SELECT 1 FROM play_reward_ledger
		WHERE user_id=$1 AND amount>0
		AND (source='checkin_milestone' OR (source IN ('checkin','checkin_makeup')
		  AND CASE WHEN jsonb_typeof(detail->'milestone_bonus')='number' THEN (detail->>'milestone_bonus')::numeric>0 ELSE FALSE END))
		AND (detail->>'checkin_date'=$2 OR (NULLIF(detail->>'checkin_date','') IS NULL
		  AND (created_at AT TIME ZONE 'Asia/Shanghai')::date=$2::date))
	)`, []any{id, date.Format("2006-01-02")}, &paid)
	return paid, err
}

var _ service.PlayCheckinTransactionRepository = (*playRepository)(nil)
