package service

import (
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

func TestValidateImageStudioPrompt(t *testing.T) {
	require.ErrorIs(t, validateImageStudioPrompt(" \n\t"), ErrImageStudioPromptRequired)
	require.NoError(t, validateImageStudioPrompt("matte black headphones"))
	require.NoError(t, validateImageStudioPrompt(strings.Repeat("图", 8000)))
	require.ErrorIs(t, validateImageStudioPrompt(strings.Repeat("图", 8001)), ErrImageStudioPromptTooLong)
	require.NoError(t, validateImageStudioPrompt(strings.Repeat("😀", 8000)))
	require.ErrorIs(t, validateImageStudioPrompt(strings.Repeat("😀", 8001)), ErrImageStudioPromptTooLong)
}
