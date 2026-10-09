package service

import (
	"context"
	"strings"
)

// FilterModelsByGroupMembership is the final catalog boundary, after defaults
// and public group allowlists have expanded their candidate names. A restricted
// empty catalog must never be mistaken for permission to advertise all defaults.
// Groups without membership restrictions retain their existing listing rules.
func (s *GatewayService) FilterModelsByGroupMembership(ctx context.Context, groupID *int64, platform string, models []string) []string {
	return s.filterModelsByGroupMembership(ctx, groupID, platform, models, "")
}

// Codex catalogs describe Responses routes; endpoint-specific Composite rules
// must be resolved before intersecting account membership.
func (s *GatewayService) FilterCodexModelsByGroupMembership(ctx context.Context, groupID *int64, platform string, models []string) []string {
	return s.filterModelsByGroupMembership(ctx, groupID, platform, models, CompositeRouteEndpointResponses)
}

func (s *GatewayService) filterModelsByGroupMembership(ctx context.Context, groupID *int64, platform string, models []string, endpoint string) []string {
	if groupID == nil || len(models) == 0 {
		return models
	}
	if s == nil || s.accountRepo == nil {
		return nil
	}
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, *groupID)
	if err != nil {
		return nil
	}
	routes, err := s.membershipCatalogRoutes(ctx, groupID)
	if err != nil {
		return nil
	}
	eligible := make([]Account, 0, len(accounts))
	restricted := false
	for _, account := range accounts {
		// An explicit Composite route can target a different provider from
		// the account mapping that originally advertised its public alias.
		if len(routes) > 0 {
			restricted = restricted || len(account.GroupAllowedModels(*groupID)) > 0
		}
		if platform != "" && platform != PlatformComposite && account.Platform != platform && !mixedListingAccountAllowed(platform, &account) {
			continue
		}
		eligible = append(eligible, account)
		restricted = restricted || len(account.GroupAllowedModels(*groupID)) > 0
	}
	if !restricted {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		matchedRoutes := membershipCatalogRoutesForModel(routes, model, endpoint)
		if len(matchedRoutes) > 0 {
			allowed := false
			for _, route := range matchedRoutes {
				target := strings.TrimSpace(route.UpstreamModel)
				if target == "" {
					target = model
				}
				for i := range accounts {
					account := &accounts[i]
					mixed := route.TargetPlatform == PlatformAnthropic || route.TargetPlatform == PlatformGemini
					if s.isAccountAllowedForPlatform(account, route.TargetPlatform, mixed) && s.isModelSupportedByAccountInGroup(ctx, account, groupID, target) {
						allowed = true
						break
					}
				}
				if allowed {
					break
				}
			}
			if allowed {
				filtered = append(filtered, model)
			}
			continue
		}
		if endpoint != "" && codexCompositeRouteMatchesModel(routes, model) {
			continue
		}
		modelCtx := ctx
		var compositeDecision CompositeRouteDecision
		if platform == PlatformComposite {
			var matched bool
			compositeDecision, matched = membershipCatalogCompositeDecision(*groupID, accounts, model)
			if !matched {
				continue
			}
			modelCtx = WithCompositeRouteDecision(ctx, compositeDecision)
		}
		for i := range eligible {
			account := &eligible[i]
			if platform == PlatformComposite {
				mixed := compositeDecision.TargetPlatform == PlatformAnthropic || compositeDecision.TargetPlatform == PlatformGemini
				if !s.isAccountAllowedForPlatform(account, compositeDecision.TargetPlatform, mixed) {
					continue
				}
			}
			if platform != "" && platform != PlatformComposite && account.Platform != platform && !mixedListingModelAllowed(platform, model) {
				continue
			}
			if s.isModelSupportedByAccountInGroup(modelCtx, account, groupID, model) {
				filtered = append(filtered, model)
				break
			}
		}
	}
	return filtered
}

// Match the request router's raw exact account ownership before falling back
// to the platform detector. Synthesized provider defaults are not explicit
// claims; account-source context also excludes unmapped peers of that provider.
func membershipCatalogCompositeDecision(groupID int64, accounts []Account, model string) (CompositeRouteDecision, bool) {
	decision := CompositeRouteDecision{Matched: true, GroupID: groupID, PublicModel: model, UpstreamModel: model}
	for _, account := range accounts {
		platform := strings.TrimSpace(account.Platform)
		if !isConcreteRequestPlatform(platform) || !explicitModelMappingClaims(account, model) {
			continue
		}
		if decision.TargetPlatform != "" && decision.TargetPlatform != platform {
			return CompositeRouteDecision{}, false
		}
		decision.TargetPlatform = platform
		decision.Source = CompositeRouteSourceAccount
	}
	if decision.TargetPlatform == "" {
		var detected bool
		decision.TargetPlatform, detected = DetectModelPlatform(model)
		if !detected {
			return CompositeRouteDecision{}, false
		}
		decision.Source = CompositeRouteSourceDetector
	}
	return decision, true
}

// The repository supplies enabled routes only. No routes means account-level
// aliases keep the source policy of checking the alias at selection/discovery.
func (s *GatewayService) membershipCatalogRoutes(ctx context.Context, groupID *int64) ([]CompositeModelRoute, error) {
	if groupID == nil || s == nil || s.compositeResolver == nil || s.compositeResolver.repo == nil {
		return nil, nil
	}
	return s.compositeResolver.repo.ListByGroup(ctx, *groupID, false)
}

func membershipCatalogRoutesForModel(routes []CompositeModelRoute, model, endpoint string) []CompositeModelRoute {
	endpoints := []string{endpoint}
	if endpoint == "" {
		endpoints = nil
		seen := make(map[string]bool)
		for _, route := range routes {
			key := normalizeCompositeRouteEndpoint(route.Endpoint)
			if !seen[key] {
				endpoints = append(endpoints, key)
				seen[key] = true
			}
		}
	}
	matched := make([]CompositeModelRoute, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if route, ok := matchCompositeRoute(routes, model, endpoint); ok {
			matched = append(matched, route)
		}
	}
	return matched
}

// GeminiModelMembershipFilter snapshots the schedulable discovery pool once.
// Model resources have a models/ prefix, whereas routing membership uses IDs.
func (s *GeminiMessagesCompatService) GeminiModelMembershipFilter(ctx context.Context, groupID *int64, platform string) (func(string) bool, error) {
	accounts, err := s.listSchedulableAccountsOnce(ctx, groupID, platform, platform != PlatformGemini)
	if err != nil {
		return nil, err
	}
	restricted := false
	if groupID != nil {
		for i := range accounts {
			restricted = restricted || len(accounts[i].GroupAllowedModels(*groupID)) > 0
		}
	}
	return func(model string) bool {
		if !restricted {
			return true
		}
		model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
		for i := range accounts {
			account := &accounts[i]
			if s.isAccountValidForPlatform(account, platform, platform == PlatformGemini) &&
				s.isModelSupportedByAccount(account, model) && account.IsModelAllowedInGroup(groupID, model) {
				return true
			}
		}
		return false
	}, nil
}
