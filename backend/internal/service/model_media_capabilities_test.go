package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCatalogMediaCapabilitiesPreservesExtensions(t *testing.T) {
	raw := json.RawMessage(`{
		"version":"v1",
		"adapter":"sensenova",
		"modalities":["image","embedding"],
		"image":{"operations":["create"],"supported_output_formats":["png"]},
		"provider_extension":{"future_flag":true}
	}`)

	normalized, err := NormalizeCatalogMediaCapabilities(raw)
	require.NoError(t, err)
	require.JSONEq(t, string(raw), string(normalized))

	declaration, err := ParseCatalogMediaCapabilities(normalized)
	require.NoError(t, err)
	require.Equal(t, []string{"image", "embedding"}, declaration.Modalities)
	require.Equal(t, "sensenova", declaration.Adapter)
}

func TestNormalizeCatalogMediaCapabilitiesKeepsLegacyRowsNullable(t *testing.T) {
	normalized, err := NormalizeCatalogMediaCapabilities(json.RawMessage("null"))
	require.NoError(t, err)
	require.Nil(t, normalized)

	normalized, err = NormalizeCatalogMediaCapabilities(nil)
	require.NoError(t, err)
	require.Nil(t, normalized)
}

func TestNormalizeCatalogMediaCapabilitiesRejectsInvalidKnownShapes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing adapter",
			raw:  `{"version":"v1","modalities":["image"],"image":{"operations":["create"]}}`,
		},
		{
			name: "image modality without operations",
			raw:  `{"version":"v1","adapter":"sensenova","modalities":["image"],"image":{"operations":[]}}`,
		},
		{
			name: "video operations without modality",
			raw:  `{"version":"v1","adapter":"video-gateway","modalities":["chat"],"video":{"operations":["create"]}}`,
		},
		{
			name: "negative video reference limit",
			raw:  `{"version":"v1","adapter":"video-gateway","modalities":["video"],"video":{"operations":["generate"],"max_reference_images":-1}}`,
		},
		{
			name: "image operation must use canonical contract name",
			raw:  `{"version":"v1","adapter":"sensenova","modalities":["image"],"image":{"operations":["generation"]}}`,
		},
		{
			name: "video operation must use canonical contract name",
			raw:  `{"version":"v1","adapter":"grok_video","modalities":["video"],"video":{"operations":["create"]}}`,
		},
		{
			name: "image dimensions must be coherent",
			raw:  `{"version":"v1","adapter":"sensenova","modalities":["image"],"image":{"operations":["create"],"min_dimension":4096,"max_dimension":512}}`,
		},
		{
			name: "video durations must be positive and unique",
			raw:  `{"version":"v1","adapter":"grok_video","modalities":["video"],"video":{"operations":["generate"],"durations_seconds":[8,8]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeCatalogMediaCapabilities(json.RawMessage(tt.raw))
			require.Error(t, err)
		})
	}
}
