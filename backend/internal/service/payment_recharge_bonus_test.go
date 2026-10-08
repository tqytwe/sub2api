//go:build unit

package service

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestNormalizeRechargeBonusTiers(t *testing.T) {
	t.Run("sorts ascending by min amount", func(t *testing.T) {
		out, err := NormalizeRechargeBonusTiers([]RechargeBonusTier{
			{MinAmount: 1000, BonusPercent: 35},
			{MinAmount: 100, BonusPercent: 20},
			{MinAmount: 500, BonusPercent: 30},
		})
		require.NoError(t, err)
		require.Equal(t, []RechargeBonusTier{
			{MinAmount: 100, BonusPercent: 20},
			{MinAmount: 500, BonusPercent: 30},
			{MinAmount: 1000, BonusPercent: 35},
		}, out)
	})

	t.Run("empty input yields empty non-nil slice", func(t *testing.T) {
		out, err := NormalizeRechargeBonusTiers(nil)
		require.NoError(t, err)
		require.NotNil(t, out)
		require.Len(t, out, 0)
	})

	t.Run("allows zero threshold and zero percent", func(t *testing.T) {
		out, err := NormalizeRechargeBonusTiers([]RechargeBonusTier{{MinAmount: 0, BonusPercent: 0}})
		require.NoError(t, err)
		require.Len(t, out, 1)
	})

	t.Run("rejects invalid values", func(t *testing.T) {
		cases := map[string][]RechargeBonusTier{
			"negative min":        {{MinAmount: -1, BonusPercent: 10}},
			"min three decimals":  {{MinAmount: 100.123, BonusPercent: 10}},
			"negative percent":    {{MinAmount: 100, BonusPercent: -5}},
			"percent over limit":  {{MinAmount: 100, BonusPercent: 1000.01}},
			"percent 3 decimals":  {{MinAmount: 100, BonusPercent: 12.345}},
			"duplicate min":       {{MinAmount: 100, BonusPercent: 10}, {MinAmount: 100, BonusPercent: 20}},
			"duplicate min 2 dec": {{MinAmount: 100, BonusPercent: 10}, {MinAmount: 100.00, BonusPercent: 20}},
		}
		for name, tiers := range cases {
			_, err := NormalizeRechargeBonusTiers(tiers)
			require.Error(t, err, name)
		}
	})

	t.Run("rejects too many tiers", func(t *testing.T) {
		tiers := make([]RechargeBonusTier, 0, maxRechargeBonusTiers+1)
		for i := 0; i <= maxRechargeBonusTiers; i++ {
			tiers = append(tiers, RechargeBonusTier{MinAmount: float64(i + 1), BonusPercent: 1})
		}
		_, err := NormalizeRechargeBonusTiers(tiers)
		require.Error(t, err)
	})
}

func TestParseRechargeBonusTiers(t *testing.T) {
	t.Run("empty or invalid json yields empty slice", func(t *testing.T) {
		require.NotNil(t, parseRechargeBonusTiers(""))
		require.Len(t, parseRechargeBonusTiers(""), 0)
		require.Len(t, parseRechargeBonusTiers("not json"), 0)
	})

	t.Run("drops invalid entries keeps first duplicate and sorts", func(t *testing.T) {
		raw := `[{"min_amount":500,"bonus_percent":30},{"min_amount":-1,"bonus_percent":5},` +
			`{"min_amount":100,"bonus_percent":20},{"min_amount":100,"bonus_percent":99},` +
			`{"min_amount":50,"bonus_percent":5000}]`
		out := parseRechargeBonusTiers(raw)
		require.Equal(t, []RechargeBonusTier{
			{MinAmount: 100, BonusPercent: 20},
			{MinAmount: 500, BonusPercent: 30},
		}, out)
	})

	t.Run("round trips encode", func(t *testing.T) {
		tiers := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 30}}
		encoded, err := encodeRechargeBonusTiers(tiers)
		require.NoError(t, err)
		require.Equal(t, tiers, parseRechargeBonusTiers(encoded))

		empty, err := encodeRechargeBonusTiers(nil)
		require.NoError(t, err)
		require.Equal(t, "", empty)
	})
}

func TestParsePaymentConfigRechargeBonus(t *testing.T) {
	svc := &PaymentConfigService{}

	t.Run("defaults", func(t *testing.T) {
		cfg := svc.parsePaymentConfig(map[string]string{})
		require.NotNil(t, cfg.RechargeBonusTiers)
		require.Len(t, cfg.RechargeBonusTiers, 0)
		require.Equal(t, "", cfg.RechargeBonusNotice)
	})

	t.Run("reads tiers and notice", func(t *testing.T) {
		cfg := svc.parsePaymentConfig(map[string]string{
			SettingRechargeBonusTiers:  `[{"min_amount":500,"bonus_percent":30},{"min_amount":100,"bonus_percent":20}]`,
			SettingRechargeBonusNotice: "**满 100 送 20%**",
		})
		require.Equal(t, []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 30}}, cfg.RechargeBonusTiers)
		require.Equal(t, "**满 100 送 20%**", cfg.RechargeBonusNotice)
	})
}

func TestUpdatePaymentConfigRechargeBonus(t *testing.T) {
	ctx := context.Background()

	t.Run("persists normalized tiers and trimmed notice", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
		svc := &PaymentConfigService{settingRepo: repo}
		tiers := []RechargeBonusTier{{MinAmount: 500, BonusPercent: 30}, {MinAmount: 100, BonusPercent: 20}}
		notice := "  活动文案  "
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{
			RechargeBonusTiers:  &tiers,
			RechargeBonusNotice: &notice,
		}))
		require.Equal(t, `[{"min_amount":100,"bonus_percent":20},{"min_amount":500,"bonus_percent":30}]`, repo.updates[SettingRechargeBonusTiers])
		require.Equal(t, "活动文案", repo.updates[SettingRechargeBonusNotice])

		cfg, err := svc.GetPaymentConfig(ctx)
		require.NoError(t, err)
		require.Equal(t, []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 30}}, cfg.RechargeBonusTiers)
	})

	t.Run("empty tiers clears setting and omitted fields are untouched", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingRechargeBonusTiers:  `[{"min_amount":100,"bonus_percent":20}]`,
			SettingRechargeBonusNotice: "keep me",
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		empty := []RechargeBonusTier{}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusTiers: &empty}))
		value, ok := repo.updates[SettingRechargeBonusTiers]
		require.True(t, ok)
		require.Equal(t, "", value)
		_, touched := repo.updates[SettingRechargeBonusNotice]
		require.False(t, touched)
		require.Equal(t, "keep me", repo.values[SettingRechargeBonusNotice])
	})

	t.Run("rejects invalid tiers", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
		svc := &PaymentConfigService{settingRepo: repo}
		bad := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 100, BonusPercent: 30}}
		err := svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusTiers: &bad})
		require.Error(t, err)
		require.Nil(t, repo.updates)
	})
}

func TestAffiliateRebateBaseAmountExcludesRechargeBonus(t *testing.T) {
	require.Equal(t, 100.0, affiliateRebateBaseAmount(&dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance, Amount: 130, BonusAmount: 30,
	}))
	require.Equal(t, 130.0, affiliateRebateBaseAmount(&dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance, Amount: 130,
	}))
	// 订阅订单不受 bonus 字段影响
	require.Equal(t, 50.0, affiliateRebateBaseAmount(&dbent.PaymentOrder{
		OrderType: payment.OrderTypeSubscription, Amount: 50, BonusAmount: 30,
	}))
	// 异常数据：赠送大于总额时钳到 0
	require.Equal(t, 0.0, affiliateRebateBaseAmount(&dbent.PaymentOrder{
		OrderType: payment.OrderTypeBalance, Amount: 10, BonusAmount: 30,
	}))
}

func TestNormalizeRechargeBonusMode(t *testing.T) {
	for raw, want := range map[string]string{"": RechargeBonusModeBonus, "bonus": RechargeBonusModeBonus, " Discount ": RechargeBonusModeDiscount} {
		mode, ok := NormalizeRechargeBonusMode(raw)
		require.True(t, ok, raw)
		require.Equal(t, want, mode, raw)
	}
	mode, ok := NormalizeRechargeBonusMode("cashback")
	require.False(t, ok)
	require.Equal(t, RechargeBonusModeBonus, mode)
}

func TestValidateRechargeBonusTiersForMode(t *testing.T) {
	tiers := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}, {MinAmount: 500, BonusPercent: 100}}
	require.NoError(t, ValidateRechargeBonusTiersForMode(RechargeBonusModeBonus, tiers))
	require.Error(t, ValidateRechargeBonusTiersForMode(RechargeBonusModeDiscount, tiers))
	require.NoError(t, ValidateRechargeBonusTiersForMode(RechargeBonusModeDiscount, tiers[:1]))
}

func TestParsePaymentConfigRechargeBonusMode(t *testing.T) {
	svc := &PaymentConfigService{}
	require.Equal(t, RechargeBonusModeBonus, svc.parsePaymentConfig(map[string]string{}).RechargeBonusMode)
	require.Equal(t, RechargeBonusModeDiscount, svc.parsePaymentConfig(map[string]string{SettingRechargeBonusMode: "discount"}).RechargeBonusMode)
	require.Equal(t, RechargeBonusModeBonus, svc.parsePaymentConfig(map[string]string{SettingRechargeBonusMode: "junk"}).RechargeBonusMode)
}

func TestUpdatePaymentConfigRechargeBonusMode(t *testing.T) {
	ctx := context.Background()

	t.Run("persists discount mode with valid tiers", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
		svc := &PaymentConfigService{settingRepo: repo}
		mode := "discount"
		tiers := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 20}}
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusTiers: &tiers, RechargeBonusMode: &mode}))
		require.Equal(t, "discount", repo.updates[SettingRechargeBonusMode])
		cfg, err := svc.GetPaymentConfig(ctx)
		require.NoError(t, err)
		require.Equal(t, RechargeBonusModeDiscount, cfg.RechargeBonusMode)
	})

	t.Run("rejects unknown mode", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{}}
		svc := &PaymentConfigService{settingRepo: repo}
		mode := "cashback"
		require.Error(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusMode: &mode}))
		require.Nil(t, repo.updates)
	})

	t.Run("switching to discount with stored tiers at 100 percent is rejected", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{
			SettingRechargeBonusTiers: `[{"min_amount":100,"bonus_percent":100}]`,
		}}
		svc := &PaymentConfigService{settingRepo: repo}
		mode := "discount"
		require.Error(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusMode: &mode}))
		require.Nil(t, repo.updates)
	})

	t.Run("saving tiers at 100 percent while stored mode is discount is rejected", func(t *testing.T) {
		repo := &paymentConfigSettingRepoStub{values: map[string]string{SettingRechargeBonusMode: "discount"}}
		svc := &PaymentConfigService{settingRepo: repo}
		tiers := []RechargeBonusTier{{MinAmount: 100, BonusPercent: 100}}
		require.Error(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusTiers: &tiers}))
		require.Nil(t, repo.updates)
		// 同样的档位在赠金模式下合法
		repo.values[SettingRechargeBonusMode] = "bonus"
		require.NoError(t, svc.UpdatePaymentConfig(ctx, UpdatePaymentConfigRequest{RechargeBonusTiers: &tiers}))
	})
}
