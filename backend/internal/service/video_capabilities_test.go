package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveVideoModelCapabilitiesFailsClosedWithoutPrice(t *testing.T) {
	group := Group{ID: 1, Platform: PlatformGrok, Status: StatusActive}
	if _, ok := ResolveVideoModelCapabilities(group, PlatformGrok, "grok-imagine-video"); ok {
		t.Fatal("expected unpriced group to have no video capability")
	}
}

func TestResolveVideoModelCapabilitiesUsesOnlyConfiguredResolutions(t *testing.T) {
	price := 0.02
	group := Group{ID: 1, Platform: PlatformGrok, Status: StatusActive, VideoPrice480P: &price}
	capability, ok := ResolveVideoModelCapabilities(group, PlatformGrok, "grok-imagine-video")
	if !ok {
		t.Fatal("expected priced group to expose video capability")
	}
	if len(capability.SupportedResolutions) != 1 || capability.SupportedResolutions[0] != VideoBillingResolution480P {
		t.Fatalf("unexpected resolutions: %+v", capability.SupportedResolutions)
	}
}

func TestResolveVideoModelCapabilitiesSeedanceUsesOnlySeedanceContract(t *testing.T) {
	price := 0.02
	group := Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, VideoPrice720P: &price}
	capability, ok := ResolveVideoModelCapabilities(group, PlatformOpenAI, "seedance-2.0")
	require.True(t, ok)
	require.ElementsMatch(t, []string{"text_to_video", "image_to_video", "video_reference", "audio_reference"}, capability.Operations)
	require.Contains(t, capability.SupportedRatios, "adaptive")
	require.Equal(t, []int{-1, 4, 5, 6, 8, 10, 12, 15}, capability.SupportedDurations)
	require.True(t, capability.SupportsDuration(VideoBillingSmartDurationSeconds))
	require.Equal(t, 9, capability.MaxReferenceImages)
	require.Equal(t, 3, capability.MaxReferenceVideos)
	require.Equal(t, 3, capability.MaxReferenceAudios)
	require.True(t, capability.GenerateAudio)
	require.True(t, capability.Watermark)

	nonSeedance, ok := ResolveVideoModelCapabilities(group, PlatformOpenAI, "agnes-video-v2.0")
	require.True(t, ok)
	require.NotContains(t, nonSeedance.SupportedRatios, "adaptive")
	require.Zero(t, nonSeedance.MaxReferenceVideos)
	require.Zero(t, nonSeedance.MaxReferenceAudios)
	require.False(t, nonSeedance.GenerateAudio)
	require.False(t, nonSeedance.SupportsDuration(VideoBillingSmartDurationSeconds))
}
