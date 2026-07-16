package service

import (
	"context"
	"sort"
	"strings"
)

// PublicCatalogModel is a curated model row for the public /models page.
type PublicCatalogModel struct {
	Name        string
	Platform    string
	UseCase     string
	PlatformKey string // service platform id (openai, anthropic, gemini)
}

// PublicModelPricingRow is one model with official and site reference prices (USD per token).
type PublicModelPricingRow struct {
	Name                string   `json:"name"`
	Platform            string   `json:"platform"`
	UseCase             string   `json:"use_case"`
	OfficialInputPrice  *float64 `json:"official_input_price"`
	OfficialOutputPrice *float64 `json:"official_output_price"`
	OurInputPrice       *float64 `json:"our_input_price"`
	OurOutputPrice      *float64 `json:"our_output_price"`
	RateMultiplier      float64  `json:"rate_multiplier"`
}

// PublicCatalogModels is the default lineup shown on /models when no channel pricing exists.
var PublicCatalogModels = []PublicCatalogModel{
	{Name: "gpt-5.6-sol", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "reasoning"},
	{Name: "gpt-5.6-terra", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "code"},
	{Name: "gpt-5.5", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "code"},
	{Name: "gpt-5-mini", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "chat"},
	{Name: "gpt-4.1", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "code"},
	{Name: "o4-mini", Platform: "OpenAI", PlatformKey: PlatformOpenAI, UseCase: "reasoning"},
	{Name: "claude-sonnet-4-6", Platform: "Anthropic", PlatformKey: PlatformAnthropic, UseCase: "code"},
	{Name: "claude-opus-4-8", Platform: "Anthropic", PlatformKey: PlatformAnthropic, UseCase: "reasoning"},
	{Name: "claude-haiku-4-5", Platform: "Anthropic", PlatformKey: PlatformAnthropic, UseCase: "chat"},
	{Name: "gemini-2.5-flash", Platform: "Google", PlatformKey: PlatformGemini, UseCase: "chat"},
	{Name: "gemini-3-flash", Platform: "Google", PlatformKey: PlatformGemini, UseCase: "reasoning"},
}

func (s *PlayService) GetPublicModelRateMultiplier(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return 1
	}
	return s.settingService.GetPublicModelRateMultiplier(ctx)
}

// ListPublicModelPricing returns official catalog prices and site reference prices.
// When channels are configured, additional supported models are merged in.
func (s *PlayService) ListPublicModelPricing(ctx context.Context, billing *BillingService) []PublicModelPricingRow {
	if billing == nil {
		return nil
	}
	mult := s.GetPublicModelRateMultiplier(ctx)

	byName := make(map[string]PublicCatalogModel, len(PublicCatalogModels)+16)
	for _, m := range PublicCatalogModels {
		byName[strings.ToLower(m.Name)] = m
	}

	if s.channelService != nil {
		channels, err := s.channelService.ListAvailable(ctx)
		if err == nil {
			for _, ch := range channels {
				if ch.Status != StatusActive {
					continue
				}
				for _, sm := range ch.SupportedModels {
					key := strings.ToLower(sm.Name)
					if _, ok := byName[key]; ok {
						continue
					}
					byName[key] = PublicCatalogModel{
						Name:        sm.Name,
						Platform:    displayPlatformName(sm.Platform),
						PlatformKey: sm.Platform,
						UseCase:     "general",
					}
				}
			}
		}
	}

	names := make([]string, 0, len(byName))
	for k := range byName {
		names = append(names, k)
	}
	sort.Strings(names)

	channelBase := s.collectChannelBasePrices(ctx)

	out := make([]PublicModelPricingRow, 0, len(names))
	for _, key := range names {
		meta := byName[key]
		officialIn, officialOut := lookupOfficialPrices(billing, meta.Name)

		ourIn, ourOut := scalePricePtr(officialIn, mult), scalePricePtr(officialOut, mult)
		if base, ok := channelBase[strings.ToLower(meta.Name)]; ok {
			if base.input != nil {
				ourIn = base.input
			}
			if base.output != nil {
				ourOut = base.output
			}
		}

		out = append(out, PublicModelPricingRow{
			Name:                meta.Name,
			Platform:            meta.Platform,
			UseCase:             meta.UseCase,
			OfficialInputPrice:  officialIn,
			OfficialOutputPrice: officialOut,
			OurInputPrice:       ourIn,
			OurOutputPrice:      ourOut,
			RateMultiplier:      mult,
		})
	}
	return out
}

type channelPricePair struct {
	input  *float64
	output *float64
}

func (s *PlayService) collectChannelBasePrices(ctx context.Context) map[string]channelPricePair {
	out := make(map[string]channelPricePair)
	if s.channelService == nil {
		return out
	}
	channels, err := s.channelService.ListAvailable(ctx)
	if err != nil {
		return out
	}
	for _, ch := range channels {
		if ch.Status != StatusActive {
			continue
		}
		for _, sm := range ch.SupportedModels {
			key := strings.ToLower(sm.Name)
			if sm.Pricing == nil {
				continue
			}
			if _, exists := out[key]; exists {
				continue
			}
			out[key] = channelPricePair{
				input:  sm.Pricing.InputPrice,
				output: sm.Pricing.OutputPrice,
			}
		}
	}
	return out
}

func lookupOfficialPrices(billing *BillingService, model string) (*float64, *float64) {
	pricing, err := billing.GetModelPricing(model)
	if err != nil || pricing == nil {
		return nil, nil
	}
	var inPtr, outPtr *float64
	if pricing.InputPricePerToken > 0 {
		v := pricing.InputPricePerToken
		inPtr = &v
	}
	if pricing.OutputPricePerToken > 0 {
		v := pricing.OutputPricePerToken
		outPtr = &v
	}
	return inPtr, outPtr
}

func scalePricePtr(v *float64, mult float64) *float64 {
	if v == nil {
		return nil
	}
	scaled := *v * mult
	return &scaled
}

func displayPlatformName(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PlatformOpenAI:
		return "OpenAI"
	case PlatformAnthropic:
		return "Anthropic"
	case PlatformGemini, "google":
		return "Google"
	case PlatformGrok, "xai":
		return "xAI"
	default:
		if platform == "" {
			return "—"
		}
		return strings.ToUpper(platform[:1]) + platform[1:]
	}
}
