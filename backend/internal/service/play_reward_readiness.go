package service

import (
	"context"
	"fmt"
	"time"
)

// PlayRewardReadiness is an operator-facing explanation of why a reward
// activity can or cannot issue redeemable rewards. It is read-only.
type PlayRewardReadiness struct {
	Activity             string   `json:"activity"`
	Enabled              bool     `json:"enabled"`
	Ready                bool     `json:"ready"`
	BlockingReasons      []string `json:"blocking_reasons"`
	CouponPoolReady      bool     `json:"coupon_pool_ready"`
	BlindboxPoolValid    bool     `json:"blindbox_pool_valid,omitempty"`
	BlindboxPoolVersion  string   `json:"blindbox_pool_version,omitempty"`
	BlindboxCost         float64  `json:"blindbox_cost,omitempty"`
	BlindboxDailyLimit   int      `json:"blindbox_daily_limit,omitempty"`
	GovernanceAvailable  bool     `json:"governance_available"`
	GovernanceReason     string   `json:"governance_reason,omitempty"`
	GovernanceDecision   string   `json:"governance_decision,omitempty"`
	BudgetRemaining      float64  `json:"budget_remaining,omitempty"`
	GovernanceRolloutPct int      `json:"governance_rollout_percent,omitempty"`
	CheckedAt            string   `json:"checked_at"`
}

// GetRewardReadiness evaluates both activities without mutating settings,
// pools, inventory, balances, or governance state.
func (s *PlayService) GetRewardReadiness(ctx context.Context) ([]PlayRewardReadiness, error) {
	if s == nil {
		return nil, fmt.Errorf("play service is unavailable")
	}
	rt := s.GetRuntime(ctx)
	now := s.serverNow()
	governance, governanceErr := s.getGrowthGovernance(ctx, now)
	governanceOK := governanceErr == nil && governance != nil && governance.AllowsReward(now)
	governanceReason := ""
	if governanceErr != nil {
		governanceReason = "unavailable"
	} else if governance == nil || !governanceOK {
		governanceReason = "not_approved"
		if governance != nil && governance.BudgetAmount > 0 && governance.BudgetRemaining <= 0 {
			governanceReason = "budget_exhausted"
		}
	}
	governanceDecision := "none"
	budgetRemaining := 0.0
	rollout := 0
	if governance != nil {
		governanceDecision = string(governance.Decision)
		budgetRemaining = governance.BudgetRemaining
		rollout = governance.RolloutPercent
	}
	readiness := func(activity CouponRewardActivity, enabled bool) (PlayRewardReadiness, error) {
		ready, err := s.couponRewardPoolReady(ctx, activity)
		if err != nil {
			return PlayRewardReadiness{}, err
		}
		item := PlayRewardReadiness{
			Activity:             string(activity),
			Enabled:              enabled,
			CouponPoolReady:      ready,
			GovernanceAvailable:  governanceOK,
			GovernanceReason:     governanceReason,
			GovernanceDecision:   governanceDecision,
			BudgetRemaining:      budgetRemaining,
			GovernanceRolloutPct: rollout,
			CheckedAt:            now.UTC().Format(time.RFC3339),
		}
		if !enabled {
			item.BlockingReasons = append(item.BlockingReasons, "feature_disabled")
		}
		if !ready {
			item.BlockingReasons = append(item.BlockingReasons, "coupon_pool_unavailable")
		}
		if s.requireGrowthGovernance && !governanceOK {
			item.BlockingReasons = append(item.BlockingReasons, "governance_"+governanceReason)
		}
		return item, nil
	}
	checkin, err := readiness(CouponRewardActivityCheckin, rt.CheckinEnabled)
	if err != nil {
		return nil, err
	}
	blindbox, err := readiness(CouponRewardActivityBlindbox, rt.BlindboxEnabled)
	if err != nil {
		return nil, err
	}
	blindbox.BlindboxDailyLimit = rt.BlindboxDailyLimit
	blindbox.BlindboxCost = rt.BlindboxPool.Cost
	blindbox.BlindboxPoolVersion = rt.BlindboxPool.Version
	blindbox.BlindboxPoolValid = ValidateBlindboxPool(rt.BlindboxPool) == nil
	if !blindbox.BlindboxPoolValid {
		blindbox.BlockingReasons = append(blindbox.BlockingReasons, "blindbox_pool_invalid")
	}
	checkin.Ready = len(checkin.BlockingReasons) == 0
	blindbox.Ready = len(blindbox.BlockingReasons) == 0
	return []PlayRewardReadiness{checkin, blindbox}, nil
}
