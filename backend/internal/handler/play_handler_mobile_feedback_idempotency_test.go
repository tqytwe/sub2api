package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// legacyMobileFeedbackIdempotencyRepo embeds the broad play repository
// contract, while implementing only the feedback persistence used by this
// handler test.
type legacyMobileFeedbackIdempotencyRepo struct {
	service.PlayRepository
	createCount int
}

func (r *legacyMobileFeedbackIdempotencyRepo) CreateMobileFeedback(_ context.Context, record service.MobileFeedbackRecord) (*service.MobileFeedbackRecord, error) {
	r.createCount++
	record.ID = int64(r.createCount)
	return &record, nil
}

func TestLegacyMobileFeedbackReplaysKeyedFallbackWithoutDuplicateUpload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(
		newMobileReplayIdempotencyRepo(),
		service.DefaultIdempotencyConfig(),
	))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(previous)
	})

	repo := &legacyMobileFeedbackIdempotencyRepo{}
	storage := &fakeMobileAssetStorage{}
	assetService := service.NewAnnouncementAssetServiceWithResolver(func() (*service.ImageResultUploader, bool) {
		return service.NewImageResultUploader(storage, "", 0, nil), true
	})
	playHandler := NewPlayHandler(service.NewPlayService(repo, nil, nil, nil, nil, nil), nil, assetService)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})
	router.POST("/api/v1/play/mobile-feedback", middleware.ClientRequestID(), playHandler.SubmitMobileFeedback)

	perform := func() *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("title", "旧服务器回退工单"))
		require.NoError(t, writer.WriteField("category", "bug"))
		require.NoError(t, writer.WriteField("content", "网络恢复后不应重复创建反馈工单"))
		part, err := writer.CreateFormFile("screenshots", "legacy-retry.png")
		require.NoError(t, err)
		_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/play/mobile-feedback", &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Idempotency-Key", "legacy-feedback-replay-1")
		request.Header.Set(middleware.ClientRequestIDHeader, "legacy-fallback-request-1")
		router.ServeHTTP(recorder, request)
		return recorder
	}

	first := perform()
	second := perform()

	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)
	require.Equal(t, "true", second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, "legacy-fallback-request-1", second.Header().Get(middleware.ClientRequestIDHeader))
	require.Equal(t, 1, repo.createCount)
	require.Equal(t, 1, storage.saveCount)
}

func TestLegacyMobileFeedbackScopesSameIdempotencyKeyByAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := service.DefaultIdempotencyCoordinator()
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(
		newMobileReplayIdempotencyRepo(),
		service.DefaultIdempotencyConfig(),
	))
	t.Cleanup(func() {
		service.SetDefaultIdempotencyCoordinator(previous)
	})

	repo := &legacyMobileFeedbackIdempotencyRepo{}
	storage := &fakeMobileAssetStorage{}
	assetService := service.NewAnnouncementAssetServiceWithResolver(func() (*service.ImageResultUploader, bool) {
		return service.NewImageResultUploader(storage, "", 0, nil), true
	})
	playHandler := NewPlayHandler(service.NewPlayService(repo, nil, nil, nil, nil, nil), nil, assetService)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID := int64(42)
		if raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer user-"); raw != c.GetHeader("Authorization") {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
				userID = parsed
			}
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
		c.Next()
	})
	router.POST("/api/v1/play/mobile-feedback", middleware.ClientRequestID(), playHandler.SubmitMobileFeedback)

	perform := func(userID int64) *httptest.ResponseRecorder {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.WriteField("title", "跨账号旧反馈"))
		require.NoError(t, writer.WriteField("category", "bug"))
		require.NoError(t, writer.WriteField("content", "legacy fallback key must be account scoped"))
		part, err := writer.CreateFormFile("screenshots", "legacy-account-scope.png")
		require.NoError(t, err)
		_, err = part.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 512)...))
		require.NoError(t, err)
		require.NoError(t, writer.Close())

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/v1/play/mobile-feedback", &body)
		request.Header.Set("Authorization", "Bearer user-"+strconv.FormatInt(userID, 10))
		request.Header.Set("Content-Type", writer.FormDataContentType())
		request.Header.Set("Idempotency-Key", "shared-legacy-key-1")
		router.ServeHTTP(recorder, request)
		return recorder
	}

	first := perform(42)
	second := perform(43)
	require.Equal(t, http.StatusCreated, first.Code)
	require.Equal(t, http.StatusCreated, second.Code)
	require.Empty(t, second.Header().Get("X-Idempotency-Replayed"))
	require.Equal(t, 2, repo.createCount)
	require.Equal(t, 2, storage.saveCount)
}
