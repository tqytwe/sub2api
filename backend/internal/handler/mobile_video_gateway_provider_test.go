package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type mobileVideoExecutionKeyFake struct {
	key            *service.APIKey
	err            error
	restoreErr     error
	validateErr    error
	userID         int64
	groupID        int64
	keyID          int64
	callCount      int
	restoreCalls   int
	validationCall int
}

func (f *mobileVideoExecutionKeyFake) GetMobileVideoExecutionKey(_ context.Context, userID, groupID, keyID int64) (*service.APIKey, error) {
	f.callCount++
	f.userID, f.groupID, f.keyID = userID, groupID, keyID
	return f.key, f.err
}

func (f *mobileVideoExecutionKeyFake) RestoreMobileVideoExecutionSnapshot(*service.MobileVideoExecutionSnapshot) (*service.APIKey, error) {
	f.restoreCalls++
	return f.key, f.restoreErr
}

func (f *mobileVideoExecutionKeyFake) ValidateMobileVideoExecutionKeyForExecution(_ context.Context, userID, groupID, keyID int64) error {
	f.validationCall++
	f.userID, f.groupID, f.keyID = userID, groupID, keyID
	return f.validateErr
}

type mobileVideoGatewaySubscriptionFake struct {
	calls        int
	subscription *service.UserSubscription
	err          error
}

func (f *mobileVideoGatewaySubscriptionFake) GetActiveSubscription(context.Context, int64, int64) (*service.UserSubscription, error) {
	f.calls++
	return f.subscription, f.err
}

type mobileVideoGatewayFake struct {
	grokCreate  int
	grokPoll    int
	grokContent int
	agnesCreate int
	agnesPoll   int
	platforms   []string
	paths       []string
	bodies      [][]byte
}

func (f *mobileVideoGatewayFake) capture(c *gin.Context) {
	if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
		f.platforms = append(f.platforms, platform)
	}
	f.paths = append(f.paths, c.Request.URL.String())
	if c.Request.Body != nil {
		body, _ := io.ReadAll(c.Request.Body)
		f.bodies = append(f.bodies, body)
	}
}

func (f *mobileVideoGatewayFake) GrokVideoGeneration(c *gin.Context) {
	f.grokCreate++
	f.capture(c)
	c.JSON(http.StatusAccepted, gin.H{"id": "grok-task", "status": "queued"})
}

func (f *mobileVideoGatewayFake) GrokVideoStatus(c *gin.Context) {
	f.grokPoll++
	f.capture(c)
	c.JSON(http.StatusOK, gin.H{"id": c.Param("request_id"), "status": "processing", "progress": 50})
}

func (f *mobileVideoGatewayFake) GrokVideoContent(c *gin.Context) {
	f.grokContent++
	f.capture(c)
	c.Data(http.StatusOK, "video/mp4", []byte("video"))
}

func (f *mobileVideoGatewayFake) AgnesVideoCreate(c *gin.Context) {
	f.agnesCreate++
	f.capture(c)
	c.JSON(http.StatusAccepted, gin.H{"id": "internal-task", "task_id": "legacy-task", "video_id": "agnes-task", "status": "queued"})
}

func (f *mobileVideoGatewayFake) AgnesVideoStatus(c *gin.Context) {
	f.agnesPoll++
	f.capture(c)
	c.JSON(http.StatusOK, gin.H{"video_id": c.Query("video_id"), "status": "completed", "metadata": gin.H{"url": "https://provider.example/video.mp4"}})
}

func mobileVideoGatewayTestKey(platform string) *service.APIKey {
	groupID := int64(9)
	return &service.APIKey{
		ID: 41, UserID: 7, Name: "[managed:nextchat] Video Execution/9", Status: service.StatusAPIKeyActive, GroupID: &groupID,
		User:  &service.User{ID: 7, Concurrency: 2},
		Group: &service.Group{ID: groupID, Platform: platform, Status: service.StatusActive, Hydrated: true},
	}
}

func mobileVideoGatewayTestJob(adapter string) *service.MobileVideoJob {
	return &service.MobileVideoJob{
		TaskID: "task-1", UserID: 7, GroupID: 9, ExecutionAPIKeyID: 41, Adapter: adapter,
		Model: "provider-owned-model", Prompt: "test", Resolution: "720p", Ratio: "16:9", DurationSeconds: 8,
	}
}

func mobileVideoGatewayExecutionSnapshot(userID, groupID, keyID int64) *service.MobileVideoExecutionSnapshot {
	groupIDCopy := groupID
	return &service.MobileVideoExecutionSnapshot{
		Version: service.MobileVideoExecutionSnapshotVersion,
		APIKey: service.APIKeyAuthSnapshot{
			Version:  1,
			APIKeyID: keyID,
			UserID:   userID,
			GroupID:  &groupIDCopy,
			Name:     "[managed:nextchat] Video Execution/9",
			User:     service.APIKeyAuthUserSnapshot{ID: userID},
			Group:    &service.APIKeyAuthGroupSnapshot{ID: groupID},
		},
		EffectiveVideoRateMultiplier: 1,
	}
}

func TestMobileVideoGatewayProviderDispatchesByDeclaredAdapter(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok)
	// The model text deliberately looks unrelated to Grok. Dispatch must follow
	// the catalog-pinned adapter rather than a model-name classifier.
	job.Model = "unclassified-video-model"

	result, err := provider.Create(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, "grok-task", result.RequestID)
	require.Equal(t, 1, gateway.grokCreate)
	require.Zero(t, gateway.agnesCreate)
	require.Equal(t, []string{service.PlatformGrok}, gateway.platforms)
	require.Equal(t, 1, key.callCount)
	require.Equal(t, int64(7), key.userID)
	require.Equal(t, int64(9), key.groupID)
	require.Equal(t, int64(41), key.keyID)
}

func TestMobileVideoGatewayProviderNeverFallsBackFromExecutionKey(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{err: errors.New("missing execution key")}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)

	_, err := provider.Create(context.Background(), mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok))
	require.Error(t, err)
	require.Equal(t, "VIDEO_EXECUTION_IDENTITY_UNAVAILABLE", mobileVideoProviderErrorCode(t, err))
	require.Zero(t, gateway.grokCreate)
	require.Zero(t, gateway.agnesCreate)
}

func TestMobileVideoGatewayProviderStopsWhenGroupAccessIsRevoked(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{err: service.ErrGroupNotAllowed}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)

	_, err := provider.Create(context.Background(), mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok))
	require.Error(t, err)
	require.Equal(t, "VIDEO_GROUP_UNAVAILABLE", mobileVideoProviderErrorCode(t, err))
	require.Zero(t, gateway.grokCreate)
}

func TestMobileVideoGatewayProviderFrozenSnapshotDoesNotRecheckRevokedSubscription(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	key.key.Group.SubscriptionType = service.SubscriptionTypeSubscription
	subscriptions := &mobileVideoGatewaySubscriptionFake{err: errors.New("subscription was revoked after task acceptance")}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, subscriptions)
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok)
	job.ExecutionSnapshot = mobileVideoGatewayExecutionSnapshot(job.UserID, job.GroupID, job.ExecutionAPIKeyID)
	job.BillingState = service.MobileVideoBillingStateReserved

	result, err := provider.Create(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, "grok-task", result.RequestID)
	require.Equal(t, 1, key.validationCall, "key/group identity is still checked before dispatch")
	require.Equal(t, 1, key.restoreCalls)
	require.Zero(t, key.callCount, "snapshot jobs must not resolve a fresh entitlement-bound key")
	require.Zero(t, subscriptions.calls, "accepted snapshot jobs must not recheck a revocable subscription")
	require.Equal(t, 1, gateway.grokCreate)
}

func TestMobileVideoGatewayProviderUsesAgnesStatusForContent(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterAgnes)
	job.ProviderRequestID = "video_123"

	result, err := provider.Content(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, "completed", result.Status)
	require.Equal(t, "https://provider.example/video.mp4", result.ArtifactURL)
	require.Zero(t, gateway.grokPoll)
	require.Equal(t, 1, gateway.agnesPoll)
	require.Equal(t, []string{service.PlatformOpenAI}, gateway.platforms)
	require.Len(t, gateway.paths, 1)
	require.Equal(t, "/agnesapi?video_id=video_123", gateway.paths[0])
}

func TestMobileVideoGatewayProviderRejectsAdapterGroupMismatch(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformOpenAI)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)

	_, err := provider.Create(context.Background(), mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok))
	require.Error(t, err)
	require.Equal(t, "VIDEO_ADAPTER_GROUP_MISMATCH", mobileVideoProviderErrorCode(t, err))
	require.Zero(t, gateway.grokCreate)
}

func TestMobileVideoGatewayCreateBodiesMatchTheirDeclaredAdapters(t *testing.T) {
	t.Run("grok", func(t *testing.T) {
		job := mobileVideoGatewayTestJob(service.MobileVideoAdapterGrok)
		method, _, body, err := mobileVideoGatewayRequestFor(job, mobileVideoGatewayOperationCreate)
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, method)
		require.Equal(t, int64(8), gjson.GetBytes(body, "duration").Int())
		require.Equal(t, "16:9", gjson.GetBytes(body, "aspect_ratio").String())
		require.False(t, gjson.GetBytes(body, "ratio").Exists())
		parsed := service.ParseGrokMediaRequest("application/json", body)
		require.Equal(t, 8, parsed.DurationSeconds)
		require.Equal(t, "16:9", parsed.AspectRatio)
	})

	t.Run("agnes", func(t *testing.T) {
		job := mobileVideoGatewayTestJob(service.MobileVideoAdapterAgnes)
		job.Model = service.AgnesVideoDefaultModel
		job.Resolution = service.VideoBillingResolution480P
		job.DurationSeconds = 10
		method, _, body, err := mobileVideoGatewayRequestFor(job, mobileVideoGatewayOperationCreate)
		require.NoError(t, err)
		require.Equal(t, http.MethodPost, method)
		require.Equal(t, int64(1024), gjson.GetBytes(body, "width").Int())
		require.Equal(t, int64(576), gjson.GetBytes(body, "height").Int())
		require.Equal(t, int64(241), gjson.GetBytes(body, "num_frames").Int())
		require.Equal(t, int64(24), gjson.GetBytes(body, "frame_rate").Int())
		for _, forbidden := range []string{"resolution", "duration", "duration_seconds", "ratio", "aspect_ratio", "generate_audio", "watermark"} {
			require.Falsef(t, gjson.GetBytes(body, forbidden).Exists(), "Agnes payload must not contain %s", forbidden)
		}
		resolution, seconds := service.ExtractAgnesVideoBillingMetadata(body)
		require.Equal(t, service.VideoBillingResolution480P, resolution)
		require.Equal(t, 10, seconds)
	})
}

func TestMobileVideoGatewayProviderUsesAgnesVideoIDForSubsequentPolling(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterAgnes)
	job.Model = service.AgnesVideoDefaultModel
	job.Resolution = service.VideoBillingResolution480P
	job.DurationSeconds = 10

	result, err := provider.Create(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, "agnes-task", result.RequestID)
	require.Equal(t, 1, gateway.agnesCreate)
}

func TestMobileVideoGatewayProviderRejectsUnpublishedAgnesPreset(t *testing.T) {
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterAgnes)
	job.Model = service.AgnesVideoDefaultModel
	job.Resolution = service.VideoBillingResolution720P
	job.DurationSeconds = 8

	_, _, _, err := mobileVideoGatewayRequestFor(job, mobileVideoGatewayOperationCreate)
	require.Error(t, err)
	require.Equal(t, "VIDEO_REQUEST_INVALID", mobileVideoProviderErrorCode(t, err))
}

func TestMobileVideoGatewayProviderUsesAgnesAPIPathForOpaqueVideoID(t *testing.T) {
	key := &mobileVideoExecutionKeyFake{key: mobileVideoGatewayTestKey(service.PlatformComposite)}
	gateway := &mobileVideoGatewayFake{}
	provider := newMobileVideoGatewayProviderWithDependencies(key, gateway, nil)
	job := mobileVideoGatewayTestJob(service.MobileVideoAdapterAgnes)
	job.Model = service.AgnesVideoDefaultModel
	job.ProviderRequestID = "opaque-task-id"

	_, err := provider.Poll(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, []string{"/agnesapi?video_id=opaque-task-id"}, gateway.paths)
}

func mobileVideoProviderErrorCode(t *testing.T, err error) string {
	t.Helper()
	var providerErr *service.MobileVideoProviderError
	require.True(t, errors.As(err, &providerErr))
	return providerErr.Code
}
