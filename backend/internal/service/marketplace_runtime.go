package service

import "context"

// MarketplaceRuntime is the fail-closed runtime view of the marketplace switch.
type MarketplaceRuntime struct {
	Enabled bool
}

// GetMarketplaceRuntime reads the marketplace switch directly from settings.
// Only the exact value "true" enables it; missing, invalid, read errors, and an
// unwired service all remain disabled.
func (s *SettingService) GetMarketplaceRuntime(ctx context.Context) MarketplaceRuntime {
	if s == nil || s.settingRepo == nil {
		return MarketplaceRuntime{}
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyMarketplaceEnabled})
	if err != nil {
		return MarketplaceRuntime{}
	}
	return MarketplaceRuntime{
		Enabled: values[SettingKeyMarketplaceEnabled] == "true",
	}
}
