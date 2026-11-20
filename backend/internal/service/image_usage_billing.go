package service

import (
	"context"
	"fmt"
	"math"

	"github.com/gin-gonic/gin"
)

const ImageEquivalentOutputPricePerMillion = 0.01

type ImageBillingReceipt struct {
	Settled                 bool    `json:"-"`
	ActualCost              float64 `json:"actual_cost"`
	BillingMode             string  `json:"billing_mode"`
	ImageCount              int     `json:"image_count"`
	MinimumCharge           float64 `json:"minimum_charge,omitempty"`
	MinimumChargeAdjustment float64 `json:"minimum_charge_adjustment,omitempty"`
}

func (r ImageBillingReceipt) EquivalentTokens() (int64, error) {
	if !r.Settled || math.IsNaN(r.ActualCost) || math.IsInf(r.ActualCost, 0) || r.ActualCost < 0 {
		return 0, fmt.Errorf("settled image billing receipt unavailable")
	}
	units := math.Round(r.ActualCost * 1e8)
	if units > 9007199254740991 {
		return 0, fmt.Errorf("image billing usage exceeds JSON safe integer range")
	}
	return int64(units), nil
}

type imageBillingReceiptKey struct{}

func WithImageBillingReceipt(ctx context.Context, receipt *ImageBillingReceipt) context.Context {
	return context.WithValue(ctx, imageBillingReceiptKey{}, receipt)
}

func imageMinimumCharge(group *Group, models ...string) *float64 {
	for _, model := range models {
		if p := matchGroupModelPricing(group, model); p != nil && p.BillingMode == BillingModeToken && p.ImageMinimumCharge != nil {
			return p.ImageMinimumCharge
		}
	}
	return nil
}

func validateImageMinimumCharge(platform string, p ChannelModelPricing) error {
	if p.ImageMinimumCharge == nil {
		return nil
	}
	value := *p.ImageMinimumCharge
	if platform != PlatformOpenAI || p.Platform != PlatformOpenAI || p.BillingMode != BillingModeToken || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1000 || QuantizeUsageBillingAmount(value) != value {
		return fmt.Errorf("image_minimum_charge requires OpenAI token pricing and a final per-image charge from 0 to 1000 with at most eight decimals")
	}
	return nil
}

// Preserve upstream and supplier cost, adjusting only the customer charge.
func applyImageMinimumCharge(cost *CostBreakdown, count int, perImage float64) (floor, adjustment float64) {
	if cost == nil || cost.BillingMode != string(BillingModeToken) || count <= 0 || perImage <= 0 {
		return 0, 0
	}
	floor = QuantizeUsageBillingAmount(float64(count) * perImage)
	if floor > cost.ActualCost {
		adjustment = QuantizeUsageBillingAmount(floor - cost.ActualCost)
		cost.ActualCost = floor
	}
	return floor, adjustment
}

func requiresObservedOpenAIImages(c *gin.Context, models ...string) bool {
	if c == nil {
		return false
	}
	if c.GetHeader("X-Image-Usage") == "billing" {
		return true
	}
	if key := getAPIKeyFromContext(c); key != nil {
		return imageMinimumCharge(key.Group, models...) != nil
	}
	return false
}
