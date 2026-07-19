package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveImageStudioSizeFromAspectTier(t *testing.T) {
	size, err := ResolveImageStudioSize("1:1", "2K", "")
	require.NoError(t, err)
	require.Equal(t, "2048x2048", size)

	size, err = ResolveImageStudioSize("9:16", "1K", "")
	require.NoError(t, err)
	require.Equal(t, "1024x1792", size)

	size, err = ResolveImageStudioSize("9:16", "4K", "")
	require.NoError(t, err)
	require.Equal(t, "4096x7168", size)

	size, err = ResolveImageStudioSize("16:9", "3K", "")
	require.NoError(t, err)
	require.Equal(t, "3072x1728", size)
}

func TestResolveImageStudioSizeSupportsLegacyAspectAliases(t *testing.T) {
	size, err := ResolveImageStudioSize("3:4", "1K", "")
	require.NoError(t, err)
	require.Equal(t, "1024x1536", size)

	size, err = ResolveImageStudioSize("4:3", "1K", "")
	require.NoError(t, err)
	require.Equal(t, "1536x1024", size)
}

func TestResolveImageStudioSizeFromRaw(t *testing.T) {
	size, err := ResolveImageStudioSize("", "", "1536x1024")
	require.NoError(t, err)
	require.Equal(t, "1536x1024", size)

	_, err = ResolveImageStudioSize("", "", "999x999")
	require.ErrorIs(t, err, ErrImageStudioSizeNotSupported)
}

func TestListImageStudioCapabilities(t *testing.T) {
	caps := ListImageStudioCapabilities()
	require.Len(t, caps.SizeOptions, 20)
	require.NotEmpty(t, caps.Aspects)
	require.NotEmpty(t, caps.Tiers)
	require.Contains(t, []string{caps.Tiers[0].ID, caps.Tiers[1].ID, caps.Tiers[2].ID, caps.Tiers[3].ID}, "3K")
	require.Equal(t, []string{"1:1", "2:3", "3:2", "9:16", "16:9"}, []string{
		caps.Aspects[0].ID,
		caps.Aspects[1].ID,
		caps.Aspects[2].ID,
		caps.Aspects[3].ID,
		caps.Aspects[4].ID,
	})
	var found3K bool
	for _, option := range caps.SizeOptions {
		if option.Aspect == "16:9" && option.Tier == "3K" {
			found3K = true
			require.Equal(t, "3072x1728", option.Size)
			require.Equal(t, ImageBillingSize4K, option.BillingTier)
		}
	}
	require.True(t, found3K)
}

func TestClassifyImageBillingTierMaps3KTo4K(t *testing.T) {
	tier, ok := ClassifyImageBillingTier("3K")
	require.True(t, ok)
	require.Equal(t, ImageBillingSize4K, tier)

	tier, ok = ClassifyImageBillingTier("3072x1728")
	require.True(t, ok)
	require.Equal(t, ImageBillingSize4K, tier)
}

func TestInferImageStudioAspectTierIsDeterministic(t *testing.T) {
	for range 20 {
		aspect, tier := InferImageStudioAspectTier("1024x1536")
		require.Equal(t, "2:3", aspect)
		require.Equal(t, "1K", tier)
	}
}

func TestImageStudioCapabilityCacheDeniesSize(t *testing.T) {
	cache := NewImageStudioCapabilityCache()
	require.False(t, cache.IsDenied("gpt-image-1.5", "4096x4096"))

	cache.Deny("gpt-image-1.5", "4096x4096")
	require.True(t, cache.IsDenied("gpt-image-1.5", "4096x4096"))
	require.False(t, cache.IsDenied("gpt-image-2", "4096x4096"))
}

func TestResolveModelCapabilitiesUsesProviderProfile(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	caps := svc.ResolveModelCapabilities(nil, "gpt-image-1.5")

	require.Equal(t, []string{"1024x1024", "1536x1024", "1024x1536"}, caps.SupportedSizes)
	require.Equal(t, []string{"auto", "low", "medium", "high"}, caps.SupportedQualities)
	require.Equal(t, []string{"auto", "opaque", "transparent"}, caps.SupportedBackgrounds)
}

func TestResolveModelCapabilitiesUsesAPIKeyProviderAndModelFamily(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}

	gpt2Variant := svc.ResolveModelCapabilities(nil, "gpt-image-2-codex")
	require.Equal(t, "custom", gpt2Variant.SizingKind)
	require.Equal(t, []string{"auto", "opaque"}, gpt2Variant.SupportedBackgrounds)
	require.Equal(t, []string{"high"}, gpt2Variant.SupportedInputFidelities)
	require.False(t, gpt2Variant.SupportsTransparency)

	grok := svc.ResolveModelCapabilities(&APIKey{
		Group: &Group{Platform: PlatformGrok},
	}, "grok-imagine-image-quality")
	require.Equal(t, PlatformGrok, grok.Platform)
	require.Equal(t, "aspect_resolution", grok.SizingKind)
	require.Equal(t, []string{"jpeg"}, grok.SupportedOutputFormats)
	require.Empty(t, grok.SupportedBackgrounds)
	require.False(t, grok.SupportsTransparency)
}

func TestValidateSizeForModelUsesDenialCache(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	require.NoError(t, svc.ValidateSizeForModel(nil, "gpt-image-1.5", "1024x1024"))

	svc.capabilityCache.Deny("gpt-image-1.5", "1024x1024")
	require.ErrorIs(t, svc.ValidateSizeForModel(nil, "gpt-image-1.5", "1024x1024"), ErrImageStudioSizeNotSupported)
}

func TestValidateQualityForModel(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	require.NoError(t, svc.ValidateQualityForModel("gpt-image-2", "high"))
	require.NoError(t, svc.ValidateQualityForModel("gpt-image-2", ""))
	require.ErrorIs(t, svc.ValidateQualityForModel("grok-imagine-image", "high"), ErrImageStudioQualityNotSupported)
}

func TestRecordGenerateFailure(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	svc.RecordGenerateFailure("gpt-image-1", "2048x2048", "invalid image size")
	require.True(t, svc.capabilityCache.IsDenied("gpt-image-1", "2048x2048"))

	svc.capabilityCache = NewImageStudioCapabilityCache()
	svc.RecordGenerateFailure("gpt-image-1", "2048x2048", "insufficient balance")
	require.False(t, svc.capabilityCache.IsDenied("gpt-image-1", "2048x2048"))
}

func TestIsImageStudioSizeRelatedError(t *testing.T) {
	require.True(t, isImageStudioSizeRelatedError("Invalid size parameter"))
	require.False(t, isImageStudioSizeRelatedError("insufficient balance"))
}
