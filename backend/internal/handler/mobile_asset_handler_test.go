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

	"github.com/DATA-DOG/go-sqlmock"
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
	createFunc             func(context.Context, int64, mobileAssetCreateInput) (*mobileAssetRecord, error)
	listFunc               func(context.Context, int64, mobileAssetListFilter) ([]mobileAssetRecord, int64, error)
	getFunc                func(context.Context, int64, string) (*mobileAssetRecord, error)
	updateOriginalNameFunc func(context.Context, int64, string, string) (*mobileAssetRecord, error)
	softDeleteFunc         func(context.Context, int64, string) error
	syncFunc               func(context.Context, int64, *time.Time) (*mobileAssetSyncResult, error)
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

func (f *fakeMobileAssetStore) UpdateOriginalName(ctx context.Context, userID int64, id, originalName string) (*mobileAssetRecord, error) {
	return f.updateOriginalNameFunc(ctx, userID, id, originalName)
}

func (f *fakeMobileAssetStore) SoftDelete(ctx context.Context, userID int64, id string) error {
	return f.softDeleteFunc(ctx, userID, id)
}

func (f *fakeMobileAssetStore) Sync(ctx context.Context, userID int64, since *time.Time) (*mobileAssetSyncResult, error) {
	if f.syncFunc == nil {
		return &mobileAssetSyncResult{Version: "1970-01-01T00:00:00Z", ETag: `"empty"`, Items: []mobileAssetRecord{}, DeletedIDs: []string{}}, nil
	}
	return f.syncFunc(ctx, userID, since)
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

func TestMobileAssetSyncReturnsDeltaAndHonorsETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	updated := time.Date(2026, 8, 20, 1, 2, 3, 4, time.UTC)
	calledSince := (*time.Time)(nil)
	store := &fakeMobileAssetStore{syncFunc: func(_ context.Context, userID int64, since *time.Time) (*mobileAssetSyncResult, error) {
		require.Equal(t, int64(42), userID)
		calledSince = since
		return &mobileAssetSyncResult{Version: updated.Format(time.RFC3339Nano), ETag: `"sync-v1"`, UpdatedAt: &updated, Items: []mobileAssetRecord{{ID: "asset-1", OriginalName: "clip.mp4"}}, DeletedIDs: []string{"asset-old"}}, nil
	}}
	h := newMobileAssetHandlerWithStore(store)
	first := performMobileAssetRequest(h.Sync, http.MethodGet, "/mobile/assets/sync?since=2026-08-19T00:00:00Z", nil, 42, nil)
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, `"sync-v1"`, first.Header().Get("ETag"))
	require.NotNil(t, calledSince)
	require.Contains(t, first.Body.String(), `"deleted_ids":["asset-old"]`)

	second := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mobile/assets/sync", nil)
	req.Header.Set("If-None-Match", `"sync-v1"`)
	ctx, _ := gin.CreateTestContext(second)
	ctx.Request = req
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
	h.Sync(ctx)
	require.Equal(t, http.StatusNotModified, second.Code)
}

func TestMobileAssetSyncETagIncludesCompleteRevisionSignature(t *testing.T) {
	version := "2026-08-20T01:02:03.000004Z"
	// These states intentionally have the same cursor timestamp and row count.
	// The old max-updated-at/count ETag would have treated them as unchanged.
	first := mobileAssetSyncETag(version, 2, `["asset-a","ready"]|["asset-b","deleted"]`)
	second := mobileAssetSyncETag(version, 2, `["asset-a","ready"]|["asset-c","ready"]`)

	require.NotEqual(t, first, second)
}

func TestSQLMobileAssetStoreSyncUsesCompleteStateForETag(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	updated := time.Date(2026, time.August, 20, 1, 2, 3, 4000, time.UTC)
	signature := `["asset-a","ready"]|["asset-b","deleted"]`
	mock.ExpectQuery(`(?s)SELECT\s+COALESCE\(MAX\(updated_at\).*string_agg\(`).
		WithArgs(int64(91)).
		WillReturnRows(sqlmock.NewRows([]string{"max_updated", "total", "revision_signature"}).
			AddRow(updated, int64(2), signature))
	mock.ExpectQuery(`SELECT .* FROM mobile_assets WHERE user_id = \$1 AND deleted_at IS NULL ORDER BY updated_at ASC, id ASC`).
		WithArgs(int64(91)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "kind", "source", "storage_key", "original_name", "content_type", "byte_size", "sha256", "status", "source_type", "source_id", "metadata", "created_at", "updated_at"}))
	mock.ExpectQuery(`SELECT id::text FROM mobile_assets WHERE user_id = \$1 AND deleted_at IS NOT NULL ORDER BY updated_at ASC, id ASC`).
		WithArgs(int64(91)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	result, err := (&sqlMobileAssetStore{db: db}).Sync(context.Background(), 91, nil)

	require.NoError(t, err)
	require.Equal(t, mobileAssetSyncETag(updated.UTC().Format(time.RFC3339Nano), 2, signature), result.ETag)
	require.NoError(t, mock.ExpectationsWereMet())
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

func TestMobileAssetHandlerRenameValidatesNameAndHidesForeignAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	assetID := uuid.NewString()
	called := false
	store := &fakeMobileAssetStore{
		updateOriginalNameFunc: func(_ context.Context, userID int64, id, name string) (*mobileAssetRecord, error) {
			called = true
			require.Equal(t, int64(8), userID)
			require.Equal(t, assetID, id)
			require.Equal(t, "renamed.mp4", name)
			return nil, errMobileAssetNotFound
		},
	}
	handler := newMobileAssetHandlerWithStore(store)

	invalid := performMobileAssetRequest(
		handler.Rename,
		http.MethodPatch,
		"/mobile/assets/"+assetID,
		[]byte(`{"original_name":"   "}`),
		8,
		gin.Params{{Key: "id", Value: assetID}},
	)
	require.Equal(t, http.StatusBadRequest, invalid.Code)
	require.False(t, called)

	unsafeName := performMobileAssetRequest(
		handler.Rename,
		http.MethodPatch,
		"/mobile/assets/"+assetID,
		[]byte(`{"original_name":"renamed\ncontent-disposition.mp4"}`),
		8,
		gin.Params{{Key: "id", Value: assetID}},
	)
	require.Equal(t, http.StatusBadRequest, unsafeName.Code)
	require.False(t, called)

	foreign := performMobileAssetRequest(
		handler.Rename,
		http.MethodPatch,
		"/mobile/assets/"+assetID,
		[]byte(`{"original_name":"  renamed.mp4  "}`),
		8,
		gin.Params{{Key: "id", Value: assetID}},
	)
	require.Equal(t, http.StatusNotFound, foreign.Code)
	require.True(t, called)
}

func TestMobileAssetHandlerRenameReplaysAndScopesIdempotencyByAccount(t *testing.T) {
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
	updates := make([]int64, 0, 2)
	store := &fakeMobileAssetStore{
		updateOriginalNameFunc: func(_ context.Context, userID int64, id, name string) (*mobileAssetRecord, error) {
			require.Equal(t, assetID, id)
			require.Equal(t, "renamed.mp4", name)
			updates = append(updates, userID)
			record := mobileAssetTestRecord(id)
			record.OriginalName = name
			return record, nil
		},
	}
	handler := newMobileAssetHandlerWithStore(store)

	perform := func(userID int64) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPatch, "/mobile/assets/"+assetID, bytes.NewBufferString(`{"original_name":"renamed.mp4"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "asset-rename-replay-1")
		context, _ := gin.CreateTestContext(recorder)
		context.Request = request
		context.Params = gin.Params{{Key: "id", Value: assetID}}
		context.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		handler.Rename(context)
		return recorder
	}

	first := perform(23)
	second := perform(23)
	otherAccount := perform(24)
	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, http.StatusOK, otherAccount.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Empty(t, otherAccount.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, []int64{23, 24}, updates)
	require.Contains(t, first.Body.String(), `"original_name":"renamed.mp4"`)
	require.Equal(t, "private, no-store", first.Header().Get("Cache-Control"))
}

func TestSQLMobileAssetStoreUpdateOriginalNameIsUserScoped(t *testing.T) {
	assetID := uuid.NewString()
	now := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	columns := []string{"id", "kind", "source", "storage_key", "original_name", "content_type", "byte_size", "sha256", "status", "source_type", "source_id", "metadata", "created_at", "updated_at"}

	t.Run("updates owned active row and returns its metadata", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectQuery(`UPDATE mobile_assets\s+SET original_name = \$1, updated_at = NOW\(\)\s+WHERE id = \$2::uuid AND user_id = \$3 AND deleted_at IS NULL\s+RETURNING`).
			WithArgs("renamed.mp4", assetID, int64(91)).
			WillReturnRows(sqlmock.NewRows(columns).AddRow(
				assetID, "video", "upload", "mobile-assets/91/"+assetID+".mp4", "renamed.mp4", "video/mp4", int64(1024), nil, "ready", nil, nil, []byte(`{}`), now, now,
			))
		mock.ExpectClose()

		record, err := (&sqlMobileAssetStore{db: db}).UpdateOriginalName(context.Background(), 91, assetID, "renamed.mp4")
		require.NoError(t, err)
		require.Equal(t, "renamed.mp4", record.OriginalName)
		require.Equal(t, "video", record.Kind)
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not reveal a missing foreign or deleted row", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectQuery(`UPDATE mobile_assets\s+SET original_name = \$1, updated_at = NOW\(\)\s+WHERE id = \$2::uuid AND user_id = \$3 AND deleted_at IS NULL\s+RETURNING`).
			WithArgs("renamed.mp4", assetID, int64(91)).
			WillReturnRows(sqlmock.NewRows(columns))
		mock.ExpectClose()

		record, err := (&sqlMobileAssetStore{db: db}).UpdateOriginalName(context.Background(), 91, assetID, "renamed.mp4")
		require.Nil(t, record)
		require.ErrorIs(t, err, errMobileAssetNotFound)
		require.NoError(t, db.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
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
