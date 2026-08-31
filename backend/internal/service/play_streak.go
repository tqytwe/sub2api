package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *PlayService) computeNextStreak(ctx context.Context, userID int64, today time.Time) (int, error) {
	yesterday := today.AddDate(0, 0, -1)
	if streak, found, err := s.repo.GetCheckinStreakOnDate(ctx, userID, yesterday); err != nil {
		return 0, err
	} else if found {
		return streak + 1, nil
	}
	return 1, nil
}

func (s *PlayService) resolveStreakMilestoneBonus(streak int, milestones []PlayStreakMilestone) float64 {
	if streak <= 0 || len(milestones) == 0 {
		return 0
	}
	var bonus float64
	for _, m := range milestones {
		if m.Days == streak && m.Bonus > 0 {
			bonus += m.Bonus
		}
	}
	return bonus
}

func (s *PlayService) resolveNextMilestone(streak int, milestones []PlayStreakMilestone) (days int, bonus float64) {
	if len(milestones) == 0 {
		return 0, 0
	}
	sorted := append([]PlayStreakMilestone(nil), milestones...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Days < sorted[j].Days })
	for _, m := range sorted {
		if m.Days > streak {
			return m.Days, m.Bonus
		}
	}
	return 0, 0
}

func (s *PlayService) enrichCheckinStatus(ctx context.Context, userID int64, status *PlayCheckinStatus, rt PlayRuntime) error {
	if status == nil || userID <= 0 || !rt.CheckinEnabled {
		return nil
	}
	now := s.serverNow()
	today := s.serverDate(now)
	yesterday := today.AddDate(0, 0, -1)

	if status.CheckedInToday {
		if streak, found, err := s.repo.GetCheckinStreakOnDate(ctx, userID, today); err != nil {
			return err
		} else if found {
			status.StreakCount = streak
		}
	} else if streak, found, err := s.repo.GetCheckinStreakOnDate(ctx, userID, yesterday); err != nil {
		return err
	} else if found {
		status.StreakCount = streak
	}

	nextDays, nextBonus := s.resolveNextMilestone(status.StreakCount, rt.StreakMilestones)
	status.NextMilestoneDays = nextDays
	status.NextMilestoneBonus = nextBonus

	boost, err := s.getRechargeBoostStatus(ctx, userID, rt)
	if err != nil {
		return err
	}
	if boost.Active {
		status.RechargeBoostActive = true
		status.BoostCheckinMultiplier = boost.CheckinMultiplier
		if boost.CheckinMultiplier > 1 {
			status.RewardAmount = rt.CheckinReward * boost.CheckinMultiplier
		}
	}

	if rt.CheckinMakeupEnabled && !status.CheckedInToday {
		missed, err := s.evaluateMakeupEligibility(ctx, userID, today, yesterday)
		if err != nil {
			return err
		}
		if missed {
			status.CanMakeup = true
			status.MakeupDate = yesterday.Format("2006-01-02")
		}
	}
	return nil
}

func (s *PlayService) evaluateMakeupEligibility(ctx context.Context, userID int64, today, yesterday time.Time) (bool, error) {
	doneYesterday, err := s.repo.HasCheckin(ctx, userID, yesterday)
	if err != nil || doneYesterday {
		return false, err
	}
	dayBefore := yesterday.AddDate(0, 0, -1)
	doneDayBefore, err := s.repo.HasCheckin(ctx, userID, dayBefore)
	if err != nil || !doneDayBefore {
		return false, err
	}
	since := s.serverNow().Add(-24 * time.Hour)
	recharged, err := s.repo.HasCompletedBalanceRechargeSince(ctx, userID, since)
	if err != nil || !recharged {
		return false, err
	}
	return true, nil
}

func (s *PlayService) CheckinMakeup(ctx context.Context, userID int64) (*PlayCheckinResult, error) {
	rt := s.GetRuntime(ctx)
	if !rt.CheckinEnabled {
		return nil, ErrPlayFeatureDisabled
	}
	if !rt.CheckinMakeupEnabled {
		return nil, ErrPlayCheckinMakeupUnavailable
	}
	if rt.CheckinReward <= 0 {
		return nil, fmt.Errorf("check-in reward not configured")
	}

	now := s.serverNow()
	growthEligibility, err := s.growthEligibility(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	if growthEligibility.RewardMode != PlayGrowthRewardRedeemable {
		return nil, ErrPlayCheckinMakeupUnavailable
	}
	// Makeup is a cash-equivalent reward just like a normal check-in. Keep the
	// operations approval/rollout decision outside the transaction for a fast
	// fail-closed response, then reserve the budget inside the same reward
	// transaction below so an approved amount cannot be spent twice.
	growthGovernance, err := s.requireGrowthGovernanceForReward(ctx, userID, now)
	if err != nil {
		return nil, err
	}
	today := s.serverDate(now)
	yesterday := today.AddDate(0, 0, -1)
	dateKey := yesterday.Format("2006-01-02")

	eligible, err := s.evaluateMakeupEligibility(ctx, userID, today, yesterday)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, ErrPlayCheckinMakeupUnavailable
	}

	dayBefore := yesterday.AddDate(0, 0, -1)
	prevStreak, found, err := s.repo.GetCheckinStreakOnDate(ctx, userID, dayBefore)
	if err != nil {
		return nil, err
	}
	streak := 1
	if found {
		streak = prevStreak + 1
	}

	boost, err := s.getRechargeBoostStatus(ctx, userID, rt)
	if err != nil {
		return nil, err
	}
	reward := rt.CheckinReward
	if boost.Active && boost.CheckinMultiplier > 1 {
		reward *= boost.CheckinMultiplier
	}
	milestoneBonus := s.resolveStreakMilestoneBonus(streak, rt.StreakMilestones)
	totalReward := reward + milestoneBonus

	idempotencyKey := fmt.Sprintf("checkin_makeup:%d:%s", userID, dateKey)
	detail := map[string]any{
		"checkin_date":    dateKey,
		"streak_count":    streak,
		"milestone_bonus": milestoneBonus,
		"makeup":          true,
	}
	var growthSnapshotID int64
	if err := s.grantBalanceWithGrowthSnapshot(ctx, userID, totalReward, PlayRewardSourceCheckinMakeup, idempotencyKey, detail, &growthSnapshotID, func(txCtx context.Context) error {
		if err := s.repo.InsertCheckin(txCtx, userID, yesterday, totalReward, streak); err != nil {
			return err
		}
		var snapshotErr error
		growthSnapshotID, snapshotErr = s.createGrowthSnapshot(txCtx, PlayGrowthEligibilitySnapshot{
			UserID: userID, Source: PlayRewardSourceCheckin, ActionID: idempotencyKey, ActivityDate: yesterday, Eligibility: growthEligibility,
		})
		if snapshotErr != nil {
			return snapshotErr
		}
		if growthSnapshotID == 0 {
			// A redeemable makeup reward must always carry immutable qualification
			// evidence. This is normally guaranteed by production wiring
			// (RequireGrowthQualification=true), but keep the transaction fail-closed
			// if a partially upgraded repository or test double returns no snapshot.
			return ErrPlayGrowthQualificationUnavailable
		}
		growthRepo, ok := s.growthQualificationRepository()
		if !ok {
			return fmt.Errorf("growth qualification repository disappeared during check-in makeup")
		}
		if err := growthRepo.LinkGrowthEligibilitySnapshot(txCtx, PlayRewardSourceCheckin, userID, yesterday, growthSnapshotID); err != nil {
			return err
		}
		// Governance budgets are scoped to the check-in activity family. The
		// balance ledger keeps the more specific `checkin_makeup` source, while
		// the 268 budget table intentionally accepts only checkin/quiz/blindbox.
		if err := s.reserveGrowthRewardBudget(txCtx, growthGovernance, userID, PlayRewardSourceCheckin, idempotencyKey, growthRewardBudgetCost(PlayRewardTypeBalance, totalReward)); err != nil {
			return err
		}
		detail["growth_eligibility_snapshot_id"] = growthSnapshotID
		detail["growth_rule_version"] = "v1"
		detail["growth_tier"] = growthEligibility.Tier
		return nil
	}); err != nil {
		if errors.Is(err, ErrPlayCheckinAlreadyDone) {
			return nil, ErrPlayCheckinMakeupAlreadyDone
		}
		return nil, err
	}

	return &PlayCheckinResult{
		RewardAmount:      totalReward,
		BalanceAdded:      totalReward,
		ServerDate:        dateKey,
		StreakCount:       streak,
		MilestoneBonus:    milestoneBonus,
		GrowthEligibility: growthEligibility,
	}, nil
}

func parsePlayStreakMilestones(raw string) []PlayStreakMilestone {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []PlayStreakMilestone
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	return items
}

func parseArenaSettlementRewards(raw string) []PlayArenaSettlementTier {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []PlayArenaSettlementTier
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].RankMax < items[j].RankMax })
	return items
}

func parsePositiveFloatSetting(raw string, fallback float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parsePositiveIntSetting(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parsePositiveInt64Setting(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
