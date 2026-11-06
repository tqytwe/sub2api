package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type PlayCheckinRepairTarget struct {
	UserID int64  `json:"user_id"`
	Date   string `json:"date"`
}
type PlayCheckinRepairPreview struct {
	Target          PlayCheckinRepairTarget `json:"target"`
	CheckinID       int64                   `json:"checkin_id"`
	SnapshotID      int64                   `json:"snapshot_id"`
	OriginalStreak  int                     `json:"original_streak"`
	EffectiveStreak int                     `json:"effective_streak"`
	ObservedStreak  int                     `json:"observed_streak"`
	RewardMode      string                  `json:"reward_mode"`
	Bonus           float64                 `json:"bonus"`
	CanRepair       bool                    `json:"can_repair"`
	Status          string                  `json:"status"`
	Digest          string                  `json:"digest"`
}
type PlayCheckinRepairConfirmation struct {
	Target PlayCheckinRepairTarget `json:"target"`
	Digest string                  `json:"digest"`
}
type PlayCheckinRepairInput struct {
	Items       []PlayCheckinRepairConfirmation `json:"items"`
	Reason      string                          `json:"reason"`
	ActorUserID int64                           `json:"-"`
}
type PlayAdminCheckinRepairRepository interface {
	RecordAdminCheckinRepair(context.Context, PlayCheckinRepairPreview, int64, string) error
}

func (s *PlayService) PreviewAdminCheckinRepair(ctx context.Context, target PlayCheckinRepairTarget) (*PlayCheckinRepairPreview, error) {
	date, err := time.Parse("2006-01-02", target.Date)
	today := s.serverDate(s.serverNow()).Format("2006-01-02")
	if err != nil || target.UserID <= 0 || target.Date > today {
		return nil, infraerrors.BadRequest("PLAY_CHECKIN_REPAIR_INVALID", "a user and a past or current business date are required")
	}
	repo, ok := s.repo.(PlayCheckinReconciliationRepository)
	if !ok {
		return nil, infraerrors.ServiceUnavailable("PLAY_CHECKIN_REPAIR_UNAVAILABLE", "check-in evidence unavailable")
	}
	out := &PlayCheckinRepairPreview{Target: target, Status: "historical_evidence_missing"}
	evidence, err := repo.GetCheckinMilestoneEvidence(ctx, target.UserID, date)
	if err != nil {
		return nil, err
	}
	if evidence == nil {
		return out, nil
	}
	out.CheckinID, out.SnapshotID = evidence.CheckinID, evidence.SnapshotID
	out.OriginalStreak, out.EffectiveStreak = evidence.StoredStreak, evidence.EffectiveStreak
	out.RewardMode = evidence.RewardMode
	streak, found, err := s.checkinStreakOnDate(ctx, target.UserID, date)
	if err != nil {
		return nil, err
	}
	if !found {
		return out, nil
	}
	out.ObservedStreak = streak
	if evidence.RewardMode == PlayGrowthRewardRedeemable {
		out.Bonus, err = s.unpaidCheckinMilestone(ctx, target.UserID, date, s.resolveStreakMilestoneBonus(streak, evidence.Milestones))
		if err != nil {
			return nil, err
		}
	} else if evidence.RewardMode != PlayGrowthRewardEnergy {
		return out, nil
	}
	out.CanRepair = out.Bonus > 0 || out.EffectiveStreak != streak
	out.Status = "no_change"
	if out.CanRepair {
		out.Status = "ready"
	}
	raw, err := json.Marshal([]any{out, evidence.Milestones})
	if err != nil {
		return nil, err
	}
	out.Digest = fmt.Sprintf("%x", sha256.Sum256(raw))
	return out, nil
}

// One confirmation covers a bounded, all-or-nothing batch. Locks are acquired
// in a stable user/date order shared with normal check-in and makeup. Current
// rules/eligibility never replace the historical immutable evidence.
func (s *PlayService) RepairAdminCheckins(ctx context.Context, input PlayCheckinRepairInput) ([]PlayCheckinRepairPreview, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.ActorUserID <= 0 || len([]rune(input.Reason)) < 10 || len([]rune(input.Reason)) > 500 || len(input.Items) < 1 || len(input.Items) > 50 {
		return nil, infraerrors.BadRequest("PLAY_CHECKIN_REPAIR_INVALID", "select 1–50 records and provide a reason of 10–500 characters")
	}
	repo, ok := s.repo.(PlayAdminCheckinRepairRepository)
	locker, canLock := s.repo.(PlayCheckinTransactionRepository)
	if !ok || !canLock {
		return nil, infraerrors.ServiceUnavailable("PLAY_CHECKIN_REPAIR_UNAVAILABLE", "repair repository unavailable")
	}
	items := append([]PlayCheckinRepairConfirmation(nil), input.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].Target.UserID == items[j].Target.UserID {
			return items[i].Target.Date < items[j].Target.Date
		}
		return items[i].Target.UserID < items[j].Target.UserID
	})
	for i, item := range items {
		if item.Target.UserID <= 0 || len(item.Digest) != 64 || (i > 0 && item.Target == items[i-1].Target) {
			return nil, infraerrors.BadRequest("PLAY_CHECKIN_REPAIR_INVALID", "duplicate record or invalid confirmation")
		}
	}
	out := make([]PlayCheckinRepairPreview, 0, len(items))
	err := s.withPlayTx(ctx, func(txCtx context.Context) error {
		var lockedUser int64
		for _, item := range items {
			if item.Target.UserID != lockedUser {
				if err := locker.LockCheckinUser(txCtx, item.Target.UserID); err != nil {
					return err
				}
				lockedUser = item.Target.UserID
			}
		}
		// Acquire every user lock before budget/balance locks to avoid
		// cross-user lock inversion with a simultaneous makeup request.
		for _, item := range items {
			preview, err := s.PreviewAdminCheckinRepair(txCtx, item.Target)
			if err != nil {
				return err
			}
			if !preview.CanRepair || preview.Digest != item.Digest {
				return infraerrors.Conflict("PLAY_CHECKIN_REPAIR_PREVIEW_STALE", "evidence changed or cannot prove a repair; refresh the preview")
			}
			if preview.Bonus > 0 {
				governance, err := s.requireGrowthGovernanceForReward(txCtx, item.Target.UserID, s.serverNow())
				if err != nil {
					return err
				}
				key := fmt.Sprintf("checkin_milestone:%d:%s:%d", item.Target.UserID, item.Target.Date, preview.ObservedStreak)
				if err := s.reserveGrowthRewardBudget(txCtx, governance, item.Target.UserID, PlayRewardSourceCheckin, key, preview.Bonus); err != nil {
					return err
				}
				detail := map[string]any{"checkin_date": item.Target.Date, "streak_count": preview.ObservedStreak, "milestone_bonus": preview.Bonus, "growth_eligibility_snapshot_id": preview.SnapshotID, "admin_actor_user_id": input.ActorUserID, "reason": input.Reason, "repair_digest": preview.Digest}
				if err := s.grantBalanceLedgerOnlyInTx(txCtx, item.Target.UserID, preview.Bonus, PlayRewardSourceCheckinMilestone, key, detail, preview.SnapshotID); err != nil {
					return err
				}
			}
			if err := repo.RecordAdminCheckinRepair(txCtx, *preview, input.ActorUserID, input.Reason); err != nil {
				return err
			}
			out = append(out, *preview)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
