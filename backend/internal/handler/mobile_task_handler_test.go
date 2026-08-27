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
	"github.com/stretchr/testify/require"
)

type fakeMobileTaskStore struct {
	createFunc     func(context.Context, int64, service.MobileTaskCreateInput) (*service.MobileTask, error)
	listFunc       func(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error)
	getFunc        func(context.Context, int64, string) (*service.MobileTask, error)
	deleteFunc     func(context.Context, int64, string) (*service.MobileTaskDeleteResult, error)
	cancelFunc     func(context.Context, int64, string) (*service.MobileTask, error)
	retryFunc      func(context.Context, int64, string, string) (*service.MobileTask, error)
	transitionFunc func(context.Context, int64, string, service.MobileTaskTransitionInput) (*service.MobileTask, error)
}

func (f *fakeMobileTaskStore) Create(ctx context.Context, userID int64, input service.MobileTaskCreateInput) (*service.MobileTask, error) {
	return f.createFunc(ctx, userID, input)
}

func (f *fakeMobileTaskStore) List(ctx context.Context, userID int64, filter service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
	return f.listFunc(ctx, userID, filter)
}

func (f *fakeMobileTaskStore) Get(ctx context.Context, userID int64, id string) (*service.MobileTask, error) {
	return f.getFunc(ctx, userID, id)
}

func (f *fakeMobileTaskStore) Delete(ctx context.Context, userID int64, id string) (*service.MobileTaskDeleteResult, error) {
	return f.deleteFunc(ctx, userID, id)
}

func (f *fakeMobileTaskStore) Cancel(ctx context.Context, userID int64, id string) (*service.MobileTask, error) {
	return f.cancelFunc(ctx, userID, id)
}

func (f *fakeMobileTaskStore) Retry(ctx context.Context, userID int64, id, requestID string) (*service.MobileTask, error) {
	return f.retryFunc(ctx, userID, id, requestID)
}

func (f *fakeMobileTaskStore) Transition(ctx context.Context, userID int64, id string, input service.MobileTaskTransitionInput) (*service.MobileTask, error) {
	if f.transitionFunc == nil {
		return nil, service.ErrMobileTaskInvalidTransition
	}
	return f.transitionFunc(ctx, userID, id, input)
}

func TestMobileTaskHandlerCreateAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	created := mobileTaskHandlerTestTask("task-1", service.MobileTaskKindChat, service.MobileTaskStatusQueued)
	store := &fakeMobileTaskStore{
		createFunc: func(_ context.Context, userID int64, input service.MobileTaskCreateInput) (*service.MobileTask, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, service.MobileTaskKindChat, input.Kind)
			require.Equal(t, "chat_completion", input.Operation)
			require.Equal(t, "request-1", input.ClientRequestID)
			return created, nil
		},
		listFunc: func(_ context.Context, userID int64, filter service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, service.MobileTaskKindChat, filter.Kind)
			require.Equal(t, service.MobileTaskStatusQueued, filter.Status)
			require.Equal(t, 2, filter.Page)
			require.Equal(t, 5, filter.PageSize)
			return &service.MobileTaskPage{Items: []service.MobileTask{*created}, Total: 1, Page: 2, PageSize: 5, Pages: 1}, nil
		},
	}
	h := newMobileTaskHandlerWithStore(store)

	create := performMobileTaskHandlerRequest(h.Create, http.MethodPost, "/mobile/tasks", []byte(`{"kind":"chat","operation":"chat_completion","client_request_id":"request-1"}`), 42, nil)
	require.Equal(t, http.StatusCreated, create.Code)
	require.Contains(t, create.Body.String(), `"kind":"chat"`)
	require.Equal(t, "private, no-store", create.Header().Get("Cache-Control"))

	list := performMobileTaskHandlerRequest(h.List, http.MethodGet, "/mobile/tasks?kind=chat&status=queued&page=2&page_size=5", nil, 42, nil)
	require.Equal(t, http.StatusOK, list.Code)
	var envelope struct {
		Data service.MobileTaskPage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &envelope))
	require.Equal(t, int64(1), envelope.Data.Total)
	require.Len(t, envelope.Data.Items, 1)
}

func TestMobileTaskHandlerGetCancelAndRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cancelled := mobileTaskHandlerTestTask("task-1", service.MobileTaskKindImage, service.MobileTaskStatusCancelled)
	retry := mobileTaskHandlerTestTask("task-2", service.MobileTaskKindImage, service.MobileTaskStatusQueued)
	retry.RetryOf = "task-1"
	store := &fakeMobileTaskStore{
		getFunc: func(_ context.Context, userID int64, id string) (*service.MobileTask, error) {
			require.Equal(t, int64(51), userID)
			require.Equal(t, "task-1", id)
			return cancelled, nil
		},
		cancelFunc: func(_ context.Context, userID int64, id string) (*service.MobileTask, error) {
			require.Equal(t, int64(51), userID)
			require.Equal(t, "task-1", id)
			return cancelled, nil
		},
		retryFunc: func(_ context.Context, userID int64, id, requestID string) (*service.MobileTask, error) {
			require.Equal(t, int64(51), userID)
			require.Equal(t, "task-1", id)
			require.Equal(t, "retry-request-1", requestID)
			return retry, nil
		},
	}
	h := newMobileTaskHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: "task-1"}}

	get := performMobileTaskHandlerRequest(h.Get, http.MethodGet, "/mobile/tasks/task-1", nil, 51, params)
	cancel := performMobileTaskHandlerRequest(h.Cancel, http.MethodPost, "/mobile/tasks/task-1/cancel", nil, 51, params)
	retryResponse := performMobileTaskHandlerRequest(h.Retry, http.MethodPost, "/mobile/tasks/task-1/retry", []byte(`{"client_request_id":"retry-request-1"}`), 51, params)
	require.Equal(t, http.StatusOK, get.Code)
	require.Equal(t, http.StatusOK, cancel.Code)
	require.Equal(t, http.StatusCreated, retryResponse.Code)
	require.Contains(t, cancel.Body.String(), `"status":"cancelled"`)
	require.Contains(t, retryResponse.Body.String(), `"retry_of":"task-1"`)
}

func TestMobileTaskHandlerDeleteAndImageHistory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	imageTask := mobileTaskHandlerTestTask("task-image", service.MobileTaskKindImage, service.MobileTaskStatusFailed)
	retry := mobileTaskHandlerTestTask("task-image-retry", service.MobileTaskKindImage, service.MobileTaskStatusQueued)
	retry.RetryOf = "task-image"
	store := &fakeMobileTaskStore{
		getFunc: func(_ context.Context, userID int64, id string) (*service.MobileTask, error) {
			require.Equal(t, int64(52), userID)
			require.Equal(t, "task-image", id)
			return imageTask, nil
		},
		deleteFunc: func(_ context.Context, userID int64, id string) (*service.MobileTaskDeleteResult, error) {
			require.Equal(t, int64(52), userID)
			require.Equal(t, "task-image", id)
			return &service.MobileTaskDeleteResult{ID: id, Deleted: true, DeletedAt: time.Date(2026, time.July, 26, 13, 0, 0, 0, time.UTC)}, nil
		},
		retryFunc: func(_ context.Context, userID int64, id, requestID string) (*service.MobileTask, error) {
			require.Equal(t, int64(52), userID)
			require.Equal(t, "task-image", id)
			require.Equal(t, "image-retry-request", requestID)
			return retry, nil
		},
		listFunc: func(_ context.Context, userID int64, filter service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
			require.Equal(t, int64(52), userID)
			require.Equal(t, service.MobileTaskKindImage, filter.Kind)
			return &service.MobileTaskPage{Items: []service.MobileTask{*imageTask}, Total: 1, Page: 1, PageSize: 20, Pages: 1}, nil
		},
	}
	h := newMobileTaskHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: "task-image"}}

	deleted := performMobileTaskHandlerRequest(h.DeleteImageHistory, http.MethodDelete, "/mobile/image-history/task-image", nil, 52, params)
	require.Equal(t, http.StatusOK, deleted.Code)
	require.Contains(t, deleted.Body.String(), `"deleted":true`)

	retried := performMobileTaskHandlerRequest(h.RetryImageHistory, http.MethodPost, "/mobile/image-history/task-image/retry", []byte(`{"client_request_id":"image-retry-request"}`), 52, params)
	require.Equal(t, http.StatusCreated, retried.Code)
	require.Contains(t, retried.Body.String(), `"retry_of":"task-image"`)

	history := performMobileTaskHandlerRequest(h.ImageHistory, http.MethodGet, "/mobile/image-history", nil, 52, nil)
	require.Equal(t, http.StatusOK, history.Code)
	require.Contains(t, history.Body.String(), `"kind":"image"`)
}

func TestMobileTaskHandlerImageHistoryRejectsNonImageTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	chatTask := mobileTaskHandlerTestTask("task-chat", service.MobileTaskKindChat, service.MobileTaskStatusFailed)
	store := &fakeMobileTaskStore{
		getFunc: func(_ context.Context, _ int64, _ string) (*service.MobileTask, error) {
			return chatTask, nil
		},
	}
	h := newMobileTaskHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: "task-chat"}}

	deleted := performMobileTaskHandlerRequest(h.DeleteImageHistory, http.MethodDelete, "/mobile/image-history/task-chat", nil, 52, params)
	require.Equal(t, http.StatusNotFound, deleted.Code)
	require.Contains(t, deleted.Body.String(), "生图历史不存在")

	retried := performMobileTaskHandlerRequest(h.RetryImageHistory, http.MethodPost, "/mobile/image-history/task-chat/retry", []byte(`{"client_request_id":"retry-chat"}`), 52, params)
	require.Equal(t, http.StatusNotFound, retried.Code)
	require.Contains(t, retried.Body.String(), "生图历史不存在")
}

func TestMobileTaskHandlerRejectsInvalidAndMapsStateErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	createCalled := false
	store := &fakeMobileTaskStore{
		createFunc: func(context.Context, int64, service.MobileTaskCreateInput) (*service.MobileTask, error) {
			createCalled = true
			return nil, nil
		},
		cancelFunc: func(context.Context, int64, string) (*service.MobileTask, error) {
			return nil, service.ErrMobileTaskNotCancellable
		},
		retryFunc: func(context.Context, int64, string, string) (*service.MobileTask, error) {
			return nil, service.ErrMobileTaskNotRetryable
		},
		getFunc: func(context.Context, int64, string) (*service.MobileTask, error) {
			return nil, service.ErrMobileTaskNotFound
		},
	}
	h := newMobileTaskHandlerWithStore(store)
	params := gin.Params{{Key: "id", Value: "missing"}}

	invalid := performMobileTaskHandlerRequest(h.Create, http.MethodPost, "/mobile/tasks", []byte(`{"kind":"unknown","operation":"x","client_request_id":"r"}`), 60, nil)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	require.False(t, createCalled)
	notFound := performMobileTaskHandlerRequest(h.Get, http.MethodGet, "/mobile/tasks/missing", nil, 60, params)
	require.Equal(t, http.StatusNotFound, notFound.Code)
	cancel := performMobileTaskHandlerRequest(h.Cancel, http.MethodPost, "/mobile/tasks/missing/cancel", nil, 60, params)
	require.Equal(t, http.StatusConflict, cancel.Code)
	require.Contains(t, cancel.Body.String(), "当前任务状态不能取消")
	retry := performMobileTaskHandlerRequest(h.Retry, http.MethodPost, "/mobile/tasks/missing/retry", []byte(`{"client_request_id":"retry-1"}`), 60, params)
	require.Equal(t, http.StatusConflict, retry.Code)
	require.Contains(t, retry.Body.String(), "当前任务状态不能重试")
}

func TestMobileTaskHandlerAcceptsVideoTask(t *testing.T) {
	store := &fakeMobileTaskStore{createFunc: func(_ context.Context, _ int64, input service.MobileTaskCreateInput) (*service.MobileTask, error) {
		task, err := service.NewMobileTask("video-task-1", input.Kind, input.Operation, input.ClientRequestID, time.Now())
		return &task, err
	}}
	h := newMobileTaskHandlerWithStore(store)
	response := performMobileTaskHandlerRequest(h.Create, http.MethodPost, "/mobile/tasks", []byte(`{"kind":"video","operation":"video_generate","client_request_id":"video-request-1"}`), 60, nil)
	require.Equal(t, http.StatusCreated, response.Code)
	require.Contains(t, response.Body.String(), `"kind":"video"`)
}

func TestMobileTaskHandlerRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	store := &fakeMobileTaskStore{
		listFunc: func(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error) {
			called = true
			return nil, nil
		},
	}
	h := newMobileTaskHandlerWithStore(store)
	recorder := performMobileTaskHandlerRequest(h.List, http.MethodGet, "/mobile/tasks", nil, 0, nil)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.False(t, called)
}

func performMobileTaskHandlerRequest(method gin.HandlerFunc, httpMethod, target string, body []byte, userID int64, params gin.Params) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytes.NewReader(body))
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	ctx.Params = params
	if userID > 0 {
		ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	method(ctx)
	return recorder
}

func mobileTaskHandlerTestTask(id string, kind service.MobileTaskKind, status service.MobileTaskStatus) *service.MobileTask {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	return &service.MobileTask{
		ID:              id,
		Kind:            kind,
		Operation:       string(kind) + "_operation",
		Status:          status,
		Progress:        0,
		Cancellable:     service.CanCancelMobileTask(service.MobileTask{Status: status}),
		Retryable:       status == service.MobileTaskStatusCancelled,
		ClientRequestID: "request-1",
		Artifacts:       []service.MobileTaskArtifact{},
		CreatedAt:       now,
		Version:         service.MobileTaskProtocolVersion,
	}
}
