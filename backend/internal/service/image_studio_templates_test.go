package service

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultImageStudioCatalogIncludesPreviewMetadata(t *testing.T) {
	catalog := defaultImageStudioCatalog()
	require.Len(t, catalog.Intents, 3)

	for _, intent := range catalog.Intents {
		require.NotEmpty(t, intent.Templates)
		for _, template := range intent.Templates {
			require.NotEmpty(t, template.Description.Zh)
			require.NotEmpty(t, template.Description.En)
			require.NotEmpty(t, template.PreviewURL)
		}
	}
}

func TestImageStudioPromptReferenceMustMatchPublishedVersion(t *testing.T) {
	id := int64(123)
	version := 4
	repo := &promptLibraryRepoStub{prompt: &Prompt{
		ID:               id,
		Status:           PromptStatusPublished,
		PublishedVersion: version,
	}}
	svc := &ImageStudioService{promptRepo: repo}

	require.NoError(t, svc.validatePromptLibraryReference(
		context.Background(), 9, &id, &version,
	))

	staleVersion := 3
	require.ErrorIs(t, svc.validatePromptLibraryReference(
		context.Background(), 9, &id, &staleVersion,
	), ErrImageStudioPromptRef)

	repo.prompt = nil
	require.ErrorIs(t, svc.validatePromptLibraryReference(
		context.Background(), 9, &id, &version,
	), ErrImageStudioPromptRef)
}

func TestValidateImageStudioPrompt(t *testing.T) {
	require.ErrorIs(t, validateImageStudioPrompt(" \n\t"), ErrImageStudioPromptRequired)
	require.NoError(t, validateImageStudioPrompt("matte black headphones"))
	require.NoError(t, validateImageStudioPrompt(strings.Repeat("图", 8000)))
	require.ErrorIs(t, validateImageStudioPrompt(strings.Repeat("图", 8001)), ErrImageStudioPromptTooLong)
	require.NoError(t, validateImageStudioPrompt(strings.Repeat("😀", 8000)))
	require.ErrorIs(t, validateImageStudioPrompt(strings.Repeat("😀", 8001)), ErrImageStudioPromptTooLong)
}

func TestBuildImageStudioPromptKeepsUserSubjectPrimary(t *testing.T) {
	tpl := ImageStudioTemplate{
		PromptTemplate: "Professional product photo, {{subject}}, pure white background RGB 255, centered, no text, no watermark",
	}

	got := buildImageStudioPrompt(tpl, ImageStudioGenerateRequest{
		UserPrompt:  "OpenAI PlusX2",
		AccentColor: "#1C76BA",
	})

	require.Contains(t, got, "Primary subject: OpenAI PlusX2")
	require.Contains(t, got, "must be about this subject")
	require.Contains(t, got, "no unrelated text")
	require.NotContains(t, got, ", no text,")
	require.Contains(t, got, "accent color #1C76BA")
}

func TestNormalizeImageStudioPromptReferenceRequiresAPositivePair(t *testing.T) {
	id := int64(123)
	version := 4
	zeroID := int64(0)
	zeroVersion := 0

	gotID, gotVersion, err := normalizeImageStudioPromptReference(nil, nil)
	require.NoError(t, err)
	require.Nil(t, gotID)
	require.Nil(t, gotVersion)

	gotID, gotVersion, err = normalizeImageStudioPromptReference(&id, &version)
	require.NoError(t, err)
	require.Equal(t, id, *gotID)
	require.Equal(t, version, *gotVersion)

	for _, input := range []struct {
		id      *int64
		version *int
	}{
		{id: &id},
		{version: &version},
		{id: &zeroID, version: &version},
		{id: &id, version: &zeroVersion},
	} {
		_, _, err = normalizeImageStudioPromptReference(input.id, input.version)
		require.ErrorIs(t, err, ErrImageStudioPromptRef)
	}
}
