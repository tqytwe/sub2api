//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func membershipPolicyAccount(groupID int64, allowed []string) Account {
	return Account{ID: 71, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 2, GroupIDs: []int64{groupID}, AccountGroups: []AccountGroup{{GroupID: groupID, AllowedModels: allowed}}}
}

func TestOpenAIMembershipPolicyRouting(t *testing.T) {
	groupID := int64(9)
	for _, tc := range []struct {
		name    string
		allowed []string
		model   string
		want    bool
	}{
		{"denied", []string{"gpt-5.4"}, "gpt-5.5", false},
		{"allowed", []string{"gpt-5.4"}, "gpt-5.4", true},
		{"wildcard", []string{"gpt-*"}, "gpt-5.5", true},
		{"empty policy", nil, "gpt-5.5", true},
		{"empty model", []string{"gpt-5.4"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := membershipPolicyAccount(groupID, tc.allowed)
			svc := &OpenAIGatewayService{}
			scheduler := &defaultOpenAIAccountScheduler{service: svc}
			ok, reason := scheduler.isAccountRequestCompatibleReason(context.Background(), &account, OpenAIAccountScheduleRequest{GroupID: &groupID, RequestedModel: tc.model})
			assert.Equal(t, tc.want, ok, "advanced scheduler: %s", reason)
			selected, _, _ := svc.selectBestAccount(context.Background(), &groupID, PlatformOpenAI, []Account{account}, tc.model, nil, false, "", false)
			require.Equal(t, tc.want, selected != nil, "legacy candidate and DB recheck")
			if !tc.want {
				require.Equal(t, "model_not_allowed_in_group", reason)
			}
		})
	}
}

func TestOpenAIMembershipPolicyPreviousResponse(t *testing.T) {
	groupID := int64(9)
	account := membershipPolicyAccount(groupID, []string{"gpt-5.4"})
	cache := &stubGatewayCache{}
	store := NewOpenAIWSStateStore(cache)
	svc := &OpenAIGatewayService{accountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}, cache: cache, cfg: newOpenAIWSV2TestConfig(), openaiWSStateStore: store}
	require.NoError(t, store.BindResponseAccount(context.Background(), groupID, "resp_membership", account.ID, time.Hour))
	id, selected, _, _ := svc.resolveAccountByPreviousResponseIDForCapability(context.Background(), &groupID, "resp_membership", "gpt-5.5", nil, "", false)
	require.Zero(t, id)
	require.Nil(t, selected, "previous response stickiness must not bypass membership policy")
}

func TestOpenAIMembershipPolicyFinalAdmissionUsesOutboundAndLatestPolicy(t *testing.T) {
	groupID := int64(9)
	for _, tc := range []struct {
		name     string
		allowed  []string
		public   string
		outbound string
		want     bool
	}{
		{"mapped outbound allowed", []string{"gpt-5.4"}, "public-model", "gpt-5.4", true},
		{"public is not membership ID", []string{"public-model"}, "public-model", "gpt-5.4", false},
		{"latest narrows membership", []string{"gpt-5.5"}, "public-model", "gpt-5.4", false},
		{"wildcard", []string{"gpt-*"}, "public-model", "gpt-5.4", true},
		{"empty unrestricted", nil, "public-model", "gpt-5.4", true},
		{"public group still enforced", []string{"gpt-*"}, "other-public-model", "gpt-5.4", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := membershipPolicyAccount(groupID, nil)
			selected.Credentials = map[string]any{"api_key": "membership-test-key"}
			selected.Groups = []*Group{{ID: groupID, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}}
			latest := selected
			latest.AccountGroups = []AccountGroup{{GroupID: groupID, AllowedModels: tc.allowed}}
			svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: &latest}, false)
			parent := &Group{ID: 44, RateMultiplier: 3, Hydrated: true, Platform: PlatformOpenAI, Status: StatusActive}
			ctx, pricingAt := WithGatewayTokenRequestPricing(context.WithValue(context.Background(), ctxkey.Group, parent))
			c := adaptiveProtocolTestContext("/v1/responses", nil)
			c.Request = c.Request.WithContext(ctx)
			key := &APIKey{GroupID: &groupID}
			c.Set("api_key", key)
			got, err := svc.admitOpenAITurnForRequest(ctx, c, &selected, tc.public, tc.outbound)
			if tc.want {
				require.NoError(t, err)
				require.NotSame(t, &latest, got)
				require.Equal(t, latest.ID, got.ID)
				require.Equal(t, latest.Credentials, got.Credentials)
				require.True(t, got.openAITurnCredentialsAdmitted)
				require.False(t, latest.openAITurnCredentialsAdmitted)
			} else {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, got)
			}
			require.Same(t, key, getAPIKeyFromContext(c))
			require.Same(t, parent, gatewayTokenRequestBillingGroupFromContext(c.Request.Context()))
			require.Equal(t, pricingAt, GatewayTokenRequestPricingAtFromContext(c.Request.Context()))
		})
	}
}

func TestOpenAIMembershipPolicyDiagnostics(t *testing.T) {
	groupID := int64(9)
	account := membershipPolicyAccount(groupID, []string{"gpt-5.4"})
	repo := &mockAccountRepoForPlatform{accounts: []Account{account}, accountsByID: map[int64]*Account{account.ID: &account}}
	svc := &OpenAIGatewayService{accountRepo: repo, cfg: &config.Config{}}
	diag := svc.DiagnoseModelAvailabilityForPlatform(context.Background(), &groupID, "gpt-5.5", PlatformOpenAI)
	require.True(t, diag.HasAccountsInPool)
	require.False(t, diag.HasModelSupport, "account mapping cannot bypass membership restriction")
}

func TestOpenAIMembershipPolicyModelDiscovery(t *testing.T) {
	group := &Group{ID: 9, Platform: PlatformOpenAI}
	account := membershipPolicyAccount(group.ID, []string{"gpt-5.4"})
	account.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-5.4": "gpt-5.4", "gpt-5.5": "gpt-5.5"}}
	require.Equal(t, []string{"gpt-5.4"}, openAIConfiguredCodexModelIDsForGroup([]Account{account}, group))
	for _, codex := range []bool{false, true} {
		body := []byte(`{"data":[{"id":"gpt-5.4"},{"id":"gpt-5.5"}]}`)
		path := "data.#.id"
		if codex {
			body = []byte(`{"models":[{"slug":"gpt-5.4"},{"slug":"gpt-5.5"}]}`)
			path = "models.#.slug"
		}
		projected, err := projectAccountModelsBody(body, &account, group, codex)
		require.NoError(t, err)
		require.Equal(t, `["gpt-5.4"]`, gjson.GetBytes(projected, path).Raw)
	}
}

func TestOpenAIMembershipPolicyHTTPRejectsFinalNarrowingBeforeSend(t *testing.T) {
	groupID := int64(9)
	selected := turnAdmissionAccount()
	selected.GroupIDs = []int64{groupID}
	selected.Groups = []*Group{{ID: groupID, Status: StatusActive}}
	latest := *selected
	repo := &turnAdmissionRepo{account: &latest}
	repo.afterRead = func(n int, account *Account) {
		if n == 2 {
			copy := *account
			copy.AccountGroups = []AccountGroup{{GroupID: groupID, AllowedModels: []string{"gpt-5.5"}}}
			repo.account = &copy
		}
	}
	svc := newTurnAdmissionGateway(repo, false)
	upstream := &httpUpstreamRecorder{err: errors.New("must not send")}
	svc.httpUpstream = upstream
	body := []byte(`{"model":"gpt-5.4","input":"hello"}`)
	c := adaptiveProtocolTestContext("/v1/responses", body)
	c.Set("api_key", &APIKey{GroupID: &groupID})
	_, err := svc.Forward(context.Background(), c, selected, body)
	require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, 2, repo.reads)
}

func TestOpenAIMembershipPolicyUnmappedAndPassthroughDiscovery(t *testing.T) {
	group := &Group{ID: 9, Platform: PlatformOpenAI}
	for _, passthrough := range []bool{false, true} {
		account := membershipPolicyAccount(group.ID, []string{"gpt-5.4"})
		account.Extra = map[string]any{"openai_passthrough": passthrough}
		for _, pinned := range []bool{false, true} {
			group.CodexModelsManifestConfig.Enabled = pinned
			response := &OpenAIModelsResponse{Body: []byte(`{"models":[{"slug":"gpt-5.4","display_name":"Keep metadata"},{"slug":"gpt-5.5"}]}`)}
			require.NoError(t, ApplyPinnedCodexModelsMapping(response, &account, group))
			require.Equal(t, `["gpt-5.4"]`, gjson.GetBytes(response.Body, "models.#.slug").Raw)
			require.Equal(t, "Keep metadata", gjson.GetBytes(response.Body, "models.0.display_name").String())
		}
	}
}

func TestOpenAIMembershipPolicyCatalogExcludesDeniedPeers(t *testing.T) {
	group := &Group{ID: 9, Platform: PlatformOpenAI}
	_, metadata, err := extractUpstreamModelCatalog([]byte(`{"models":[{"slug":"gpt-6-astra","multi_agent_reasoning_effort":"high","multi_agent_version":"v2"}]}`), false)
	require.NoError(t, err)
	account := membershipPolicyAccount(group.ID, []string{"public-astra"})
	account.Credentials = map[string]any{"base_url": "https://relay.example/v1", "model_mapping": map[string]any{"public-astra": "gpt-6-astra"}}
	account.SetUpstreamModelMetadataSnapshot(UpstreamModelMetadataSnapshot{Models: metadata})
	peer := membershipPolicyAccount(group.ID, []string{"other-model"})
	peer.ID++
	peer.Credentials = account.Credentials
	body, err := buildCodexModelsManifestForAccounts(PlatformOpenAI, []string{"public-astra"}, []Account{account, peer}, group, nil, true)
	require.NoError(t, err)
	model := decodeCodexManifestModels(t, body)[0]
	require.Equal(t, "high", model["multi_agent_reasoning_effort"])
	require.Equal(t, "v2", model["multi_agent_version"])
}

// The source checks routed aliases during selection and actual outbound IDs at
// send time. An explicit alias therefore needs both names in a restricted
// membership policy; the group's public allowlist still needs only the alias.
func TestOpenAIMembershipPolicyAccountAliasRequiresBothPolicyNames(t *testing.T) {
	groupID := int64(9)
	for _, tc := range []struct {
		name       string
		allowed    []string
		selectable bool
		admitted   bool
	}{
		{"alias only", []string{"public-alias"}, true, false},
		{"target only", []string{"gpt-5.4"}, false, true},
		{"alias and target", []string{"public-alias", "gpt-5.4"}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := membershipPolicyAccount(groupID, tc.allowed)
			account.Credentials = map[string]any{"model_mapping": map[string]any{"public-alias": "gpt-5.4"}}
			account.Groups = []*Group{{ID: groupID, Status: StatusActive, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"public-alias"}}}}
			scheduler := &defaultOpenAIAccountScheduler{}
			require.Equal(t, tc.selectable, scheduler.isAccountRequestCompatible(context.Background(), &account, OpenAIAccountScheduleRequest{GroupID: &groupID, RequestedModel: "public-alias"}))
			svc := newTurnAdmissionGateway(&turnAdmissionRepo{account: &account}, false)
			c := adaptiveProtocolTestContext("/v1/responses", nil)
			c.Set("api_key", &APIKey{GroupID: &groupID})
			_, err := svc.admitOpenAITurnForRequest(context.Background(), c, &account, "public-alias", account.GetMappedModel("public-alias"))
			require.Equal(t, tc.admitted, err == nil, "%v", err)
		})
	}
}

// Non-OpenAI accounts share the forwarding predicate, but source admission
// evaluates only their selected membership snapshot and never rereads them.
func TestOpenAIMembershipPolicyNonOpenAIFinalSnapshot(t *testing.T) {
	groupID := int64(19)
	for _, tc := range []struct {
		name                  string
		allowed               []string
		outbound              string
		withKey, simple, want bool
	}{
		{"mapped target denied", []string{"public-grok"}, "grok-4", true, false, false},
		{"mapped target allowed", []string{"grok-4"}, "grok-4", true, false, true},
		{"wildcard allowed", []string{"grok-*"}, "grok-4", true, false, true},
		{"unrestricted", nil, "grok-4", true, false, true},
		{"keyless stays unscoped", []string{"other"}, "grok-4", false, false, true},
		{"simple still checks snapshot", []string{"other"}, "grok-4", true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected := membershipPolicyAccount(groupID, tc.allowed)
			selected.Platform = PlatformGrok
			repo := &turnAdmissionRepo{err: errors.New("non-OpenAI must not use authoritative reader")}
			svc := newTurnAdmissionGateway(repo, false)
			if tc.simple {
				svc.cfg.RunMode = config.RunModeSimple
			}
			c := adaptiveProtocolTestContext("/v1/chat/completions", nil)
			if tc.withKey {
				c.Set("api_key", &APIKey{GroupID: &groupID})
			}
			got, err := svc.admitOpenAITurnForRequest(context.Background(), c, &selected, "public-grok", tc.outbound)
			if tc.want {
				require.NoError(t, err)
				require.Same(t, &selected, got)
			} else {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, got)
			}
			require.Zero(t, repo.reads, "parity adds no non-OpenAI authoritative read")
		})
	}
}

func TestOpenAIMembershipPolicySimpleModeKeepsModelRestriction(t *testing.T) {
	groupID := int64(19)
	for _, allowed := range []bool{false, true} {
		name := "denied"
		if allowed {
			name = "allowed without group binding"
		}
		t.Run(name, func(t *testing.T) {
			selected := membershipPolicyAccount(groupID, []string{"gpt-5.4"})
			// Simple mode can also select ungrouped accounts globally; absence
			// of a membership row must not introduce a group-binding veto.
			if allowed {
				selected.GroupIDs = nil
				selected.AccountGroups = nil
			}
			repo := &turnAdmissionRepo{account: &selected}
			svc := newTurnAdmissionGateway(repo, false)
			svc.cfg.RunMode = config.RunModeSimple
			c := adaptiveProtocolTestContext("/v1/responses", nil)
			c.Set("api_key", &APIKey{GroupID: &groupID})
			model := "gpt-5.5"
			if allowed {
				model = "gpt-5.4"
			}
			got, err := svc.admitOpenAITurnForRequest(context.Background(), c, &selected, "public-alias", model)
			if allowed {
				require.NoError(t, err)
				require.NotNil(t, got)
			} else {
				require.True(t, IsOpenAITurnAdmissionError(err), "%v", err)
				require.Nil(t, got)
			}
			require.Equal(t, 1, repo.reads)
		})
	}
}
