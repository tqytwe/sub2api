package service

import (
	"context"
	"slices"
	"strings"
)

const DefaultCommandCodeTestModel = "deepseek/deepseek-v4-flash"

func (a *Account) IsCommandCode() bool {
	return a != nil && a.Platform == PlatformCommandCode
}

func resolveCommandCodeProtocol(account *Account, inbound, model string, catalog []string) string {
	if pinned := account.GetAPIProtocol(); pinned != APIProtocolAdaptive {
		return pinned
	}
	choose := func(protocols []string) string {
		if slices.Contains(protocols, inbound) {
			return inbound
		}
		return protocols[0]
	}
	model = strings.ToLower(strings.TrimSpace(model))
	customRules, configured := account.openCodeGoProtocolRules()
	for _, rule := range customRules {
		if rule.Pattern == model || (strings.HasSuffix(rule.Pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(rule.Pattern, "*"))) {
			return choose(append([]string{rule.Protocol}, rule.Protocols...))
		}
	}
	if len(catalog) > 0 {
		return choose(catalog)
	}
	if !configured {
		_, unqualified, hasProvider := strings.Cut(model, "/")
		if hasProvider {
			model = unqualified
		}
		switch {
		case strings.HasPrefix(model, "claude-"):
			return APIProtocolAnthropic
		case strings.HasPrefix(model, "gpt-"):
			return choose([]string{APIProtocolResponses, APIProtocolChatCompletions})
		}
	}
	return APIProtocolChatCompletions
}

func (s *OpenAIGatewayService) modelRoutedUpstreamProtocol(ctx context.Context, account *Account, inbound, model string) string {
	if account.IsCommandCode() {
		return resolveCommandCodeProtocol(account, inbound, model, s.modelCatalogProtocols(ctx, account, model))
	}
	return openCodeGoNativeProtocol(account, model)
}
