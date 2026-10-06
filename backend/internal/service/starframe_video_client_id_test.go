//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestStarframeEchoesOriginalClientTaskIDInCreateAndStatus(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{responses: []*http.Response{
		grokMediaContentStatusResponse(`{"id":"task-1","status":"queued"}`),
		grokMediaContentStatusResponse(`{"id":"task-1","status":"completed","metadata":{"url":"/v1/videos/task-1/content"}}`),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: &starframeTestCache{}, httpUpstream: upstream}
	owner := StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}
	body := []byte(`{"model":"ch-custom-fast","prompt":"waves","mode":"references","client_task_id":"order.1","duration":10,"resolution":"720p"}`)
	c, recorder := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	result, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, owner, starframeTestBilling(body))
	require.NoError(t, err)
	require.Equal(t, "order.1", gjson.Get(recorder.Body.String(), "client_task_id").String())
	task, err := svc.LoadStarframeVideoTask(context.Background(), result.ResponseID, owner)
	require.NoError(t, err)
	c, recorder = grokMediaContentTestContext(http.MethodGet, "/v1/videos/"+result.ResponseID, nil)
	_, err = svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointStatusLegacy, task, nil, owner)
	require.NoError(t, err)
	require.Equal(t, "order.1", gjson.Get(recorder.Body.String(), "client_task_id").String())
}

func TestStarframeRejectsMissingOrNonpositiveVideoPricing(t *testing.T) {
	svc := &OpenAIGatewayService{}
	require.Error(t, svc.ValidateStarframeVideoPricing(context.Background(), nil, "ch-custom", "720p"))
	key := &APIKey{Group: &Group{}}
	require.Error(t, svc.ValidateStarframeVideoPricing(context.Background(), key, "ch-custom", "720p"))
	price := 0.02
	key.Group.VideoModelPrices = map[string]map[string]float64{"ch-custom": {"720p": price}}
	require.NoError(t, svc.ValidateStarframeVideoPricing(context.Background(), key, "ch-custom", "720p"))
	require.Error(t, svc.ValidateStarframeVideoPricing(context.Background(), key, "ch-custom-fast", "720p"))
	price = 0
	key.Group.VideoPrice720P = &price
	require.Error(t, svc.ValidateStarframeVideoPricing(context.Background(), key, "ch-unknown", "720p"))
}
