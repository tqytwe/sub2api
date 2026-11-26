package service

import (
	"context"
	"fmt"
	"math"
	"strings"

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

func openAIUsageBillingModels(result *OpenAIForwardResult, fields ChannelUsageFields) []string {
	// Use the rewrite captured during forwarding, never current key settings:
	// skipped endpoints and policy changes after admission retain their price.
	requestedBillingModel := fields.OriginalModel
	if fields.APIKeyMappedModel != "" {
		requestedBillingModel = fields.APIKeyMappedModel
	}
	channelBillingModel := fields.ChannelMappedModel
	if !fields.ChannelMappingApplied && channelBillingModel == fields.OriginalModel {
		// An unchanged channel field is the audit alias, not an explicit price
		// override. Do not resurrect its price when the key target is unpriced.
		channelBillingModel = requestedBillingModel
	}
	resultBillingModel := result.Model
	if !fields.ChannelMappingApplied && result.Model == fields.OriginalModel {
		resultBillingModel = requestedBillingModel
	}
	billingModel := forwardResultBillingModel(resultBillingModel, result.UpstreamModel)
	if result.BillingModel != "" {
		billingModel = strings.TrimSpace(result.BillingModel)
	}
	if fields.BillingModelSource == BillingModelSourceChannelMapped && fields.ChannelMappedModel != "" &&
		((fields.APIKeyMappedModel != "" && fields.ChannelMappingApplied) || fields.ChannelMappedModel != fields.OriginalModel) {
		billingModel = fields.ChannelMappedModel
	}
	if fields.BillingModelSource == BillingModelSourceRequested && fields.OriginalModel != "" {
		billingModel = requestedBillingModel
	}
	if result.StarframeVideoBilling != nil {
		billingModel = result.StarframeVideoBilling.Model
	}
	return usageBillingModelCandidates(
		billingModel,
		result.BillingModel,
		channelBillingModel,
		requestedBillingModel,
		result.UpstreamModel,
		resultBillingModel,
	)
}

type imageBillingModelKey struct{}

func WithImageBillingModel(ctx context.Context, fields ChannelUsageFields) context.Context {
	return context.WithValue(ctx, imageBillingModelKey{}, fields)
}

func (s *OpenAIGatewayService) requiresObservedOpenAIImages(ctx context.Context, c *gin.Context, model, upstreamModel string) bool {
	if c == nil {
		return false
	}
	if c.GetHeader("X-Image-Usage") == "billing" {
		return true
	}
	key := getAPIKeyFromContext(c)
	if key == nil || key.Group == nil {
		return false
	}
	fields, _ := ctx.Value(imageBillingModelKey{}).(ChannelUsageFields)
	models := openAIUsageBillingModels(&OpenAIForwardResult{Model: model, UpstreamModel: upstreamModel}, fields)
	primary := firstUsageBillingModel(models)
	if price := s.resolveOpenAIChannelPricing(ctx, primary, key); price != nil && price.Mode == BillingModeImage {
		return true
	}
	if price := matchGroupModelPricing(key.Group, primary); price != nil && price.BillingMode == BillingModeImage {
		return true
	}
	return imageMinimumCharge(key.Group, models...) != nil
}

func (s *OpenAIGatewayService) resolveOpenAIImagesBillingCount(ctx context.Context, c *gin.Context, model, upstreamModel string, reported, requested int) int {
	if reported > 0 || s.requiresObservedOpenAIImages(ctx, c, model, upstreamModel) {
		return max(0, reported)
	}
	return resolveBillableImageCount(reported, requested, c != nil && imageCountBillingEnabled(getAPIKeyFromContext(c)))
}
