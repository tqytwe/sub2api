package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveImageStudioProviderCapabilityUsesProviderSpecificProfiles(t *testing.T) {
	gpt15, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "gpt-image-1.5")
	require.True(t, ok)
	require.Equal(t, "openai:gpt-image-1.5:v1", gpt15.ProfileID)
	require.Equal(t, []string{"create", "edit"}, gpt15.Operations)
	require.Equal(t, "fixed", gpt15.SizingKind)
	require.Equal(t, []string{"1024x1024", "1536x1024", "1024x1536"}, gpt15.SupportedSizes)
	require.Equal(t, []string{"auto", "low", "medium", "high"}, gpt15.SupportedQualities)
	require.Equal(t, []string{"auto", "opaque", "transparent"}, gpt15.SupportedBackgrounds)
	require.Equal(t, []string{"png", "jpeg", "webp"}, gpt15.SupportedOutputFormats)
	require.True(t, gpt15.SupportsTransparency)
	require.Equal(t, 4, gpt15.MaxReferenceImages)

	gpt2, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "gpt-image-2")
	require.True(t, ok)
	require.Equal(t, "custom", gpt2.SizingKind)
	require.NotContains(t, gpt2.SupportedSizes, "4096x4096")
	require.Contains(t, gpt2.SupportedSizes, "3840x2160")
	require.False(t, gpt2.SupportsTransparency)
	require.Equal(t, "fixed", gpt2.InputFidelityMode)
	require.Equal(t, []string{"high"}, gpt2.SupportedInputFidelities)

	grok, ok := ResolveImageStudioProviderCapability(PlatformGrok, "grok-imagine-image-quality")
	require.True(t, ok)
	require.Equal(t, "grok:grok-imagine-image-quality:v1", grok.ProfileID)
	require.Equal(t, "aspect_resolution", grok.SizingKind)
	require.Equal(t, []string{"1:1", "2:3", "3:2", "9:16", "16:9"}, grok.SupportedAspectRatios)
	require.Equal(t, []string{"1k", "2k"}, grok.SupportedResolutions)
	require.Equal(t, []string{"jpeg"}, grok.SupportedOutputFormats)
	require.Empty(t, grok.SupportedBackgrounds)
	require.Nil(t, grok.OutputCompression)
	require.Equal(t, 3, grok.MaxReferenceImages)

	gemini, ok := ResolveImageStudioProviderCapability(PlatformGemini, "gemini-3.1-flash-image")
	require.True(t, ok)
	require.Equal(t, "gemini:gemini-3.1-flash-image:v1", gemini.ProfileID)
	require.Equal(t, []string{"create", "edit"}, gemini.Operations)
	require.Equal(t, "aspect_resolution", gemini.SizingKind)
	require.Equal(t, []string{"1:1", "2:3", "3:2", "9:16", "16:9"}, gemini.SupportedAspectRatios)
	require.Equal(t, []string{"1k", "2k"}, gemini.SupportedResolutions)
	require.Equal(t, []string{"png"}, gemini.SupportedOutputFormats)
	require.Equal(t, 4, gemini.MaxReferenceImages)

	agnes, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "agnes-image-2.1-flash")
	require.True(t, ok)
	require.Equal(t, PlatformOpenAI, agnes.Platform)
	require.Equal(t, "agnes", agnes.ProviderID)
	require.Equal(t, "agnes:agnes-image-2.1-flash:v1", agnes.ProfileID)
	require.Equal(t, []string{"create"}, agnes.Operations)
	require.Equal(t, "aspect_resolution", agnes.SizingKind)
	require.Equal(t, []string{"1:1", "2:3", "3:2", "9:16", "16:9"}, agnes.SupportedAspectRatios)
	require.Equal(t, []string{"1k", "2k", "3k", "4k"}, agnes.SupportedResolutions)
	require.Contains(t, agnes.SupportedSizes, "3072x3072")
	require.Empty(t, agnes.SupportedQualities)
	require.Empty(t, agnes.SupportedBackgrounds)
	require.Empty(t, agnes.SupportedOutputFormats)
	require.Empty(t, agnes.SupportedInputFidelities)
	require.False(t, agnes.SupportsTransparency)
	require.Equal(t, 0, agnes.MaxReferenceImages)
}

func TestResolveImageStudioProviderCapabilityGPTImage2VariantsInheritBaseProfile(t *testing.T) {
	base, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "gpt-image-2")
	require.True(t, ok)

	for _, model := range []string{
		"gpt-image-2-codex",
		"gpt-image-2-preview-2026-07-17",
	} {
		t.Run(model, func(t *testing.T) {
			got, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, model)
			require.True(t, ok)
			require.Equal(t, "openai:"+model+":v1", got.ProfileID)

			want := base
			want.ProfileID = got.ProfileID
			require.Equal(t, want, got)
		})
	}
}

func TestResolveImageStudioModelCapabilityInfersModelFamilyWithoutTransportPlatform(t *testing.T) {
	tests := []struct {
		model    string
		platform string
		profile  string
	}{
		{
			model:    "gpt-image-2",
			platform: PlatformOpenAI,
			profile:  "openai:gpt-image-2:v1",
		},
		{
			model:    "models/gemini-3.1-flash-image",
			platform: PlatformGemini,
			profile:  "gemini:models/gemini-3.1-flash-image:v1",
		},
		{
			model:    "imagen-4.0-generate-preview",
			platform: PlatformGemini,
			profile:  "gemini:imagen-4.0-generate-preview:v1",
		},
		{
			model:    "grok-imagine-image-quality",
			platform: PlatformGrok,
			profile:  "grok:grok-imagine-image-quality:v1",
		},
		{
			model:    "flux-pro-image",
			platform: PlatformOpenAI,
			profile:  "openai_compatible:flux-pro-image:v1",
		},
		{
			model:    "agnes-image-2.1-flash",
			platform: PlatformOpenAI,
			profile:  "agnes:agnes-image-2.1-flash:v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			capability, ok := ResolveImageStudioModelCapability(tt.model)
			require.True(t, ok)
			require.Equal(t, tt.platform, capability.Platform)
			require.Equal(t, tt.profile, capability.ProfileID)
		})
	}
}

func TestResolveImageStudioModelCapabilityRejectsNonImageModels(t *testing.T) {
	for _, model := range []string{
		"gpt-5.2",
		"text-embedding-3-large",
		"gemini-3.1-pro",
		"claude-opus-4-1",
	} {
		t.Run(model, func(t *testing.T) {
			_, ok := ResolveImageStudioModelCapability(model)
			require.False(t, ok)
		})
	}
}

func TestResolveImageStudioProviderCapabilityRejectsCrossPlatformModels(t *testing.T) {
	_, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "grok-imagine-image-quality")
	require.False(t, ok)

	_, ok = ResolveImageStudioProviderCapability(PlatformGrok, "gpt-image-2")
	require.False(t, ok)

	_, ok = ResolveImageStudioProviderCapability(PlatformGemini, "gpt-image-2")
	require.False(t, ok)

	_, ok = ResolveImageStudioProviderCapability(PlatformGemini, "agnes-image-2.1-flash")
	require.False(t, ok)

	_, ok = ResolveImageStudioProviderCapability("anthropic", "gpt-image-2")
	require.False(t, ok)
}

func TestValidateImageStudioProviderOptions(t *testing.T) {
	gpt15, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "gpt-image-1.5")
	require.True(t, ok)

	compression := 82
	require.NoError(t, ValidateImageStudioProviderOptions(gpt15, "edit", ImageStudioGenerateRequest{
		Background:        "transparent",
		OutputFormat:      "webp",
		OutputCompression: &compression,
		InputFidelity:     "high",
		ReferenceIDs:      []string{"one", "two"},
	}))

	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		InputFidelity: "high",
	}), ErrImageStudioInputFidelityNotSupported)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		Style: "vivid",
	}), ErrImageStudioStyleNotSupported)

	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		Background:   "transparent",
		OutputFormat: "jpeg",
	}), ErrImageStudioOutputFormatNotSupported)
	require.NoError(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		Background:   "transparent",
		OutputFormat: "webp",
	}))

	pngCompression := 80
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		OutputFormat:      "png",
		OutputCompression: &pngCompression,
	}), ErrImageStudioOutputCompressionNotSupported)

	tooHigh := 101
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt15, "create", ImageStudioGenerateRequest{
		OutputFormat:      "jpeg",
		OutputCompression: &tooHigh,
	}), ErrImageStudioOutputCompressionNotSupported)

	gpt2, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "gpt-image-2")
	require.True(t, ok)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt2, "create", ImageStudioGenerateRequest{
		Background:   "transparent",
		OutputFormat: "png",
	}), ErrImageStudioBackgroundNotSupported)
	require.NoError(t, ValidateImageStudioProviderOptions(gpt2, "edit", ImageStudioGenerateRequest{
		Background:    "opaque",
		OutputFormat:  "jpg",
		InputFidelity: "high",
	}))
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gpt2, "edit", ImageStudioGenerateRequest{
		InputFidelity: "low",
	}), ErrImageStudioInputFidelityNotSupported)

	grok, ok := ResolveImageStudioProviderCapability(PlatformGrok, "grok-imagine-image-quality")
	require.True(t, ok)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(grok, "edit", ImageStudioGenerateRequest{
		OutputFormat: "webp",
		ReferenceIDs: []string{"one"},
	}), ErrImageStudioOutputFormatNotSupported)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(grok, "edit", ImageStudioGenerateRequest{
		ReferenceIDs: []string{"one", "two", "three", "four"},
	}), ErrImageStudioReferenceLimit)

	gemini, ok := ResolveImageStudioProviderCapability(PlatformGemini, "gemini-3.1-flash-image")
	require.True(t, ok)
	require.NoError(t, ValidateImageStudioProviderOptions(gemini, "edit", ImageStudioGenerateRequest{
		OutputFormat: "png",
		ReferenceIDs: []string{"one", "two", "three", "four"},
	}))
	require.ErrorIs(t, ValidateImageStudioProviderOptions(gemini, "create", ImageStudioGenerateRequest{
		OutputFormat: "webp",
	}), ErrImageStudioOutputFormatNotSupported)

	agnes, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "agnes-image-2.1-flash")
	require.True(t, ok)
	require.NoError(t, ValidateImageStudioProviderOptions(agnes, "create", ImageStudioGenerateRequest{}))
	require.ErrorIs(t, ValidateImageStudioProviderOptions(agnes, "edit", ImageStudioGenerateRequest{
		ReferenceIDs: []string{"one"},
	}), ErrImageStudioOperationNotSupported)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(agnes, "create", ImageStudioGenerateRequest{
		OutputFormat: "png",
	}), ErrImageStudioOutputFormatNotSupported)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(agnes, "create", ImageStudioGenerateRequest{
		Background: "opaque",
	}), ErrImageStudioBackgroundNotSupported)
	require.ErrorIs(t, ValidateImageStudioProviderOptions(agnes, "create", ImageStudioGenerateRequest{
		Style: "vivid",
	}), ErrImageStudioStyleNotSupported)
}
