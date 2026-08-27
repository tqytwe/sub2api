package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mobileVideoGroupStoreFake struct {
	groups []service.Group
	err    error
}

func (f *mobileVideoGroupStoreFake) GetNextChatSelectableGroups(context.Context, int64) ([]service.Group, error) {
	return f.groups, f.err
}

type mobileVideoTaskStoreFake struct {
	created []service.MobileTaskCreateInput
	tasks   map[string]*service.MobileTask
}

func (f *mobileVideoTaskStoreFake) Create(_ context.Context, _ int64, input service.MobileTaskCreateInput) (*service.MobileTask, error) {
	if f.tasks == nil {
		f.tasks = map[string]*service.MobileTask{}
	}
	if existing := f.tasks[input.ClientRequestID]; existing != nil {
		return existing, nil
	}
	f.created = append(f.created, input)
	task, err := service.NewMobileTask("video-task-1", input.Kind, input.Operation, input.ClientRequestID, time.Now())
	if err != nil {
		return nil, err
	}
	task.Resource = input.Resource
	f.tasks[input.ClientRequestID] = &task
	return &task, nil
}

func (f *mobileVideoTaskStoreFake) List(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
	items := make([]service.MobileTask, 0, len(f.tasks))
	for _, task := range f.tasks {
		items = append(items, *task)
	}
	return &service.MobileTaskPage{Items: items, Total: int64(len(items)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func (f *mobileVideoTaskStoreFake) Get(_ context.Context, _ int64, id string) (*service.MobileTask, error) {
	for _, task := range f.tasks {
		if task.ID == id {
			return task, nil
		}
	}
	return nil, service.ErrMobileTaskNotFound
}

func (f *mobileVideoTaskStoreFake) Cancel(context.Context, int64, string) (*service.MobileTask, error) {
	return nil, service.ErrMobileTaskNotCancellable
}

func (f *mobileVideoTaskStoreFake) Retry(context.Context, int64, string, string) (*service.MobileTask, error) {
	return nil, service.ErrMobileTaskNotRetryable
}

func TestMobileVideoHandlerRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newMobileVideoHandlerWithDependencies(&mobileVideoTaskStoreFake{}, &mobileVideoGroupStoreFake{})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/mobile/video/bootstrap", nil)
	h.Bootstrap(ctx)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestMobileVideoHandlerBootstrapFailsClosedForNonVideoGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.02
	h := newMobileVideoHandlerWithDependencies(&mobileVideoTaskStoreFake{}, &mobileVideoGroupStoreFake{groups: []service.Group{
		{ID: 1, Name: "chat", Platform: service.PlatformOpenAI, Status: service.StatusActive},
		{ID: 2, Name: "video", Platform: service.PlatformGrok, Status: service.StatusActive, VideoPrice720P: &price, DefaultMappedModel: "grok-imagine-video-1.5"},
	}})
	recorder := performMobileVideoRequest(h.Bootstrap, http.MethodGet, "/mobile/video/bootstrap", nil, 42)
	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data mobileVideoBootstrap `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Groups, 2)
	require.False(t, envelope.Data.Groups[0].VideoAvailable)
	require.Equal(t, "VIDEO_GROUP_UNAVAILABLE", envelope.Data.Groups[0].VideoUnavailableCode)
	require.True(t, envelope.Data.Groups[1].VideoAvailable)
}

func TestMobileVideoHandlerRejectsUnauthorizedVideoGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.02
	tasks := &mobileVideoTaskStoreFake{}
	h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoGroupStoreFake{groups: []service.Group{{
		ID: 7, Platform: service.PlatformGrok, Status: service.StatusActive, VideoPrice720P: &price,
	}}})
	recorder := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", []byte(`{"group_id":8,"model":"grok-imagine-video-1.5","prompt":"waves","resolution":"720p","duration_seconds":8,"client_request_id":"video-1"}`), 42)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "VIDEO_CAPABILITY_UNAVAILABLE")
	require.Empty(t, tasks.created)
}

func TestMobileVideoHandlerCreatesQueuedTaskWithoutPublicSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.02
	tasks := &mobileVideoTaskStoreFake{}
	h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoGroupStoreFake{groups: []service.Group{{
		ID: 7, Platform: service.PlatformGrok, Status: service.StatusActive, VideoPrice720P: &price,
		DefaultMappedModel: "grok-imagine-video-1.5",
	}}})
	recorder := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", []byte(`{"group_id":7,"model":"grok-imagine-video-1.5","prompt":"waves","resolution":"720p","duration_seconds":8,"client_request_id":"video-1"}`), 42)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Len(t, tasks.created, 1)
	require.Equal(t, service.MobileTaskKindVideo, tasks.created[0].Kind)
	require.NotContains(t, recorder.Body.String(), "api_key")
	require.NotContains(t, recorder.Body.String(), "price")
}

func TestMobileVideoHandlerRejectsUnavailableOrInvalidReferenceAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.02
	imageID := uuid.NewString()
	audioID := uuid.NewString()
	missingID := uuid.NewString()
	baseGroup := service.Group{
		ID: 7, Platform: service.PlatformGrok, Status: service.StatusActive,
		VideoPrice720P: &price, DefaultMappedModel: "grok-imagine-video-1.5",
	}
	assets := newMobileAssetHandlerWithStore(&fakeMobileAssetStore{
		getFunc: func(_ context.Context, userID int64, id string) (*mobileAssetRecord, error) {
			if userID != 42 || id == missingID {
				return nil, errMobileAssetNotFound
			}
			record := mobileAssetTestRecord(id)
			switch id {
			case imageID:
				record.Kind = "image"
				record.ContentType = "image/png"
			case audioID:
				record.Kind = "audio"
				record.ContentType = "audio/mpeg"
			}
			return record, nil
		},
	})

	tests := []struct {
		name       string
		references []string
		status     int
		code       string
	}{
		{
			name:       "audio requires an audio-reference capable model",
			references: []string{audioID},
			status:     http.StatusForbidden,
			code:       "VIDEO_CAPABILITY_UNAVAILABLE",
		},
		{
			name:       "two images exceed the per-model reference maximum",
			references: []string{imageID, imageID},
			status:     http.StatusBadRequest,
			code:       "VIDEO_REQUEST_INVALID",
		},
		{
			name:       "a missing or cross-account asset is not accepted",
			references: []string{missingID},
			status:     http.StatusBadRequest,
			code:       "VIDEO_REQUEST_INVALID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks := &mobileVideoTaskStoreFake{}
			h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoGroupStoreFake{groups: []service.Group{baseGroup}}).SetAssetHandler(assets)
			body, err := json.Marshal(map[string]any{
				"group_id": 7, "model": "grok-imagine-video-1.5", "prompt": "waves",
				"resolution": "720p", "duration_seconds": 8, "client_request_id": "video-reference-" + tt.name,
				"reference_asset_ids": tt.references,
			})
			require.NoError(t, err)
			recorder := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", body, 42)
			require.Equal(t, tt.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), tt.code)
			require.Empty(t, tasks.created)
		})
	}
}

func TestMobileVideoHandlerAcceptsAuthorizedVideoReference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	price := 0.02
	videoID := uuid.NewString()
	tasks := &mobileVideoTaskStoreFake{}
	assets := newMobileAssetHandlerWithStore(&fakeMobileAssetStore{
		getFunc: func(_ context.Context, userID int64, id string) (*mobileAssetRecord, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, videoID, id)
			record := mobileAssetTestRecord(id)
			record.Kind = "video"
			record.ContentType = "video/mp4"
			return record, nil
		},
	})
	h := newMobileVideoHandlerWithDependencies(tasks, &mobileVideoGroupStoreFake{groups: []service.Group{{
		ID: 7, Platform: service.PlatformGrok, Status: service.StatusActive,
		VideoPrice720P: &price, DefaultMappedModel: "grok-imagine-video-1.5",
	}}}).SetAssetHandler(assets)
	body := []byte(`{"group_id":7,"model":"grok-imagine-video-1.5","prompt":"waves","resolution":"720p","duration_seconds":8,"client_request_id":"video-with-reference","reference_asset_ids":["` + videoID + `"]}`)
	recorder := performMobileVideoRequest(h.Create, http.MethodPost, "/mobile/video/jobs", body, 42)
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Len(t, tasks.created, 1)
}

func performMobileVideoRequest(method gin.HandlerFunc, httpMethod, target string, body []byte, userID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytesReader(body))
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	method(ctx)
	return recorder
}

func bytesReader(body []byte) *bytes.Reader { return bytes.NewReader(body) }
