//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestStarframeCreateRequiresExplicitBillingSnapshot(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1","status":"queued"}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: &starframeTestCache{}, httpUpstream: upstream}
	body := []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p"}`)
	c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30})
	require.Error(t, err)
	require.Empty(t, upstream.requests)
}
