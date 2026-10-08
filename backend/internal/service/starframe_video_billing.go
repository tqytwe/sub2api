package service

import (
	"context"
	"fmt"
	"math"
)

// StarframeVideoBilling is a trusted preflight snapshot, never upstream/client JSON.
type StarframeVideoBilling struct {
	Model      string  `json:"model"`
	Duration   int     `json:"duration"`
	Resolution string  `json:"resolution"`
	UnitPrice  float64 `json:"unit_price"`
}

func validStarframeVideoBilling(info StarframeVideoRequest, snapshot *StarframeVideoBilling) bool {
	if snapshot == nil || snapshot.Model == "" || snapshot.Model != info.Model || snapshot.Duration != info.Duration || snapshot.Resolution != info.Resolution {
		return false
	}
	if snapshot.Duration < 1 || snapshot.Duration > 15 || snapshot.UnitPrice <= 0 || math.IsNaN(snapshot.UnitPrice) || math.IsInf(snapshot.UnitPrice, 0) {
		return false
	}
	switch snapshot.Resolution {
	case VideoBillingResolution480P, VideoBillingResolution720P, VideoBillingResolution1080P:
		return true
	default:
		return false
	}
}

func (s *OpenAIGatewayService) SnapshotStarframeVideoBilling(ctx context.Context, apiKey *APIKey, info StarframeVideoRequest) (*StarframeVideoBilling, error) {
	apiKey = s.apiKeyWithFreshGroupMediaPricing(ctx, apiKey)
	if apiKey == nil || apiKey.Group == nil {
		return nil, fmt.Errorf("StarFrame video group pricing is not configured")
	}
	var price *float64
	if configured, ok := apiKey.Group.VideoModelPrices[info.Model][info.Resolution]; ok {
		price = &configured
	} else {
		// Only an explicitly configured matching flat tier may supplement an exact ID.
		switch info.Resolution {
		case VideoBillingResolution480P:
			price = apiKey.Group.VideoPrice480P
		case VideoBillingResolution720P:
			price = apiKey.Group.VideoPrice720P
		case VideoBillingResolution1080P:
			price = apiKey.Group.VideoPrice1080P
		}
	}
	if price == nil {
		return nil, fmt.Errorf("StarFrame requires an explicit positive group video price for this model and resolution")
	}
	snapshot := &StarframeVideoBilling{Model: info.Model, Duration: info.Duration, Resolution: info.Resolution, UnitPrice: *price}
	if !validStarframeVideoBilling(info, snapshot) {
		return nil, fmt.Errorf("StarFrame requires an explicit positive group video price and exact duration/resolution")
	}
	return snapshot, nil
}

func (s *OpenAIGatewayService) calculatePinnedStarframeVideoCost(result *OpenAIForwardResult, multiplier float64) (*CostBreakdown, error) {
	info := StarframeVideoRequest{Model: result.Model, Duration: result.VideoDurationSeconds, Resolution: result.VideoResolution}
	snapshot := result.StarframeVideoBilling
	if result.VideoCount != 1 || !validStarframeVideoBilling(info, snapshot) {
		return nil, fmt.Errorf("StarFrame billing snapshot missing or inconsistent; token/default pricing is forbidden")
	}
	// Flat explicit prices pin this request without any channel/model fallback.
	prices := &VideoPriceConfig{Price480P: &snapshot.UnitPrice, Price720P: &snapshot.UnitPrice, Price1080P: &snapshot.UnitPrice}
	return s.billingService.CalculateVideoCost(snapshot.Model, snapshot.Resolution, 1, snapshot.Duration, prices, multiplier), nil
}
