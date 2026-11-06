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

func (r *playRepository) EnsureDailyArenaPeriodWithPolicy(ctx context.Context, now time.Time, policy service.PlayArenaDailyPolicy) (*service.PlayArenaPeriod, error) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return nil, err
	}
	now = now.In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	raw, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	// Insert the period and its policy together. ON CONFLICT must preserve the
	// winning creator's snapshot, including when another instance has old config.
	_, err = r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_arena_periods
		(name,start_at,end_at,status,period_type,reward_rules_json,reward_rules_frozen_at)
		VALUES ($1,$2,$3,'active','daily',$4::jsonb,NOW()) ON CONFLICT DO NOTHING`,
		start.Format("2006-01-02"), start, start.AddDate(0, 0, 1), string(raw))
	if err != nil {
		return nil, err
	}
	return r.GetActiveArenaPeriodByType(ctx, now, "daily")
}

func (r *playRepository) GetDailyArenaPolicy(ctx context.Context, id int64) (*service.PlayArenaDailyPolicy, error) {
	var raw string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT reward_rules_json::text FROM play_arena_periods
		WHERE id=$1 AND period_type='daily' AND reward_rules_frozen_at IS NOT NULL
		AND jsonb_typeof(reward_rules_json)='object'`, []any{id}, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var policy service.PlayArenaDailyPolicy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return nil, err
	}
	if policy.Budget <= 0 || len(policy.Rewards) == 0 {
		return nil, fmt.Errorf("invalid frozen daily arena policy for period %d", id)
	}
	return &policy, nil
}

func (r *playRepository) FreezeDailyArenaPolicy(ctx context.Context, id int64, policy service.PlayArenaDailyPolicy) error {
	raw, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	_, err = r.sqlExec(ctx).ExecContext(ctx, `UPDATE play_arena_periods
		SET reward_rules_json=$2::jsonb,reward_rules_frozen_at=NOW(),updated_at=NOW()
		WHERE id=$1 AND period_type='daily' AND reward_rules_frozen_at IS NULL`, id, string(raw))
	return err
}

var _ service.PlayArenaDailyPolicyRepository = (*playRepository)(nil)
