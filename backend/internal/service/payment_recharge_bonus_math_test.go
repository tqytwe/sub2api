package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchRechargeBonusTier(t *testing.T) {
	tiers := []RechargeBonusTier{
		{MinAmount: 100, BonusPercent: 20},
		{MinAmount: 500, BonusPercent: 30},
		{MinAmount: 1000, BonusPercent: 35},
	}
	cases := []struct {
		amount  float64
		percent float64
		ok      bool
	}{
		{amount: 0, ok: false},
		{amount: 99.99, ok: false},
		{amount: 100, percent: 20, ok: true},
		{amount: 499.99, percent: 20, ok: true},
		{amount: 500, percent: 30, ok: true},
		{amount: 1000, percent: 35, ok: true},
		{amount: 1000000, percent: 35, ok: true},
	}
	for _, tc := range cases {
		tier, ok := matchRechargeBonusTier(tiers, tc.amount)
		require.Equal(t, tc.ok, ok, "amount %v", tc.amount)
		if ok {
			require.Equal(t, tc.percent, tier.BonusPercent, "amount %v", tc.amount)
		}
	}

	t.Run("float boundary 0.1+0.2 still matches 0.3 threshold", func(t *testing.T) {
		_, ok := matchRechargeBonusTier([]RechargeBonusTier{{MinAmount: 0.3, BonusPercent: 1}}, 0.1+0.2)
		require.True(t, ok)
	})

	t.Run("no tiers never matches", func(t *testing.T) {
		_, ok := matchRechargeBonusTier(nil, 100)
		require.False(t, ok)
	})
}

func TestCalculateRechargeBonusRounding(t *testing.T) {
	require.Equal(t, 20.0, calculateRechargeBonus(100, 20))
	require.Equal(t, 120.0, addRechargeBonus(100, 20))
	// 33.33 * 15% = 4.9995 → 5.00
	require.Equal(t, 5.0, calculateRechargeBonus(33.33, 15))
	require.Zero(t, calculateRechargeBonus(100, 0))
	require.Zero(t, calculateRechargeBonus(0, 20))

	// 阈值按支付金额命中，赠送按到账基数计算：1000 CNY × 0.14 = 140 USD，命中 1000 档 30% → 42
	cfg := &PaymentConfig{
		BalanceRechargeMultiplier: 0.14,
		RechargeBonusTiers:        []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 30}},
	}
	require.Equal(t, rechargeBonusQuote{PayBase: 1000, Credited: 182, Bonus: 42, Percent: 30}, quoteRechargeBonus(cfg, 1000, "CNY"))
	// 命中 0% 档位视为无优惠
	cfg.RechargeBonusTiers = []RechargeBonusTier{{MinAmount: 10, BonusPercent: 0}}
	require.Equal(t, rechargeBonusQuote{PayBase: 50, Credited: 7}, quoteRechargeBonus(cfg, 50, "CNY"))
}

func TestQuoteRechargeBonus(t *testing.T) {
	tiers := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 50}}

	t.Run("bonus mode keeps pay base and inflates credit", func(t *testing.T) {
		cfg := &PaymentConfig{BalanceRechargeMultiplier: 1, RechargeBonusTiers: tiers, RechargeBonusMode: RechargeBonusModeBonus}
		q := quoteRechargeBonus(cfg, 100, "USD")
		require.Equal(t, rechargeBonusQuote{PayBase: 100, Credited: 120, Bonus: 20, Percent: 20}, q)

		// 未命中：无优惠
		q = quoteRechargeBonus(cfg, 50, "USD")
		require.Equal(t, rechargeBonusQuote{PayBase: 50, Credited: 50}, q)
	})

	t.Run("discount mode keeps credit and reduces pay base", func(t *testing.T) {
		cfg := &PaymentConfig{BalanceRechargeMultiplier: 1, RechargeBonusTiers: tiers, RechargeBonusMode: RechargeBonusModeDiscount}
		q := quoteRechargeBonus(cfg, 500, "USD")
		require.Equal(t, rechargeBonusQuote{PayBase: 250, Credited: 500, Bonus: 250, Percent: 50}, q)

		// 倍率 0.14：1000 CNY 到账 140 USD；20% off 实付 800 CNY，免费部分 = 140 − 112 = 28 USD
		cfg.BalanceRechargeMultiplier = 0.14
		q = quoteRechargeBonus(cfg, 1000, "CNY")
		require.Equal(t, rechargeBonusQuote{PayBase: 500, Credited: 140, Bonus: 70, Percent: 50}, q)
		q = quoteRechargeBonus(cfg, 200, "CNY")
		require.Equal(t, rechargeBonusQuote{PayBase: 160, Credited: 28, Bonus: 5.6, Percent: 20}, q)
	})

	t.Run("discount rounds pay base to currency precision", func(t *testing.T) {
		cfg := &PaymentConfig{BalanceRechargeMultiplier: 1, RechargeBonusTiers: []RechargeBonusTier{{MinAmount: 1, BonusPercent: 15}}, RechargeBonusMode: RechargeBonusModeDiscount}
		require.Equal(t, 85.85, quoteRechargeBonus(cfg, 101, "USD").PayBase)
		require.Equal(t, 86.0, quoteRechargeBonus(cfg, 101, "JPY").PayBase)
	})

	t.Run("discount percent at or above 100 is ignored fail-safe", func(t *testing.T) {
		cfg := &PaymentConfig{BalanceRechargeMultiplier: 1, RechargeBonusTiers: []RechargeBonusTier{{MinAmount: 1, BonusPercent: 100}}, RechargeBonusMode: RechargeBonusModeDiscount}
		require.Equal(t, rechargeBonusQuote{PayBase: 100, Credited: 100}, quoteRechargeBonus(cfg, 100, "USD"))
	})

	t.Run("nil config and empty tiers yield plain conversion", func(t *testing.T) {
		require.Equal(t, rechargeBonusQuote{PayBase: 100, Credited: 100}, quoteRechargeBonus(nil, 100, "USD"))
		require.Equal(t, rechargeBonusQuote{PayBase: 100, Credited: 14}, quoteRechargeBonus(&PaymentConfig{BalanceRechargeMultiplier: 0.14}, 100, "CNY"))
	})
}
