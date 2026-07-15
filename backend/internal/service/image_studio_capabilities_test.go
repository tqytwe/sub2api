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
	require.Len(t, caps.SizeOptions, 15)
	require.NotEmpty(t, caps.Aspects)
	require.NotEmpty(t, caps.Tiers)
	require.Equal(t, []string{"1:1", "2:3", "3:2", "9:16", "16:9"}, []string{
		caps.Aspects[0].ID,
		caps.Aspects[1].ID,
		caps.Aspects[2].ID,
		caps.Aspects[3].ID,
		caps.Aspects[4].ID,
	})
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

func TestResolveModelCapabilitiesOpenByDefault(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	caps := svc.ResolveModelCapabilities(nil, "gpt-image-1.5")

	require.Contains(t, caps.SupportedSizes, "2048x2048")
	require.Contains(t, caps.SupportedSizes, "4096x4096")
	require.Equal(t, []string{"standard", "high"}, caps.SupportedQualities)
}

func TestValidateSizeForModelUsesDenialCache(t *testing.T) {
	svc := &ImageStudioService{capabilityCache: NewImageStudioCapabilityCache()}
	require.NoError(t, svc.ValidateSizeForModel(nil, "gpt-image-1.5", "4096x4096"))

	svc.capabilityCache.Deny("gpt-image-1.5", "4096x4096")
	require.ErrorIs(t, svc.ValidateSizeForModel(nil, "gpt-image-1.5", "4096x4096"), ErrImageStudioSizeNotSupported)
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
