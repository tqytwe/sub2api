package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewayModelsAccountRepoStub struct {
	service.AccountRepository

	byGroup map[int64][]service.Account
}

type gatewayModelsCatalogRepoStub struct {
	service.ModelCatalogRepository

	entries []service.SiteModelCatalogEntry
}

type gatewayModelsResponseForTest struct {
	Object string                    `json:"object"`
	Data   []gatewayModelItemForTest `json:"data"`
}

type gatewayModelItemForTest struct {
	ID                      string                                `json:"id"`
	Object                  string                                `json:"object"`
	Created                 int64                                 `json:"created"`
	OwnedBy                 string                                `json:"owned_by"`
	CreatedAt               string                                `json:"created_at"`
	SupportsReasoningEffort bool                                  `json:"supportsReasoningEffort"`
	ReasoningEffort         string                                `json:"reasoningEffort"`
	ReasoningEfforts        []gatewayReasoningEffortOptionForTest `json:"reasoningEfforts"`
	Modalities              []string                              `json:"modalities"`
	Platform                string                                `json:"platform"`
	Adapter                 string                                `json:"adapter"`
	CapabilityVersion       string                                `json:"capability_version"`
	ImageCapabilities       *service.ModelImageCapabilities       `json:"image_capabilities"`
	VideoCapabilities       *service.MobileVideoCapabilities      `json:"video_capabilities"`
}

type gatewayReasoningEffortOptionForTest struct {
	Value   string `json:"value"`
	Label   string `json:"label"`
	Default bool   `json:"default"`
}

func (s *gatewayModelsAccountRepoStub) ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]service.Account, error) {
	accounts, ok := s.byGroup[groupID]
	if !ok {
		return nil, nil
	}
	out := make([]service.Account, len(accounts))
	copy(out, accounts)
	return out, nil
}

func (s *gatewayModelsCatalogRepoStub) ListCatalog(_ context.Context, filter service.CatalogListFilter) ([]service.SiteModelCatalogEntry, error) {
	out := make([]service.SiteModelCatalogEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		if filter.VisibleAuth != nil && entry.VisibleAuth != *filter.VisibleAuth {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func newGatewayModelsHandlerForTest(repo service.AccountRepository) *GatewayHandler {
	return &GatewayHandler{
		gatewayService: service.NewGatewayService(
			repo,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		),
	}
}

func newGatewayModelsHandlerWithCatalogForTest(
	accountRepo service.AccountRepository,
	entries []service.SiteModelCatalogEntry,
) *GatewayHandler {
	h := newGatewayModelsHandlerForTest(accountRepo)
	h.modelCatalogService = service.NewModelCatalogService(
		&gatewayModelsCatalogRepoStub{entries: entries},
		nil, nil, nil, nil, nil, nil,
	)
	return h
}

func TestDefaultModelIDsForCompositeIncludesAntigravityDefaults(t *testing.T) {
	antigravityIDs := defaultModelIDsForPlatform(service.PlatformAntigravity)
	require.NotEmpty(t, antigravityIDs)

	compositeIDs := defaultModelIDsForPlatform(service.PlatformComposite)
	require.Contains(t, compositeIDs, antigravityIDs[0])
}

func TestGatewayModels_GeminiGroupDoesNotLeakDefaultsWithoutExplicitMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(20)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{ID: 1, Platform: service.PlatformGemini},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformGemini},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "list", got.Object)
	require.Empty(t, modelIDsForTest(got.Data))
}

func TestGatewayModels_Grok45AdvertisesReasoningEffortForGrokBuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(4409)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformGrok,
						Credentials: map[string]any{
							"model_mapping": map[string]any{"grok-4.5": "grok-4.5"},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformGrok},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data, 1)
	model := got.Data[0]
	require.Equal(t, "grok-4.5", model.ID)
	require.True(t, model.SupportsReasoningEffort)
	require.Equal(t, "high", model.ReasoningEffort)
	require.Equal(t, []gatewayReasoningEffortOptionForTest{
		{Value: "low", Label: "Low"},
		{Value: "medium", Label: "Medium"},
		{Value: "high", Label: "High", Default: true},
	}, model.ReasoningEfforts)
}

func TestGatewayModels_GeminiGroupFiltersMappedModelsByPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(21)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformAnthropic,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"claude-sonnet-4-6": "claude-sonnet-4-6",
							},
						},
					},
					{
						ID:       2,
						Platform: service.PlatformGemini,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gemini-2.5-flash": "gemini-2.5-flash",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformGemini},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gemini-2.5-flash"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_CustomModelsListDisabledKeepsOriginalModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(22)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformOpenAI,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gpt-5.5": "gpt-5.5",
								"gpt-5.4": "gpt-5.4",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: false,
				Models:  []string{"gpt-5.5"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gpt-5.4", "gpt-5.5"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_CustomModelsListFiltersAndOrdersMappedModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(23)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformOpenAI,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gpt-5.4":         "gpt-5.4",
								"gpt-5.5":         "gpt-5.5",
								"legacy-gpt-2024": "legacy-gpt-2024",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "missing-model", "gpt-5.4"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gpt-5.5", "gpt-5.4"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_CompositeCustomModelsListFiltersAcrossConcretePlatforms(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(33)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformOpenAI,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gpt-5.4": "gpt-5.4",
								"gpt-5.5": "gpt-5.5",
							},
						},
					},
					{
						ID:       2,
						Platform: service.PlatformGemini,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gemini-2.5-flash": "gemini-2.5-flash",
							},
						},
					},
					{
						ID:       3,
						Platform: service.PlatformAntigravity,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"ag-custom-model": "ag-custom-model",
							},
						},
					},
					{
						ID:       4,
						Platform: service.PlatformKimi,
						Credentials: map[string]any{
							"model_mapping": map[string]any{"kimi-custom": "kimi-upstream"},
						},
					},
					{
						ID:       5,
						Platform: service.PlatformZhipu,
						Credentials: map[string]any{
							"model_mapping": map[string]any{"glm-custom": "glm-upstream"},
						},
					},
					{
						ID:       6,
						Platform: service.PlatformDeepseek,
						Credentials: map[string]any{
							"model_mapping": map[string]any{"deepseek-custom": "deepseek-upstream"},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformComposite,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gemini-2.5-flash", "missing-model", "ag-custom-model", "gpt-5.5", "kimi-custom", "glm-custom", "deepseek-custom"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"gemini-2.5-flash", "ag-custom-model", "gpt-5.5", "kimi-custom", "glm-custom", "deepseek-custom"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_CompositeDecoratesConcreteImageAndVideoContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_267)
	price := 0.14
	imagePrice := 0.04
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {
				{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{
					"model_mapping": map[string]any{"sensenova-u1-fast": "sensenova-u1-fast"},
				}},
				{ID: 2, Platform: service.PlatformGrok, Credentials: map[string]any{
					"model_mapping": map[string]any{"grok-imagine-video": "grok-imagine-video"},
				}},
				{ID: 3, Platform: "new-provider", Credentials: map[string]any{
					"model_mapping": map[string]any{"new-provider-chat": "new-provider-chat"},
				}},
			},
		}},
		[]service.SiteModelCatalogEntry{
			{
				ModelName: "sensenova-u1-fast", Platform: service.PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1", "adapter":"sensenova", "modalities":["image"],
					"image":{"operations":["create"],"sizing_kind":"fixed",
					"supported_sizes":["1664x2496","2496x1664","1760x2368","2368x1760","1824x2272","2272x1824","2048x2048","2752x1536","1536x2752","3072x1376","1344x3136"],
					"max_reference_images":0}
				}`),
			},
			{
				ModelName: "grok-imagine-video", Platform: service.PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
					"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[8]}
				}`),
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID: groupID, Platform: service.PlatformComposite, AllowImageGeneration: true,
			ImagePrice1K:     &imagePrice,
			VideoModelPrices: map[string]map[string]float64{"grok-imagine-video": {"720p": price}},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	byID := make(map[string]gatewayModelItemForTest, len(got.Data))
	for _, model := range got.Data {
		byID[model.ID] = model
	}
	require.Contains(t, byID, "new-provider-chat")
	require.Empty(t, byID["new-provider-chat"].Modalities)

	fast := byID["sensenova-u1-fast"]
	require.Equal(t, []string{"image"}, fast.Modalities)
	require.Equal(t, "sensenova", fast.Adapter)
	require.NotNil(t, fast.ImageCapabilities)
	require.Nil(t, fast.VideoCapabilities)

	video := byID["grok-imagine-video"]
	require.Equal(t, []string{"video"}, video.Modalities)
	require.Equal(t, "grok_video", video.Adapter)
	require.Nil(t, video.ImageCapabilities)
	require.NotNil(t, video.VideoCapabilities)
	require.Equal(t, []string{"720p"}, video.VideoCapabilities.SupportedResolutions)
}

func TestGatewayModels_CompositeDoesNotInjectStaticDefaultsOutsideRuntimeMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(34)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{ID: 1, Platform: service.PlatformOpenAI},
					{ID: 2, Platform: service.PlatformGrok},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformComposite},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	ids := modelIDsForTest(got.Data)
	// Grok has a server-owned runtime mapping that the scheduler really uses;
	// it remains a valid mapped model. OpenAI's static defaults must not be
	// injected merely because there is an otherwise unmapped OpenAI account.
	require.NotContains(t, ids, "gpt-5.5")
	require.Contains(t, ids, "grok-4.3")
	require.NotContains(t, ids, "claude-sonnet-4-6")
}

// An authenticated API key must not receive a static OpenAI default either.
// Composite groups expose only concrete scheduler mappings.
func TestGatewayModels_CompositeUnmappedAccountsContributeNoDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(35)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{ID: 1, Platform: service.PlatformOpenAI},
					{ID: 2, Platform: service.PlatformKimi},
					{ID: 3, Platform: service.PlatformZhipu},
					{ID: 4, Platform: service.PlatformDeepseek},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformComposite},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	ids := modelIDsForTest(got.Data)
	require.Empty(t, ids)
	require.NotContains(t, ids, "claude-sonnet-4-6")
}

// 独立 CN 分组沿用 default 分支的 Claude 默认列表（Claude Code 客户端请求的
// 就是这些模型名并经账号 model_mapping 转换），composite 支持不得改变该回退。
func TestDefaultModelIDsForPlatform_CNProvidersKeepClaudeDefaults(t *testing.T) {
	want := make([]string, 0, len(claude.DefaultModels))
	for _, model := range claude.DefaultModels {
		want = append(want, model.ID)
	}
	for _, platform := range []string{service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek} {
		require.Equal(t, want, defaultModelIDsForPlatform(platform), "platform=%s", platform)
	}
}

func TestGatewayModels_CustomModelsListKeepsConcreteModelAllowedByWildcardMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(26)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformAnthropic,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"claude-*": "claude-sonnet-4-6",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"claude-sonnet-4-6"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"claude-sonnet-4-6"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_AnthropicCustomModelsListKeepsOnlyMappedModels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(28)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformAnthropic,
						Type:     service.AccountTypeOAuth,
					},
					{
						ID:       2,
						Platform: service.PlatformAnthropic,
						Type:     service.AccountTypeAPIKey,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"deepseek-v4-pro": "deepseek-v4-pro",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"claude-fable-5", "claude-opus-4-8", "deepseek-v4-pro"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"deepseek-v4-pro"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_AnthropicCustomModelsListDisabledKeepsMappedModelList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(29)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformAnthropic,
						Type:     service.AccountTypeOAuth,
					},
					{
						ID:       2,
						Platform: service.PlatformAnthropic,
						Type:     service.AccountTypeAPIKey,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"deepseek-v4-pro": "deepseek-v4-pro",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: false,
				Models:  []string{"claude-fable-5", "deepseek-v4-pro"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"deepseek-v4-pro"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_AnthropicCustomModelsListDoesNotExposeOAuthDefaultsWithoutMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(30)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformAnthropic,
						Type:     service.AccountTypeOAuth,
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"claude-opus-4-6-thinking", "claude-sonnet-4-5"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, modelIDsForTest(got.Data))
}

func TestGatewayModels_CustomModelsListCanReturnEmptyWhenSelectionsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(24)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{
						ID:       1,
						Platform: service.PlatformOpenAI,
						Credentials: map[string]any{
							"model_mapping": map[string]any{
								"gpt-5.4": "gpt-5.4",
							},
						},
					},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, modelIDsForTest(got.Data))
}

func TestGatewayModels_CustomModelsListDoesNotSelectDefaultsWithoutMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(25)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{ID: 1, Platform: service.PlatformOpenAI},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "legacy-gpt-2024", "gpt-5.4"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, modelIDsForTest(got.Data))
}

func TestGatewayModels_OpenAICustomModelsListReturnsEmptyWithoutMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(27)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{
			byGroup: map[int64][]service.Account{
				groupID: {
					{ID: 1, Platform: service.PlatformOpenAI},
				},
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "gpt-5.4"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, modelIDsForTest(got.Data))
}

func TestGatewayModels_OpenAIGroupAdvertisesSenseNovaOnlyWhenMapped(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name             string
		mapping          map[string]any
		modelsListConfig service.GroupModelsListConfig
		want             []string
	}{
		{
			name: "mapped image models are advertised",
			mapping: map[string]any{
				"sensenova-u1.5-lite": "sensenova-u1.5-lite",
				"sensenova-u1-fast":   "sensenova-u1-fast",
			},
			want: []string{"sensenova-u1-fast", "sensenova-u1.5-lite"},
		},
		{
			name: "group allowlist narrows mapped image models",
			mapping: map[string]any{
				"sensenova-u1.5-lite": "sensenova-u1.5-lite",
				"sensenova-u1-fast":   "sensenova-u1-fast",
			},
			modelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"sensenova-u1-fast"},
			},
			want: []string{"sensenova-u1-fast"},
		},
		{
			name: "unmapped image models are not leaked",
			mapping: map[string]any{
				"gpt-image-2": "gpt-image-2",
			},
			want: []string{"gpt-image-2"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(2_608_26)
			h := newGatewayModelsHandlerForTest(
				&gatewayModelsAccountRepoStub{
					byGroup: map[int64][]service.Account{
						groupID: {{
							ID:       1,
							Platform: service.PlatformOpenAI,
							Credentials: map[string]any{
								"model_mapping": tt.mapping,
							},
						}},
					},
				},
			)

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
				Group: &service.Group{
					ID:               groupID,
					Platform:         service.PlatformOpenAI,
					ModelsListConfig: tt.modelsListConfig,
				},
			})

			h.Models(c)

			require.Equal(t, http.StatusOK, rec.Code)
			var got gatewayModelsResponseForTest
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.ElementsMatch(t, tt.want, modelIDsForTest(got.Data))
		})
	}
}

func TestGatewayModels_EnrichesOnlyMappedAndGroupScopedCatalogMediaContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_260)
	imagePrice := 0.04
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {{
				ID:       1,
				Platform: service.PlatformOpenAI,
				Credentials: map[string]any{
					"model_mapping": map[string]any{
						"sensenova-u1-fast": "sensenova-u1-fast",
						"plain-chat-model":  "plain-chat-model",
					},
				},
			}},
		}},
		[]service.SiteModelCatalogEntry{
			{
				ModelName:   "sensenova-u1-fast",
				Platform:    service.PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"sensenova",
					"modalities":["image"],
					"image":{
						"operations":["create"],
						"supported_sizes":["2048x2048"]
					}
				}`),
			},
			{
				ModelName:   "unmapped-video-model",
				Platform:    service.PlatformOpenAI,
				VisibleAuth: true,
				GroupIDs:    []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"2026-08-26.1",
					"adapter":"grok-video",
					"modalities":["video"],
					"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
				}`),
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:                   groupID,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
			ImagePrice1K:         &imagePrice,
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotContains(t, modelIDsForTest(got.Data), "unmapped-video-model")

	byID := make(map[string]gatewayModelItemForTest, len(got.Data))
	for _, model := range got.Data {
		byID[model.ID] = model
	}
	fast := byID["sensenova-u1-fast"]
	require.Equal(t, []string{"image"}, fast.Modalities)
	require.Equal(t, "sensenova", fast.Adapter)
	require.Equal(t, "2026-08-26.1", fast.CapabilityVersion)
	require.NotNil(t, fast.ImageCapabilities)
	require.Equal(t, []string{"create"}, fast.ImageCapabilities.Operations)
	require.Empty(t, fast.ImageCapabilities.SupportedRatios)
	require.Empty(t, fast.ImageCapabilities.SupportedFormats)
	require.Nil(t, fast.VideoCapabilities)
	var rawPayload struct {
		Data []struct {
			ID                string          `json:"id"`
			ImageCapabilities json.RawMessage `json:"image_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rawPayload))
	foundImageContract := false
	for _, item := range rawPayload.Data {
		if item.ID != "sensenova-u1-fast" {
			continue
		}
		foundImageContract = true
		var imageFields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(item.ImageCapabilities, &imageFields))
		require.NotContains(t, imageFields, "supported_ratios")
		require.NotContains(t, imageFields, "supported_formats")
		require.NotContains(t, imageFields, "supported_aspect_ratios")
		require.NotContains(t, imageFields, "supported_output_formats")
		break
	}
	require.True(t, foundImageContract)

	plain := byID["plain-chat-model"]
	require.Empty(t, plain.Modalities)
	require.Empty(t, plain.Adapter)
	require.Nil(t, plain.ImageCapabilities)
	require.Nil(t, plain.VideoCapabilities)
}

func TestGatewayModels_DoesNotExposeCatalogMediaContractWithoutMappedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_261)
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {{ID: 1, Platform: service.PlatformOpenAI}},
		}},
		[]service.SiteModelCatalogEntry{{
			ModelName:   "gpt-image-2",
			Platform:    service.PlatformOpenAI,
			VisibleAuth: true,
			GroupIDs:    []int64{groupID},
			MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1",
				"adapter":"openai_images",
				"modalities":["image"],
				"image":{"operations":["create","edit"]}
			}`),
		}},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-image-2"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Empty(t, got.Data)
}

func TestGatewayModels_CompositeCustomModelsListOnlySelectsRuntimeMappedModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalMappingOptions := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(originalMappingOptions) })
	// This case verifies an empty OpenAI mapping. Other handler tests enable
	// Grok's optional cross-client aliases, which would legitimately make
	// gpt-* selectable and turn this isolation check into an order-dependent one.
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{})

	groupID := int64(2_608_267)
	h := newGatewayModelsHandlerForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {
				{ID: 1, Platform: service.PlatformOpenAI},
				{ID: 2, Platform: service.PlatformGrok},
			},
		}},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformComposite,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "grok-4.3"},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, []string{"grok-4.3"}, modelIDsForTest(got.Data))
}

func TestGatewayModels_DoesNotDecorateVideoWithoutExecutableGroupBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_262)
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {{
				ID: 1, Platform: service.PlatformGrok,
				Credentials: map[string]any{"model_mapping": map[string]any{"grok-imagine-video": "grok-imagine-video"}},
			}},
		}},
		[]service.SiteModelCatalogEntry{{
			ModelName: "grok-imagine-video", Platform: service.PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID},
			MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
				"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
			}`),
		}},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformGrok, AllowImageGeneration: true},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data, 1)
	require.Equal(t, "grok-imagine-video", got.Data[0].ID)
	require.Empty(t, got.Data[0].Modalities)
	require.Nil(t, got.Data[0].VideoCapabilities)
}

func TestGatewayModels_ProjectsVideoCapabilitiesToMobileTransportSchema(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_265)
	price := 0.14
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {{
				ID: 1, Platform: service.PlatformGrok,
				Credentials: map[string]any{"model_mapping": map[string]any{"grok-imagine-video": "grok-imagine-video"}},
			}},
		}},
		[]service.SiteModelCatalogEntry{{
			ModelName: "grok-imagine-video", Platform: service.PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID},
			MediaCapabilities: json.RawMessage(`{
				"version":"2026-08-26.1", "adapter":"grok_video", "modalities":["video"],
				"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
			}`),
		}},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{ID: groupID, Platform: service.PlatformGrok, AllowImageGeneration: true, VideoPrice720P: &price},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var payload struct {
		Data []struct {
			ID                string          `json:"id"`
			VideoCapabilities json.RawMessage `json:"video_capabilities"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.Equal(t, "grok-imagine-video", payload.Data[0].ID)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload.Data[0].VideoCapabilities, &fields))
	require.Contains(t, fields, "supported_resolutions")
	require.Contains(t, fields, "supported_ratios")
	require.Contains(t, fields, "supported_durations")
	require.NotContains(t, fields, "supported_aspect_ratios")
	require.NotContains(t, fields, "durations_seconds")
}

func TestGatewayModels_CompositeDuplicateVideoUsesMobileResolverContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(2_608_271)
	price := 0.14
	h := newGatewayModelsHandlerWithCatalogForTest(
		&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
			groupID: {
				{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{
					"model_mapping": map[string]any{"shared-video": "shared-video"},
				}},
				{ID: 2, Platform: service.PlatformGrok, Credentials: map[string]any{
					"model_mapping": map[string]any{"shared-video": "shared-video"},
				}},
			},
		}},
		[]service.SiteModelCatalogEntry{
			{
				ModelName: "shared-video", Platform: service.PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"openai-v1", "adapter":"agnes_video", "modalities":["video"],
					"video":{"operations":["generate"],"supported_resolutions":["480p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[3]}
				}`),
			},
			{
				ModelName: "shared-video", Platform: service.PlatformGrok, VisibleAuth: true, GroupIDs: []int64{groupID},
				MediaCapabilities: json.RawMessage(`{
					"version":"grok-v1", "adapter":"grok_video", "modalities":["video"],
					"video":{"operations":["generate"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[8]}
				}`),
			},
		},
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		Group: &service.Group{
			ID: groupID, Platform: service.PlatformComposite,
			VideoModelPrices: map[string]map[string]float64{
				"shared-video": {"480p": price, "720p": price},
			},
		},
	})

	h.Models(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var got gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Data, 1)
	model := got.Data[0]
	// Both accounts are mapped and schedulable. The shared resolver selects the
	// Grok declaration deterministically, so adapter/version/limits must come
	// from that same row instead of the first OpenAI list entry.
	require.Equal(t, "shared-video", model.ID)
	require.Equal(t, []string{"video"}, model.Modalities)
	require.Equal(t, service.PlatformGrok, model.Platform)
	require.Equal(t, service.MobileVideoAdapterGrok, model.Adapter)
	require.Equal(t, service.MobileVideoCapabilitiesVersion, model.CapabilityVersion)
	require.NotNil(t, model.VideoCapabilities)
	require.Equal(t, []string{"720p"}, model.VideoCapabilities.SupportedResolutions)
	require.Equal(t, []string{"16:9"}, model.VideoCapabilities.SupportedRatios)
	require.Equal(t, []int{8}, model.VideoCapabilities.SupportedDurations)
}

func modelIDsForTest(models []gatewayModelItemForTest) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}
