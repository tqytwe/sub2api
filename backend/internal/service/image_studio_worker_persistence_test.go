package service

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStudioPersistenceRepoStub struct {
	ImageStudioRepository
	completeErr error
	getItemErr  error
	item        *ImageStudioItem
	asset       *ImageStudioAssetRecord
	actualCost  *float64
}

func (s *imageStudioPersistenceRepoStub) CompleteItem(
	_ context.Context,
	_, _, _ string,
	_ string,
	actualCost *float64,
	_ string,
	asset *ImageStudioAssetRecord,
	_ time.Time,
) error {
	if asset != nil {
		copyAsset := *asset
		s.asset = &copyAsset
	}
	if actualCost != nil {
		cost := *actualCost
		s.actualCost = &cost
	}
	return s.completeErr
}

func (s *imageStudioPersistenceRepoStub) GetItem(context.Context, string, string) (*ImageStudioItem, error) {
	if s.getItemErr != nil {
		return nil, s.getItemErr
	}
	if s.item == nil {
		return nil, errors.New("item unavailable")
	}
	copyItem := *s.item
	return &copyItem, nil
}

func TestImageStudioCompleteWorkerItemPersistsOriginalAndPrivateThumbnail(t *testing.T) {
	repo := &imageStudioPersistenceRepoStub{}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := &ImageStudioService{repo: repo, assetStore: store}
	job := &ImageStudioJob{ID: "job-derivative", UserID: 10}
	item := &ImageStudioItem{ID: "item-derivative", JobID: job.ID, SortOrder: 2}
	imageData := encodeImageStudioTestPNG(t, 800, 400)

	err := svc.CompleteWorkerItem(
		context.Background(),
		job,
		item,
		"worker-a",
		&ImageStudioImagePayload{Data: imageData, ContentType: "image/png"},
		0.04,
		nil,
		time.Now().UTC(),
	)

	require.NoError(t, err)
	require.NotNil(t, repo.asset)
	require.Equal(t, item.ID, repo.asset.ID)
	require.Equal(t, 800, repo.asset.Width)
	require.Equal(t, 400, repo.asset.Height)
	require.NotEmpty(t, repo.asset.StorageKey)
	require.NotEmpty(t, repo.asset.ThumbnailStorageKey)
	require.NotEqual(t, repo.asset.StorageKey, repo.asset.ThumbnailStorageKey)
	require.Equal(t, "image/png", repo.asset.ThumbnailContentType)
	require.Positive(t, repo.asset.ThumbnailByteSize)
	original, err := store.Read(repo.asset.StorageKey)
	require.NoError(t, err)
	require.Equal(t, imageData, original)
	thumbnail, err := store.Read(repo.asset.ThumbnailStorageKey)
	require.NoError(t, err)
	require.NotEmpty(t, thumbnail)
}

func TestImageStudioCompleteWorkerItemCapsActualCostAtHeldSnapshot(t *testing.T) {
	repo := &imageStudioPersistenceRepoStub{}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := &ImageStudioService{repo: repo, assetStore: store}
	hold := 0.10
	job := &ImageStudioJob{
		ID:         "job-held-snapshot",
		UserID:     10,
		Count:      1,
		HoldAmount: &hold,
	}
	item := &ImageStudioItem{ID: "item-held-snapshot", JobID: job.ID}

	err := svc.CompleteWorkerItem(
		context.Background(),
		job,
		item,
		"worker-held-snapshot",
		&ImageStudioImagePayload{
			Data:        encodeImageStudioTestPNG(t, 64, 64),
			ContentType: "image/png",
		},
		0.25,
		nil,
		time.Now().UTC(),
	)

	require.NoError(t, err)
	require.NotNil(t, repo.actualCost)
	require.InDelta(t, hold, *repo.actualCost, 0.000001)
}

func TestImageStudioCompleteWorkerItemDeletesObjectsAfterDefinitiveLeaseFailure(t *testing.T) {
	repo := &imageStudioPersistenceRepoStub{
		completeErr: ErrImageStudioLeaseLost,
		item: &ImageStudioItem{
			ID:     "item-lease-lost",
			Status: ImageStudioItemStatusPersisting,
		},
	}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := &ImageStudioService{repo: repo, assetStore: store}

	err := svc.CompleteWorkerItem(
		context.Background(),
		&ImageStudioJob{ID: "job-lease-lost", UserID: 10},
		&ImageStudioItem{ID: "item-lease-lost", JobID: "job-lease-lost"},
		"worker-expired",
		&ImageStudioImagePayload{
			Data:        encodeImageStudioTestPNG(t, 64, 64),
			ContentType: "image/png",
		},
		0.04,
		nil,
		time.Now().UTC(),
	)

	require.ErrorIs(t, err, ErrImageStudioLeaseLost)
	require.Empty(t, imageStudioStoredFiles(t, store.root))
}

func TestImageStudioCompleteWorkerItemKeepsObjectsWhenCommitStateCannotBeVerified(t *testing.T) {
	repo := &imageStudioPersistenceRepoStub{
		completeErr: errors.New("commit response unavailable"),
		getItemErr:  errors.New("database unavailable"),
	}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := &ImageStudioService{repo: repo, assetStore: store}

	err := svc.CompleteWorkerItem(
		context.Background(),
		&ImageStudioJob{ID: "job-uncertain", UserID: 10},
		&ImageStudioItem{ID: "item-uncertain", JobID: "job-uncertain"},
		"worker-uncertain",
		&ImageStudioImagePayload{
			Data:        encodeImageStudioTestPNG(t, 64, 64),
			ContentType: "image/png",
		},
		0.04,
		nil,
		time.Now().UTC(),
	)

	require.Error(t, err)
	require.Len(t, imageStudioStoredFiles(t, store.root), 2)
}

func TestImageStudioCompleteWorkerItemUsesDeterministicKeysAcrossLeaseTakeover(t *testing.T) {
	repo := &imageStudioPersistenceRepoStub{
		completeErr: errors.New("commit response unavailable"),
		getItemErr:  errors.New("database unavailable"),
	}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := &ImageStudioService{repo: repo, assetStore: store}
	job := &ImageStudioJob{ID: "job-takeover", UserID: 10}
	item := &ImageStudioItem{ID: "item-takeover", JobID: job.ID}
	image := &ImageStudioImagePayload{
		Data:        encodeImageStudioTestPNG(t, 64, 64),
		ContentType: "image/png",
	}

	require.Error(t, svc.CompleteWorkerItem(
		context.Background(), job, item, "worker-old", image, 0.04, nil, time.Now().UTC(),
	))
	require.Error(t, svc.CompleteWorkerItem(
		context.Background(), job, item, "worker-new", image, 0.04, nil, time.Now().UTC(),
	))

	require.Len(t, imageStudioStoredFiles(t, store.root), 2)
	require.Contains(t, repo.asset.StorageKey, item.ID+"-original")
	require.Contains(t, repo.asset.ThumbnailStorageKey, item.ID+"-thumbnail")
}

func imageStudioStoredFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	}))
	return files
}
