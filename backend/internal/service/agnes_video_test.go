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
