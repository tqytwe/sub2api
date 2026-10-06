//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStarframeLookupRejectsUpstreamKeyReplacement(t *testing.T) {
	for _, endpoint := range []AgnesVideoEndpoint{AgnesVideoEndpointStatusLegacy, AgnesVideoEndpointContent} {
		t.Run(string(endpoint), func(t *testing.T) {
			upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
				grokMediaContentStatusResponse(`{"id":"task-1","status":"queued"}`),
				grokMediaContentStatusResponse(`{"id":"task-1","status":"completed"}`),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: &starframeTestCache{}, httpUpstream: upstream}
			account := starframeTestAccount()
			owner := StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}
			body := []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p"}`)
			c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
			result, err := svc.ForwardStarframeVideo(context.Background(), c, account, AgnesVideoEndpointCreate, nil, body, owner, starframeTestBilling(body))
			require.NoError(t, err)
			task, err := svc.LoadStarframeVideoTask(context.Background(), result.ResponseID, owner)
			require.NoError(t, err)
			account.Credentials["api_key"] = "replacement-test-only"
			c, _ = grokMediaContentTestContext(http.MethodGet, "/v1/videos/"+result.ResponseID, nil)
			_, err = svc.ForwardStarframeVideo(context.Background(), c, account, endpoint, task, nil, owner)
			require.Error(t, err)
			require.Len(t, upstream.requests, 1, "replacement key must not query or download another upstream user's raw ID")
		})
	}
}
