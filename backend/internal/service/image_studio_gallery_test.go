package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageStudioGalleryRepoStub struct {
	ImageStudioRepository
	jobs      []ImageStudioJob
	total     int
	asset     *ImageStudioAsset
	listPage  int
	listSize  int
	listUser  int64
	assetUser int64
}

func (s *imageStudioGalleryRepoStub) ListJobsPage(
	_ context.Context,
	userID int64,
	page, pageSize int,
) ([]ImageStudioJob, int, error) {
	s.listUser = userID
	s.listPage = page
	s.listSize = pageSize
	out := append([]ImageStudioJob(nil), s.jobs...)
	return out, s.total, nil
}

func (s *imageStudioGalleryRepoStub) GetAsset(_ context.Context, userID int64, _ string) (*ImageStudioAsset, error) {
	s.assetUser = userID
	if s.asset == nil {
		return nil, ErrImageStudioAssetNotFound
	}
	copyAsset := *s.asset
	return &copyAsset, nil
}

func (s *imageStudioGalleryRepoStub) GetJob(_ context.Context, userID int64, _ string) (*ImageStudioJob, error) {
	s.listUser = userID
	if len(s.jobs) == 0 {
		return nil, ErrImageStudioJobNotFound
	}
	copyJob := s.jobs[0]
	copyJob.Assets = append([]ImageStudioAsset(nil), s.jobs[0].Assets...)
	return &copyJob, nil
}

func (s *imageStudioGalleryRepoStub) ListActiveJobs(_ context.Context, userID int64) ([]ImageStudioJob, error) {
	s.listUser = userID
	out := append([]ImageStudioJob(nil), s.jobs...)
	for i := range out {
		out[i].Assets = append([]ImageStudioAsset(nil), s.jobs[i].Assets...)
	}
	return out, nil
}

func TestImageStudioListJobsPageEnrichesPrivateThumbnailURLs(t *testing.T) {
	repo := &imageStudioGalleryRepoStub{
		total: 25,
		jobs: []ImageStudioJob{{
			ID:     "job-gallery",
			Status: ImageStudioJobStatusCompleted,
			Assets: []ImageStudioAsset{{
				ID:                  "asset-gallery",
				StorageKey:          "10/original.png",
				ThumbnailStorageKey: "10/thumbnail.png",
			}},
		}},
	}
	svc := &ImageStudioService{
		repo:           repo,
		settingService: newImageStudioEnabledSettingService(),
	}
	svc.playService = NewPlayService(nil, nil, nil, svc.settingService, nil, nil)

	jobs, total, err := svc.ListJobsPage(context.Background(), 10, 2, 12)

	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Equal(t, int64(10), repo.listUser)
	require.Equal(t, 2, repo.listPage)
	require.Equal(t, 12, repo.listSize)
	require.Len(t, jobs, 1)
	require.Equal(t, "/api/v1/image-studio/assets/asset-gallery/content", jobs[0].Assets[0].PreviewURL)
	require.Equal(t, "/api/v1/image-studio/assets/asset-gallery/thumbnail", jobs[0].Assets[0].ThumbnailURL)
}

func TestImageStudioListJobsPageLeavesHistoricalAssetsOnContentFallback(t *testing.T) {
	repo := &imageStudioGalleryRepoStub{
		total: 1,
		jobs: []ImageStudioJob{{
			ID:     "job-historical",
			Status: ImageStudioJobStatusCompleted,
			Assets: []ImageStudioAsset{{
				ID:         "asset-original-only",
				StorageKey: "10/original.png",
			}},
		}},
	}
	svc := &ImageStudioService{
		repo:           repo,
		settingService: newImageStudioEnabledSettingService(),
	}
	svc.playService = NewPlayService(nil, nil, nil, svc.settingService, nil, nil)

	jobs, _, err := svc.ListJobsPage(context.Background(), 10, 1, 12)

	require.NoError(t, err)
	require.Equal(t, "/api/v1/image-studio/assets/asset-original-only/content", jobs[0].Assets[0].PreviewURL)
	require.Empty(t, jobs[0].Assets[0].ThumbnailURL)
}

func TestImageStudioGetJobEnrichesPrivateThumbnailURL(t *testing.T) {
	repo := &imageStudioGalleryRepoStub{jobs: []ImageStudioJob{{
		ID:     "job-poll",
		Status: ImageStudioJobStatusCompleted,
		Assets: []ImageStudioAsset{{
			ID:                  "asset-poll",
			StorageKey:          "10/original.png",
			ThumbnailStorageKey: "10/thumbnail.png",
		}},
	}}}
	svc := &ImageStudioService{repo: repo}

	job, err := svc.GetJob(context.Background(), 10, "job-poll")

	require.NoError(t, err)
	require.Equal(t, "/api/v1/image-studio/assets/asset-poll/thumbnail", job.Assets[0].ThumbnailURL)
}

func TestImageStudioListActiveJobsDoesNotPublishAssetsBeforeSettlement(t *testing.T) {
	repo := &imageStudioGalleryRepoStub{jobs: []ImageStudioJob{{
		ID:     "job-active",
		Status: ImageStudioJobStatusRunning,
		Assets: []ImageStudioAsset{{
			ID:                  "asset-active",
			StorageKey:          "10/original.png",
			ThumbnailStorageKey: "10/thumbnail.png",
		}},
	}}}
	svc := &ImageStudioService{
		repo:           repo,
		settingService: newImageStudioEnabledSettingService(),
	}
	svc.playService = NewPlayService(nil, nil, nil, svc.settingService, nil, nil)

	jobs, err := svc.ListActiveJobs(context.Background(), 10)

	require.NoError(t, err)
	require.Empty(t, jobs[0].Assets)
}

func TestImageStudioOpenAssetThumbnailReadsOwnedPrivateDerivative(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "thumb-owned", "image/png", []byte("thumbnail"))
	require.NoError(t, err)
	repo := &imageStudioGalleryRepoStub{asset: &ImageStudioAsset{
		ID:                   "asset-owned",
		ThumbnailStorageKey:  key,
		ThumbnailContentType: "image/png",
	}}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	data, contentType, err := svc.OpenAssetThumbnail(context.Background(), 42, "asset-owned")

	require.NoError(t, err)
	require.Equal(t, int64(42), repo.assetUser)
	require.Equal(t, []byte("thumbnail"), data)
	require.Equal(t, "image/png", contentType)
}

func TestImageStudioOpenAssetThumbnailRejectsMissingDerivative(t *testing.T) {
	svc := &ImageStudioService{
		repo: &imageStudioGalleryRepoStub{asset: &ImageStudioAsset{
			ID:         "asset-original-only",
			StorageKey: "42/original.png",
		}},
		assetStore: NewImageStudioAssetStore(t.TempDir()),
	}

	_, _, err := svc.OpenAssetThumbnail(context.Background(), 42, "asset-original-only")

	require.ErrorIs(t, err, ErrImageStudioAssetNotFound)
}

func newImageStudioEnabledSettingService() *SettingService {
	return NewSettingService(&imageStudioSettingRepoStub{values: map[string]string{
		SettingKeyImageStudioEnabled: "true",
	}}, &config.Config{})
}
