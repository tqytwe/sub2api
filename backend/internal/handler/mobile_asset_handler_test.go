package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeMobileAssetStorage struct {
	key         string
	contentType string
	data        []byte
	saveCount   int
	deleted     bool
	deleteFunc  func() error
}

func (s *fakeMobileAssetStorage) Save(_ context.Context, key, contentType string, data []byte) (string, error) {
	s.saveCount++
	s.key, s.contentType, s.data = key, contentType, append([]byte(nil), data...)
	return "/private/" + key, nil
}

func (s *fakeMobileAssetStorage) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	if key != s.key || s.deleted {
		return nil, "", errMobileAssetNotFound
	}
	return io.NopCloser(bytes.NewReader(s.data)), s.contentType, nil
}

func (s *fakeMobileAssetStorage) Delete(_ context.Context, key string) error {
	if s.deleteFunc != nil {
		if err := s.deleteFunc(); err != nil {
			return err
		}
	}
	if key == s.key {
		s.deleted = true
	}
	return nil
}

func TestMobileAssetDeleteHidesMetadataBeforeObjectCleanup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	metadataDeleted := false
	store := &fakeMobileAssetStore{
		getFunc: func(context.Context, int64, string) (*mobileAssetRecord, error) {
			return mobileAssetTestRecord(assetID), nil
		},
		softDeleteFunc: func(context.Context, int64, string) error {
			metadataDeleted = true
			return nil
		},
	}
	storage := &fakeMobileAssetStorage{
		key: "users/8/image.png",
		deleteFunc: func() error {
			require.True(t, metadataDeleted)
			return io.ErrUnexpectedEOF
		},
	}
	handler := newMobileAssetHandlerWithStoreAndStorage(store, storage)
	params := gin.Params{{Key: "id", Value: assetID}}

	recorder := performMobileAssetRequest(handler.Delete, http.MethodDelete, "/mobile/assets/"+assetID, nil, 8, params)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"cleanup_pending":true`)
}

type fakeMobileAssetStore struct {
	createFunc     func(context.Context, int64, mobileAssetCreateInput) (*mobileAssetRecord, error)
	listFunc       func(context.Context, int64, mobileAssetListFilter) ([]mobileAssetRecord, int64, error)
	getFunc        func(context.Context, int64, string) (*mobileAssetRecord, error)
	softDeleteFunc func(context.Context, int64, string) error
}

func (f *fakeMobileAssetStore) Create(ctx context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
	return f.createFunc(ctx, userID, input)
}

func (f *fakeMobileAssetStore) List(ctx context.Context, userID int64, filter mobileAssetListFilter) ([]mobileAssetRecord, int64, error) {
	return f.listFunc(ctx, userID, filter)
}

func (f *fakeMobileAssetStore) Get(ctx context.Context, userID int64, id string) (*mobileAssetRecord, error) {
	return f.getFunc(ctx, userID, id)
}

func (f *fakeMobileAssetStore) SoftDelete(ctx context.Context, userID int64, id string) error {
	return f.softDeleteFunc(ctx, userID, id)
}

func TestMobileAssetHandlerRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	store := &fakeMobileAssetStore{
		listFunc: func(context.Context, int64, mobileAssetListFilter) ([]mobileAssetRecord, int64, error) {
			called = true
			return nil, 0, nil
		},
	}
	handler := newMobileAssetHandlerWithStore(store)

	recorder := performMobileAssetRequest(handler.List, http.MethodGet, "/mobile/assets", nil, 0, nil)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.False(t, called)
}

func TestMobileAssetHandlerRejectsInvalidCreateInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	called := false
	store := &fakeMobileAssetStore{
		createFunc: func(context.Context, int64, mobileAssetCreateInput) (*mobileAssetRecord, error) {
			called = true
			return nil, nil
		},
	}
	handler := newMobileAssetHandlerWithStore(store)
	body := []byte(`{"kind":"archive","source":"share","storage_key":"x","original_name":"x.zip","content_type":"application/zip","byte_size":1}`)

	recorder := performMobileAssetRequest(handler.Create, http.MethodPost, "/mobile/assets", body, 7, nil)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.False(t, called)
	require.Contains(t, recorder.Body.String(), "素材类型不支持")
}

func TestMobileAssetHandlerGetAndDeleteAreUserIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	ownerID := int64(8)
	deleted := false
	store := &fakeMobileAssetStore{
		getFunc: func(_ context.Context, userID int64, id string) (*mobileAssetRecord, error) {
			if userID != ownerID || id != assetID || deleted {
				return nil, errMobileAssetNotFound
			}
			return mobileAssetTestRecord(assetID), nil
		},
		softDeleteFunc: func(_ context.Context, userID int64, id string) error {
			if userID != ownerID || id != assetID || deleted {
				return errMobileAssetNotFound
			}
			deleted = true
			return nil
		},
	}
	storage := &fakeMobileAssetStorage{key: "users/8/image.png", contentType: "image/png", data: []byte("image")}
	handler := newMobileAssetHandlerWithStoreAndStorage(store, storage)
	params := gin.Params{{Key: "id", Value: assetID}}

	foreignGet := performMobileAssetRequest(handler.Get, http.MethodGet, "/mobile/assets/"+assetID, nil, 7, params)
	foreignDelete := performMobileAssetRequest(handler.Delete, http.MethodDelete, "/mobile/assets/"+assetID, nil, 7, params)
	require.Equal(t, http.StatusNotFound, foreignGet.Code)
	require.Equal(t, http.StatusNotFound, foreignDelete.Code)
	require.False(t, deleted)

	ownerGet := performMobileAssetRequest(handler.Get, http.MethodGet, "/mobile/assets/"+assetID, nil, ownerID, params)
	ownerDelete := performMobileAssetRequest(handler.Delete, http.MethodDelete, "/mobile/assets/"+assetID, nil, ownerID, params)
	require.Equal(t, http.StatusOK, ownerGet.Code)
	require.Equal(t, http.StatusOK, ownerDelete.Code)
	require.True(t, deleted)
}

func TestMobileAssetHandlerSanitizesMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	store := &fakeMobileAssetStore{
		createFunc: func(_ context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
			require.Equal(t, int64(12), userID)
			require.Equal(t, map[string]any{"width": float64(1024), "height": float64(768)}, input.Metadata)
			record := mobileAssetTestRecord(assetID)
			record.Metadata = input.Metadata
			return record, nil
		},
	}
	handler := newMobileAssetHandlerWithStore(store)
	body := []byte(`{
		"kind":"image",
		"source":"share",
		"storage_key":"users/12/image.png",
		"original_name":"image.png",
		"content_type":"image/png",
		"byte_size":2048,
		"metadata":{
			"width":1024,
			"height":768,
			"token":"secret-token",
			"api_key":"secret-key",
			"prompt":"full private prompt",
			"chat":"full private chat"
		}
	}`)

	recorder := performMobileAssetRequest(handler.Create, http.MethodPost, "/mobile/assets", body, 12, nil)

	require.Equal(t, http.StatusCreated, recorder.Code)
	responseBody := recorder.Body.String()
	require.Contains(t, responseBody, `"width":1024`)
	require.NotContains(t, responseBody, "secret-token")
	require.NotContains(t, responseBody, "secret-key")
	require.NotContains(t, responseBody, "full private prompt")
	require.NotContains(t, responseBody, "full private chat")
	require.NotContains(t, responseBody, `"token"`)
	require.NotContains(t, responseBody, `"prompt"`)
	require.NotContains(t, responseBody, `"chat"`)
}

func TestMobileAssetHandlerListsWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	store := &fakeMobileAssetStore{
		listFunc: func(_ context.Context, userID int64, filter mobileAssetListFilter) ([]mobileAssetRecord, int64, error) {
			require.Equal(t, int64(15), userID)
			require.Equal(t, mobileAssetListFilter{Kind: "image", Status: "ready", Page: 2, PageSize: 3}, filter)
			return []mobileAssetRecord{*mobileAssetTestRecord(assetID)}, 7, nil
		},
	}
	handler := newMobileAssetHandlerWithStore(store)

	recorder := performMobileAssetRequest(handler.List, http.MethodGet, "/mobile/assets?kind=image&status=ready&page=2&page_size=3", nil, 15, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			Items    []mobileAssetRecord `json:"items"`
			Total    int64               `json:"total"`
			Page     int                 `json:"page"`
			PageSize int                 `json:"page_size"`
			Pages    int                 `json:"pages"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data.Items, 1)
	require.Equal(t, assetID, envelope.Data.Items[0].ID)
	require.Equal(t, int64(7), envelope.Data.Total)
	require.Equal(t, 2, envelope.Data.Page)
	require.Equal(t, 3, envelope.Data.PageSize)
	require.Equal(t, 3, envelope.Data.Pages)
	require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
}

func TestMobileAssetHandlerUploadsAndStreamsAuthenticatedContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	storage := &fakeMobileAssetStorage{}
	store := &fakeMobileAssetStore{
		createFunc: func(_ context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
			require.Equal(t, int64(23), userID)
			require.Equal(t, "image", input.Kind)
			require.Equal(t, "upload", input.Source)
			require.NotEmpty(t, input.SHA256)
			require.Contains(t, input.StorageKey, "mobile-assets/23/")
			record := mobileAssetTestRecord(assetID)
			record.StorageKey = input.StorageKey
			record.ContentType = input.ContentType
			record.ByteSize = input.ByteSize
			return record, nil
		},
		getFunc: func(_ context.Context, userID int64, id string) (*mobileAssetRecord, error) {
			require.Equal(t, int64(23), userID)
			require.Equal(t, assetID, id)
			record := mobileAssetTestRecord(assetID)
			record.StorageKey = storage.key
			record.ContentType = storage.contentType
			record.ByteSize = int64(len(storage.data))
			return record, nil
		},
	}
	handler := newMobileAssetHandlerWithStoreAndStorage(store, storage)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "sample.png")
	require.NoError(t, err)
	_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/mobile/assets", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 23})
	handler.Upload(ctx)

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.NotEmpty(t, storage.data)
	require.NotContains(t, recorder.Body.String(), "storage_key")
	require.Contains(t, recorder.Body.String(), "/api/v1/mobile/assets/"+assetID+"/content")

	content := performMobileAssetRequest(handler.Content, http.MethodGet, "/mobile/assets/"+assetID+"/content", nil, 23, gin.Params{{Key: "id", Value: assetID}})
	require.Equal(t, http.StatusOK, content.Code)
	require.Equal(t, storage.data, content.Body.Bytes())
}

func TestMobileAssetHandlerUploadReplaysWithoutSavingBytesAgain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(
		newMobileReplayIdempotencyRepo(),
		service.DefaultIdempotencyConfig(),
	))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(previous)
	})

	assetID := uuid.NewString()
	storage := &fakeMobileAssetStorage{}
	var createCount int
	store := &fakeMobileAssetStore{
		createFunc: func(_ context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
			createCount++
			require.Equal(t, int64(23), userID)
			require.NotEmpty(t, input.StorageKey)
			record := mobileAssetTestRecord(assetID)
			record.StorageKey = input.StorageKey
			record.ContentType = input.ContentType
			record.ByteSize = input.ByteSize
			return record, nil
		},
	}
	handler := newMobileAssetHandlerWithStoreAndStorage(store, storage)

	perform := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "retry.png")
		require.NoError(t, err)
		_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/mobile/assets", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Idempotency-Key", "asset-upload-replay-1")
		context, _ := gin.CreateTestContext(recorder)
		context.Request = request
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 23})
		handler.Upload(context)
		return recorder
	}

	first := perform()
	second := perform()
	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 1, storage.saveCount)
	require.Equal(t, 1, createCount)
	require.Contains(t, second.Body.String(), assetID)
}

func TestMobileAssetHandlerUploadScopesSameIdempotencyKeyByAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(
		newMobileReplayIdempotencyRepo(),
		service.DefaultIdempotencyConfig(),
	))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(previous)
	})

	storage := &fakeMobileAssetStorage{}
	createdFor := make([]int64, 0, 2)
	store := &fakeMobileAssetStore{
		createFunc: func(_ context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
			createdFor = append(createdFor, userID)
			record := mobileAssetTestRecord(uuid.NewString())
			record.StorageKey = input.StorageKey
			record.ContentType = input.ContentType
			record.ByteSize = input.ByteSize
			return record, nil
		},
	}
	handler := newMobileAssetHandlerWithStoreAndStorage(store, storage)

	perform := func(userID int64) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "shared-key.png")
		require.NoError(t, err)
		_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/mobile/assets", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Idempotency-Key", "shared-mobile-key-1")
		context, _ := gin.CreateTestContext(recorder)
		context.Request = request
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		handler.Upload(context)
		return recorder
	}

	first := perform(23)
	second := perform(24)
	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)
	require.Empty(t, second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, []int64{23, 24}, createdFor)
	require.Equal(t, 2, storage.saveCount)
}

func performMobileAssetRequest(method gin.HandlerFunc, httpMethod, target string, body []byte, userID int64, params gin.Params) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(httpMethod, target, bytes.NewReader(body))
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Params = params
	if userID > 0 {
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	method(context)
	return recorder
}

func mobileAssetTestRecord(id string) *mobileAssetRecord {
	now := time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	return &mobileAssetRecord{
		ID:           id,
		Kind:         "image",
		Source:       "share",
		StorageKey:   "users/8/image.png",
		OriginalName: "image.png",
		ContentType:  "image/png",
		ByteSize:     2048,
		Status:       "ready",
		Metadata:     map[string]any{"width": float64(1024)},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
