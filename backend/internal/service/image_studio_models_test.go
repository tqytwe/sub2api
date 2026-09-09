package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateImageStudioAPIKey_RejectsUnusableKeys(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	tests := []struct {
		name string
		key  *APIKey
	}{
		{name: "missing", key: nil},
		{name: "disabled", key: &APIKey{Status: StatusAPIKeyDisabled}},
		{name: "expired status", key: &APIKey{Status: StatusAPIKeyExpired}},
		{name: "quota status", key: &APIKey{Status: StatusAPIKeyQuotaExhausted}},
		{name: "runtime expired", key: &APIKey{Status: StatusAPIKeyActive, ExpiresAt: &past}},
		{name: "runtime quota exhausted", key: &APIKey{Status: StatusAPIKeyActive, Quota: 1, QuotaUsed: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, ValidateImageStudioAPIKey(tt.key), ErrImageStudioAPIKey)
		})
	}

	require.NoError(t, ValidateImageStudioAPIKey(&APIKey{Status: StatusAPIKeyActive}))
}

type imageStudioModelResolverStub struct {
	modelsByPlatform map[string][]string
	models           []string
	seenPlatform     string
}

func (s *imageStudioModelResolverStub) GetAvailableModels(_ context.Context, _ *int64, platform string) []string {
	s.seenPlatform = platform
	if s.modelsByPlatform != nil {
		return append([]string(nil), s.modelsByPlatform[platform]...)
	}
	return append([]string(nil), s.models...)
}

type imageStudioCatalogContractResolverStub struct {
	resolutions map[string]ImageStudioCatalogContractResolution
	err         error
	seenGroup   Group
	seenModels  []string
}

func (s *imageStudioCatalogContractResolverStub) ResolveImageStudioCatalogContracts(
	_ context.Context,
	group Group,
	models []string,
) (map[string]ImageStudioCatalogContractResolution, error) {
	s.seenGroup = group
	s.seenModels = append([]string(nil), models...)
	if s.err != nil {
		return nil, s.err
	}
	return s.resolutions, nil
}

func TestListImageModelOptionsForAPIKey_PrefersExplicitCatalogContracts(t *testing.T) {
	groupID := int64(71)
	catalog := &imageStudioCatalogContractResolverStub{resolutions: map[string]ImageStudioCatalogContractResolution{
		"sensenova-u1-fast": {
			Declared: true,
			Capabilities: &ImageStudioModelCapabilities{
				Platform:               PlatformOpenAI,
				ProviderID:             imageModelAdapterSenseNovaID,
				ProfileID:              imageModelAdapterSenseNovaID + ":sensenova-u1-fast:v1",
				Revision:               imageStudioCapabilityRevision,
				Operations:             []string{"create"},
				SizingKind:             "fixed",
				SupportedSizes:         []string{"2048x2048"},
				SupportedOutputFormats: []string{"png"},
				DefaultSize:            "2048x2048",
				DefaultOutputFormat:    "png",
			},
		},
		// An explicitly reviewed but unsupported declaration must not fall back
		// to the static profile for this otherwise-known image model.
		"gpt-image-2": {Declared: true},
	}}
	svc := &ImageStudioService{
		gateway: &imageStudioModelResolverStub{models: []string{
			"gpt-image-1",
			"gpt-image-2",
			"sensenova-u1-fast",
			"new-image-model",
		}},
		catalog: catalog,
	}
	key := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}

	options, err := svc.listImageModelOptionsForAPIKey(context.Background(), key)

	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-1", "gpt-image-2", "sensenova-u1-fast", "new-image-model"}, catalog.seenModels)
	byID := make(map[string]ImageStudioModelOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	require.Contains(t, byID, "gpt-image-1", "legacy exact IDs remain available during catalog migration")
	require.NotContains(t, byID, "gpt-image-2", "an explicit invalid declaration must fail closed")
	require.NotContains(t, byID, "new-image-model", "a newly mapped name needs a catalog declaration")

	fast, found := byID["sensenova-u1-fast"]
	require.True(t, found)
	require.Equal(t, []string{"create"}, fast.Operations)
	require.Equal(t, []string{"2048x2048"}, fast.SupportedSizes)
	require.Equal(t, []string{"png"}, fast.SupportedOutputFormats)
}

func TestImageStudioCatalogCompositeGroupListsSenseNovaWithOpenAIDispatch(t *testing.T) {
	const (
		genericModel   = "vendor-visual-v42"
		senseNovaModel = "sensenova-u1-fast"
	)
	groupID := int64(72)
	generic, supported := imageStudioCapabilitiesFromCatalogContract(GatewayModelContract{
		ID:                genericModel,
		Platform:          PlatformComposite,
		Adapter:           catalogOpenAIImagesAdapterID,
		CapabilityVersion: "v1",
		Modalities:        []string{"image"},
		ImageCapabilities: &ModelImageCapabilities{
			Operations:     []string{"create"},
			SupportedSizes: []string{"1024x1024"},
		},
	})
	require.True(t, supported)
	senseNova, supported := ResolveImageStudioProviderCapability(PlatformOpenAI, senseNovaModel)
	require.True(t, supported)

	resolver := &imageStudioModelResolverStub{models: []string{genericModel, senseNovaModel}}
	svc := &ImageStudioService{
		gateway: resolver,
		catalog: &imageStudioCatalogContractResolverStub{resolutions: map[string]ImageStudioCatalogContractResolution{
			genericModel:   {Declared: true, Capabilities: &generic},
			senseNovaModel: {Declared: true, Capabilities: &senseNova},
		}},
	}
	options, err := svc.listImageModelOptionsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformComposite,
			AllowImageGeneration: true,
		},
	})

	require.NoError(t, err)
	require.Equal(t, "", resolver.seenPlatform, "composite mappings must be collected from concrete accounts")
	require.Len(t, options, 2)
	byID := make(map[string]ImageStudioModelOption, len(options))
	for _, option := range options {
		byID[option.ID] = option
	}
	require.Contains(t, byID, genericModel)
	require.Contains(t, byID, senseNovaModel)
	require.Equal(t, imageModelAdapterSenseNovaID, byID[senseNovaModel].ProviderID)
	require.Equal(t, PlatformOpenAI, byID[senseNovaModel].Platform)

	platform, err := imageStudioDispatchPlatformForCapability(&APIKey{Group: &Group{Platform: PlatformComposite}}, byID[senseNovaModel].ImageStudioModelCapabilities)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, platform)
}

func TestListImageModelsForAPIKey_UsesGroupMapping(t *testing.T) {
	groupID := int64(7)
	resolver := &imageStudioModelResolverStub{
		models: []string{"gpt-5.2", "gpt-image-1", "gpt-image-2"},
	}
	svc := &ImageStudioService{
		gateway: resolver,
	}
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}

	models, err := svc.listImageModelsForAPIKey(context.Background(), apiKey)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, resolver.seenPlatform)
	require.Equal(t, []string{"gpt-image-2", "gpt-image-1"}, models)
}

func TestListImageModelsForAPIKey_UsesOpenAIPlatformMappingsForOpenAICompatibleImageModels(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{
		modelsByPlatform: map[string][]string{
			PlatformOpenAI: {
				"gpt-5.2",
				"text-embedding-3-large",
				"gpt-image-2",
				"agnes-image-2.1-flash",
				"agnes-image-2.0-flash",
				"gemini-3.1-flash-image",
				"imagen-4.0-generate-preview",
				"flux-pro-image",
			},
			PlatformGemini: {
				"gemini-native-only-image",
			},
			PlatformGrok: {
				"grok-imagine-image-quality",
			},
		},
	}
	svc := &ImageStudioService{gateway: resolver}

	models, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, resolver.seenPlatform)
	require.Equal(t, []string{
		"gpt-image-2",
		"agnes-image-2.1-flash",
		"agnes-image-2.0-flash",
		"gemini-3.1-flash-image",
		"flux-pro-image",
		"imagen-4.0-generate-preview",
	}, models)
}

func TestListImageModelsForAPIKey_DoesNotLeakOtherPlatformMappingsIntoOpenAIGroup(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{
		modelsByPlatform: map[string][]string{
			PlatformOpenAI: {"gpt-5.2"},
			PlatformGemini: {
				"gemini-3.1-flash-image-preview",
				"imagen-4.0-generate-preview",
			},
			PlatformGrok: {"grok-imagine-image-quality"},
		},
	}
	svc := &ImageStudioService{gateway: resolver}

	models, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	})

	require.ErrorIs(t, err, ErrImageStudioNoImageModels)
	require.Equal(t, PlatformOpenAI, resolver.seenPlatform)
	require.Nil(t, models)
}

func TestListImageModelsForAPIKey_DoesNotUseProviderDefaultsWhenGatewayHasNoMappedModels(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{}
	svc := &ImageStudioService{gateway: resolver}

	models, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformGrok,
			AllowImageGeneration: true,
		},
	})

	require.ErrorIs(t, err, ErrImageStudioNoImageModels)
	require.Equal(t, PlatformGrok, resolver.seenPlatform)
	require.Nil(t, models)
}

func TestListImageModelsForAPIKey_RejectsMissingOrUnknownPlatformInsteadOfOpenAIFallback(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{
		modelsByPlatform: map[string][]string{
			PlatformOpenAI: {"gpt-image-2"},
			"krio":         {"gpt-image-2"},
		},
	}
	svc := &ImageStudioService{gateway: resolver}

	tests := []struct {
		name string
		key  *APIKey
	}{
		{
			name: "unknown platform",
			key: &APIKey{
				GroupID: &groupID,
				Group: &Group{
					ID:                   groupID,
					Platform:             "krio",
					AllowImageGeneration: true,
				},
			},
		},
		{
			name: "empty platform",
			key: &APIKey{
				GroupID: &groupID,
				Group: &Group{
					ID:                   groupID,
					Platform:             "",
					AllowImageGeneration: true,
				},
			},
		},
		{
			name: "missing group",
			key:  &APIKey{GroupID: &groupID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver.seenPlatform = ""

			models, err := svc.listImageModelsForAPIKey(context.Background(), tt.key)

			require.ErrorIs(t, err, ErrImageStudioProviderNotSupported)
			require.Empty(t, resolver.seenPlatform)
			require.Nil(t, models)
		})
	}
}

func TestListImageModelsForAPIKey_CustomModelsListEnabledEmptyFailsClosed(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{
		models: []string{"gpt-image-2"},
	}
	svc := &ImageStudioService{gateway: resolver}

	models, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
			ModelAllowlist: GroupModelAllowlist{
				Enabled: true,
			},
		},
	})

	require.ErrorIs(t, err, ErrImageStudioNoImageModels)
	require.Equal(t, PlatformOpenAI, resolver.seenPlatform)
	require.Nil(t, models)
}

func TestListImageModelsForAPIKey_UsesProviderDefaultsWhenGroupHasNoMapping(t *testing.T) {
	groupID := int64(9)
	svc := &ImageStudioService{}

	geminiModels, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformGemini,
			AllowImageGeneration: true,
		},
	})
	require.NoError(t, err)
	require.Contains(t, geminiModels, "gemini-3.1-flash-image")
	require.Contains(t, geminiModels, "gemini-2.5-flash-image")
	require.NotContains(t, geminiModels, "gpt-image-2")

	grokModels, err := svc.listImageModelsForAPIKey(context.Background(), &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformGrok,
			AllowImageGeneration: true,
		},
	})
	require.NoError(t, err)
	require.Contains(t, grokModels, "grok-imagine-image-quality")
	require.Contains(t, grokModels, "grok-imagine-image")
	require.NotContains(t, grokModels, "gpt-image-2")
}

func TestResolveImageModel_RejectsUnavailableSelection(t *testing.T) {
	groupID := int64(7)
	svc := &ImageStudioService{
		gateway: &imageStudioModelResolverStub{
			models: []string{"gpt-image-1"},
		},
	}
	apiKey := &APIKey{
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}

	got, err := svc.resolveImageModel(context.Background(), apiKey, "gpt-image-2")
	require.ErrorIs(t, err, ErrImageStudioModelNotAllowed)
	require.Empty(t, got)

	got, err = svc.resolveImageModel(context.Background(), apiKey, "")
	require.NoError(t, err)
	require.Equal(t, "gpt-image-1", got)
}

func TestListImageModelsForAPIKey_BlocksDisabledGroup(t *testing.T) {
	svc := &ImageStudioService{
		gateway: &imageStudioModelResolverStub{
			models: []string{"gpt-image-2"},
		},
	}
	apiKey := &APIKey{
		Group: &Group{
			AllowImageGeneration: false,
		},
	}

	models, err := svc.listImageModelsForAPIKey(context.Background(), apiKey)
	require.ErrorIs(t, err, ErrImageStudioImageNotAllowed)
	require.Nil(t, models)
}

func TestFilterImageModelsByCustomList_RespectsWildcard(t *testing.T) {
	got := filterImageModelsByCustomList(
		[]string{"gpt-image-1", "gpt-image-2", "gpt-5.2"},
		[]string{"gpt-image-*"},
	)
	require.Equal(t, []string{"gpt-image-1", "gpt-image-2"}, got)
}
