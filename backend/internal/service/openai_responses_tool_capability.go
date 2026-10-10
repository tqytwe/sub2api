package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/tidwall/gjson"
)

type openAIResponsesHasEffectiveToolsKey struct{}

const openAIResponsesToolsProtocolMismatch = "responses_tools_protocol_mismatch"

var ErrNoAvailableResponsesToolsAccounts = fmt.Errorf("no available accounts support Responses tool calls for this model: %w", ErrNoAvailableAccounts)

// The fallback guard is model-specific after mapping. Do not force every tools
// request to Responses: other models and compatible providers keep their route.
func (s *OpenAIGatewayService) openAIResponsesToolsProtocolCompatible(ctx context.Context, account *Account, requestedModel string, requireCompact bool) bool {
	if account == nil || !account.IsOpenAI() {
		return true
	}
	hasTools, _ := ctx.Value(openAIResponsesHasEffectiveToolsKey{}).(bool)
	if !hasTools || !openai.IsGPT61SolModelSpelling(s.resolveOpenAIRequestSchedulingModel(ctx, account, requestedModel, requireCompact)) {
		return true
	}
	return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses) &&
		!shouldForwardOpenAIResponsesViaRawChatCompletions(account)
}

func (s openAISelectionFilterStats) noAvailableError(requestedModel string, compactBlocked bool, extra string) error {
	blocked := s.reasons[openAIResponsesToolsProtocolMismatch]
	if !compactBlocked && blocked > 0 && blocked+s.reasons["excluded"] == s.pool {
		return ErrNoAvailableResponsesToolsAccounts
	}
	return noAvailableOpenAISelectionError(requestedModel, compactBlocked, s.summary(extra))
}

// WithOpenAIResponsesToolRequirements retains only the effective-tool presence,
// including additional_tools and tool-search discoveries, never the payload.
// Parse only the relevant projection so native Responses extensions stay intact.
func WithOpenAIResponsesToolRequirements(ctx context.Context, body []byte) (context.Context, error) {
	if _, known := ctx.Value(openAIResponsesHasEffectiveToolsKey{}).(bool); known {
		return ctx, nil
	}
	// Forward promotes legacy messages/functions before its final tools guard.
	// Reuse that rule for this projection, including native-input precedence;
	// the caller's body and billing identity are not replaced here.
	normalized, _, err := normalizeOpenAIResponsesLegacyIngress(body)
	if err != nil {
		return ctx, err
	}
	// Only presence is needed here. EffectiveResponsesTools also decodes input
	// extensions and validates discovery contents, which would change native
	// passthrough and Responses Lite validation. Discovery promotion requires a
	// declared tool_search, so it cannot turn an empty declaration set nonempty.
	view := gjson.ParseBytes(normalized)
	hasTools := openAIResponsesToolDeclarationPresent(view.Get("tools"))
	if input := view.Get("input"); !hasTools && input.IsArray() {
		input.ForEach(func(_, item gjson.Result) bool {
			if item.Get("type").String() == "additional_tools" {
				hasTools = openAIResponsesToolDeclarationPresent(item.Get("tools"))
			}
			return !hasTools
		})
	}
	return context.WithValue(ctx, openAIResponsesHasEffectiveToolsKey{}, hasTools), nil
}

func openAIResponsesToolDeclarationPresent(tools gjson.Result) bool {
	if !tools.Exists() || tools.Type == gjson.Null {
		return false
	}
	// Keep malformed declarations on the tool-capable route and let the existing
	// forward validator report its original error, including error.param.
	return !tools.IsArray() || tools.Get("#").Int() > 0
}
