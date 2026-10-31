package service

import (
	"context"
	"math"
)

// ProbeAPIKeyGroup checks scheduler eligibility without reserving slots,
// registering sessions, setting account errors, or binding sticky accounts.
func (s *GatewayService) ProbeAPIKeyGroup(ctx context.Context, group *Group, model, kind string) (APIKeyGroupAvailability, error) {
	ctx = s.withGroupContext(ctx, group)
	endpoint, _ := ctx.Value(apiKeyGroupEndpointKey{}).(string)
	if endpoint == "" {
		endpoint = CompositeRouteEndpointAny
	}
	if group.Platform == PlatformComposite {
		routeEndpoint := endpoint
		if endpoint == "live" || endpoint == "videos" || endpoint == "videos_grok" {
			routeEndpoint = CompositeRouteEndpointAny
		}
		decision, ok, err := s.resolveCompositeRouteDecision(ctx, group, model, routeEndpoint)
		if err != nil {
			return APIKeyGroupAvailability{}, err
		}
		if !ok {
			return APIKeyGroupAvailability{}, nil
		}
		model = decision.UpstreamModel
		ctx = WithCompositeRouteDecision(ctx, decision)
	}
	platform, forced, err := s.resolvePlatform(ctx, &group.ID, group, model)
	if err != nil {
		return APIKeyGroupAvailability{}, err
	}
	websocket, _ := ctx.Value(apiKeyGroupWebSocketKey{}).(bool)
	if websocket && platform != PlatformOpenAI && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if (endpoint == CompositeRouteEndpointMessages || endpoint == CompositeRouteEndpointCountTokens) && platform == PlatformOpenAI && !group.AllowMessagesDispatch {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == CompositeRouteEndpointImages && platform != PlatformOpenAI && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == CompositeRouteEndpointEmbeddings && platform != PlatformOpenAI {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == "live" && (platform != PlatformOpenAI || !group.AllowLive) {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == "videos" && platform != PlatformOpenAI && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == "videos_grok" && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if (endpoint == "audio" || endpoint == "audio_openai" || endpoint == "audio_realtime") && platform != PlatformOpenAI && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == "audio_openai" && platform != PlatformOpenAI {
		return APIKeyGroupAvailability{}, nil
	}
	if endpoint == "voice_grok" && platform != PlatformGrok {
		return APIKeyGroupAvailability{}, nil
	}
	if kind == "token" {
		switch endpoint {
		case CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions, CompositeRouteEndpointMessages, CompositeRouteEndpointGemini:
			ctx, _ = WithGatewayTokenRequestPricing(ctx)
			ctx = s.withGatewayProfitControlGate(ctx, &group.ID)
		}
	}
	if s.checkChannelPricingRestriction(ctx, &group.ID, model) {
		return APIKeyGroupAvailability{}, nil
	}
	if platform == PlatformGrok {
		switch endpoint {
		case "audio_realtime":
			model = ""
		case "audio", "voice_grok":
			model = "grok-4.5"
		}
	}
	accounts, _, err := s.listSchedulableAccounts(ctx, &group.ID, platform, forced)
	if err != nil {
		return APIKeyGroupAvailability{}, err
	}
	ctx = s.withWindowCostPrefetch(ctx, accounts)
	ctx = s.withRPMPrefetch(ctx, accounts)
	eligible := make([]AccountWithConcurrency, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if endpoint == "live" && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityLive) {
			continue
		}
		if endpoint == "videos" && platform == PlatformOpenAI && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityVideos) {
			continue
		}
		if endpoint == CompositeRouteEndpointEmbeddings && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings) {
			continue
		}
		if (endpoint == CompositeRouteEndpointChatCompletions || endpoint == CompositeRouteEndpointResponses) && account.IsOpenAICompatible() && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
			continue
		}
		if endpoint == CompositeRouteEndpointResponses && platform == PlatformOpenAI && OpenAIImageGenerationIntentFromContext(ctx) && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses) {
			continue
		}

		if (endpoint == "audio" || endpoint == "audio_openai" || endpoint == "audio_realtime") && platform == PlatformOpenAI && !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio) {
			continue
		}
		if websocket && account.Platform == PlatformOpenAI && !openAIAccountTransportCompatible(s.cfg, NewOpenAIWSProtocolResolver(s.cfg), account, OpenAIUpstreamTransportResponsesWebsocketV2Ingress) {
			continue
		}
		if !s.isAccountSchedulableForSelection(account) || !s.isGatewayAccountProfitEligible(ctx, account) ||
			(group.RequirePrivacySet && !account.IsPrivacySet()) ||
			(model != "" && !s.isModelSupportedByAccountInGroup(ctx, account, &group.ID, model)) ||
			!s.isAccountSchedulableForModelSelection(ctx, account, model) ||
			!s.isAccountSchedulableForQuota(account) ||
			!s.isAccountSchedulableForWindowCost(ctx, account, false) ||
			!s.isAccountSchedulableForRPM(ctx, account, false) ||
			s.isUpstreamModelRestrictedByChannel(ctx, group.ID, account, model) {
			continue
		}
		eligible = append(eligible, AccountWithConcurrency{ID: account.ID, MaxConcurrency: account.Concurrency})
	}
	if len(eligible) == 0 {
		return APIKeyGroupAvailability{}, nil
	}
	if s.concurrencyService == nil {
		return APIKeyGroupAvailability{Available: true}, nil
	}
	loads, err := s.concurrencyService.GetAccountsLoadBatch(ctx, eligible)
	if err != nil {
		return APIKeyGroupAvailability{}, err
	}
	minLoad := math.Inf(1)
	for _, account := range eligible {
		if account.MaxConcurrency <= 0 {
			return APIKeyGroupAvailability{Available: true}, nil
		}
		load, ok := loads[account.ID]
		if !ok || load == nil {
			continue
		}
		ratio := float64(load.CurrentConcurrency) / float64(account.MaxConcurrency)
		if ratio < minLoad {
			minLoad = ratio
		}
	}
	if math.IsInf(minLoad, 1) {
		return APIKeyGroupAvailability{}, nil
	}
	return APIKeyGroupAvailability{Available: true, Busy: minLoad >= 1, Load: minLoad}, nil
}
