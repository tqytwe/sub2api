package service

import "strings"

const (
	BillingSurchargeModeNone                 = "none"
	BillingSurchargeModePercentOnChargedCost = "percent_on_charged_cost"
	BillingSurchargeModeAdditiveMultiplier   = "additive_multiplier"
)

type BillingSurchargeConfig struct {
	OverrideEnabled bool
	Enabled         bool
	Mode            string
	Value           float64
}

type BillingSurchargeBreakdown struct {
	Enabled       bool
	Mode          string
	Value         float64
	BaseCost      float64
	SurchargeCost float64
	BilledCost    float64
}

func NormalizeBillingSurchargeMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case BillingSurchargeModePercentOnChargedCost:
		return BillingSurchargeModePercentOnChargedCost
	case BillingSurchargeModeAdditiveMultiplier:
		return BillingSurchargeModeAdditiveMultiplier
	default:
		return BillingSurchargeModeNone
	}
}

func ApplyBillingSurcharge(cost *CostBreakdown, cfg BillingSurchargeConfig) BillingSurchargeBreakdown {
	baseActual := 0.0
	totalCost := 0.0
	if cost != nil {
		baseActual = cost.ActualCost
		totalCost = cost.TotalCost
	}
	out := BillingSurchargeBreakdown{
		Mode:       BillingSurchargeModeNone,
		BaseCost:   baseActual,
		BilledCost: baseActual,
	}
	mode := NormalizeBillingSurchargeMode(cfg.Mode)
	if !cfg.Enabled || mode == BillingSurchargeModeNone || cfg.Value <= 0 || baseActual <= 0 {
		return out
	}
	var surcharge float64
	switch mode {
	case BillingSurchargeModePercentOnChargedCost:
		surcharge = baseActual * cfg.Value
	case BillingSurchargeModeAdditiveMultiplier:
		surcharge = totalCost * cfg.Value
	}
	if surcharge <= 0 {
		return out
	}
	out.Enabled = true
	out.Mode = mode
	out.Value = cfg.Value
	out.SurchargeCost = surcharge
	out.BilledCost = baseActual + surcharge
	return out
}

func ResolveGroupBillingSurcharge(group *Group, global BillingSurchargeConfig) BillingSurchargeConfig {
	if group == nil || !group.BillingSurchargeOverrideEnabled {
		return BillingSurchargeConfig{
			Enabled: global.Enabled,
			Mode:    NormalizeBillingSurchargeMode(global.Mode),
			Value:   global.Value,
		}
	}
	return BillingSurchargeConfig{
		OverrideEnabled: true,
		Enabled:         group.BillingSurchargeEnabled,
		Mode:            NormalizeBillingSurchargeMode(group.BillingSurchargeMode),
		Value:           group.BillingSurchargeValue,
	}
}
