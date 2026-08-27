package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelCatalogGatewayContractsNeverAddCatalogModels(t *testing.T) {
	validImage := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image"],
		"image":{"operations":["create"],"supported_sizes":["2048x2048"]}
	}`)
	invalidImage := json.RawMessage(`{
		"version":"v1",
		"adapter":"not-a-real-adapter",
		"modalities":["image"],
		"image":{"operations":["create"]}
	}`)
	visible := true
	imagePrice := 0.04
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{
		{
			ModelName:         "sensenova-u1-fast",
			Platform:          PlatformOpenAI,
			VisibleAuth:       true,
			GroupIDs:          []int64{17},
			MediaCapabilities: validImage,
		},
		{
			ModelName:   "catalog-chat-only",
			Platform:    PlatformOpenAI,
			VisibleAuth: true,
			GroupIDs:    []int64{17},
		},
		{
			ModelName:         "wrong-adapter",
			Platform:          PlatformOpenAI,
			VisibleAuth:       true,
			GroupIDs:          []int64{17},
			MediaCapabilities: invalidImage,
		},
		{
			ModelName:   "other-group-model",
			Platform:    PlatformOpenAI,
			VisibleAuth: true,
			GroupIDs:    []int64{18},
		},
	}}, nil, nil, nil, nil, nil, nil)

	contracts, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 17, Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice2K: &imagePrice},
		[]string{"mapped-chat", "sensenova-u1-fast", "wrong-adapter", "wildcard-*"},
	)
	require.NoError(t, err)
	// A visible catalog row must never make a model appear without an account
	// mapping. It only decorates an ID the scheduler already exposed.
	require.Equal(t, []string{"mapped-chat", "sensenova-u1-fast", "wrong-adapter"}, gatewayContractIDs(contracts))
	require.Equal(t, []string{"image"}, contracts[1].Modalities)
	require.NotNil(t, contracts[1].ImageCapabilities)
	// An explicitly reviewed but unsupported declaration stays a bare model in
	// the OpenAI-compatible list and cannot leak a stale media capability.
	require.Empty(t, contracts[2].Modalities)
	require.Nil(t, contracts[2].ImageCapabilities)

	// Catalog lookup must retain the visible-auth filter. This guards against a
	// caller accidentally treating a hidden admin draft as a passthrough model.
	entries, err := svc.ListCatalog(context.Background(), CatalogListFilter{VisibleAuth: &visible})
	require.NoError(t, err)
	require.Len(t, entries, 4)
}

func TestModelCatalogMediaContractRejectsUndeclaredAndMismatchedAdapters(t *testing.T) {
	validImage := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image"],
		"image":{"operations":["create"]}
	}`)
	badAdapter := json.RawMessage(`{
		"version":"v1",
		"adapter":"agnes",
		"modalities":["image"],
		"image":{"operations":["create"]}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{
		{ModelName: "sensenova-u1-fast", Platform: PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{7}, MediaCapabilities: validImage},
		{ModelName: "sensenova-u1.5-lite", Platform: PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{7}, MediaCapabilities: badAdapter},
		{ModelName: "legacy-chat", Platform: PlatformOpenAI, VisibleAuth: true, GroupIDs: []int64{7}},
	}}, nil, nil, nil, nil, nil, nil)

	imagePrice := 0.04
	group := Group{ID: 7, Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice2K: &imagePrice}
	contract, err := svc.ResolveModelMediaContract(context.Background(), group, "sensenova-u1-fast")
	require.NoError(t, err)
	require.NotNil(t, contract)
	require.True(t, contract.Supports("image", "create"))
	require.False(t, contract.Supports("image", "edit"))

	_, err = svc.ResolveModelMediaContract(context.Background(), group, "legacy-chat")
	require.ErrorIs(t, err, ErrModelMediaCapabilityNotDeclared)

	_, err = svc.ResolveModelMediaContract(context.Background(), group, "sensenova-u1.5-lite")
	require.ErrorIs(t, err, ErrModelMediaAdapterUnsupported)
}

func TestModelCatalogVideoContractRequiresPriceButNotImagePermission(t *testing.T) {
	video := json.RawMessage(`{
		"version":"v1",
		"adapter":"grok_video",
		"modalities":["video"],
		"video":{
			"operations":["generate"],
			"supported_resolutions":["720p"],
			"supported_aspect_ratios":["16:9"],
			"durations_seconds":[6]
		}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: "grok-imagine-video", Platform: PlatformGrok, VisibleAuth: true,
		GroupIDs: []int64{9}, MediaCapabilities: video,
	}}}, nil, nil, nil, nil, nil, nil)

	withoutPrice, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 9, Platform: PlatformGrok, AllowImageGeneration: false},
		[]string{"grok-imagine-video"},
	)
	require.NoError(t, err)
	require.Len(t, withoutPrice, 1)
	require.Empty(t, withoutPrice[0].Modalities)

	price := 0.14
	withPriceButNoImagePermission, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{
			ID: 9, Platform: PlatformGrok, AllowImageGeneration: false,
			VideoPrice720P: &price,
		},
		[]string{"grok-imagine-video"},
	)
	require.NoError(t, err)
	require.Len(t, withPriceButNoImagePermission, 1)
	require.True(t, withPriceButNoImagePermission[0].Supports("video", "generate"))
}

func TestModelCatalogImageContractProjectsStorageFieldsToStableTransport(t *testing.T) {
	declaration := json.RawMessage(`{
		"version":"v1",
		"adapter":"gemini_images",
		"modalities":["image"],
		"image":{
			"operations":["create","edit"],
			"sizing_kind":"aspect_resolution",
			"supported_sizes":["1024x1024"],
			"supported_aspect_ratios":["1:1"],
			"supported_output_formats":["png"],
			"max_reference_images":1
		}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: "gemini-2.5-flash-image", Platform: PlatformGemini, VisibleAuth: true,
		GroupIDs: []int64{27}, MediaCapabilities: declaration,
	}}}, nil, nil, nil, nil, nil, nil)

	imagePrice := 0.04
	contracts, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 27, Platform: PlatformGemini, AllowImageGeneration: true, ImagePrice2K: &imagePrice},
		[]string{"gemini-2.5-flash-image"},
	)
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.NotNil(t, contracts[0].ImageCapabilities)
	require.Equal(t, []string{"1:1"}, contracts[0].ImageCapabilities.SupportedRatios)
	require.Equal(t, []string{"png"}, contracts[0].ImageCapabilities.SupportedFormats)

	encoded, err := json.Marshal(contracts[0].ImageCapabilities)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Contains(t, fields, "operations")
	require.Contains(t, fields, "supported_sizes")
	require.Contains(t, fields, "supported_ratios")
	require.Contains(t, fields, "supported_formats")
	require.NotContains(t, fields, "supported_aspect_ratios")
	require.NotContains(t, fields, "supported_output_formats")
}

func TestModelCatalogMediaContractDoesNotProjectForgedExactAdapterCapabilities(t *testing.T) {
	forged := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image"],
		"image":{"operations":["create","edit"],"max_reference_images":8}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: "sensenova-u1-fast", Platform: PlatformOpenAI, VisibleAuth: true,
		GroupIDs: []int64{38}, MediaCapabilities: forged,
	}}}, nil, nil, nil, nil, nil, nil)

	contracts, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 38, Platform: PlatformOpenAI, AllowImageGeneration: true},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.Empty(t, contracts[0].Modalities)
	require.Nil(t, contracts[0].ImageCapabilities)
}

func TestCatalogImageContractRequiresImageGenerationPermission(t *testing.T) {
	declaration := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image"],
		"image":{"operations":["create"]}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: "sensenova-u1-fast", Platform: PlatformOpenAI, VisibleAuth: true,
		GroupIDs: []int64{45}, MediaCapabilities: declaration,
	}}}, nil, nil, nil, nil, nil, nil)

	disabled, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 45, Platform: PlatformOpenAI, AllowImageGeneration: false},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, disabled, 1)
	require.Empty(t, disabled[0].Modalities)

	imagePrice := 0.04
	enabled, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 45, Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice2K: &imagePrice},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	require.True(t, enabled[0].Supports("image", "create"))
}

func TestCatalogImageContractRequiresConfiguredBilling(t *testing.T) {
	declaration := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image"],
		"image":{"operations":["create"]}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: "sensenova-u1-fast", Platform: PlatformOpenAI, VisibleAuth: true,
		GroupIDs: []int64{46}, MediaCapabilities: declaration,
	}}}, nil, nil, nil, nil, nil, nil)

	withoutPrice, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{ID: 46, Platform: PlatformOpenAI, AllowImageGeneration: true},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, withoutPrice, 1)
	require.Empty(t, withoutPrice[0].Modalities)

	price := 0.04
	withConfiguredPrice, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{
			ID: 46, Platform: PlatformOpenAI, AllowImageGeneration: true,
			ImagePrice2K: &price,
		},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, withConfiguredPrice, 1)
	require.True(t, withConfiguredPrice[0].Supports("image", "create"))

	modelPrice := 0.06
	withModelPrice, err := svc.ResolveGatewayModelContracts(
		context.Background(),
		Group{
			ID: 46, Platform: PlatformOpenAI, AllowImageGeneration: true,
			ModelPricing: []ChannelModelPricing{{
				Models: []string{"sensenova-u1-fast"}, BillingMode: BillingModeImage,
				PerRequestPrice: &modelPrice,
			}},
		},
		[]string{"sensenova-u1-fast"},
	)
	require.NoError(t, err)
	require.Len(t, withModelPrice, 1)
	require.True(t, withModelPrice[0].Supports("image", "create"))
}

func TestCatalogOpenAIImagesContractAllowsOpaqueMappedModelWithoutNameInference(t *testing.T) {
	const modelID = "vendor-visual-v42"
	declaration := json.RawMessage(`{
		"version":"v1",
		"adapter":"openai_images",
		"modalities":["image"],
		"image":{"operations":["create"],"supported_sizes":["1024x1024","1536x1024"]}
	}`)
	svc := NewModelCatalogService(&modelCatalogVisibilityRepoStub{entries: []SiteModelCatalogEntry{{
		ModelName: modelID, Platform: PlatformOpenAI, VisibleAuth: true,
		GroupIDs: []int64{61}, MediaCapabilities: declaration,
	}}}, nil, nil, nil, nil, nil, nil)
	imagePrice := 0.04
	group := Group{ID: 61, Platform: PlatformOpenAI, AllowImageGeneration: true, ImagePrice2K: &imagePrice}

	contracts, err := svc.ResolveGatewayModelContracts(context.Background(), group, []string{modelID})
	require.NoError(t, err)
	require.Len(t, contracts, 1)
	require.True(t, contracts[0].Supports("image", "create"))

	resolutions, err := svc.ResolveImageStudioCatalogContracts(context.Background(), group, []string{modelID})
	require.NoError(t, err)
	resolution, found := resolutions[modelID]
	require.True(t, found)
	require.True(t, resolution.Declared)
	require.NotNil(t, resolution.Capabilities)
	require.Equal(t, catalogOpenAIImagesAdapterID, resolution.Capabilities.ProviderID)
	require.Equal(t, PlatformOpenAI, resolution.Capabilities.Platform)
	require.Equal(t, catalogOpenAIImagesProfileID(modelID, "v1"), resolution.Capabilities.ProfileID)
	require.True(t, resolution.Capabilities.RejectUndeclaredQuality)

	// Catalog rows decorate only mapped, schedulable candidates. The explicit
	// declaration cannot make an opaque ID appear without its account mapping.
	unmapped, err := svc.ResolveImageStudioCatalogContracts(context.Background(), group, []string{"other-visual-v42"})
	require.NoError(t, err)
	require.NotContains(t, unmapped, modelID)
}

func TestCatalogMediaCapabilitiesRejectLegacyGenerationOperationAlias(t *testing.T) {
	for _, declaration := range []json.RawMessage{
		json.RawMessage(`{
			"version":"v1","adapter":"sensenova","modalities":["image"],
			"image":{"operations":["generation"]}
		}`),
		json.RawMessage(`{
			"version":"v1","adapter":"grok_video","modalities":["video"],
			"video":{"operations":["generation"],"supported_resolutions":["720p"],"supported_aspect_ratios":["16:9"],"durations_seconds":[6]}
		}`),
	} {
		_, err := NormalizeCatalogMediaCapabilities(declaration)
		require.Error(t, err)
		require.Contains(t, err.Error(), "operations")
	}
}

func gatewayContractIDs(contracts []GatewayModelContract) []string {
	ids := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		ids = append(ids, contract.ID)
	}
	return ids
}
