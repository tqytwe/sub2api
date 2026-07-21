package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStudioDeleteRepoStub struct {
	ImageStudioRepository
	deleteJobFn          func(context.Context, int64, string) ([]string, error)
	listExpiredFn        func(context.Context, time.Time) ([]string, error)
	listKeysFn           func(context.Context, string) ([]string, error)
	deleteExpiredFn      func(context.Context, time.Time) (int64, error)
	pendingDeleteKeys    []string
	deleteFailureCalls   int
	assetPurgeCandidates []ImageStudioAssetPurgeCandidate
	listAssetPurgeCalled bool
	markPurgedIDs        []string
}

type imageStudioObjectReconciliationRepoStub struct {
	ImageStudioRepository
	tracked map[string]struct{}
	err     error
}

func (s *imageStudioObjectReconciliationRepoStub) FilterTrackedObjectStorageKeys(
	context.Context,
	[]string,
) (map[string]struct{}, error) {
	return s.tracked, s.err
}

func (s *imageStudioDeleteRepoStub) DeleteJobWithStorageKeys(
	ctx context.Context,
	userID int64,
	jobID string,
) ([]string, error) {
	return s.deleteJobFn(ctx, userID, jobID)
}

func (s *imageStudioDeleteRepoStub) ListExpiredJobIDs(ctx context.Context, before time.Time) ([]string, error) {
	return s.listExpiredFn(ctx, before)
}

func (s *imageStudioDeleteRepoStub) ListAssetStorageKeysForJob(ctx context.Context, jobID string) ([]string, error) {
	return s.listKeysFn(ctx, jobID)
}

func (s *imageStudioDeleteRepoStub) DeleteExpiredJobsBefore(ctx context.Context, before time.Time) (int64, error) {
	return s.deleteExpiredFn(ctx, before)
}

func (s *imageStudioDeleteRepoStub) ListPendingObjectDeletions(context.Context, int) ([]string, error) {
	return append([]string(nil), s.pendingDeleteKeys...), nil
}

func (s *imageStudioDeleteRepoStub) AcknowledgeObjectDeletion(_ context.Context, storageKey string) error {
	for i, key := range s.pendingDeleteKeys {
		if key == storageKey {
			s.pendingDeleteKeys = append(s.pendingDeleteKeys[:i], s.pendingDeleteKeys[i+1:]...)
			break
		}
	}
	return nil
}

func (s *imageStudioDeleteRepoStub) RecordObjectDeletionFailure(context.Context, string, error) error {
	s.deleteFailureCalls++
	return nil
}

func (s *imageStudioDeleteRepoStub) ListExpiredAssetsForPurge(
	context.Context,
	time.Time,
	int,
) ([]ImageStudioAssetPurgeCandidate, error) {
	s.listAssetPurgeCalled = true
	return append([]ImageStudioAssetPurgeCandidate(nil), s.assetPurgeCandidates...), nil
}

func (s *imageStudioDeleteRepoStub) MarkAssetsPurged(
	_ context.Context,
	assetIDs []string,
	_ time.Time,
) (int64, error) {
	s.markPurgedIDs = append(s.markPurgedIDs, assetIDs...)
	return int64(len(assetIDs)), nil
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

func TestImageStudioPurgeExpiredJobsPreservesMetadataWhenAtomicDeleteFails(t *testing.T) {
	deleteErr := errors.New("atomic outbox and metadata delete failed")
	repo := &imageStudioDeleteRepoStub{
		deleteExpiredFn: func(context.Context, time.Time) (int64, error) {
			return 0, deleteErr
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}

	deleted, err := svc.PurgeExpiredJobs(context.Background(), time.Now())

	require.ErrorIs(t, err, deleteErr)
	require.Zero(t, deleted)
}

func TestImageStudioPurgeExpiredJobsKeepsOutboxWhenObjectDeleteFails(t *testing.T) {
	deleteCalled := false
	repo := &imageStudioDeleteRepoStub{
		pendingDeleteKeys: []string{"../invalid.png"},
		deleteExpiredFn: func(context.Context, time.Time) (int64, error) {
			deleteCalled = true
			return 1, nil
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}

	deleted, err := svc.PurgeExpiredJobs(context.Background(), time.Now())

	require.Error(t, err)
	require.Equal(t, int64(1), deleted)
	require.True(t, deleteCalled)
	require.Equal(t, []string{"../invalid.png"}, repo.pendingDeleteKeys)
	require.Equal(t, 2, repo.deleteFailureCalls)
}

func TestImageStudioPurgeExpiredJobsSkipsAssetBytePurgeByDefault(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "asset-live", "image/png", []byte("image"))
	require.NoError(t, err)
	repo := &imageStudioDeleteRepoStub{
		assetPurgeCandidates: []ImageStudioAssetPurgeCandidate{{
			ID:          "asset-live",
			StorageKeys: []string{key},
		}},
		deleteExpiredFn: func(context.Context, time.Time) (int64, error) {
			return 0, nil
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	deleted, err := svc.PurgeExpiredJobs(context.Background(), time.Now())

	require.NoError(t, err)
	require.Zero(t, deleted)
	require.False(t, repo.listAssetPurgeCalled)
	require.Empty(t, repo.markPurgedIDs)
	_, err = store.Read(key)
	require.NoError(t, err)
}

func TestImageStudioPurgeExpiredJobsPurgesAssetBytesWhenEnabled(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "asset-expired", "image/png", []byte("image"))
	require.NoError(t, err)
	repo := &imageStudioDeleteRepoStub{
		assetPurgeCandidates: []ImageStudioAssetPurgeCandidate{{
			ID:          "asset-expired",
			StorageKeys: []string{key},
		}},
		deleteExpiredFn: func(context.Context, time.Time) (int64, error) {
			return 0, nil
		},
	}
	svc := &ImageStudioService{
		repo:       repo,
		assetStore: store,
		settingService: NewSettingService(&imageStudioSettingRepoStub{values: map[string]string{
			SettingKeyImageStudioAssetPurgeEnabled: "true",
		}}, nil),
	}

	deleted, err := svc.PurgeExpiredJobs(context.Background(), time.Now())

	require.NoError(t, err)
	require.Zero(t, deleted)
	require.True(t, repo.listAssetPurgeCalled)
	require.Equal(t, []string{"asset-expired"}, repo.markPurgedIDs)
	_, err = store.Read(key)
	require.Error(t, err)
}

func TestImageStudioReconcileUntrackedObjectsDeletesOnlyOldOrphans(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	trackedKey, err := store.Save(42, "tracked", "image/png", []byte("tracked"))
	require.NoError(t, err)
	orphanKey, err := store.Save(42, "orphan", "image/png", []byte("orphan"))
	require.NoError(t, err)
	for _, key := range []string{trackedKey, orphanKey} {
		path, resolveErr := store.resolve(key)
		require.NoError(t, resolveErr)
		old := time.Now().Add(-2 * imageStudioUntrackedObjectGrace)
		require.NoError(t, os.Chtimes(path, old, old))
	}
	svc := &ImageStudioService{
		repo: &imageStudioObjectReconciliationRepoStub{
			tracked: map[string]struct{}{trackedKey: {}},
		},
		assetStore: store,
	}

	require.NoError(t, svc.reconcileUntrackedObjects(context.Background(), time.Now()))
	_, err = store.Read(trackedKey)
	require.NoError(t, err)
	_, err = store.Read(orphanKey)
	require.Error(t, err)
}

func TestImageStudioReconcileUntrackedObjectsPreservesFilesWhenMetadataLookupFails(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(42, "uncertain", "image/png", []byte("uncertain"))
	require.NoError(t, err)
	path, err := store.resolve(key)
	require.NoError(t, err)
	old := time.Now().Add(-2 * imageStudioUntrackedObjectGrace)
	require.NoError(t, os.Chtimes(path, old, old))
	lookupErr := errors.New("metadata unavailable")
	svc := &ImageStudioService{
		repo:       &imageStudioObjectReconciliationRepoStub{err: lookupErr},
		assetStore: store,
	}

	require.ErrorIs(t, svc.reconcileUntrackedObjects(context.Background(), time.Now()), lookupErr)
	_, readErr := store.Read(key)
	require.NoError(t, readErr)
}
