//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// groupAllowedModelsFixture 构造走负载感知路径的 GatewayService：分组 10 下有两个账号，
// 优先级更高的账号 1 在本分组只允许 claude-sonnet-4-6，账号 2 不限制。
type groupAllowedModelsFixture struct {
	svc     *GatewayService
	ctx     context.Context
	groupID int64
	cache   *mockGatewayCacheForPlatform
	repo    *mockAccountRepoForPlatform
}

func newGroupAllowedModelsFixture(t *testing.T, sessionBindings map[string]int64, routing map[string][]int64) *groupAllowedModelsFixture {
	t.Helper()

	groupID := int64(10)
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{
				ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Priority: 1,
				Status: StatusActive, Schedulable: true, Concurrency: 5,
				AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"claude-sonnet-4-6"}}},
				GroupIDs:      []int64{groupID},
			},
			{
				ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Priority: 2,
				Status: StatusActive, Schedulable: true, Concurrency: 5,
				AccountGroups: []AccountGroup{{GroupID: groupID}},
				GroupIDs:      []int64{groupID},
			},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}

	group := &Group{
		ID:                  groupID,
		Platform:            PlatformAnthropic,
		Status:              StatusActive,
		Hydrated:            true,
		ModelRoutingEnabled: len(routing) > 0,
		ModelRouting:        routing,
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cache := &mockGatewayCacheForPlatform{sessionBindings: sessionBindings}

	svc := &GatewayService{
		accountRepo:        repo,
		groupRepo:          &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		cache:              cache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{}),
	}
	return &groupAllowedModelsFixture{
		svc:     svc,
		ctx:     context.WithValue(context.Background(), ctxkey.Group, group),
		groupID: groupID,
		cache:   cache,
		repo:    repo,
	}
}

func TestSelectAccountWithLoadAwareness_SkipsAccountRestrictedInGroup(t *testing.T) {
	t.Parallel()

	f := newGroupAllowedModelsFixture(t, nil, nil)
	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "", "claude-opus-4-8", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Account.ID, "模型不在账号的分组清单里时，优先级更高也要跳过")

	result, err = f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "", "claude-sonnet-4-6", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Account.ID, "清单内的模型照常按优先级选中")
}

func TestSelectAccountWithLoadAwareness_GroupRestrictionIgnoresStickyAccount(t *testing.T) {
	t.Parallel()

	f := newGroupAllowedModelsFixture(t, map[string]int64{"sticky": 1}, nil)
	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "sticky", "claude-opus-4-8", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Account.ID, "粘性账号在本分组不允许该模型时不得沿用")
	require.Equal(t, int64(2), f.cache.sessionBindings["sticky"], "粘性会话应重新绑定到允许的账号")
}

func TestSelectAccountWithLoadAwareness_GroupRestrictionFiltersRoutedAccounts(t *testing.T) {
	t.Parallel()

	f := newGroupAllowedModelsFixture(t, nil, map[string][]int64{"claude-opus-4-8": {1}})
	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "", "claude-opus-4-8", nil, "", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Account.ID, "模型路由指向的账号被分组限制时应回退到允许的账号")
}

func TestSelectAccountWithLoadAwareness_RejectsWhenGroupAllowsNoAccount(t *testing.T) {
	t.Parallel()

	f := newGroupAllowedModelsFixture(t, nil, nil)
	f.repo.accounts[1].AccountGroups[0].AllowedModels = []string{"claude-sonnet-4-6"}

	result, err := f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, "", "claude-opus-4-8", nil, "", 0)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, result)
}

func TestSelectAccountForModelWithExclusions_SkipsAccountRestrictedInGroup(t *testing.T) {
	t.Parallel()

	f := newGroupAllowedModelsFixture(t, nil, nil)
	account, err := f.svc.SelectAccountForModelWithExclusions(f.ctx, &f.groupID, "", "claude-opus-4-8", nil)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(2), account.ID)
}

func TestGetAvailableModels_AppliesGroupAllowedModels(t *testing.T) {
	groupID := int64(40)
	repo := &modelsListAccountRepoStub{
		byGroup: map[int64][]Account{
			groupID: {
				{
					ID:       1,
					Platform: PlatformAnthropic,
					Credentials: map[string]any{"model_mapping": map[string]any{
						"claude-opus-4-8":   "claude-opus-4-8",
						"claude-sonnet-4-6": "claude-sonnet-4-6",
					}},
					AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"claude-sonnet-4-6"}}},
				},
				{
					// 没有映射（支持全部模型），但在本分组只允许一个模型
					ID:            2,
					Platform:      PlatformAnthropic,
					AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"claude-haiku-4-5", "claude-3-*"}}},
				},
			},
		},
	}
	svc := &GatewayService{accountRepo: repo}

	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformAnthropic)
	require.Equal(t, []string{"claude-haiku-4-5", "claude-sonnet-4-6"}, models, "只公布分组允许的模型，通配项不展开")
}

func TestGatewayMembershipModels_AllSelectionPaths(t *testing.T) {
	for _, mode := range []string{"single", "mixed", "load"} {
		for _, sticky := range []bool{false, true} {
			for _, routed := range []bool{false, true} {
				name := mode
				if sticky {
					name += "/sticky"
				}
				if routed {
					name += "/routed"
				}
				t.Run(name, func(t *testing.T) {
					var bindings map[string]int64
					session := ""
					if sticky {
						bindings = map[string]int64{"sticky": 1}
						session = "sticky"
					}
					var routing map[string][]int64
					if routed {
						routing = map[string][]int64{"claude-opus-4-8": {1, 2}}
					}
					f := newGroupAllowedModelsFixture(t, bindings, routing)
					var selected *Account
					var err error
					switch mode {
					case "single":
						selected, err = f.svc.selectAccountForModelWithPlatform(f.ctx, &f.groupID, session, "claude-opus-4-8", nil, PlatformAnthropic)
					case "mixed":
						selected, err = f.svc.selectAccountWithMixedScheduling(f.ctx, &f.groupID, session, "claude-opus-4-8", nil, PlatformAnthropic)
					case "load":
						var result *AccountSelectionResult
						result, err = f.svc.SelectAccountWithLoadAwareness(f.ctx, &f.groupID, session, "claude-opus-4-8", nil, "", 0)
						if result != nil {
							selected = result.Account
							if result.ReleaseFunc != nil {
								defer result.ReleaseFunc()
							}
						}
					}
					require.NoError(t, err)
					require.NotNil(t, selected)
					require.Equal(t, int64(2), selected.ID, "membership must apply before every sticky, routed, and fallback selection")
				})
			}
		}
	}
}

func TestGatewayMembershipModels_Diagnosis(t *testing.T) {
	f := newGroupAllowedModelsFixture(t, nil, nil)
	f.repo.accounts = f.repo.accounts[:1]
	diag := f.svc.DiagnoseModelAvailabilityForPlatform(f.ctx, &f.groupID, "claude-opus-4-8", PlatformAnthropic)
	require.True(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport)
	diag = f.svc.DiagnoseModelAvailabilityForPlatform(f.ctx, &f.groupID, "claude-sonnet-4-6", PlatformAnthropic)
	require.True(t, diag.HasModelSupport)
}

func TestGeminiMembershipModels_SelectionAndSticky(t *testing.T) {
	for _, sticky := range []bool{false, true} {
		name := "normal"
		if sticky {
			name = "sticky"
		}
		t.Run(name, func(t *testing.T) {
			f := newGroupAllowedModelsFixture(t, nil, nil)
			for i := range f.repo.accounts {
				f.repo.accounts[i].Platform = PlatformGemini
			}
			f.repo.accounts[0].AccountGroups[0].AllowedModels = []string{"gemini-2.5-flash"}
			group := &Group{ID: f.groupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true}
			svc := &GeminiMessagesCompatService{accountRepo: f.repo, groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{f.groupID: group}}, cache: f.cache, cfg: testConfig()}
			session := ""
			if sticky {
				session = "sticky"
				f.cache.sessionBindings = map[string]int64{"gemini:sticky": 1}
			}
			selected, err := svc.SelectAccountForModel(context.Background(), &f.groupID, session, "gemini-2.5-pro")
			require.NoError(t, err)
			require.Equal(t, int64(2), selected.ID)
			selected, err = svc.SelectAccountForModel(context.Background(), &f.groupID, "", "gemini-2.5-flash")
			require.NoError(t, err)
			require.Equal(t, int64(1), selected.ID)
		})
	}
}

func TestGetAvailableModels_MembershipConstrainsPassthroughDefaults(t *testing.T) {
	groupID := int64(41)
	repo := &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Credentials:   map[string]any{"model_mapping": map[string]any{"custom-model": "upstream-model"}},
			AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"custom-model"}}}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Extra:         map[string]any{"openai_passthrough": true},
			AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"custom-only"}}}},
	}}}
	svc := &GatewayService{accountRepo: repo}
	got := svc.GetAvailableModels(context.Background(), &groupID, PlatformOpenAI)
	require.ElementsMatch(t, []string{"custom-model", "custom-only"}, got, "restricted passthrough must not supplement all built-in models")
}

func TestGatewayMembershipModels_CompositeUsesRoutedModel(t *testing.T) {
	for _, loadAware := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "load"}[loadAware], func(t *testing.T) {
			f := newGroupAllowedModelsFixture(t, nil, nil)
			group := f.svc.groupRepo.(*mockGroupRepoForGateway).groups[f.groupID]
			group.Platform = PlatformComposite
			f.repo.accounts[0].AccountGroups[0].AllowedModels = []string{"public-alias"}
			f.repo.accounts[1].AccountGroups[0].AllowedModels = []string{"claude-opus-4-8"}
			ctx := WithCompositeRouteDecision(f.ctx, CompositeRouteDecision{Matched: true, Source: CompositeRouteSourceExplicit, GroupID: f.groupID, PublicModel: "public-alias", TargetPlatform: PlatformAnthropic, UpstreamModel: "claude-opus-4-8", Endpoint: CompositeRouteEndpointAny})
			var account *Account
			var err error
			if loadAware {
				var result *AccountSelectionResult
				result, err = f.svc.SelectAccountWithLoadAwareness(ctx, &f.groupID, "", "claude-opus-4-8", nil, "", 0)
				if result != nil {
					account = result.Account
					if result.ReleaseFunc != nil {
						defer result.ReleaseFunc()
					}
				}
			} else {
				account, err = f.svc.SelectAccountForModelWithExclusions(ctx, &f.groupID, "", "public-alias", nil)
			}
			require.NoError(t, err)
			require.NotNil(t, account)
			require.Equal(t, int64(2), account.ID, "membership restrictions must follow the routed model without reusing public admission names")
		})
	}
}

func TestGatewayMembershipModels_DetailedFailureDiagnosis(t *testing.T) {
	f := newGroupAllowedModelsFixture(t, nil, nil)
	stats := f.svc.logDetailedSelectionFailure(f.ctx, &f.groupID, "", "claude-opus-4-8", PlatformAnthropic, f.repo.accounts[:1], nil, false)
	require.Equal(t, 1, stats.ModelUnsupported)
	require.Zero(t, stats.Eligible)
}

func TestGatewayMembershipModels_CompositeDiscoveryUsesRoutedModel(t *testing.T) {
	groupID := int64(605)
	routes := []CompositeModelRoute{{ID: 1, GroupID: groupID, PublicModel: "vision-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGrok, UpstreamModel: "grok-4.5", Endpoint: CompositeRouteEndpointResponses, Enabled: true}}
	for _, tc := range []struct {
		name    string
		allowed []string
		want    []string
	}{
		{"routed target allowed", []string{"grok-4.5"}, []string{"vision-alias"}},
		{"public alias does not grant routed target", []string{"vision-alias"}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: tc.allowed}}}
			svc := &GatewayService{accountRepo: &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {account}}}, compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes})}
			require.ElementsMatch(t, tc.want, svc.FilterModelsByGroupMembership(context.Background(), &groupID, PlatformComposite, []string{"vision-alias"}))
		})
	}
}

func TestGatewayMembershipModels_CompositeCapabilityUsesRoutedModel(t *testing.T) {
	groupID := int64(606)
	routes := []CompositeModelRoute{{ID: 1, GroupID: groupID, PublicModel: "vision-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGrok, UpstreamModel: "grok-4.5", Endpoint: CompositeRouteEndpointResponses, Enabled: true}}
	account := Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeOAuth, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"grok-4.5"}}}}
	body, err := buildCodexModelsManifestForAccounts(PlatformComposite, []string{"vision-alias"}, []Account{account}, &Group{ID: groupID, Platform: PlatformComposite}, routes, true)
	require.NoError(t, err)
	models := decodeCodexManifestModels(t, body)
	require.Len(t, models, 1)
	require.Equal(t, []any{"text", "image"}, models[0]["input_modalities"])
}

func TestGatewayMembershipModels_CompositeMappingKeepsRoutedPublicAlias(t *testing.T) {
	groupID := int64(608)
	account := Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"vision-alias": "grok-4.5", "grok-4.5": "grok-4.5"}}, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"grok-4.5"}}}}
	routes := []CompositeModelRoute{{ID: 1, GroupID: groupID, PublicModel: "vision-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGrok, UpstreamModel: "grok-4.5", Endpoint: CompositeRouteEndpointResponses, Enabled: true}}
	svc := &GatewayService{accountRepo: &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: {account}}}, compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: routes})}
	models := svc.GetAvailableModels(context.Background(), &groupID, PlatformGrok)
	models = svc.FilterModelsByGroupMembership(context.Background(), &groupID, PlatformGrok, models)
	require.ElementsMatch(t, []string{"vision-alias", "grok-4.5"}, models)
}

func TestGatewayMembershipModels_CompositeDiscoveryIgnoresUnrelatedUnmappedAccount(t *testing.T) {
	groupID := int64(609)
	accounts := []Account{
		{ID: 1, Platform: PlatformGemini, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: []string{"no-such-model-*"}}}},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}
	svc := &GatewayService{accountRepo: &modelsListAccountRepoStub{byGroup: map[int64][]Account{groupID: accounts}}}
	require.Empty(t, svc.FilterModelsByGroupMembership(context.Background(), &groupID, PlatformComposite, []string{"gemini-2.5-pro"}), "an unrestricted different provider must not revive the restricted model")
}
