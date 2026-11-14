package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelCatalogUsesOneSnapshotAndAuthoritativeMembership(t *testing.T) {
	const groupID int64 = 27
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"gpt-5.5"}}}},
		{ID: 2, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"grok-only": "grok-only"}}},
	}}}
	svc := &GatewayService{accountRepo: repo}
	models, err := svc.GetGroupModelCatalog(context.Background(), groupID, PlatformComposite)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.5", "grok-only"}, models)
	require.EqualValues(t, 1, repo.listByGroupCalls.Load(), "platforms share one account snapshot")
	repo.byGroup[groupID] = nil
	models, err = svc.GetGroupModelCatalog(context.Background(), groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.NotNil(t, models)
	require.Empty(t, models, "no accounts is an empty catalog, never platform defaults")
	failure := errors.New("catalog read failed")
	repo.err = failure
	_, err = svc.GetGroupModelCatalog(context.Background(), groupID, PlatformOpenAI)
	require.ErrorIs(t, err, failure)
}

func TestGroupModelCatalogCompositeAliasesKeepTargetMembership(t *testing.T) {
	const groupID int64 = 28
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials:   map[string]any{"model_mapping": map[string]any{"target-model": "target-model", "blocked-target": "blocked-target"}},
			AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"target-model"}}}},
	}}}
	routes := []CompositeModelRoute{
		{ID: 1, GroupID: groupID, PublicModel: "public-alias", TargetPlatform: PlatformOpenAI, UpstreamModel: "target-model", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointResponses, Enabled: true},
		{ID: 2, GroupID: groupID, PublicModel: "blocked-alias", TargetPlatform: PlatformOpenAI, UpstreamModel: "blocked-target", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointAny, Enabled: true},
		{ID: 3, GroupID: groupID, PublicModel: "disabled-alias", TargetPlatform: PlatformOpenAI, UpstreamModel: "target-model", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointAny, Enabled: false},
	}
	svc := &GatewayService{accountRepo: repo, compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes})}
	models, err := svc.GetGroupModelCatalog(context.Background(), groupID, PlatformComposite)
	require.NoError(t, err)
	require.Contains(t, models, "public-alias")
	require.Contains(t, models, "target-model")
	require.NotContains(t, models, "blocked-target")
	require.NotContains(t, models, "blocked-alias")
	require.NotContains(t, models, "disabled-alias")
	require.EqualValues(t, 1, repo.listByGroupCalls.Load())
}

func TestGroupModelCatalogCompositeUsesRequestOwnership(t *testing.T) {
	for _, withRoutes := range []bool{false, true} {
		for _, ambiguous := range []bool{false, true} {
			t.Run(fmt.Sprintf("routes=%t/ambiguous=%t", withRoutes, ambiguous), func(t *testing.T) {
				groupID := int64(29)
				accounts := []Account{{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
					Credentials:   map[string]any{"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5"}},
					AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"other-model"}}}},
					{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}}
				if ambiguous {
					accounts[0].AccountGroups = nil
					accounts = append(accounts, Account{ID: 3, Platform: PlatformDeepseek, Type: AccountTypeAPIKey,
						Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.5": "deepseek-v4-pro"}}})
				}
				repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: accounts}}
				svc := &GatewayService{accountRepo: repo}
				if withRoutes {
					svc.compositeResolver = NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
						{ID: 1, GroupID: groupID, PublicModel: "unrelated-alias", TargetPlatform: PlatformOpenAI,
							UpstreamModel: "other-model", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointResponses, Enabled: true},
					}})
				}
				decision, matched := membershipCatalogCompositeDecision(groupID, accounts, "gpt-5.5")
				if ambiguous {
					require.False(t, matched, "the request router rejects conflicting platform ownership")
				} else {
					require.True(t, matched)
					modelCtx := WithCompositeRouteDecision(context.Background(), decision)
					for i := range accounts {
						require.False(t, svc.isModelSupportedByAccountInGroup(modelCtx, &accounts[i], &groupID, "gpt-5.5"), "unmapped peers cannot serve an account-owned alias")
					}
				}
				models, err := svc.GetGroupModelCatalog(context.Background(), groupID, PlatformComposite)
				require.NoError(t, err)
				require.NotContains(t, models, "gpt-5.5", "discovery must not advertise an uncallable model")
				require.EqualValues(t, 1, repo.listByGroupCalls.Load(), "reuse the original catalog snapshot")
			})
		}
	}
}
