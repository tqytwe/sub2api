//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStarframeMaterialValidationBeforeSubmissionClaim(t *testing.T) {
	for _, refs := range []string{
		`{"image":"https://example.com/a.png","images":["https://example.com/a.png","https://example.com/b.png"]}`,
		`{"images":[]}`, `{"images":["https://example.com/a.png"]}`,
		`{"video":"https://example.com/a.mp4","videos":["https://example.com/a.mp4","https://example.com/b.mp4"]}`,
		`{"audio":"https://example.com/a.mp3","audios":["https://example.com/a.mp3","https://example.com/b.mp3"]}`,
		`{"image":"data:image/png;base64,xxx"}`, `{"image":"https://127.0.0.1/a"}`, `{"image":123}`, `"not-object"`,
	} {
		upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1"}`)}
		cache := &starframeTestCache{}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, starframeVideos: &starframeMemoryStore{}, cache: cache, httpUpstream: upstream}
		body := []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p","references":` + refs + `}`)
		c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
		_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}, starframeTestBilling(body))
		require.Error(t, err, refs)
		require.Empty(t, upstream.requests)
		require.Empty(t, cache.claims)
	}
	for _, first := range []string{"data:image/png;base64,xxx", "https://localhost/a", "https://10.0.0.1/a"} {
		_, err := ParseStarframeVideoRequest([]byte(`{"model":"ch-custom","prompt":"waves","mode":"frames","client_task_id":"order-1","duration":10,"resolution":"720p","frames":{"first_frame":"` + first + `","last_frame":"https://example.com/b.png"}}`))
		require.Error(t, err, first)
	}
}
