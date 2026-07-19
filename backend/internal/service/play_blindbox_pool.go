package service

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/shopspring/decimal"
)

const blindboxWeightTotal int64 = 10_000

type PlayBlindboxTier struct {
	Amount float64 `json:"amount"`
	Weight int64   `json:"weight"`
}

type PlayBlindboxPool struct {
	Version string             `json:"version"`
	Cost    float64            `json:"cost"`
	RTPCap  float64            `json:"rtp_cap"`
	Tiers   []PlayBlindboxTier `json:"tiers"`
}

type blindboxPoolConfigDiagnostic struct {
	Reason string
}

func defaultBlindboxPool() PlayBlindboxPool {
	return PlayBlindboxPool{
		Version: "season-1-v1",
		Cost:    0.5,
		RTPCap:  0.9,
		Tiers: []PlayBlindboxTier{
			{Amount: 0.05, Weight: 4000},
			{Amount: 0.20, Weight: 3000},
			{Amount: 0.50, Weight: 1800},
			{Amount: 1, Weight: 800},
			{Amount: 3, Weight: 300},
			{Amount: 10, Weight: 90},
			{Amount: 20, Weight: 10},
		},
	}
}

func defaultVIPBlindboxPools() []PlayVIPBlindboxPool {
	return []PlayVIPBlindboxPool{
		{Tier: 0, Pool: defaultBlindboxPool()},
		{Tier: 1, Pool: PlayBlindboxPool{
			Version: "season-1-vip-v1",
			Cost:    0.5,
			RTPCap:  0.92,
			Tiers: []PlayBlindboxTier{
				{Amount: 0.05, Weight: 3895},
				{Amount: 0.20, Weight: 3000},
				{Amount: 0.50, Weight: 1900},
				{Amount: 1, Weight: 805},
				{Amount: 3, Weight: 300},
				{Amount: 10, Weight: 90},
				{Amount: 20, Weight: 10},
			},
		}},
		{Tier: 2, Pool: PlayBlindboxPool{
			Version: "season-1-vip-v2",
			Cost:    0.5,
			RTPCap:  0.94,
			Tiers: []PlayBlindboxTier{
				{Amount: 0.05, Weight: 3710},
				{Amount: 0.20, Weight: 3000},
				{Amount: 0.50, Weight: 2050},
				{Amount: 1, Weight: 840},
				{Amount: 3, Weight: 300},
				{Amount: 10, Weight: 90},
				{Amount: 20, Weight: 10},
			},
		}},
		{Tier: 3, Pool: PlayBlindboxPool{
			Version: "season-1-vip-v3",
			Cost:    0.5,
			RTPCap:  0.96,
			Tiers: []PlayBlindboxTier{
				{Amount: 0.05, Weight: 3570},
				{Amount: 0.20, Weight: 3000},
				{Amount: 0.50, Weight: 2150},
				{Amount: 1, Weight: 870},
				{Amount: 3, Weight: 310},
				{Amount: 10, Weight: 90},
				{Amount: 20, Weight: 10},
			},
		}},
		{Tier: 4, Pool: PlayBlindboxPool{
			Version: "season-1-vip-v4",
			Cost:    0.5,
			RTPCap:  0.98,
			Tiers: []PlayBlindboxTier{
				{Amount: 0.05, Weight: 3434},
				{Amount: 0.20, Weight: 3000},
				{Amount: 0.50, Weight: 2250},
				{Amount: 1, Weight: 900},
				{Amount: 3, Weight: 315},
				{Amount: 10, Weight: 91},
				{Amount: 20, Weight: 10},
			},
		}},
		{Tier: 5, Pool: PlayBlindboxPool{
			Version: "season-1-vip-v5",
			Cost:    0.5,
			RTPCap:  1,
			Tiers: []PlayBlindboxTier{
				{Amount: 0.05, Weight: 3307},
				{Amount: 0.20, Weight: 3000},
				{Amount: 0.50, Weight: 2350},
				{Amount: 1, Weight: 920},
				{Amount: 3, Weight: 320},
				{Amount: 10, Weight: 93},
				{Amount: 20, Weight: 10},
			},
		}},
	}
}

func resolveVIPBlindboxPool(base PlayBlindboxPool, vip PlayVIPStatus) PlayBlindboxPool {
	for _, item := range resolveVIPBlindboxPools(base) {
		if item.Tier == vip.Tier {
			return cloneBlindboxPoolValue(item.Pool)
		}
	}
	return cloneBlindboxPoolValue(base)
}

func resolveNextVIPBlindboxPool(base PlayBlindboxPool, vip PlayVIPStatus) *PlayBlindboxPool {
	if vip.NextTier <= vip.Tier {
		return nil
	}
	for _, item := range resolveVIPBlindboxPools(base) {
		if item.Tier == vip.NextTier {
			next := cloneBlindboxPoolValue(item.Pool)
			return &next
		}
	}
	return nil
}

func resolveVIPBlindboxPools(base PlayBlindboxPool) []PlayVIPBlindboxPool {
	if blindboxPoolsEquivalent(base, defaultBlindboxPool()) {
		return defaultVIPBlindboxPools()
	}
	pools := make([]PlayVIPBlindboxPool, 0, len(defaultPlayVIPTiers()))
	for _, tier := range defaultPlayVIPTiers() {
		pools = append(pools, PlayVIPBlindboxPool{
			Tier: tier.Tier,
			Pool: cloneBlindboxPoolValue(base),
		})
	}
	return pools
}

func cloneBlindboxPoolValue(pool PlayBlindboxPool) PlayBlindboxPool {
	pool.Tiers = append([]PlayBlindboxTier(nil), pool.Tiers...)
	return pool
}

func blindboxPoolsEquivalent(a, b PlayBlindboxPool) bool {
	if a.Version != b.Version ||
		math.Abs(a.Cost-b.Cost) > 1e-12 ||
		math.Abs(a.RTPCap-b.RTPCap) > 1e-12 ||
		len(a.Tiers) != len(b.Tiers) {
		return false
	}
	for i := range a.Tiers {
		if math.Abs(a.Tiers[i].Amount-b.Tiers[i].Amount) > 1e-12 ||
			a.Tiers[i].Weight != b.Tiers[i].Weight {
			return false
		}
	}
	return true
}

func (p PlayBlindboxPool) ExpectedReward() float64 {
	var weightedReward float64
	for _, tier := range p.Tiers {
		weightedReward += tier.Amount * float64(tier.Weight)
	}
	return weightedReward / float64(blindboxWeightTotal)
}

func ValidateBlindboxPool(pool PlayBlindboxPool) error {
	if strings.TrimSpace(pool.Version) == "" {
		return fmt.Errorf("blindbox pool version is required")
	}
	if !isFiniteFloat(pool.Cost) || pool.Cost <= 0 {
		return fmt.Errorf("blindbox pool cost must be positive and finite")
	}
	if !isFiniteFloat(pool.RTPCap) || pool.RTPCap <= 0 || pool.RTPCap > 1 {
		return fmt.Errorf("blindbox pool rtp cap must be finite and within (0, 1]")
	}
	if len(pool.Tiers) < 1 || len(pool.Tiers) > 32 {
		return fmt.Errorf("blindbox pool must contain between 1 and 32 tiers")
	}

	var totalWeight int64
	for i, tier := range pool.Tiers {
		if !isFiniteFloat(tier.Amount) || tier.Amount < 0 {
			return fmt.Errorf("blindbox pool tier %d amount must be non-negative and finite", i)
		}
		if tier.Weight <= 0 {
			return fmt.Errorf("blindbox pool tier %d weight must be positive", i)
		}
		if tier.Weight > blindboxWeightTotal-totalWeight {
			return fmt.Errorf("blindbox pool weights must total %d", blindboxWeightTotal)
		}
		totalWeight += tier.Weight
	}
	if totalWeight != blindboxWeightTotal {
		return fmt.Errorf("blindbox pool weights must total %d", blindboxWeightTotal)
	}

	expectedReward := blindboxExpectedRewardDecimal(pool)
	rtpLimit := decimal.NewFromFloat(pool.Cost).Mul(decimal.NewFromFloat(pool.RTPCap))
	if expectedReward.GreaterThan(rtpLimit) {
		return fmt.Errorf(
			"blindbox pool expected reward %s exceeds rtp cap %s",
			expectedReward.String(),
			rtpLimit.String(),
		)
	}
	return nil
}

func parseBlindboxPool(raw string) (PlayBlindboxPool, *blindboxPoolConfigDiagnostic) {
	fallback := defaultBlindboxPool()
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}

	var pool PlayBlindboxPool
	if err := json.Unmarshal([]byte(raw), &pool); err != nil {
		return fallback, &blindboxPoolConfigDiagnostic{
			Reason: "malformed_json",
		}
	}
	if err := ValidateBlindboxPool(pool); err != nil {
		return fallback, &blindboxPoolConfigDiagnostic{
			Reason: "invalid_pool",
		}
	}
	return pool, nil
}

func pickBlindboxRewardAt(pool PlayBlindboxPool, draw int64) float64 {
	var cumulative int64
	for _, tier := range pool.Tiers {
		cumulative += tier.Weight
		if draw < cumulative {
			return tier.Amount
		}
	}
	return pool.Tiers[len(pool.Tiers)-1].Amount
}

func cryptoBlindboxDrawSource(max int64) (int64, error) {
	if max <= 0 {
		return 0, fmt.Errorf("blindbox draw maximum must be positive")
	}
	draw, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return draw.Int64(), nil
}

func (s *PlayService) pickBlindboxReward(pool PlayBlindboxPool) (float64, error) {
	if err := ValidateBlindboxPool(pool); err != nil {
		return 0, err
	}
	if s == nil || s.blindboxDrawSource == nil {
		return 0, fmt.Errorf("blindbox draw source is not configured")
	}

	draw, err := s.blindboxDrawSource(blindboxWeightTotal)
	if err != nil {
		return 0, fmt.Errorf("blindbox draw source: %w", err)
	}
	if draw < 0 || draw >= blindboxWeightTotal {
		return 0, fmt.Errorf("blindbox draw out of range: %d", draw)
	}
	return pickBlindboxRewardAt(pool, draw), nil
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func blindboxExpectedRewardDecimal(pool PlayBlindboxPool) decimal.Decimal {
	weightedReward := decimal.Zero
	for _, tier := range pool.Tiers {
		weightedReward = weightedReward.Add(
			decimal.NewFromFloat(tier.Amount).Mul(decimal.NewFromInt(tier.Weight)),
		)
	}
	return weightedReward.Div(decimal.NewFromInt(blindboxWeightTotal))
}
