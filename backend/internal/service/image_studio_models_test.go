package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type imageStudioModelResolverStub struct {
	models []string
}

func (s *imageStudioModelResolverStub) GetAvailableModels(_ context.Context, _ *int64, _ string) []string {
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
