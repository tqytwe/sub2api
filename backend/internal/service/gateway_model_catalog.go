package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// GetGroupModelCatalog is the fail-closed source for multi-group discovery.
// Unlike the legacy listing API, an error or missing accounts never means
// "advertise defaults". One repository snapshot supplies every concrete platform.
func (s *GatewayService) GetGroupModelCatalog(ctx context.Context, groupID int64, platform string) ([]string, error) {
	if s == nil || s.accountRepo == nil || groupID <= 0 {
		return nil, fmt.Errorf("model discovery is unavailable")
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	routes, err := s.membershipCatalogRoutes(ctx, &groupID)
	if err != nil {
		return nil, err
	}
	platforms := []string{platform}
	if platform == PlatformComposite {
		present := make(map[string]struct{}, len(accounts))
		for _, account := range accounts {
			present[account.Platform] = struct{}{}
		}
		platforms = CompositeSchedulableProviderPlatforms(present)
	}
	seen := make(map[string]struct{})
	for _, concrete := range platforms {
		var visible []Account
		for _, account := range accounts {
			if account.Platform == concrete || mixedListingAccountAllowed(concrete, &account) {
				visible = append(visible, account)
			}
		}
		if len(visible) == 0 {
			continue
		}
		models := s.availableModelsFromAccounts(ctx, visible, &groupID, concrete, routes)
		if models == nil && !IsCNProvider(concrete) {
			// Defaults are a source only for an actual account that can expose
			// the model; group restrictions remain authoritative even when empty.
			for _, model := range DefaultModelIDsForPlatform(concrete) {
				for _, account := range visible {
					if account.Platform != concrete && !mixedListingModelAllowed(concrete, model) {
						continue
					}
					if s.isModelSupportedByAccountInGroup(ctx, &account, &groupID, model) {
						models = append(models, model)
						break
					}
				}
			}
		}
		for _, model := range models {
			for _, account := range visible {
				if account.Platform != concrete && !mixedListingModelAllowed(concrete, model) {
					continue
				}
				if s.isModelSupportedByAccountInGroup(ctx, &account, &groupID, model) {
					seen[model] = struct{}{}
					break
				}
			}
		}
	}
	if platform == PlatformComposite {
		activeRoutes := make([]CompositeModelRoute, 0, len(routes))
		candidates := make(map[string]struct{}, len(seen)+len(routes))
		for model := range seen {
			candidates[model] = struct{}{}
		}
		for _, route := range routes {
			if !route.Enabled {
				continue
			}
			activeRoutes = append(activeRoutes, route)
			if public := strings.TrimSpace(route.PublicModel); public != "" && normalizeCompositeRouteMatchType(route.MatchType) == CompositeRouteMatchExact {
				candidates[public] = struct{}{}
			}
		}
		available := make(map[string]struct{}, len(candidates))
		for model := range candidates {
			accountModelAvailable := false
			if _, fromAccount := seen[model]; fromAccount {
				if decision, matched := membershipCatalogCompositeDecision(groupID, accounts, model); matched {
					modelCtx := WithCompositeRouteDecision(ctx, decision)
					for i := range accounts {
						mixed := decision.TargetPlatform == PlatformAnthropic || decision.TargetPlatform == PlatformGemini
						if s.isAccountAllowedForPlatform(&accounts[i], decision.TargetPlatform, mixed) && s.isModelSupportedByAccountInGroup(modelCtx, &accounts[i], &groupID, model) {
							accountModelAvailable = true
							break
						}
					}
				}
			}
			// Ordinary discovery covers every endpoint. A selected explicit route
			// overrides account mappings, including when its target is unusable.
			for _, endpoint := range []string{CompositeRouteEndpointAny, CompositeRouteEndpointMessages,
				CompositeRouteEndpointCountTokens, CompositeRouteEndpointResponses, CompositeRouteEndpointChatCompletions,
				CompositeRouteEndpointEmbeddings, CompositeRouteEndpointImages, CompositeRouteEndpointGemini} {
				route, matched := matchCompositeRoute(activeRoutes, model, endpoint)
				if !matched {
					if accountModelAvailable {
						available[model] = struct{}{}
						break
					}
					continue
				}
				if route.TargetPlatform == PlatformTypeSafe && endpoint != CompositeRouteEndpointAny {
					continue
				}
				upstream := strings.TrimSpace(route.UpstreamModel)
				if upstream == "" {
					upstream = model
				}
				for _, account := range accounts {
					if s.isAccountAllowedForPlatform(&account, route.TargetPlatform, route.TargetPlatform == PlatformAnthropic || route.TargetPlatform == PlatformGemini) && s.isModelSupportedByAccountInGroup(ctx, &account, &groupID, upstream) {
						available[model] = struct{}{}
						break
					}
				}
				if _, ok := available[model]; ok {
					break
				}
			}
		}
		seen = available
	}
	models := make([]string, 0, len(seen))
	for model := range seen {
		models = append(models, model)
	}
	sort.Strings(models)
	return models, nil
}
