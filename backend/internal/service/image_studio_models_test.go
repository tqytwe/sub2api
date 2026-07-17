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
	models       []string
	seenPlatform string
}

func (s *imageStudioModelResolverStub) GetAvailableModels(_ context.Context, _ *int64, platform string) []string {
	s.seenPlatform = platform
	return append([]string(nil), s.models...)
}

func TestListImageModelsForAPIKey_UsesGroupMapping(t *testing.T) {
	groupID := int64(7)
	svc := &ImageStudioService{
		gateway: &imageStudioModelResolverStub{
			models: []string{"gpt-5.2", "gpt-image-1", "gpt-image-2"},
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

	models, err := svc.listImageModelsForAPIKey(context.Background(), apiKey)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-2", "gpt-image-1"}, models)
}

func TestListImageModelsForAPIKey_UsesMappedImageModelsAcrossOpenAICompatibleProtocols(t *testing.T) {
	groupID := int64(8)
	resolver := &imageStudioModelResolverStub{
		models: []string{
			"gpt-5.2",
			"text-embedding-3-large",
			"gpt-image-2",
			"gemini-3.1-flash-image",
			"imagen-4.0-generate-preview",
			"grok-imagine-image-quality",
			"flux-pro-image",
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
	require.Empty(t, resolver.seenPlatform)
	require.Equal(t, []string{
		"gpt-image-2",
		"gemini-3.1-flash-image",
		"grok-imagine-image-quality",
		"flux-pro-image",
		"imagen-4.0-generate-preview",
	}, models)
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
