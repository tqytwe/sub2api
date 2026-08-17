package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuildAgnesVideoURLHandlesVersionedBaseURL(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://apihub.agnes-ai.com/v1",
		},
	}

	createURL, err := svc.buildAgnesVideoURL(account, AgnesVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://apihub.agnes-ai.com/v1/videos", createURL)

	statusURL, err := svc.buildAgnesVideoURL(account, AgnesVideoEndpointStatusVideo, "video_123")
	require.NoError(t, err)
	require.Equal(t, "https://apihub.agnes-ai.com/agnesapi?video_id=video_123", statusURL)

	legacyURL, err := svc.buildAgnesVideoURL(account, AgnesVideoEndpointStatusLegacy, "task_123")
	require.NoError(t, err)
	require.Equal(t, "https://apihub.agnes-ai.com/v1/videos/task_123", legacyURL)
}

func TestBuildAgnesVideoURLDoesNotDuplicateCreateEndpoint(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://apihub.agnes-ai.com/v1/videos",
		},
	}

	createURL, err := svc.buildAgnesVideoURL(account, AgnesVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://apihub.agnes-ai.com/v1/videos", createURL)
}

func TestExtractAgnesVideoResponseIDPrefersVideoID(t *testing.T) {
	body := []byte(`{"id":"task_1","task_id":"task_2","video_id":"video_3"}`)
	require.Equal(t, "video_3", ExtractAgnesVideoResponseID(body))
	require.Equal(t, "agnes-video:video_3", AgnesVideoSessionHash(" video_3 "))
}

func TestExtractAgnesVideoBillingMetadata(t *testing.T) {
	resolution, seconds := ExtractAgnesVideoBillingMetadata([]byte(`{"dimensions":"1280x720","num_frames":137,"frame_rate":24}`))
	require.Equal(t, VideoBillingResolution720P, resolution)
	require.Equal(t, 6, seconds)
	require.Equal(t, "agnes-video:task_123", StableAgnesVideoBillingRequestID("task_123"))
}

func TestExtractAgnesVideoBillingMetadataReadsOpenAIVideoFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "seedance seconds", body: `{"model":"seedance2.5","size":"1280x720","seconds":"6"}`},
		{name: "openai duration", body: `{"model":"veo-3.1","size":"1280x720","duration":6}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolution, seconds := ExtractAgnesVideoBillingMetadata([]byte(tt.body))
			require.Equal(t, VideoBillingResolution720P, resolution)
			require.Equal(t, 6, seconds)
		})
	}
}
