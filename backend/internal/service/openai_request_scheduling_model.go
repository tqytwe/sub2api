package service

import (
	"context"
	"strings"
)

// resolveOpenAIInitialUpstreamModel mirrors the first Forward attempt. Legacy
// compact may choose the configured fallback before sending; native v2 does not.
// Keep billing/public model resolution separate from this outbound identity.
func (s *OpenAIGatewayService) resolveOpenAIInitialUpstreamModel(account *Account, requestedModel string, requireCompact bool) string {
	upstreamModel := resolveOpenAIAccountUpstreamModelForRequest(account, requestedModel, requireCompact)
	if requireCompact && !shouldForwardOpenAIResponsesViaRawChatCompletions(account) {
		if fallback := s.resolveOpenAICompactFallbackModel(account, requestedModel); fallback != "" {
			return fallback
		}
	}
	return upstreamModel
}

func (s *OpenAIGatewayService) resolveOpenAIRequestSchedulingModel(ctx context.Context, account *Account, requestedModel string, requireCompact bool) string {
	if account == nil || !account.IsOpenAI() {
		return canonicalOpenAIAccountSchedulingModel(account, requestedModel)
	}
	if forwardModel, ok := openAIForwardModelFromContext(ctx); ok {
		requestedModel = forwardModel.model
		requireCompact = forwardModel.useCompactModelMapping
	}
	return s.resolveOpenAIInitialUpstreamModel(account, strings.TrimSpace(requestedModel), requireCompact)
}

// Some legacy candidate passes defer the compact capability check to classify
// an empty pool. Preserve the actual mapping mode through those passes without
// overriding the handler's channel mapping or explicit native-v2 false flag.
func withOpenAIForwardModelDefault(ctx context.Context, requestedModel string, requireCompact bool) context.Context {
	if _, ok := openAIForwardModelFromContext(ctx); ok {
		return ctx
	}
	return WithOpenAIForwardModel(ctx, requestedModel, requireCompact)
}

func openAIAccountOutboundModelRateLimited(ctx context.Context, account *Account, requestModel, outboundModel string) bool {
	return account.isRateLimitActiveForKey(outboundModel) ||
		(openAIImageGenerationRateLimitApplies(ctx, requestModel, outboundModel) && account.isRateLimitActiveForKey(openAIImageGenerationRateLimitKey))
}

func (s *OpenAIGatewayService) isOpenAIAccountSchedulableForRequest(ctx context.Context, account *Account, requestedModel string, requireCompact bool) bool {
	if account == nil {
		return false
	}
	if !account.IsOpenAI() {
		return account.IsSchedulableForModelWithContext(ctx, requestedModel)
	}
	return account.IsSchedulable() && !openAIAccountOutboundModelRateLimited(ctx, account, requestedModel,
		s.resolveOpenAIRequestSchedulingModel(ctx, account, requestedModel, requireCompact))
}

// Sticky and previous-response bindings must use the same model cooldown as
// candidate selection. Keep the shared helper's behavior for other platforms.
func (s *OpenAIGatewayService) shouldClearOpenAIStickySessionForRequest(ctx context.Context, account *Account, requestedModel string, requireCompact bool) bool {
	if account == nil || !account.IsOpenAI() {
		return shouldClearStickySession(account, requestedModel)
	}
	return !s.isOpenAIAccountSchedulableForRequest(ctx, account, requestedModel, requireCompact)
}
