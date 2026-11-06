package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *playRepository) SaveCheckinMilestoneRules(ctx context.Context, userID int64, date time.Time, milestones []service.PlayStreakMilestone) error {
	if milestones == nil {
		milestones = []service.PlayStreakMilestone{}
	}
	raw, err := json.Marshal(milestones)
	if err != nil {
		return err
	}
	_, err = r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_checkin_rule_snapshots(checkin_id,milestones)
		SELECT id,$3::jsonb FROM play_checkins WHERE user_id=$1 AND checkin_date=$2::date
		ON CONFLICT(checkin_id) DO NOTHING`, userID, date.Format("2006-01-02"), string(raw))
	return err
}

func (r *playRepository) GetCheckinMilestoneEvidence(ctx context.Context, userID int64, date time.Time) (*service.PlayCheckinMilestoneEvidence, error) {
	var out service.PlayCheckinMilestoneEvidence
	var raw string
	err := scanSingleRow(ctx, r.sqlExec(ctx), `SELECT c.id,s.id,c.streak_count,s.reward_mode,p.milestones::text,COALESCE((SELECT a.observed_streak FROM play_checkin_reconciliations a WHERE a.checkin_id=c.id ORDER BY a.id DESC LIMIT 1),c.streak_count)
		FROM play_checkins c JOIN play_checkin_rule_snapshots p ON p.checkin_id=c.id
		JOIN play_growth_eligibility_snapshots s ON s.id=c.growth_eligibility_snapshot_id
		AND s.user_id=c.user_id AND s.activity_date=c.checkin_date AND s.source='checkin'
		WHERE c.user_id=$1 AND c.checkin_date=$2::date`, []any{userID, date.Format("2006-01-02")},
		&out.CheckinID, &out.SnapshotID, &out.StoredStreak, &out.RewardMode, &raw, &out.EffectiveStreak)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(raw), &out.Milestones); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *playRepository) RecordCheckinReconciliation(ctx context.Context, checkinID int64, action string, previous, observed int, amount float64) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_checkin_reconciliations
		(checkin_id,action_id,previous_streak,observed_streak,milestone_added,reason)
		VALUES($1,$2,$3,$4,$5,'makeup_connected_streak')`, checkinID, action, previous, observed, amount)
	return err
}

var _ service.PlayCheckinReconciliationRepository = (*playRepository)(nil)

func (r *playRepository) RecordAdminCheckinRepair(ctx context.Context, preview service.PlayCheckinRepairPreview, actor int64, reason string) error {
	_, err := r.sqlExec(ctx).ExecContext(ctx, `INSERT INTO play_checkin_reconciliations
 (checkin_id,action_id,previous_streak,observed_streak,milestone_added,reason,actor_user_id,preview_digest)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, preview.CheckinID, "admin_checkin_repair:"+preview.Digest, preview.EffectiveStreak, preview.ObservedStreak, preview.Bonus, reason, actor, preview.Digest)
	return err
}

var _ service.PlayAdminCheckinRepairRepository = (*playRepository)(nil)
