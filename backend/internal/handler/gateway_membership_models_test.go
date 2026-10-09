package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayModels_MembershipFiltersDefaultsAndPublicAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, platform           string
		mapping                  map[string]any
		membership, public, want []string
	}{
		{name: "unmapped concrete", platform: service.PlatformGemini, membership: []string{"gemini-2.5-flash"}, want: []string{"gemini-2.5-flash"}},
		{name: "restricted empty mapped", platform: service.PlatformGemini, mapping: map[string]any{"gemini-2.5-pro": "gemini-2.5-pro"}, membership: []string{"gemini-2.5-flash"}, want: []string{}},
		{name: "wildcard restricted empty fallback", platform: service.PlatformGemini, membership: []string{"no-such-model-*"}, want: []string{}},
		{name: "parent allowlist cannot revive defaults", platform: service.PlatformAnthropic, mapping: map[string]any{"custom-model": "custom-model"}, membership: []string{"custom-model"}, public: []string{"custom-model", "claude-opus-4-8"}, want: []string{"custom-model"}},
		{name: "parent wildcard cannot revive defaults", platform: service.PlatformAnthropic, membership: []string{"claude-opus-4-8"}, public: []string{"claude-*"}, want: []string{"claude-opus-4-8"}},
		{name: "wildcard mapping concrete membership", platform: service.PlatformAnthropic, mapping: map[string]any{"custom-*": "upstream"}, membership: []string{"custom-allowed"}, want: []string{"custom-allowed"}},
		{name: "passthrough concrete", platform: service.PlatformOpenAI, membership: []string{"gpt-5.4"}, want: []string{"gpt-5.4"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, codex := range []bool{false, true} {
				groupID := int64(601)
				account := service.Account{ID: 1, Platform: tc.platform, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: tc.membership}}}
				if tc.mapping != nil {
					account.Credentials = map[string]any{"model_mapping": tc.mapping}
				}
				if tc.platform == service.PlatformOpenAI {
					account.Extra = map[string]any{"openai_passthrough": true}
				}
				h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {account}}})
				group := &service.Group{ID: groupID, Platform: tc.platform, ModelAllowlist: service.GroupModelAllowlist{Enabled: len(tc.public) > 0, Models: tc.public}}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
				if codex {
					c.Request.Header.Set("User-Agent", "codex_cli_rs/0.132.0")
				}
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: group})
				if codex {
					h.CodexModels(c)
				} else {
					h.Models(c)
				}
				require.Equal(t, http.StatusOK, rec.Code)
				if codex {
					var got codexModelsResponseForTest
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
					var ids []string
					for _, m := range got.Models {
						ids = append(ids, m.Slug)
					}
					require.ElementsMatch(t, tc.want, ids, "Codex discovery must not broaden account membership")
				} else {
					var got gatewayModelsResponseForTest
					require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
					require.ElementsMatch(t, tc.want, modelIDsForTest(got.Data))
				}
			}
		})
	}
}

func TestGatewayModels_MembershipCompositeDoesNotReviveDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(602)
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {
		{ID: 1, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: []string{"no-such-model-*"}}}},
		// Neither provider contributes a concrete catalog. The Composite
		// fallback must not advertise Gemini through an Anthropic-only route.
		{ID: 2, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: []string{"gemini-*"}}}},
	}}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: groupID, Platform: service.PlatformComposite}})
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, got.Data)
}

func TestGatewayModels_MembershipUnionRetainsOtherAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(603)
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {
		{ID: 1, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"custom-model": "custom-model"}}, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: []string{"custom-model"}}}},
		{ID: 2, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, AccountGroups: []service.AccountGroup{{GroupID: groupID}}},
	}}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: groupID, Platform: service.PlatformAnthropic, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"custom-model", "claude-opus-4-8"}}}})
	h.Models(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.ElementsMatch(t, []string{"custom-model", "claude-opus-4-8"}, modelIDsForTest(got.Data))
}

func TestGeminiNativeModels_MembershipFiltersUpstreamAndFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		allowed  []string
		expected []string
	}{
		{"upstream", 200, `{"models":[{"name":"models/gemini-2.5-pro","custom":{"kept":true}},{"name":"models/gemini-2.5-flash"}],"nextPageToken":"next"}`, []string{"gemini-2.5-pro"}, []string{"models/gemini-2.5-pro"}},
		{"restricted empty", 200, `{"models":[{"name":"models/gemini-2.5-pro"}],"nextPageToken":"next"}`, []string{"gemini-none-*"}, []string{}},
		{"fallback", 403, `{"error":"insufficient authentication scopes"}`, []string{"gemini-2.5-pro"}, []string{"models/gemini-2.5-pro"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(604)
			repo := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {
				{ID: 1, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: tc.allowed}}},
			}}}}
			h := newGatewayModelsHandlerForTest(repo)
			h.geminiCompatService = service.NewGeminiMessagesCompatService(repo, nil, nil, nil, nil, nil, &geminiMixedModelsUpstream{status: tc.status, body: tc.body}, nil, &config.Config{})
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformGemini}})
			h.GeminiV1BetaListModels(c)
			require.Equal(t, 200, rec.Code)
			var got struct {
				Models []struct {
					Name string `json:"name"`
				}
				NextPageToken string `json:"nextPageToken"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			var ids []string
			for _, model := range got.Models {
				ids = append(ids, model.Name)
			}
			require.ElementsMatch(t, tc.expected, ids)
			if tc.status == 200 {
				require.Equal(t, "next", got.NextPageToken)
				require.Equal(t, "native-id", rec.Header().Get("X-Request-Id"))
			}
			if tc.name == "upstream" {
				require.Contains(t, rec.Body.String(), `"custom":{"kept":true}`)
			}
		})
	}
}

func TestGatewayModels_MembershipAntigravityStaticCatalog(t *testing.T) {
	groupID := int64(607)
	h := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{groupID: {
		{ID: 1, Platform: service.PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-2.5-flash": "gemini-2.5-flash"}}, AccountGroups: []service.AccountGroup{{GroupID: groupID, AllowedModels: []string{"gemini-2.5-flash"}}}},
	}}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/antigravity/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{ID: groupID, Platform: service.PlatformAntigravity}})
	h.AntigravityModels(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gemini-2.5-flash"}, modelIDsForTest(got.Data))
}
