package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStudioReferenceLifecycleRepoStub struct {
	ImageStudioRepository
	expiredReferences        []ImageStudioReference
	listExpiredReferencesErr error
	deleteReferenceErr       error
	deleteReferenceCalls     int
	deletedReferenceIDs      []string
	jobReferenceKeys         []string
	listJobReferenceKeysErr  error
	deleteJobKeys            []string
	expiredJobIDs            []string
	assetKeys                []string
	pendingObjectKeys        []string
	deleteExpiredCalls       int
}

func (s *imageStudioReferenceLifecycleRepoStub) ListExpiredReferences(
	context.Context,
	time.Time,
) ([]ImageStudioReference, error) {
	if s.listExpiredReferencesErr != nil {
		return nil, s.listExpiredReferencesErr
	}
	return append([]ImageStudioReference(nil), s.expiredReferences...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) DeleteReference(
	_ context.Context,
	referenceID string,
) error {
	s.deleteReferenceCalls++
	if s.deleteReferenceErr != nil {
		return s.deleteReferenceErr
	}
	s.deletedReferenceIDs = append(s.deletedReferenceIDs, referenceID)
	return nil
}

func (s *imageStudioReferenceLifecycleRepoStub) ListJobReferenceStorageKeysForJob(
	context.Context,
	string,
) ([]string, error) {
	if s.listJobReferenceKeysErr != nil {
		return nil, s.listJobReferenceKeysErr
	}
	return append([]string(nil), s.jobReferenceKeys...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) DeleteJobWithStorageKeys(
	context.Context,
	int64,
	string,
) ([]string, error) {
	return append([]string(nil), s.deleteJobKeys...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) ListExpiredJobIDs(
	context.Context,
	time.Time,
) ([]string, error) {
	return append([]string(nil), s.expiredJobIDs...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) ListAssetStorageKeysForJob(
	context.Context,
	string,
) ([]string, error) {
	return append([]string(nil), s.assetKeys...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) DeleteExpiredJobsBefore(
	context.Context,
	time.Time,
) (int64, error) {
	s.deleteExpiredCalls++
	s.pendingObjectKeys = append(s.pendingObjectKeys, s.assetKeys...)
	s.pendingObjectKeys = append(s.pendingObjectKeys, s.jobReferenceKeys...)
	return int64(len(s.expiredJobIDs)), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) ListPendingObjectDeletions(
	context.Context,
	int,
) ([]string, error) {
	return append([]string(nil), s.pendingObjectKeys...), nil
}

func (s *imageStudioReferenceLifecycleRepoStub) AcknowledgeObjectDeletion(
	_ context.Context,
	storageKey string,
) error {
	for i, key := range s.pendingObjectKeys {
		if key == storageKey {
			s.pendingObjectKeys = append(s.pendingObjectKeys[:i], s.pendingObjectKeys[i+1:]...)
			break
		}
	}
	return nil
}

func (s *imageStudioReferenceLifecycleRepoStub) RecordObjectDeletionFailure(
	context.Context,
	string,
	error,
) error {
	return nil
}

func TestImageStudioPurgeExpiredReferencesDeletesMetadataOnlyAfterObject(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(10, "upload-reference-1", "image/png", encodeImageStudioReferencePNG(t, 2, 2))
	require.NoError(t, err)
	repo := &imageStudioReferenceLifecycleRepoStub{
		expiredReferences: []ImageStudioReference{{ID: "upload-reference-1", StorageKey: key}},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	deletedJobs, err := svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.NoError(t, err)
	require.Zero(t, deletedJobs)
	require.Equal(t, []string{"upload-reference-1"}, repo.deletedReferenceIDs)
	_, err = store.Read(key)
	require.Error(t, err)
}

func TestImageStudioPurgeExpiredReferencesPreservesMetadataWhenLookupFails(t *testing.T) {
	lookupErr := errors.New("expired reference lookup failed")
	repo := &imageStudioReferenceLifecycleRepoStub{listExpiredReferencesErr: lookupErr}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}

	deletedJobs, err := svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.ErrorIs(t, err, lookupErr)
	require.Zero(t, deletedJobs)
	require.Zero(t, repo.deleteReferenceCalls)
	require.Zero(t, repo.deleteExpiredCalls)
}

func TestImageStudioPurgeExpiredReferencesPreservesMetadataWhenObjectDeleteFails(t *testing.T) {
	repo := &imageStudioReferenceLifecycleRepoStub{
		expiredReferences: []ImageStudioReference{{
			ID:         "upload-reference-1",
			StorageKey: "../invalid-reference.png",
		}},
	}
	svc := &ImageStudioService{repo: repo, assetStore: NewImageStudioAssetStore(t.TempDir())}

	deletedJobs, err := svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.Error(t, err)
	require.Zero(t, deletedJobs)
	require.Zero(t, repo.deleteReferenceCalls)
	require.Zero(t, repo.deleteExpiredCalls)
}

func TestImageStudioPurgeExpiredReferencesRetriesMetadataDelete(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(10, "upload-reference-1", "image/png", encodeImageStudioReferencePNG(t, 2, 2))
	require.NoError(t, err)
	metadataErr := errors.New("metadata delete failed")
	repo := &imageStudioReferenceLifecycleRepoStub{
		expiredReferences:  []ImageStudioReference{{ID: "upload-reference-1", StorageKey: key}},
		deleteReferenceErr: metadataErr,
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	deletedJobs, err := svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.ErrorIs(t, err, metadataErr)
	require.Zero(t, deletedJobs)
	require.Equal(t, 1, repo.deleteReferenceCalls)
	_, err = store.Read(key)
	require.Error(t, err)

	repo.deleteReferenceErr = nil
	deletedJobs, err = svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.NoError(t, err)
	require.Zero(t, deletedJobs)
	require.Equal(t, 2, repo.deleteReferenceCalls)
	require.Equal(t, []string{"upload-reference-1"}, repo.deletedReferenceIDs)
}

func TestImageStudioDeleteJobAlsoDeletesJobOwnedReferences(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	referenceKey, err := store.Save(10, "job-reference-1", "image/png", encodeImageStudioReferencePNG(t, 2, 2))
	require.NoError(t, err)
	repo := &imageStudioReferenceLifecycleRepoStub{jobReferenceKeys: []string{referenceKey}}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	require.NoError(t, svc.DeleteJob(context.Background(), 10, "job-1"))

	_, err = store.Read(referenceKey)
	require.Error(t, err)
}

func TestImageStudioPurgeExpiredJobsAlsoDeletesJobOwnedReferences(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	referenceKey, err := store.Save(10, "job-reference-1", "image/png", encodeImageStudioReferencePNG(t, 2, 2))
	require.NoError(t, err)
	repo := &imageStudioReferenceLifecycleRepoStub{
		expiredJobIDs:    []string{"job-1"},
		jobReferenceKeys: []string{referenceKey},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}

	deletedJobs, err := svc.PurgeExpiredJobs(context.Background(), time.Now().UTC())

	require.NoError(t, err)
	require.Equal(t, int64(1), deletedJobs)
	_, err = store.Read(referenceKey)
	require.Error(t, err)
}
