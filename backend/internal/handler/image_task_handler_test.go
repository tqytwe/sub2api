package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type asyncImageZeroReader struct{}

func (asyncImageZeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type asyncImageMemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*service.ImageTaskRecord
}

type asyncImageMemoryAssetStore struct {
	data        []byte
	contentType string
	err         error
}

func (s *asyncImageMemoryAssetStore) Save(_ context.Context, _ string, _ string, _ []byte) (string, error) {
	return "", nil
}

func (s *asyncImageMemoryAssetStore) Open(_ context.Context, _ string) (io.ReadCloser, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return io.NopCloser(bytes.NewReader(s.data)), s.contentType, nil
}

func (s *asyncImageMemoryStore) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	s.tasks[task.ID] = &copy
	return nil
}

func (s *asyncImageMemoryStore) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task := s.tasks[id]
	if task == nil {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *task
	copy.Result = append(json.RawMessage(nil), task.Result...)
	copy.Error = append(json.RawMessage(nil), task.Error...)
	return &copy, nil
}

func TestAsyncImageHandlerSubmitAndPoll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	release := make(chan struct{})
	h := &AsyncImageHandler{tasks: tasks}
	h.execute = func(_ string, c *gin.Context) {
		<-release
		c.JSON(http.StatusOK, gin.H{"created": 123, "data": []gin.H{{"url": "https://example.test/image.png"}}})
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	requestCtx, cancelRequest := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`)).WithContext(requestCtx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	require.Equal(t, "3", w.Header().Get("Retry-After"))

	var accepted struct {
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
		PollURL string `json:"poll_url"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &accepted))
	require.Equal(t, service.ImageTaskStatusProcessing, accepted.Status)
	require.Equal(t, "/v1/images/tasks/"+accepted.TaskID, accepted.PollURL)
	require.Equal(t, accepted.PollURL, w.Header().Get("Location"))

	// The detached background request must survive completion of/cancellation
	// from the short submission request.
	cancelRequest()
	close(release)
	require.Eventually(t, func() bool {
		got, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: 7, APIKeyID: 9}, accepted.TaskID)
		return err == nil && got.Status == service.ImageTaskStatusCompleted
	}, time.Second, 10*time.Millisecond)

	pollReq := httptest.NewRequest(http.MethodGet, accepted.PollURL, nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusOK, pollWriter.Code)
	require.Equal(t, "no-store", pollWriter.Header().Get("Cache-Control"))
	require.Empty(t, pollWriter.Header().Get("Retry-After"))
	require.Contains(t, pollWriter.Body.String(), "https://example.test/image.png")
}

// When result storage is not configured the feature is fully disabled: the
// endpoints must return 404 without creating a task or writing to Redis.
func TestAsyncImageHandlerDisabledReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithOptions(store, time.Hour, time.Minute) // enabled == false
	h := &AsyncImageHandler{tasks: tasks}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.POST("/v1/images/generations/async", h.Submit)
	router.GET("/v1/images/tasks/:task_id", h.Get)

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not enabled")

	pollReq := httptest.NewRequest(http.MethodGet, "/v1/images/tasks/imgtask_missing", nil)
	pollWriter := httptest.NewRecorder()
	router.ServeHTTP(pollWriter, pollReq)
	require.Equal(t, http.StatusNotFound, pollWriter.Code)

	// No task was created / persisted.
	require.Empty(t, store.tasks)
}

func TestAsyncImageHandlerGetAssetRequiresTaskOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	assetStore := &asyncImageMemoryAssetStore{data: []byte("image-bytes"), contentType: "image/png"}
	h := NewAsyncImageHandler(tasks, nil, assetStore)

	owner := service.ImageTaskOwner{UserID: 7, APIKeyID: 9}
	task, err := tasks.Create(context.Background(), owner)
	require.NoError(t, err)
	require.NoError(t, tasks.Complete(context.Background(), task.ID, http.StatusOK, json.RawMessage(`{"data":[{"url":"/v1/images/task-assets/images/`+task.ID+`-0.png"}]}`)))

	router := gin.New()
	apiKeyID := int64(9)
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      apiKeyID,
			UserID:  7,
			GroupID: &groupID,
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, AllowImageGeneration: true},
		})
		c.Next()
	})
	router.GET("/v1/images/task-assets/*filepath", h.GetAsset)

	req := httptest.NewRequest(http.MethodGet, "/v1/images/task-assets/images/"+task.ID+"-0.png", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, "image-bytes", w.Body.String())

	apiKeyID = 10
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestAsyncImageHandlerMultipartPartTooLargeReturnsOpenAICompatible413(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
			tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
			openAI := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}
			h := NewAsyncImageHandler(tasks, openAI, nil)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := int64(3)
				c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
					ID:      9,
					UserID:  7,
					GroupID: &groupID,
					Group: &service.Group{
						ID:                   groupID,
						Platform:             platform,
						AllowImageGeneration: true,
					},
				})
				c.Next()
			})
			router.POST("/v1/images/edits/async", h.Submit)

			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			model := "gpt-image-2"
			if platform == service.PlatformGrok {
				model = "grok-imagine-edit"
			}
			require.NoError(t, writer.WriteField("model", model))
			part, err := writer.CreateFormFile("image", "source.png")
			require.NoError(t, err)
			_, err = io.CopyN(part, asyncImageZeroReader{}, (20<<20)+1)
			require.NoError(t, err)
			require.NoError(t, writer.Close())

			req := httptest.NewRequest(http.MethodPost, "/v1/images/edits/async", bytes.NewReader(body.Bytes()))
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
			require.Equal(t, "invalid_request_error", gjson.GetBytes(w.Body.Bytes(), "error.type").String())
			require.Equal(t, "Multipart field image exceeds 20 MiB limit", gjson.GetBytes(w.Body.Bytes(), "error.message").String())
			require.Empty(t, store.tasks)
		})
	}
}

func TestAsyncImageHandlerGrokMalformedMultipartJSONBodyReturnsOpenAICompatible400(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := &asyncImageMemoryStore{tasks: make(map[string]*service.ImageTaskRecord)}
	tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
	openAI := &OpenAIGatewayHandler{gatewayService: &service.OpenAIGatewayService{}}
	h := NewAsyncImageHandler(tasks, openAI, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(3)
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID:      9,
			UserID:  7,
			GroupID: &groupID,
			Group: &service.Group{
				ID:                   groupID,
				Platform:             service.PlatformGrok,
				AllowImageGeneration: true,
			},
		})
		c.Next()
	})
	router.POST("/v1/images/edits/async", h.Submit)

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/images/edits/async",
		strings.NewReader(`{"model":"grok-imagine-edit","prompt":"test"}`),
	)
	req.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "invalid_request_error", gjson.GetBytes(w.Body.Bytes(), "error.type").String())
	require.Equal(t, "multipart boundary is required", gjson.GetBytes(w.Body.Bytes(), "error.message").String())
	require.Empty(t, store.tasks)
}
