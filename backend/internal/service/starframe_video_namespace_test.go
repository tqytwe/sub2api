//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestStarframeClientTaskIDIsOwnerNamespaced(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(`{"id":"task-1","status":"queued"}`),
		grokMediaContentStatusResponse(`{"id":"task-2","status":"queued"}`),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: &starframeTestCache{}, httpUpstream: upstream}
	body := []byte(`{"model":"ch-custom-fast","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p","aspect_ratio":"16:9","references":{"image":"https://example.com/ref.png"},"custom":"preserved"}`)
	var ids []string
	for _, owner := range []StarframeVideoOwner{{UserID: 10, APIKeyID: 20, GroupID: 30}, {UserID: 11, APIKeyID: 21, GroupID: 30}} {
		c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
		_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, owner, starframeTestBilling(body))
		require.NoError(t, err)
		forwarded, err := io.ReadAll(upstream.request.Body)
		require.NoError(t, err)
		id := gjson.GetBytes(forwarded, "client_task_id").String()
		require.NotEqual(t, "order-1", id)
		require.LessOrEqual(t, len(id), 128)
		require.True(t, isSafeUpstreamPathSegment(id))
		ids = append(ids, id)
		originalRest, err := sjson.DeleteBytes(body, "client_task_id")
		require.NoError(t, err)
		forwardedRest, err := sjson.DeleteBytes(forwarded, "client_task_id")
		require.NoError(t, err)
		require.JSONEq(t, string(originalRest), string(forwardedRest))
	}
	require.NotEqual(t, ids[0], ids[1], "different owners must not collide at shared upstream key")
	c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}, starframeTestBilling(body))
	require.Error(t, err)
	require.Len(t, upstream.requests, 2)
}
