package service

import (
	"encoding/json"
	"math"
)

const (
	AccountCostMultiplierExtraKey = "cost_multiplier"
	DefaultAccountCostMultiplier  = 0.1
)

// CostMultiplier is the estimated provider cost fraction stored on the account.
// It does not affect user billing or group selling multipliers.
func (a *Account) CostMultiplier() float64 {
	if a != nil {
		if value, ok := accountCostMultiplierNumber(a.Extra[AccountCostMultiplierExtraKey]); ok {
			return value
		}
	}
	return DefaultAccountCostMultiplier
}

func accountCostMultiplierNumber(raw any) (float64, bool) {
	var value float64
	switch v := raw.(type) {
	case float64:
		value = v
	case float32:
		value = float64(v)
	case int:
		value = float64(v)
	case int64:
		value = float64(v)
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, false
		}
		value = parsed
	default:
		return 0, false
	}
	return value, !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1000000
}
