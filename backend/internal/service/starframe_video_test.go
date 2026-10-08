//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func starframeTestAccount() *Account {
	return &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-only", "base_url": "https://relay.example/v1", "video_protocol": "starframe", "openai_capabilities": []string{"starframe"}}}
}

func starframeTestBilling(body []byte) *StarframeVideoBilling {
	info, _ := ParseStarframeVideoRequest(body)
	return &StarframeVideoBilling{Model: info.Model, Duration: info.Duration, Resolution: info.Resolution, UnitPrice: 0.037}
}

func TestStarframeRequiresExplicitProtocolAndBase(t *testing.T) {
	a := starframeTestAccount()
	require.True(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityStarframe))
	delete(a.Credentials, "video_protocol")
	require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityStarframe))
	a.Credentials["video_protocol"] = "starframe"
	delete(a.Credentials, "base_url")
	require.False(t, a.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityStarframe))
}

func TestParseStarframePreservesCustomModelAndExactBilling(t *testing.T) {
	body := []byte(`{"model":"ch-user-choice-fast","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p","aspect_ratio":"16:9"}`)
	info, err := ParseStarframeVideoRequest(body)
	require.NoError(t, err)
	require.Equal(t, "ch-user-choice-fast", info.Model)
	require.Equal(t, 10, info.Duration)
	require.Equal(t, "720p", info.Resolution)
	for _, invalid := range []string{
		`{"model":"ch-any","prompt":"p","mode":"references","client_task_id":"x","duration":16,"resolution":"720p"}`,
		`{"model":"ch-any","prompt":"p","mode":"references","client_task_id":"x","duration":"5","resolution":"720p"}`,
		`{"model":"ch-any","prompt":"p","mode":"references","client_task_id":"x","duration":5.5,"resolution":"720p"}`,
		`{"model":"ch-any","prompt":"p","mode":"references","client_task_id":"x","duration":5,"resolution":"4k"}`,
		`{"model":"ch-any","prompt":"p","mode":"frames","client_task_id":"x","duration":5,"resolution":"720p"}`,
		`{"model":"ch-any","prompt":"p","mode":"references","client_task_id":"../x","duration":5,"resolution":"720p"}`,
	} {
		_, err := ParseStarframeVideoRequest([]byte(invalid))
		require.Error(t, err, invalid)
	}
}

func TestStarframeURLOnlyUsesFixedTaskPath(t *testing.T) {
	for _, id := range []string{"../other", "https://evil.example", "task/other", "task?url=x", "task%2fother", ""} {
		_, err := buildStarframeVideoURL("https://relay.example/v1", AgnesVideoEndpointContent, id)
		require.Error(t, err, id)
	}
	u, err := buildStarframeVideoURL("https://relay.example/v1", AgnesVideoEndpointContent, "task-1")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example/v1/videos/task-1/content", u)
}

func TestStarframeContentDoesNotRedirectOrReadArbitraryURL(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://evil.example"}}, Body: io.NopCloser(strings.NewReader("redirect"))}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "/v1/videos/sfv_test/content", nil)
	_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointContent, &StarframeVideoTask{UpstreamID: "task-1", LocalID: "sfv_test", AccountID: 9, BaseURL: "https://relay.example/v1", UpstreamKeyFingerprint: starframeUpstreamKeyFingerprint("test-only")}, nil, StarframeVideoOwner{})
	require.Error(t, err)
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()))
	require.Equal(t, "https://relay.example/v1/videos/task-1/content", upstream.request.URL.String())
	require.Empty(t, recorder.Header().Get("Location"))
}
