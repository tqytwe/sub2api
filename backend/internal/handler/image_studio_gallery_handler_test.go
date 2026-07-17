package handler

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	imageStudioGalleryAssetID = "11111111-1111-4111-8111-111111111111"
	imageStudioGalleryJobID   = "22222222-2222-4222-8222-222222222222"
)

type imageStudioGalleryHandlerRepoStub struct {
	service.ImageStudioRepository
	listCalls []int
	asset     *service.ImageStudioAsset
	job       *service.ImageStudioJob
}

func (s *imageStudioGalleryHandlerRepoStub) ListJobsPage(
	_ context.Context,
	_ int64,
	page int,
	_ int,
) ([]service.ImageStudioJob, int, error) {
	s.listCalls = append(s.listCalls, page)
	if page > 3 {
		return []service.ImageStudioJob{}, 25, nil
	}
	return []service.ImageStudioJob{{ID: "job-page", Status: service.ImageStudioJobStatusCompleted}}, 25, nil
}

func (s *imageStudioGalleryHandlerRepoStub) GetAsset(context.Context, int64, string) (*service.ImageStudioAsset, error) {
	copyAsset := *s.asset
	return &copyAsset, nil
}

func (s *imageStudioGalleryHandlerRepoStub) GetJob(context.Context, int64, string) (*service.ImageStudioJob, error) {
	copyJob := *s.job
	copyJob.Assets = append([]service.ImageStudioAsset(nil), s.job.Assets...)
	return &copyJob, nil
}

type imageStudioGalleryHandlerSettingRepoStub struct {
	service.SettingRepository
}

func (*imageStudioGalleryHandlerSettingRepoStub) GetMultiple(
	context.Context,
	[]string,
) (map[string]string, error) {
	return map[string]string{service.SettingKeyImageStudioEnabled: "true"}, nil
}

func TestImageStudioListJobsReturnsStablePaginationEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	settingService := service.NewSettingService(&imageStudioGalleryHandlerSettingRepoStub{}, &config.Config{})
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	studio := service.NewImageStudioService(
		&imageStudioGalleryHandlerRepoStub{},
		nil,
		nil,
		nil,
		settingService,
		playService,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	handler := NewImageStudioHandler(studio, nil, nil, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/image-studio/jobs?page=2&page_size=12", nil)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.ListJobs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "job-page", gjson.Get(recorder.Body.String(), "data.jobs.0.id").String())
	require.Equal(t, int64(25), gjson.Get(recorder.Body.String(), "data.total").Int())
	require.Equal(t, int64(2), gjson.Get(recorder.Body.String(), "data.page").Int())
	require.Equal(t, int64(12), gjson.Get(recorder.Body.String(), "data.page_size").Int())
	require.Equal(t, int64(3), gjson.Get(recorder.Body.String(), "data.pages").Int())
}

func TestImageStudioListJobsFallsBackToLastPageWhenRequestedPageIsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &imageStudioGalleryHandlerRepoStub{}
	handler := newImageStudioGalleryHandlerForTest(t, repo, nil)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/image-studio/jobs?page=99&page_size=12", nil)
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.ListJobs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int{99, 3}, repo.listCalls)
	require.Equal(t, int64(3), gjson.Get(recorder.Body.String(), "data.page").Int())
	require.Equal(t, "job-page", gjson.Get(recorder.Body.String(), "data.jobs.0.id").String())
}

func TestImageStudioMediaHandlersUseConsistentPrivateCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewImageStudioAssetStore(t.TempDir())
	original := imageStudioHandlerPNG(t)
	originalKey, err := store.Save(42, imageStudioGalleryAssetID, "image/png", original)
	require.NoError(t, err)
	thumbnailKey, err := store.Save(42, imageStudioGalleryAssetID+"-thumb", "image/png", original)
	require.NoError(t, err)
	asset := &service.ImageStudioAsset{
		ID:                   imageStudioGalleryAssetID,
		StorageKey:           originalKey,
		ContentType:          "image/png",
		ThumbnailStorageKey:  thumbnailKey,
		ThumbnailContentType: "image/png",
	}
	repo := &imageStudioGalleryHandlerRepoStub{
		asset: asset,
		job: &service.ImageStudioJob{
			ID:     imageStudioGalleryJobID,
			UserID: 42,
			Status: service.ImageStudioJobStatusCompleted,
			Assets: []service.ImageStudioAsset{*asset},
		},
	}
	handler := newImageStudioGalleryHandlerForTest(t, repo, store)

	tests := []struct {
		name string
		path string
		id   string
		call func(*gin.Context)
	}{
		{name: "content", path: "/assets/" + imageStudioGalleryAssetID + "/content", id: imageStudioGalleryAssetID, call: handler.AssetContent},
		{name: "thumbnail", path: "/assets/" + imageStudioGalleryAssetID + "/thumbnail", id: imageStudioGalleryAssetID, call: handler.AssetThumbnail},
		{name: "download", path: "/assets/" + imageStudioGalleryAssetID + "/download", id: imageStudioGalleryAssetID, call: handler.AssetDownload},
		{name: "zip", path: "/jobs/" + imageStudioGalleryJobID + "/download", id: imageStudioGalleryJobID, call: handler.JobDownload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, tt.path, nil)
			ctx.Params = gin.Params{{Key: "id", Value: tt.id}}
			ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

			tt.call(ctx)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
		})
	}
}

func newImageStudioGalleryHandlerForTest(
	t *testing.T,
	repo service.ImageStudioRepository,
	store *service.ImageStudioAssetStore,
) *ImageStudioHandler {
	t.Helper()
	settingService := service.NewSettingService(&imageStudioGalleryHandlerSettingRepoStub{}, &config.Config{})
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	studio := service.NewImageStudioService(
		repo,
		store,
		nil,
		nil,
		settingService,
		playService,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	return NewImageStudioHandler(studio, nil, nil, nil)
}

func imageStudioHandlerPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(0, 0, color.RGBA{R: 220, G: 30, B: 70, A: 255})
	img.Set(1, 0, color.RGBA{R: 20, G: 140, B: 220, A: 255})
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}
