package service

import (
	"context"
	"encoding/json"
	"math"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const paymentRechargeSnapshotVersion = 1

type paymentRechargeCampaignBonus struct {
	BonusPct    float64 `json:"bonus_pct"`
	CampaignIDs []int64 `json:"campaign_ids,omitempty"`
}

// PaymentRechargeQuote is the immutable calculation basis for a balance top-up.
type PaymentRechargeQuote struct {
	SchemaVersion              int           `json:"schema_version"`
	InputAmount                float64       `json:"input_amount"`
	BalanceRechargeMultiplier  float64       `json:"balance_recharge_multiplier"`
	BaseCredited               float64       `json:"base_credited"`
	CurrentVIP                 PlayVIPStatus `json:"current_vip"`
	VIPBonusPct                float64       `json:"vip_bonus_pct"`
	CampaignBonusPct           float64       `json:"campaign_bonus_pct"`
	CampaignIDs                []int64       `json:"campaign_ids,omitempty"`
	EffectiveBonusPct          float64       `json:"effective_bonus_pct"`
	EffectiveCreditMultiplier  float64       `json:"effective_credit_multiplier"`
	CreditedAmount             float64       `json:"credited_amount"`
	TotalRechargedBefore       float64       `json:"total_recharged_before"`
	TotalRechargedAfterBase    float64       `json:"total_recharged_after_base"`
	VIPUpgradeAppliesNextOrder bool          `json:"vip_upgrade_applies_next_order"`
}

func (q PaymentRechargeQuote) Snapshot() map[string]any {
	var out map[string]any
	raw, err := json.Marshal(q)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func (s *PaymentService) BuildRechargeQuote(ctx context.Context, userID int64, inputAmount float64) (*PaymentRechargeQuote, error) {
	if s == nil || s.userRepo == nil || s.configService == nil {
		return nil, nil
	}
	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.buildRechargeQuoteForUser(ctx, user, inputAmount, cfg.BalanceRechargeMultiplier)
}

func (s *PaymentService) buildRechargeQuoteForUser(ctx context.Context, user *User, inputAmount, multiplier float64) (*PaymentRechargeQuote, error) {
	if user == nil {
		return nil, nil
	}
	tiers := defaultPlayVIPTiers()
	if s != nil {
		if s.playService != nil {
			tiers = s.playService.GetRuntime(ctx).VIPTiers
		} else if s.configService != nil && s.configService.settingRepo != nil {
			tiers = (&SettingService{settingRepo: s.configService.settingRepo}).GetPlayRuntime(ctx).VIPTiers
		}
	}
	campaign := paymentRechargeCampaignBonus{}
	if s != nil && s.playService != nil {
		bonusPct, campaignIDs, err := s.playService.ResolveRechargeCampaignBonus(ctx)
		if err != nil {
			return nil, err
		}
		campaign = paymentRechargeCampaignBonus{BonusPct: bonusPct, CampaignIDs: campaignIDs}
	}
	quote := buildPaymentRechargeQuote(inputAmount, multiplier, user.TotalRecharged, tiers, campaign)
	return &quote, nil
}

func buildPaymentRechargeQuote(inputAmount, multiplier, totalRechargedBefore float64, tiers []PlayVIPTier, campaign paymentRechargeCampaignBonus) PaymentRechargeQuote {
	if math.IsNaN(inputAmount) || math.IsInf(inputAmount, 0) || inputAmount < 0 {
		inputAmount = 0
	}
	if math.IsNaN(totalRechargedBefore) || math.IsInf(totalRechargedBefore, 0) || totalRechargedBefore < 0 {
		totalRechargedBefore = 0
	}
	multiplier = normalizeBalanceRechargeMultiplier(multiplier)
	currentVIP := resolveVIPStatus(totalRechargedBefore, tiers)
	vipBonusPct := clampRechargeBonusPct(currentVIP.RechargeBonusPct, maxVIPRechargeBonusPct)
	campaignBonusPct := clampRechargeBonusPct(campaign.BonusPct, 1000)
	baseCredited := calculateCreditedBalance(inputAmount, multiplier)
	effectiveBonusPct := roundPct(vipBonusPct + campaignBonusPct)
	credited := decimal.NewFromFloat(baseCredited).
		Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(effectiveBonusPct).Div(decimal.NewFromInt(100)))).
		Round(2).
		InexactFloat64()
	return PaymentRechargeQuote{
		SchemaVersion:              paymentRechargeSnapshotVersion,
		InputAmount:                roundMoney(inputAmount),
		BalanceRechargeMultiplier:  multiplier,
		BaseCredited:               baseCredited,
		CurrentVIP:                 currentVIP,
		VIPBonusPct:                vipBonusPct,
		CampaignBonusPct:           campaignBonusPct,
		CampaignIDs:                append([]int64(nil), campaign.CampaignIDs...),
		EffectiveBonusPct:          effectiveBonusPct,
		EffectiveCreditMultiplier:  roundMultiplier(decimal.NewFromFloat(multiplier).Mul(decimal.NewFromFloat(1).Add(decimal.NewFromFloat(effectiveBonusPct).Div(decimal.NewFromInt(100))))),
		CreditedAmount:             credited,
		TotalRechargedBefore:       roundMoney(totalRechargedBefore),
		TotalRechargedAfterBase:    roundMoney(totalRechargedBefore + baseCredited),
		VIPUpgradeAppliesNextOrder: true,
	}
}

func clampRechargeBonusPct(value float64, max float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0
	}
	if value > max {
		return max
	}
	return roundPct(value)
}

func roundMoney(value float64) float64 {
	return decimal.NewFromFloat(value).Round(2).InexactFloat64()
}

func roundPct(value float64) float64 {
	return decimal.NewFromFloat(value).Round(4).InexactFloat64()
}

func roundMultiplier(value decimal.Decimal) float64 {
	return value.Round(6).InexactFloat64()
}

func paymentOrderRechargeSnapshot(o *dbent.PaymentOrder) map[string]any {
	if o == nil || len(o.RechargeSnapshot) == 0 {
		return nil
	}
	return o.RechargeSnapshot
}

func paymentOrderRechargeBaseCredited(o *dbent.PaymentOrder) (float64, bool) {
	if o == nil || o.OrderType != payment.OrderTypeBalance {
		return 0, false
	}
	snapshot := paymentOrderRechargeSnapshot(o)
	if snapshot == nil {
		return o.Amount, false
	}
	if base, ok := snapshotFloat(snapshot, "base_credited"); ok && base > 0 {
		return roundMoney(base), true
	}
	return o.Amount, false
}

func paymentOrderRefundTotalRechargedDelta(o *dbent.PaymentOrder, refundAmount float64) float64 {
	if o == nil || o.OrderType != payment.OrderTypeBalance || refundAmount <= 0 {
		return 0
	}
	baseCredited, fromSnapshot := paymentOrderRechargeBaseCredited(o)
	if baseCredited <= 0 {
		return 0
	}
	if o.Amount <= 0 || !fromSnapshot {
		return -roundMoney(math.Min(refundAmount, baseCredited))
	}
	ratio := refundAmount / o.Amount
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	return -roundMoney(baseCredited * ratio)
}

func snapshotFloat(snapshot map[string]any, key string) (float64, bool) {
	raw, ok := snapshot[key]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
