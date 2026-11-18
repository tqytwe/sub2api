package service

import (
	"context"
	"fmt"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrChannelPricingBatchInvalid = infraerrors.BadRequest("CHANNEL_PRICING_BATCH_INVALID", "channel pricing batch is invalid")

type ChannelPricingBatchItem struct {
	ChannelID int64                 `json:"channel_id"`
	Pricing   []ChannelModelPricing `json:"model_pricing"`
}

type ChannelPricingBatchRepository interface {
	BatchReplaceModelPricing(context.Context, []ChannelPricingBatchItem) error
}

func (s *ChannelService) BatchUpdateModelPricing(ctx context.Context, items []ChannelPricingBatchItem) error {
	if len(items) == 0 || len(items) > 100 {
		return ErrChannelPricingBatchInvalid
	}
	seen := make(map[int64]struct{}, len(items))
	for i := range items {
		item := &items[i]
		if item.ChannelID <= 0 || item.Pricing == nil || len(item.Pricing) > 1000 {
			return ErrChannelPricingBatchInvalid
		}
		if _, ok := seen[item.ChannelID]; ok {
			return ErrChannelPricingBatchInvalid
		}
		seen[item.ChannelID] = struct{}{}
		if err := validateChannelConfig(item.Pricing, nil); err != nil {
			return fmt.Errorf("channel %d pricing: %w", item.ChannelID, err)
		}
		for j := range item.Pricing {
			item.Pricing[j].ChannelID = item.ChannelID
		}
	}
	repo, ok := s.repo.(ChannelPricingBatchRepository)
	if !ok {
		return ErrChannelPricingBatchInvalid
	}
	if err := repo.BatchReplaceModelPricing(ctx, items); err != nil {
		return err
	}
	s.invalidateCache()
	return nil
}
