package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Daily policies use a JSON object; monthly snapshots keep their existing array
// format. A nil stored policy identifies a legacy period, never a zero budget.
type PlayArenaDailyPolicy struct {
	Rewards []PlayArenaSettlementTier `json:"rewards"`
	Budget  float64                   `json:"budget"`
}

type PlayArenaDailyPolicyRepository interface {
	EnsureDailyArenaPeriodWithPolicy(context.Context, time.Time, PlayArenaDailyPolicy) (*PlayArenaPeriod, error)
	GetDailyArenaPolicy(context.Context, int64) (*PlayArenaDailyPolicy, error)
	FreezeDailyArenaPolicy(context.Context, int64, PlayArenaDailyPolicy) error
}

func dailyArenaRuntimePolicy(rt PlayRuntime) PlayArenaDailyPolicy {
	policy := PlayArenaDailyPolicy{Rewards: rt.DailyArenaTopRewards, Budget: rt.DailyArenaDailyBudget}
	if len(policy.Rewards) == 0 {
		policy.Rewards = parsePlayDailyArenaRewards("")
	}
	if policy.Budget <= 0 {
		policy.Budget = defaultPlayDailyArenaDailyBudgetUSD
	}
	return policy
}

func (s *PlayService) PrepareCurrentArenaDailySeason(ctx context.Context, now time.Time) (*PlayArenaPeriod, error) {
	rt := s.GetRuntime(ctx)
	if !rt.ArenaEnabled || !rt.DailyArenaEnabled {
		return nil, nil
	}
	return s.ensureDailyArenaPeriod(ctx, now, rt)
}

func (s *PlayService) ensureDailyArenaPeriod(ctx context.Context, now time.Time, rt PlayRuntime) (*PlayArenaPeriod, error) {
	if repo, ok := s.repo.(PlayArenaDailyPolicyRepository); ok {
		return repo.EnsureDailyArenaPeriodWithPolicy(ctx, now, dailyArenaRuntimePolicy(rt))
	}
	return s.repo.EnsureDailyArenaPeriod(ctx, now)
}

func (s *PlayService) dailyArenaPolicyForPeriod(ctx context.Context, period *PlayArenaPeriod) (PlayArenaDailyPolicy, error) {
	if period != nil {
		if repo, ok := s.repo.(PlayArenaDailyPolicyRepository); ok {
			policy, err := repo.GetDailyArenaPolicy(ctx, period.ID)
			if err != nil {
				return PlayArenaDailyPolicy{}, err
			}
			if policy != nil {
				return *policy, nil
			}
		}
	}
	// Pre-upgrade periods retain their old live-policy contract until reviewed
	// activation freezes it. Do not claim to reconstruct a historical snapshot.
	return dailyArenaRuntimePolicy(s.GetRuntime(ctx)), nil
}

// The same budget cutoff is used for public estimates, previews and payouts.
// Decimal arithmetic avoids losing the last winner to a floating-point sum.
func dailyArenaRewardForRank(rank int, policy PlayArenaDailyPolicy) float64 {
	if rank <= 0 || rank > rewardTierMaxRank(policy.Rewards) {
		return 0
	}
	spent := decimal.Zero
	budget := decimal.NewFromFloat(policy.Budget)
	for i := 1; i <= rank; i++ {
		amount := arenaRewardForRank(i, policy.Rewards)
		if amount <= 0 {
			continue
		}
		spent = spent.Add(decimal.NewFromFloat(amount))
		if spent.GreaterThan(budget) {
			return 0
		}
	}
	return arenaRewardForRank(rank, policy.Rewards)
}
