//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMobileVideoRequestRequiresServerSafeFields(t *testing.T) {
	req := MobileVideoJobCreateInput{
		GroupID:           42,
		Model:             "grok-imagine-video-1.5",
		Prompt:            "a short test clip",
		Resolution:        "720p",
		DurationSeconds:   8,
		ClientRequestID:   "client-video-1",
		ReferenceAssetIDs: []string{"asset-1"},
	}
	require.NoError(t, ValidateMobileVideoJobCreateInput(req))
	taskInput, err := BuildMobileVideoTaskCreateInput(req)
	require.NoError(t, err)
	require.Equal(t, MobileTaskKindVideo, taskInput.Kind)
	require.Equal(t, "video.generate", taskInput.Operation)
	encoded, err := json.Marshal(taskInput.Resource)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "api_key")
	require.NotContains(t, string(encoded), "price")
}

func TestMobileVideoRequestAcceptsSeedanceAdaptiveRatioAndReferenceCount(t *testing.T) {
	refs := make([]string, 15)
	for index := range refs {
		refs[index] = "asset-" + string(rune('a'+index))
	}
	req := MobileVideoJobCreateInput{
		GroupID: 1, Model: "seedance-2.0", Prompt: "a short test clip", Resolution: "720p",
		Ratio: "adaptive", DurationSeconds: 8, ClientRequestID: "seedance-client-id", ReferenceAssetIDs: refs,
	}
	require.NoError(t, ValidateMobileVideoJobCreateInput(req))
}

func TestMobileVideoRequestAcceptsSmartDurationForCapabilityValidation(t *testing.T) {
	req := MobileVideoJobCreateInput{
		GroupID: 1, Model: "seedance-2.0", Prompt: "a short test clip", Resolution: "720p",
		Ratio: "adaptive", DurationSeconds: VideoBillingSmartDurationSeconds, ClientRequestID: "seedance-smart-duration",
	}
	require.NoError(t, ValidateMobileVideoJobCreateInput(req))
	capability := MobileVideoCapabilities{Resolutions: []string{"720p"}, Durations: []int{VideoBillingSmartDurationSeconds}}
	require.NoError(t, ValidateMobileVideoCapability(&Group{VideoPrice720P: float64Ptr(0.1)}, "seedance-2.0", capability, req))
	require.Equal(t, VideoBillingDefaultDurationSeconds, NormalizeVideoBillingDurationSecondsOrDefault(req.DurationSeconds))
}

func TestMobileVideoRequestRejectsInvalidParameters(t *testing.T) {
	base := MobileVideoJobCreateInput{GroupID: 1, Model: "video-model", Prompt: "prompt", Resolution: "720p", DurationSeconds: 8, ClientRequestID: "id"}
	tests := []struct {
		name   string
		mutate func(*MobileVideoJobCreateInput)
		want   error
	}{
		{"group", func(v *MobileVideoJobCreateInput) { v.GroupID = 0 }, ErrMobileVideoGroupRequired},
		{"model", func(v *MobileVideoJobCreateInput) { v.Model = "" }, ErrMobileVideoModelRequired},
		{"prompt", func(v *MobileVideoJobCreateInput) { v.Prompt = "" }, ErrMobileVideoPromptRequired},
		{"resolution", func(v *MobileVideoJobCreateInput) { v.Resolution = "4k" }, ErrMobileVideoResolutionInvalid},
		{"duration", func(v *MobileVideoJobCreateInput) { v.DurationSeconds = 0 }, ErrMobileVideoDurationInvalid},
		{"request id", func(v *MobileVideoJobCreateInput) { v.ClientRequestID = "" }, ErrMobileVideoClientRequestIDRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := base
			tt.mutate(&got)
			require.ErrorIs(t, ValidateMobileVideoJobCreateInput(got), tt.want)
		})
	}
}

func TestVideoGroupCapabilityIsExplicit(t *testing.T) {
	price := 0.1
	require.True(t, (&Group{VideoPrice720P: &price}).HasVideoGenerationCapability())
	require.False(t, (&Group{Platform: PlatformOpenAI}).HasVideoGenerationCapability())
}

func TestMobileVideoTaskFingerprintChangesWhenPromptChanges(t *testing.T) {
	base := MobileVideoJobCreateInput{GroupID: 1, Model: "video-model", Prompt: "first", Resolution: "720p", DurationSeconds: 8, ClientRequestID: "id"}
	first, err := BuildMobileVideoTaskCreateInput(base)
	require.NoError(t, err)
	base.Prompt = "second"
	second, err := BuildMobileVideoTaskCreateInput(base)
	require.NoError(t, err)
	require.NotEqual(t, first.Resource.ID, second.Resource.ID)
}
