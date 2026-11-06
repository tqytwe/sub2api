package service

import (
	"context"
	"fmt"
	"time"
)

type PlayCheckinMilestoneEvidence struct {
	CheckinID       int64
	SnapshotID      int64
	StoredStreak    int
	EffectiveStreak int
	RewardMode      string
	Milestones      []PlayStreakMilestone
}

type PlayCheckinReconciliationRepository interface {
	SaveCheckinMilestoneRules(context.Context, int64, time.Time, []PlayStreakMilestone) error
	GetCheckinMilestoneEvidence(context.Context, int64, time.Time) (*PlayCheckinMilestoneEvidence, error)
	RecordCheckinReconciliation(context.Context, int64, string, int, int, float64) error
}

func (s *PlayService) saveCheckinMilestoneRules(ctx context.Context, userID int64, date time.Time, rt PlayRuntime) error {
	if repo, ok := s.repo.(PlayCheckinReconciliationRepository); ok {
		return repo.SaveCheckinMilestoneRules(ctx, userID, date, rt.StreakMilestones)
	}
	return nil
}

// A makeup can connect today's previously isolated check-in to its actual
// streak. Use today's immutable qualification AND milestone configuration,
// never the user's newly acquired eligibility or an edited reward setting.
// Legacy check-ins without both snapshots remain candidates for manual review.
func (s *PlayService) reconcileCheckinAfterMakeup(ctx context.Context, userID int64, today time.Time, governance *PlayGrowthGovernanceState) (float64, error) {
	repo, ok := s.repo.(PlayCheckinReconciliationRepository)
	if !ok {
		return 0, nil
	}
	evidence, err := repo.GetCheckinMilestoneEvidence(ctx, userID, today)
	if err != nil || evidence == nil {
		return 0, err
	}
	streak, found, err := s.checkinStreakOnDate(ctx, userID, today)
	if err != nil || !found {
		return 0, err
	}
	if streak == evidence.StoredStreak {
		return 0, nil
	}
	bonus := 0.0
	if evidence.RewardMode == PlayGrowthRewardRedeemable {
		bonus, err = s.unpaidCheckinMilestone(ctx, userID, today, s.resolveStreakMilestoneBonus(streak, evidence.Milestones))
		if err != nil {
			return 0, err
		}
	}
	dateKey := today.Format("2006-01-02")
	key := fmt.Sprintf("checkin_milestone:%d:%s:%d", userID, dateKey, streak)
	if bonus > 0 {
		if err := s.reserveGrowthRewardBudget(ctx, governance, userID, PlayRewardSourceCheckin, key, bonus); err != nil {
			return 0, err
		}
		if err := s.grantBalanceLedgerOnlyInTx(ctx, userID, bonus, PlayRewardSourceCheckinMilestone, key,
			map[string]any{"checkin_date": dateKey, "streak_count": streak, "milestone_bonus": bonus, "reason": "makeup_connected_streak", "growth_eligibility_snapshot_id": evidence.SnapshotID}, evidence.SnapshotID); err != nil {
			return 0, err
		}
	}
	if err := repo.RecordCheckinReconciliation(ctx, evidence.CheckinID, key, evidence.StoredStreak, streak, bonus); err != nil {
		return 0, err
	}
	return bonus, nil
}
