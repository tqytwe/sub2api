package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrPlayGrowthGovernanceUnavailable = infraerrors.ServiceUnavailable(
		"PLAY_GROWTH_GOVERNANCE_UNAVAILABLE",
		"growth rewards are temporarily unavailable while operations approval is checked",
	)
	ErrPlayGrowthGovernanceNotApproved = infraerrors.Forbidden(
		"PLAY_GROWTH_GOVERNANCE_NOT_APPROVED",
		"redeemable growth rewards are not enabled by operations",
	)
	ErrPlayGrowthGovernanceBudgetExhausted = infraerrors.Conflict(
		"PLAY_GROWTH_GOVERNANCE_BUDGET_EXHAUSTED",
		"the approved growth reward budget has been exhausted",
	)
	ErrPlayGrowthGovernanceRolloutExcluded = infraerrors.Forbidden(
		"PLAY_GROWTH_GOVERNANCE_ROLLOUT_EXCLUDED",
		"this account is outside the current growth reward rollout",
	)
	ErrPlayGrowthGovernanceInvalid = infraerrors.BadRequest(
		"PLAY_GROWTH_GOVERNANCE_INVALID",
		"growth governance approval is invalid",
	)
)

const (
	playGrowthGovernanceMinimumRollout = 10
	playGrowthGovernanceMaximumRollout = 20
	playGrowthGovernanceMinimumCohort  = 14 * 24 * time.Hour
	// A two-week participation window contains a 30-day real-call metric. The
	// newest participant must therefore have a full 30-day observation period
	// before operations can use the report to approve a redeemable rollout.
	playGrowthGovernanceObservationLag = 30 * 24 * time.Hour
)

// RequireGrowthGovernance enables the production fail-closed policy. It is
// deliberately independent from RequireGrowthQualification: a deployment may
// have qualification storage while operations has not yet approved a rollout.
func (s *PlayService) RequireGrowthGovernance(required bool) {
	if s != nil {
		s.requireGrowthGovernance = required
	}
}

// StableGrowthRolloutBucket returns a deterministic 1..100 bucket for a user.
// The salt and rule version are part of the contract so a user does not move
// between rollout buckets as process instances restart.
func StableGrowthRolloutBucket(userID int64) int {
	if userID <= 0 {
		return 100
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("play-growth:%s:%d", PlayGrowthQualificationRuleVersion(), userID)))
	return int(binary.BigEndian.Uint32(digest[:4])%100) + 1
}

func GrowthRolloutAllowsUser(userID int64, rolloutPercent int) bool {
	return userID > 0 && rolloutPercent >= playGrowthGovernanceMinimumRollout &&
		rolloutPercent <= playGrowthGovernanceMaximumRollout &&
		StableGrowthRolloutBucket(userID) <= rolloutPercent
}

// growthRewardBudgetCost converts each reward branch to the same budget unit.
// Balance rewards use their positive credited amount; coupon and redeem-code
// grants reserve one conservative unit because their final discount/value is
// selected under the issuer lock. This intentionally over-reserves rather than
// allowing a cash-equivalent grant to bypass the approved ceiling.
func growthRewardBudgetCost(rewardType PlayRewardType, amount float64) float64 {
	switch rewardType {
	case PlayRewardTypeBalance:
		if amount > 0 && !math.IsNaN(amount) && !math.IsInf(amount, 0) {
			return amount
		}
	case PlayRewardTypeCoupon, PlayRewardTypeRedeem:
		return 1
	}
	return 0
}

// ValidatePlayGrowthGovernanceApproval validates operator input and persisted
// cohort evidence before any reward switch can be enabled.
func ValidatePlayGrowthGovernanceApproval(input PlayGrowthGovernanceApprovalInput, now time.Time) error {
	if input.ActorID <= 0 {
		return fmt.Errorf("actor id is required")
	}
	if math.IsNaN(input.BudgetAmount) || math.IsInf(input.BudgetAmount, 0) || input.BudgetAmount <= 0 {
		return fmt.Errorf("budget amount must be finite and positive")
	}
	if input.RolloutPercent < playGrowthGovernanceMinimumRollout || input.RolloutPercent > playGrowthGovernanceMaximumRollout {
		return fmt.Errorf("rollout percent must be between %d and %d", playGrowthGovernanceMinimumRollout, playGrowthGovernanceMaximumRollout)
	}
	if strings.TrimSpace(input.RuleVersion) != PlayGrowthQualificationRuleVersion() {
		return fmt.Errorf("rule version must be %s", PlayGrowthQualificationRuleVersion())
	}
	reasonLength := len([]rune(strings.TrimSpace(input.Reason)))
	if reasonLength < 10 || reasonLength > 500 {
		return fmt.Errorf("reason must contain 10 to 500 characters")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cohort := input.Cohort
	if cohort.WindowStart.IsZero() || cohort.WindowEnd.IsZero() || !cohort.WindowEnd.After(cohort.WindowStart) {
		return fmt.Errorf("cohort window is invalid")
	}
	if cohort.WindowEnd.Sub(cohort.WindowStart) < playGrowthGovernanceMinimumCohort {
		return fmt.Errorf("cohort window must be at least 14 days")
	}
	if cohort.WindowEnd.After(now.Add(-playGrowthGovernanceObservationLag)) {
		return fmt.Errorf("cohort window must end at least %d days before approval so 30-day metrics are complete", int(playGrowthGovernanceObservationLag.Hours()/24))
	}
	if !cohort.Complete() {
		return fmt.Errorf("cohort metrics are incomplete")
	}
	// ActualRewardCost is a financial cohort metric. BudgetAmount is a
	// reservation ceiling: coupon/redeem branches conservatively consume one
	// unit before their final value is selected, and a blindbox reserves prize
	// value rather than its net ledger amount. They are intentionally not
	// compared as if they were the same currency. Operations reviews both
	// values before approving a rollout.
	for name, ratio := range map[string]float64{
		"real_call_7d_ratio":      cohort.RealCall7dRatio,
		"real_call_30d_ratio":     cohort.RealCall30dRatio,
		"first_recharge_ratio":    cohort.FirstRechargeRatio,
		"coupon_redemption_ratio": cohort.CouponRedemptionRatio,
		"d7_retention_ratio":      cohort.D7RetentionRatio,
	} {
		if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
			return fmt.Errorf("%s must be between 0 and 1", name)
		}
	}
	for name, ratio := range map[string]*float64{
		"abnormal_redemption_ratio":   cohort.AbnormalRedemptionRatio,
		"appeal_false_positive_ratio": cohort.AppealFalsePositiveRatio,
	} {
		if ratio == nil || math.IsNaN(*ratio) || math.IsInf(*ratio, 0) || *ratio < 0 || *ratio > 1 {
			return fmt.Errorf("%s is unavailable or outside 0..1", name)
		}
	}
	return nil
}

func (g PlayGrowthGovernanceState) AllowsReward(now time.Time) bool {
	if g.Decision != PlayGrowthGovernanceDecisionApproved || !g.Approved {
		return false
	}
	if g.BudgetAmount <= 0 || g.BudgetRemaining <= 0 {
		return false
	}
	if g.RolloutPercent < playGrowthGovernanceMinimumRollout || g.RolloutPercent > playGrowthGovernanceMaximumRollout {
		return false
	}
	if g.RuleVersion != PlayGrowthQualificationRuleVersion() || !g.Cohort.Complete() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !g.Cohort.WindowEnd.After(now)
}

func (g PlayGrowthGovernanceState) AllowsRewardForUser(userID int64, now time.Time) bool {
	return g.AllowsReward(now) && GrowthRolloutAllowsUser(userID, g.RolloutPercent)
}

func (s *PlayService) getGrowthGovernance(ctx context.Context, now time.Time) (*PlayGrowthGovernanceState, error) {
	if s == nil {
		return nil, ErrPlayGrowthGovernanceUnavailable
	}
	repo, ok := s.repo.(PlayGrowthGovernanceRepository)
	if !ok || repo == nil {
		if s.requireGrowthGovernance {
			return nil, ErrPlayGrowthGovernanceUnavailable
		}
		return nil, nil
	}
	state, err := repo.GetGrowthGovernance(ctx, now)
	if err != nil {
		return nil, err
	}
	if state == nil {
		if s.requireGrowthGovernance {
			return nil, ErrPlayGrowthGovernanceUnavailable
		}
		return nil, nil
	}
	return state, nil
}

// requireGrowthGovernanceForReward is called immediately before a
// redeemable action. Explorer energy actions intentionally skip this gate.
func (s *PlayService) requireGrowthGovernanceForReward(ctx context.Context, userID int64, now time.Time) (*PlayGrowthGovernanceState, error) {
	state, err := s.getGrowthGovernance(ctx, now)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, nil
	}
	if !state.AllowsReward(now) {
		if state.Decision != PlayGrowthGovernanceDecisionApproved || !state.Approved {
			return nil, ErrPlayGrowthGovernanceNotApproved
		}
		if state.BudgetAmount <= 0 || state.BudgetRemaining <= 0 {
			return nil, ErrPlayGrowthGovernanceBudgetExhausted
		}
		return nil, ErrPlayGrowthGovernanceInvalid
	}
	if !GrowthRolloutAllowsUser(userID, state.RolloutPercent) {
		return nil, ErrPlayGrowthGovernanceRolloutExcluded
	}
	return state, nil
}

// growthGovernanceForStatus is deliberately non-throwing for user-facing
// status endpoints. A status response can explain that rewards are paused
// while mutation endpoints continue to fail closed with a typed error.
func (s *PlayService) growthGovernanceForStatus(ctx context.Context, userID int64, now time.Time) (*PlayGrowthGovernanceState, bool, string) {
	state, err := s.getGrowthGovernance(ctx, now)
	if err != nil {
		return nil, false, "unavailable"
	}
	if state == nil {
		if s.requireGrowthGovernance {
			return nil, false, "not_approved"
		}
		return nil, true, ""
	}
	if !state.AllowsReward(now) {
		if state.Decision != PlayGrowthGovernanceDecisionApproved || !state.Approved {
			return state, false, "not_approved"
		}
		if state.BudgetAmount <= 0 || state.BudgetRemaining <= 0 {
			return state, false, "budget_exhausted"
		}
		return state, false, "invalid"
	}
	if !GrowthRolloutAllowsUser(userID, state.RolloutPercent) {
		return state, false, "rollout_excluded"
	}
	return state, true, ""
}

// reserveGrowthRewardBudget uses the optional atomic repository boundary. A
// legacy test double may omit it, but production wiring fails closed when the
// governance requirement is enabled.
func (s *PlayService) reserveGrowthRewardBudget(ctx context.Context, state *PlayGrowthGovernanceState, userID int64, source, actionID string, amount float64) error {
	if amount <= 0 || state == nil {
		return nil
	}
	repo, ok := s.repo.(PlayGrowthBudgetRepository)
	if !ok || repo == nil {
		if s.requireGrowthGovernance {
			return ErrPlayGrowthGovernanceUnavailable
		}
		return nil
	}
	reserved, err := repo.ReserveGrowthRewardBudget(ctx, state.ID, userID, source, actionID, amount)
	if err != nil {
		return err
	}
	if !reserved {
		return ErrPlayGrowthGovernanceBudgetExhausted
	}
	return nil
}

func (s *PlayService) GetGrowthGovernance(ctx context.Context) (*PlayGrowthGovernanceState, error) {
	return s.getGrowthGovernance(ctx, s.serverNow())
}

func (s *PlayService) GetGrowthCohort(ctx context.Context, start, end time.Time) (PlayGrowthCohortMetrics, error) {
	repo, ok := s.repo.(PlayGrowthGovernanceRepository)
	if !ok || repo == nil {
		return PlayGrowthCohortMetrics{}, ErrPlayGrowthGovernanceUnavailable
	}
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return PlayGrowthCohortMetrics{}, fmt.Errorf("cohort window is invalid")
	}
	return repo.GetGrowthCohort(ctx, start, end)
}

func (s *PlayService) ApproveGrowthGovernance(ctx context.Context, input PlayGrowthGovernanceApprovalInput) (*PlayGrowthGovernanceState, error) {
	if strings.TrimSpace(input.RuleVersion) == "" {
		input.RuleVersion = PlayGrowthQualificationRuleVersion()
	}
	if err := ValidatePlayGrowthGovernanceApproval(input, s.serverNow()); err != nil {
		return nil, ErrPlayGrowthGovernanceInvalid.WithCause(err)
	}
	repo, ok := s.repo.(PlayGrowthGovernanceRepository)
	if !ok || repo == nil {
		return nil, ErrPlayGrowthGovernanceUnavailable
	}
	return repo.CreateGrowthApproval(ctx, input)
}

func (s *PlayService) RevokeGrowthGovernance(ctx context.Context, actorID int64, reason string) (*PlayGrowthGovernanceState, error) {
	if actorID <= 0 {
		return nil, ErrPlayGrowthGovernanceInvalid.WithCause(fmt.Errorf("actor id is required"))
	}
	if n := len([]rune(strings.TrimSpace(reason))); n < 10 || n > 500 {
		return nil, ErrPlayGrowthGovernanceInvalid.WithCause(fmt.Errorf("reason must contain 10 to 500 characters"))
	}
	repo, ok := s.repo.(PlayGrowthGovernanceRepository)
	if !ok || repo == nil {
		return nil, ErrPlayGrowthGovernanceUnavailable
	}
	return repo.RevokeGrowthApproval(ctx, actorID, reason)
}

func (s *PlayService) GetGrowthRewardSpend(ctx context.Context, start, end time.Time) (float64, error) {
	repo, ok := s.repo.(PlayGrowthGovernanceRepository)
	if !ok || repo == nil {
		return 0, ErrPlayGrowthGovernanceUnavailable
	}
	return repo.GetGrowthRewardSpend(ctx, start, end)
}
