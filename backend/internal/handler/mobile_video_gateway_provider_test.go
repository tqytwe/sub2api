package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type mobileVideoAssetReaderFake struct {
	items map[string]struct {
		data        []byte
		contentType string
	}
}

func (f mobileVideoAssetReaderFake) ReadAssetForVideo(_ context.Context, _ int64, id string) ([]byte, string, error) {
	item := f.items[id]
	return item.data, item.contentType, nil
}

type mobileVideoAssetURLReaderFake struct {
	mobileVideoAssetReaderFake
	urls map[string]string
}

func (f mobileVideoAssetURLReaderFake) ReferenceURLForVideo(_ context.Context, _ int64, id string) (string, string, error) {
	item := f.items[id]
	return f.urls[id], item.contentType, nil
}

func TestMobileVideoGatewayBodySeparatesReferenceKinds(t *testing.T) {
	provider := &MobileVideoGatewayProvider{assets: mobileVideoAssetReaderFake{items: map[string]struct {
		data        []byte
		contentType string
	}{
		"image": {data: []byte("image"), contentType: "image/png"},
		"video": {data: []byte("video"), contentType: "video/mp4"},
		"audio": {data: []byte("audio"), contentType: "audio/mpeg"},
	}}}
	body, err := provider.mobileVideoGatewayBody(context.Background(), &service.MobileVideoJob{
		UserID: 7, Model: "grok-imagine-video-1.5", Prompt: "waves", Resolution: "720p",
		DurationSeconds: 8, ReferenceAssetIDs: []string{"image", "video", "audio"},
	})
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload["reference_images"], 1)
	require.Len(t, payload["reference_videos"], 1)
	require.Len(t, payload["reference_audios"], 1)
	require.Len(t, payload["images"], 1)
	require.NotEmpty(t, payload["image"])
	require.NotContains(t, payload["reference_images"].([]any)[0].(string), "video/mp4")
	require.NotContains(t, payload["reference_images"].([]any)[0].(string), "audio/mpeg")
}

func TestMobileVideoGatewayBodyUsesSeedanceStructuredTaskContract(t *testing.T) {
	provider := &MobileVideoGatewayProvider{assets: mobileVideoAssetReaderFake{items: map[string]struct {
		data        []byte
		contentType string
	}{
		"image": {data: []byte("image"), contentType: "image/png"},
		"video": {data: []byte("video"), contentType: "video/mp4"},
		"audio": {data: []byte("audio"), contentType: "audio/mpeg"},
	}}}
	body, err := provider.mobileVideoGatewayBody(context.Background(), &service.MobileVideoJob{
		UserID: 7, Model: "seedance-2.0", Prompt: "waves", Resolution: "720p", Ratio: "adaptive",
		DurationSeconds: 8, GenerateAudio: true, ReferenceAssetIDs: []string{"image", "video", "audio"},
	})
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, "seedance-2.0", payload["model"])
	require.Equal(t, float64(8), payload["duration"])
	require.Equal(t, "adaptive", payload["ratio"])
	require.True(t, payload["generate_audio"].(bool))
	require.NotContains(t, payload, "prompt")
	content := payload["content"].([]any)
	require.Len(t, content, 4)
	require.Equal(t, "text", content[0].(map[string]any)["type"])
	require.Equal(t, "reference_image", content[1].(map[string]any)["role"])
	require.Equal(t, "reference_video", content[2].(map[string]any)["role"])
	require.Equal(t, "reference_audio", content[3].(map[string]any)["role"])
}

func TestMobileVideoGatewayBodyUsesPrivateReferenceURLsWhenAvailable(t *testing.T) {
	provider := &MobileVideoGatewayProvider{assets: mobileVideoAssetURLReaderFake{
		mobileVideoAssetReaderFake: mobileVideoAssetReaderFake{items: map[string]struct {
			data        []byte
			contentType string
		}{"video": {data: []byte("not-read"), contentType: "video/mp4"}}},
		urls: map[string]string{"video": "https://storage.example/private/video?signature=short-lived"},
	}}
	body, err := provider.mobileVideoGatewayBody(context.Background(), &service.MobileVideoJob{
		UserID: 7, Model: "seedance-2.0", Prompt: "waves", Resolution: "720p", Ratio: "adaptive",
		DurationSeconds: 8, ReferenceAssetIDs: []string{"video"},
	})
	require.NoError(t, err)
	require.Equal(t, "https://storage.example/private/video?signature=short-lived", gjson.GetBytes(body, "content.1.video_url.url").String())
	require.NotContains(t, string(body), "base64")
}

func TestMobileVideoGatewayRequestUsesTaskScopedIdempotency(t *testing.T) {
	taskID := uuid.NewString()
	job := &service.MobileVideoJob{TaskID: taskID}
	request := mobileVideoGatewayRequest(context.Background(), http.MethodPost, "/v1/videos", nil, job, "create")
	require.Equal(t, service.MobileVideoTaskIdempotencyKey(taskID), request.Header.Get("Idempotency-Key"))
	require.Equal(t, taskID, request.Header.Get("X-Request-ID"))
	require.Equal(t, service.MobileVideoTaskIdempotencyKey(taskID), request.Header.Get(middleware.ClientRequestIDHeader))
	clientRequestID, ok := request.Context().Value(ctxkey.ClientRequestID).(string)
	require.True(t, ok)
	require.Equal(t, service.MobileVideoTaskIdempotencyKey(taskID), clientRequestID)

	pollRequest := mobileVideoGatewayRequest(context.Background(), http.MethodGet, "/v1/videos/"+taskID, nil, job, "poll")
	require.Empty(t, pollRequest.Header.Get("Idempotency-Key"), "polling must not reuse a create idempotency header")
}

type mobileVideoSubscriptionResolverFake struct {
	subscription *service.UserSubscription
	userID       int64
	groupID      int64
}

func (f *mobileVideoSubscriptionResolverFake) GetActiveSubscription(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	f.userID, f.groupID = userID, groupID
	return f.subscription, nil
}

func TestMobileVideoGatewayRestoresSubscriptionAndGroupContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.01
	group := &service.Group{ID: 22, Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription, VideoPrice720P: &price}
	apiKey := &service.APIKey{ID: 9, UserID: 42, GroupID: int64Ptr(22), Group: group, User: &service.User{ID: 42, Role: service.RoleUser, Concurrency: 2}}
	job := &service.MobileVideoJob{TaskID: uuid.NewString(), UserID: 42, GroupID: 22}
	subscription := &service.UserSubscription{ID: 77, UserID: 42, GroupID: 22}
	resolver := &mobileVideoSubscriptionResolverFake{subscription: subscription}
	provider := &MobileVideoGatewayProvider{subscriptions: resolver}
	got, err := provider.activeSubscription(context.Background(), job, apiKey)
	require.NoError(t, err)
	require.Same(t, subscription, got)
	require.Equal(t, int64(42), resolver.userID)
	require.Equal(t, int64(22), resolver.groupID)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = mobileVideoGatewayRequest(context.Background(), http.MethodPost, "/v1/videos", nil, job, "create")
	mobileVideoGatewayAttachIdentity(ctx, job, apiKey, subscription)
	gotSubscription, ok := middleware.GetSubscriptionFromContext(ctx)
	require.True(t, ok)
	require.Same(t, subscription, gotSubscription)
	gotGroup, ok := ctx.Request.Context().Value(ctxkey.Group).(*service.Group)
	require.True(t, ok)
	require.Equal(t, int64(22), gotGroup.ID)
}

func TestMobileVideoGatewayResolvesCompositeProviderFromModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	group := &service.Group{ID: 22, Platform: service.PlatformComposite, Status: service.StatusActive}
	apiKey := &service.APIKey{UserID: 42, GroupID: int64Ptr(22), Group: group}
	job := &service.MobileVideoJob{UserID: 42, GroupID: 22, Model: "grok-imagine-video-1.5"}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = mobileVideoGatewayRequest(context.Background(), http.MethodPost, "/v1/videos", nil, job, "create")
	mobileVideoGatewayAttachIdentity(ctx, job, apiKey, nil)
	ensureCompositeTargetPlatform(ctx, apiKey, job.Model)

	platform, ok := service.ResolvedTargetPlatformFromContext(ctx.Request.Context())
	require.True(t, ok)
	require.Equal(t, service.PlatformGrok, platform)
	require.Equal(t, service.PlatformGrok, effectiveAPIKeyPlatform(ctx, apiKey))
}

func int64Ptr(value int64) *int64 { return &value }
