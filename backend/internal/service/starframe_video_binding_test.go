//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"
)

type starframeTestCache struct {
	schedulerTestGatewayCache
	data   map[string][]byte
	claims map[string]bool
	ttl    time.Duration
}

func (s *starframeTestCache) SetGrokVideoPendingBilling(_ context.Context, key string, body []byte, ttl time.Duration) error {
	if s.data == nil {
		s.data = make(map[string][]byte)
	}
	s.data[key] = body
	s.ttl = ttl
	return nil
}
func (s *starframeTestCache) GetGrokVideoPendingBilling(_ context.Context, key string) ([]byte, error) {
	return s.data[key], nil
}
func (s *starframeTestCache) ClaimGrokVideoBilled(_ context.Context, key string, _ time.Duration) (bool, error) {
	if s.claims == nil {
		s.claims = make(map[string]bool)
	}
	if s.claims[key] {
		return false, nil
	}
	s.claims[key] = true
	return true, nil
}

func TestStarframeCreateBindsBeforeReplyAndIsolatesOwner(t *testing.T) {
	cache := &starframeTestCache{}
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1","model":"ch-custom","status":"queued"}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, cache: cache, httpUpstream: upstream}
	owner := StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30}
	body := []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"order-1","duration":10,"resolution":"720p"}`)
	c, recorder := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	result, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, owner, starframeTestBilling(body))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(result.ResponseID, "sfv_"))
	require.Contains(t, recorder.Body.String(), result.ResponseID)
	require.NotContains(t, recorder.Body.String(), "task-1")
	require.Equal(t, 24*time.Hour, cache.ttl)
	forwarded, err := io.ReadAll(upstream.request.Body)
	require.NoError(t, err)
	expectedBody, err := sjson.SetBytes(body, "client_task_id", starframeUpstreamClientTaskID(owner, "order-1"))
	require.NoError(t, err)
	require.Equal(t, expectedBody, forwarded)
	task, err := svc.LoadStarframeVideoTask(context.Background(), result.ResponseID, owner)
	require.NoError(t, err)
	require.Equal(t, "task-1", task.UpstreamID)
	require.Equal(t, int64(9), task.AccountID)
	for _, other := range []StarframeVideoOwner{{UserID: 11, APIKeyID: 20, GroupID: 30}, {UserID: 10, APIKeyID: 21, GroupID: 30}, {UserID: 10, APIKeyID: 20, GroupID: 31}} {
		_, err := svc.LoadStarframeVideoTask(context.Background(), result.ResponseID, other)
		require.Error(t, err)
	}
	c, _ = grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	_, err = svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, body, owner, starframeTestBilling(body))
	require.Error(t, err)
	require.Len(t, upstream.requests, 1, "duplicate client task must not submit again")
}

func TestStarframeStatusRewritesOnlyLocalMetadataAndNeverBills(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{response: grokMediaContentStatusResponse(`{"id":"task-1","status":"completed","metadata":{"url":"https://evil.example/secret","fail_reason":"preserved"}}`)}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	task := &StarframeVideoTask{UpstreamID: "task-1", LocalID: "sfv_test", AccountID: 9, BaseURL: "https://relay.example/v1", UpstreamKeyFingerprint: starframeUpstreamKeyFingerprint("test-only")}
	c, recorder := grokMediaContentTestContext(http.MethodGet, "/v1/videos/sfv_test", nil)
	result, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointStatusLegacy, task, nil, StarframeVideoOwner{})
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "/v1/videos/sfv_test/content")
	require.Contains(t, recorder.Body.String(), "preserved")
	require.NotContains(t, recorder.Body.String(), "evil.example")
	require.NotContains(t, recorder.Body.String(), "task-1")
	require.Zero(t, result.VideoCount)
	require.Empty(t, result.RequestID)
}

func TestStarframeLookupRejectsAccountAndBaseChanges(t *testing.T) {
	for _, change := range []string{"account", "base", "protocol"} {
		a := starframeTestAccount()
		switch change {
		case "account":
			a.ID = 10
		case "base":
			a.Credentials["base_url"] = "https://other.example"
		case "protocol":
			a.Credentials["video_protocol"] = "agnes"
		}
		upstream := &grokMediaContentUpstreamStub{}
		svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
		c, _ := grokMediaContentTestContext(http.MethodGet, "/v1/videos/sfv_test", nil)
		_, err := svc.ForwardStarframeVideo(context.Background(), c, a, AgnesVideoEndpointStatusLegacy, &StarframeVideoTask{UpstreamID: "task-1", LocalID: "sfv_test", AccountID: 9, BaseURL: "https://relay.example/v1"}, nil, StarframeVideoOwner{})
		require.Error(t, err, change)
		require.Empty(t, upstream.requests)
	}
}

func TestStarframeCreateFailsClosedWithoutCache(t *testing.T) {
	upstream := &grokMediaContentUpstreamStub{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, _ := grokMediaContentTestContext(http.MethodPost, "/v1/videos", nil)
	_, err := svc.ForwardStarframeVideo(context.Background(), c, starframeTestAccount(), AgnesVideoEndpointCreate, nil, []byte(`{"model":"ch-custom","prompt":"waves","mode":"references","client_task_id":"order-1","duration":5,"resolution":"720p"}`), StarframeVideoOwner{UserID: 10, APIKeyID: 20, GroupID: 30})
	require.Error(t, err)
	require.Empty(t, upstream.requests)
}
