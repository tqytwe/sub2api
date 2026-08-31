package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type growthCheckinSnapshotLink struct {
	Source       string
	UserID       int64
	ActivityDate time.Time
	SnapshotID   int64
}

// growthCheckinRepo models the existing play repository contract while
// recording the qualification evidence that the check-in flows persist.
type growthCheckinRepo struct {
	PlayRepository
	streak      int
	streakFound bool
	checkins    map[string]bool
	signals     PlayGrowthEligibilitySignals

	snapshots []PlayGrowthEligibilitySnapshot
	links     []growthCheckinSnapshotLink
	inserted  []struct {
		date   time.Time
		reward float64
		streak int
	}
	ledgerEntries  []PlayRewardLedgerEntry
	balanceUpdates []float64
	energyEntries  []PlayGrowthEnergyLedgerEntry
	events         []string
}

// growthCheckinGovernedRepo adds the production governance ports to the
// in-memory check-in double. It records the reservation in the same event
// stream as the activity, snapshot, and balance ledger so the transaction
// ordering is part of the regression contract.
type growthCheckinGovernedRepo struct {
	*growthCheckinRepo
	governance         *PlayGrowthGovernanceState
	budgetReservations []growthBudgetReservation
}

type growthBudgetReservation struct {
	ApprovalID int64
	UserID     int64
	Source     string
	ActionID   string
	Amount     float64
}

func (r *growthCheckinGovernedRepo) GetGrowthGovernance(context.Context, time.Time) (*PlayGrowthGovernanceState, error) {
	return r.governance, nil
}

func (r *growthCheckinGovernedRepo) GetGrowthCohort(context.Context, time.Time, time.Time) (PlayGrowthCohortMetrics, error) {
	return PlayGrowthCohortMetrics{}, nil
}

func (r *growthCheckinGovernedRepo) CreateGrowthApproval(context.Context, PlayGrowthGovernanceApprovalInput) (*PlayGrowthGovernanceState, error) {
	return r.governance, nil
}

func (r *growthCheckinGovernedRepo) RevokeGrowthApproval(context.Context, int64, string) (*PlayGrowthGovernanceState, error) {
	return r.governance, nil
}

func (r *growthCheckinGovernedRepo) GetGrowthRewardSpend(context.Context, time.Time, time.Time) (float64, error) {
	return 0, nil
}

func (r *growthCheckinGovernedRepo) ReserveGrowthRewardBudget(_ context.Context, approvalID, userID int64, source, actionID string, amount float64) (bool, error) {
	r.growthCheckinRepo.events = append(r.growthCheckinRepo.events, "reserve")
	r.budgetReservations = append(r.budgetReservations, growthBudgetReservation{
		ApprovalID: approvalID,
		UserID:     userID,
		Source:     source,
		ActionID:   actionID,
		Amount:     amount,
	})
	return true, nil
}

// The shared coupon test issuer intentionally omits the redeem weight because
// most of its tests exercise quiz pools. Check-in uses the production default
// 80/20 split, so this wrapper returns a complete valid pool contract.
type growthCheckinRewardIssuer struct {
	*playCouponRewardIssuer
}

func (i *growthCheckinRewardIssuer) GetPublishedRewardPool(_ context.Context, activity CouponRewardActivity) (*CouponRewardPoolVersion, error) {
	return &CouponRewardPoolVersion{
		Activity:           activity,
		Status:             CouponRewardPoolStatusPublished,
		CouponWeightBP:     8000,
		RedeemCodeWeightBP: 2000,
		BalanceWeightBP:    0,
	}, nil
}

func (r *growthCheckinRepo) GetGrowthEligibilitySignals(_ context.Context, _ int64, _, _, now time.Time) (PlayGrowthEligibilitySignals, error) {
	if !r.signals.CreatedAt.IsZero() {
		return r.signals, nil
	}
	return PlayGrowthEligibilitySignals{
		EmailVerified:  true,
		CreatedAt:      now.AddDate(0, 0, -7),
		HasRecentUsage: true,
	}, nil
}

func (r *growthCheckinRepo) CreateGrowthEligibilitySnapshot(_ context.Context, snapshot PlayGrowthEligibilitySnapshot) (int64, error) {
	r.snapshots = append(r.snapshots, snapshot)
	return int64(900 + len(r.snapshots)), nil
}

func (r *growthCheckinRepo) LinkGrowthEligibilitySnapshot(_ context.Context, source string, userID int64, activityDate time.Time, snapshotID int64) error {
	r.events = append(r.events, "snapshot")
	r.links = append(r.links, growthCheckinSnapshotLink{
		Source:       source,
		UserID:       userID,
		ActivityDate: activityDate,
		SnapshotID:   snapshotID,
	})
	return nil
}

func (r *growthCheckinRepo) LinkBlindboxGrowthEligibilitySnapshot(context.Context, int64, string, int64) error {
	return nil
}

func (r *growthCheckinRepo) InsertGrowthEnergyLedger(_ context.Context, entry PlayGrowthEnergyLedgerEntry) error {
	r.energyEntries = append(r.energyEntries, entry)
	return nil
}

func (r *growthCheckinRepo) GetCheckinStreakOnDate(_ context.Context, _ int64, _ time.Time) (int, bool, error) {
	return r.streak, r.streakFound, nil
}

func (r *growthCheckinRepo) HasCheckin(_ context.Context, _ int64, date time.Time) (bool, error) {
	return r.checkins[date.Format("2006-01-02")], nil
}

func (r *growthCheckinRepo) InsertCheckin(_ context.Context, _ int64, date time.Time, reward float64, streak int) error {
	r.events = append(r.events, "action")
	key := date.Format("2006-01-02")
	if r.checkins == nil {
		r.checkins = make(map[string]bool)
	}
	if r.checkins[key] {
		return ErrPlayCheckinAlreadyDone
	}
	r.checkins[key] = true
	r.inserted = append(r.inserted, struct {
		date   time.Time
		reward float64
		streak int
	}{date: date, reward: reward, streak: streak})
	return nil
}

func (r *growthCheckinRepo) HasCompletedBalanceRechargeSince(context.Context, int64, time.Time) (bool, error) {
	return true, nil
}

func (r *growthCheckinRepo) InsertRewardLedger(_ context.Context, entry PlayRewardLedgerEntry) error {
	r.events = append(r.events, "ledger")
	r.ledgerEntries = append(r.ledgerEntries, entry)
	return nil
}

func (r *growthCheckinRepo) UpdatePlayBalance(_ context.Context, _ int64, amount float64) error {
	r.balanceUpdates = append(r.balanceUpdates, amount)
	return nil
}

func newGrowthCheckinSettingService(makeup bool) *SettingService {
	return NewSettingService(&blindboxOpenSettingRepo{values: map[string]string{
		SettingKeyPlayCheckinEnabled:       "true",
		SettingKeyPlayCheckinDailyReward:   "0.5",
		SettingKeyPlayCheckinMakeupEnabled: fmt.Sprintf("%t", makeup),
	}}, nil)
}

func TestQualifiedCheckinPersistsImmutableSnapshotLink(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	repo := &growthCheckinRepo{}
	issuer := &growthCheckinRewardIssuer{playCouponRewardIssuer: &playCouponRewardIssuer{result: newPlayCouponRewardIssueResult(now, "checkin-coupon-v1")}}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, newGrowthCheckinSettingService(false), nil, client)
	svc.now = func() time.Time { return now }
	svc.rewardDrawSource = func(max int64) (int64, error) {
		require.Equal(t, int64(couponWeightBasisPoints), max)
		return 0, nil // the default check-in split selects the coupon branch
	}
	svc.SetCouponRewardIssuer(issuer)

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, PlayRewardTypeCoupon, result.RewardType)
	require.Equal(t, PlayGrowthTierActive, result.GrowthEligibility.Tier)
	require.Len(t, repo.inserted, 1)
	require.Len(t, repo.snapshots, 1)
	require.Equal(t, PlayRewardSourceCheckin, repo.snapshots[0].Source)
	require.Equal(t, PlayGrowthTierActive, repo.snapshots[0].Eligibility.Tier)
	require.Equal(t, "checkin:42:2026-08-29", repo.snapshots[0].ActionID)
	require.Equal(t, []growthCheckinSnapshotLink{{
		Source:       PlayRewardSourceCheckin,
		UserID:       42,
		ActivityDate: repo.inserted[0].date,
		SnapshotID:   901,
	}}, repo.links)
	require.Empty(t, repo.ledgerEntries, "coupon rewards retain proof through the activity snapshot FK")
	require.Equal(t, []string{"action", "snapshot"}, repo.events)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplorerCheckinRecordsGrowthEnergyWithoutCashReward(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	repo := &growthCheckinRepo{signals: PlayGrowthEligibilitySignals{
		EmailVerified: true,
		CreatedAt:     now.AddDate(0, 0, -8),
	}}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, newGrowthCheckinSettingService(false), nil, client)
	svc.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.Checkin(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, PlayGrowthTierExplorer, result.GrowthEligibility.Tier)
	require.Equal(t, PlayGrowthRewardEnergy, result.GrowthEligibility.RewardMode)
	require.EqualValues(t, 1, result.GrowthEnergy)
	require.Equal(t, PlayRewardTypeNone, result.RewardType)
	require.Zero(t, result.BalanceAdded)
	require.Len(t, repo.inserted, 1)
	require.Len(t, repo.snapshots, 1)
	require.Equal(t, PlayGrowthTierExplorer, repo.snapshots[0].Eligibility.Tier)
	require.Len(t, repo.energyEntries, 1)
	require.Equal(t, PlayRewardSourceCheckin, repo.energyEntries[0].Source)
	require.Equal(t, "checkin:42:2026-08-29", repo.energyEntries[0].ActionID)
	require.EqualValues(t, 1, repo.energyEntries[0].Amount)
	require.Equal(t, int64(901), repo.energyEntries[0].EligibilitySnapshotID)
	require.Empty(t, repo.ledgerEntries)
	require.Empty(t, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQualifiedCheckinMakeupLinksSnapshotToBalanceLedgerDetail(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	dayBefore := yesterday.AddDate(0, 0, -1)
	repo := &growthCheckinRepo{
		streak:      2,
		streakFound: true,
		checkins: map[string]bool{
			dayBefore.Format("2006-01-02"): true,
		},
	}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, newGrowthCheckinSettingService(true), nil, client)
	svc.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.CheckinMakeup(context.Background(), 42)
	require.NoError(t, err)
	require.InDelta(t, 0.5, result.BalanceAdded, 1e-12)
	require.Len(t, repo.inserted, 1)
	require.Equal(t, yesterday.Format("2006-01-02"), repo.inserted[0].date.Format("2006-01-02"))
	require.Len(t, repo.snapshots, 1)
	require.Equal(t, PlayRewardSourceCheckin, repo.snapshots[0].Source)
	require.Equal(t, "checkin_makeup:42:"+yesterday.Format("2006-01-02"), repo.snapshots[0].ActionID)
	require.Equal(t, []growthCheckinSnapshotLink{{
		Source:       PlayRewardSourceCheckin,
		UserID:       42,
		ActivityDate: yesterday,
		SnapshotID:   901,
	}}, repo.links)
	require.Len(t, repo.ledgerEntries, 1)
	require.Equal(t, PlayRewardSourceCheckinMakeup, repo.ledgerEntries[0].Source)
	require.Equal(t, int64(901), repo.ledgerEntries[0].Detail["growth_eligibility_snapshot_id"])
	require.Equal(t, "v1", repo.ledgerEntries[0].Detail["growth_rule_version"])
	require.Equal(t, PlayGrowthTierActive, repo.ledgerEntries[0].Detail["growth_tier"])
	require.Equal(t, int64(901), repo.ledgerEntries[0].GrowthEligibilitySnapshotID)
	require.Equal(t, PlayGrowthQualificationRuleVersion(), repo.ledgerEntries[0].GrowthRuleVersion)
	require.Equal(t, []string{"action", "snapshot", "ledger"}, repo.events)
	require.Equal(t, []float64{0.5}, repo.balanceUpdates)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQualifiedCheckinMakeupReservesGovernanceBudgetInsideRewardTransaction(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	dayBefore := yesterday.AddDate(0, 0, -1)
	base := &growthCheckinRepo{
		streak:      2,
		streakFound: true,
		checkins: map[string]bool{
			dayBefore.Format("2006-01-02"): true,
		},
	}
	input := validGrowthApproval(now)
	state := &PlayGrowthGovernanceState{
		ID:              77,
		Decision:        PlayGrowthGovernanceDecisionApproved,
		Approved:        true,
		BudgetAmount:    input.BudgetAmount,
		BudgetRemaining: input.BudgetAmount,
		RolloutPercent:  input.RolloutPercent,
		Cohort:          input.Cohort,
		RuleVersion:     input.RuleVersion,
	}
	userID := int64(0)
	for candidate := int64(1); candidate < 10000; candidate++ {
		if state.AllowsRewardForUser(candidate, now) {
			userID = candidate
			break
		}
	}
	require.Positive(t, userID)
	repo := &growthCheckinGovernedRepo{growthCheckinRepo: base, governance: state}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, newGrowthCheckinSettingService(true), nil, client)
	svc.RequireGrowthGovernance(true)
	svc.now = func() time.Time { return now }

	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := svc.CheckinMakeup(context.Background(), userID)
	require.NoError(t, err)
	require.InDelta(t, 0.5, result.BalanceAdded, 1e-12)
	require.Equal(t, []string{"action", "snapshot", "reserve", "ledger"}, base.events)
	require.Equal(t, []growthBudgetReservation{{
		ApprovalID: 77,
		UserID:     userID,
		Source:     PlayRewardSourceCheckin,
		ActionID:   "checkin_makeup:" + fmt.Sprint(userID) + ":" + yesterday.Format("2006-01-02"),
		Amount:     0.5,
	}}, repo.budgetReservations)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckinMakeupFailsClosedWithoutGrowthGovernanceApproval(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)
	dayBefore := yesterday.AddDate(0, 0, -1)
	repo := &growthCheckinRepo{
		streak:      2,
		streakFound: true,
		checkins: map[string]bool{
			dayBefore.Format("2006-01-02"): true,
		},
	}
	client, mock := newCouponRewardEntClient(t)
	svc := NewPlayService(repo, nil, nil, newGrowthCheckinSettingService(true), nil, client)
	svc.RequireGrowthGovernance(true)
	svc.now = func() time.Time { return now }

	_, err := svc.CheckinMakeup(context.Background(), 42)
	require.ErrorIs(t, err, ErrPlayGrowthGovernanceUnavailable)
	require.Empty(t, repo.inserted)
	require.NoError(t, mock.ExpectationsWereMet())
}
