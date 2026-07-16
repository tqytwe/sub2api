package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type imageStudioDeleteRepoStub struct {
	ImageStudioRepository
	deleteJobFn func(context.Context, int64, string) ([]string, error)
}

func (s *imageStudioDeleteRepoStub) DeleteJobWithStorageKeys(
	ctx context.Context,
	userID int64,
	jobID string,
) ([]string, error) {
	return s.deleteJobFn(ctx, userID, jobID)
}

func TestImageStudioDeleteJobPreservesAssetsWhenUserDoesNotOwnJob(t *testing.T) {
	ctx := context.Background()
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "asset-1", "image/png", []byte("image"))
	require.NoError(t, err)

	repo := &imageStudioDeleteRepoStub{
		deleteJobFn: func(_ context.Context, userID int64, jobID string) ([]string, error) {
			require.Equal(t, int64(7), userID)
			require.Equal(t, "job-1", jobID)
			return nil, ErrImageStudioJobNotFound
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	err = svc.DeleteJob(ctx, 7, "job-1")

	require.ErrorIs(t, err, ErrImageStudioJobNotFound)
	data, readErr := store.Read(key)
	require.NoError(t, readErr)
	require.Equal(t, []byte("image"), data)
}

func TestImageStudioDeleteJobDeletesAssetsOnlyAfterRepositoryDelete(t *testing.T) {
	ctx := context.Background()
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "asset-1", "image/png", []byte("image"))
	require.NoError(t, err)

	repo := &imageStudioDeleteRepoStub{
		deleteJobFn: func(_ context.Context, _ int64, _ string) ([]string, error) {
			data, readErr := store.Read(key)
			require.NoError(t, readErr)
			require.Equal(t, []byte("image"), data)
			return []string{key}, nil
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	require.NoError(t, svc.DeleteJob(ctx, 42, "job-1"))
	_, err = store.Read(key)
	require.Error(t, err)
}

func TestImageStudioDeleteJobPreservesAssetsWhenAtomicDeleteFails(t *testing.T) {
	ctx := context.Background()
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "asset-1", "image/png", []byte("image"))
	require.NoError(t, err)
	lookupErr := errors.New("storage key lookup failed")

	repo := &imageStudioDeleteRepoStub{
		deleteJobFn: func(_ context.Context, _ int64, _ string) ([]string, error) {
			return nil, lookupErr
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	err = svc.DeleteJob(ctx, 42, "job-1")

	require.ErrorIs(t, err, lookupErr)
	_, readErr := store.Read(key)
	require.NoError(t, readErr)
}

func TestImageStudioDeleteJobReturnsAssetDeletionErrors(t *testing.T) {
	repo := &imageStudioDeleteRepoStub{
		deleteJobFn: func(_ context.Context, _ int64, _ string) ([]string, error) {
			return []string{"../invalid.png"}, nil
		},
	}
	svc := &ImageStudioService{
		repo:       repo,
		assetStore: NewImageStudioAssetStore(t.TempDir()),
	}

	require.Error(t, svc.DeleteJob(context.Background(), 42, "job-1"))
}
