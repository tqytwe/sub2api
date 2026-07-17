//go:build integration

package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestImageStudioCreatePendingJobEnforcesPerUserLimit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-limit-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	for i := 0; i < 2; i++ {
		job := integrationImageStudioJob(user.ID, i)
		require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), func(context.Context) error {
			return nil
		}))
	}

	third := integrationImageStudioJob(user.ID, 3)
	err := repo.CreatePendingJob(ctx, third, integrationImageStudioItems(third), func(context.Context) error {
		return nil
	})
	require.ErrorIs(t, err, service.ErrImageStudioConcurrentJobLimit)
}

func TestImageStudioCreatePendingJobIdempotentReplayDoesNotReserveTwice(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	idempotentRepo, ok := repo.(service.ImageStudioIdempotentJobRepository)
	require.True(t, ok)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-idempotent-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	keyHash := service.HashIdempotencyKey("same-submit")
	first := integrationImageStudioJob(user.ID, 20)
	first.IdempotencyKeyHash = keyHash
	first.IdempotencyFingerprint = "fingerprint-a"
	var reserveCalls int

	existingID, created, err := idempotentRepo.CreatePendingJobIdempotent(
		ctx,
		first,
		integrationImageStudioItems(first),
		func(context.Context) error {
			reserveCalls++
			return nil
		},
	)
	require.NoError(t, err)
	require.True(t, created)
	require.Empty(t, existingID)

	retry := integrationImageStudioJob(user.ID, 21)
	retry.IdempotencyKeyHash = keyHash
	retry.IdempotencyFingerprint = first.IdempotencyFingerprint
	existingID, created, err = idempotentRepo.CreatePendingJobIdempotent(
		ctx,
		retry,
		integrationImageStudioItems(retry),
		func(context.Context) error {
			reserveCalls++
			return nil
		},
	)

	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, existingID)
	require.Equal(t, 1, reserveCalls)
	var jobCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1 AND idempotency_key_hash = $2`, user.ID, keyHash).Scan(&jobCount))
	require.Equal(t, 1, jobCount)
}

func TestImageStudioCreatePendingJobIdempotentRejectsPayloadConflict(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	idempotentRepo, ok := repo.(service.ImageStudioIdempotentJobRepository)
	require.True(t, ok)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-idempotent-conflict-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	keyHash := service.HashIdempotencyKey("conflicting-submit")
	first := integrationImageStudioJob(user.ID, 22)
	first.IdempotencyKeyHash = keyHash
	first.IdempotencyFingerprint = "fingerprint-a"
	_, created, err := idempotentRepo.CreatePendingJobIdempotent(
		ctx,
		first,
		integrationImageStudioItems(first),
		nil,
	)
	require.NoError(t, err)
	require.True(t, created)

	retry := integrationImageStudioJob(user.ID, 23)
	retry.IdempotencyKeyHash = keyHash
	retry.IdempotencyFingerprint = "fingerprint-b"
	existingID, created, err := idempotentRepo.CreatePendingJobIdempotent(
		ctx,
		retry,
		integrationImageStudioItems(retry),
		nil,
	)

	require.ErrorIs(t, err, service.ErrIdempotencyKeyConflict)
	require.False(t, created)
	require.Empty(t, existingID)
}

func TestImageStudioReferencesArePrivateAndPreserveRequestedOrder(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	userA := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-a-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	userB := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-b-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	expiresAt := time.Now().UTC().Add(time.Hour)
	refA1 := &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           userA.ID,
		StorageKey:       fmt.Sprintf("%d/reference-a1.png", userA.ID),
		OriginalFilename: "reference-a1.png",
		ContentType:      "image/png",
		ByteSize:         12,
		ExpiresAt:        &expiresAt,
	}
	refA2 := &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           userA.ID,
		StorageKey:       fmt.Sprintf("%d/reference-a2.webp", userA.ID),
		OriginalFilename: "reference-a2.webp",
		ContentType:      "image/webp",
		ByteSize:         18,
		ExpiresAt:        &expiresAt,
	}
	refB := &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           userB.ID,
		StorageKey:       fmt.Sprintf("%d/reference-b.png", userB.ID),
		OriginalFilename: "reference-b.png",
		ContentType:      "image/png",
		ByteSize:         20,
		ExpiresAt:        &expiresAt,
	}
	require.NoError(t, repo.CreateReference(ctx, refA1))
	require.NoError(t, repo.CreateReference(ctx, refA2))
	require.NoError(t, repo.CreateReference(ctx, refB))

	refs, err := repo.ListReferencesByID(ctx, userA.ID, []string{refA2.ID, refB.ID, refA1.ID})

	require.NoError(t, err)
	require.Len(t, refs, 2)
	require.Equal(t, refA2.ID, refs[0].ID)
	require.Equal(t, refA1.ID, refs[1].ID)
	require.NotContains(t, []string{refs[0].ID, refs[1].ID}, refB.ID)
}

func TestImageStudioReferencesExcludeExpiredRows(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-expired-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	expiredAt := time.Now().UTC().Add(-time.Hour)
	reference := &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		StorageKey:       fmt.Sprintf("%d/reference-expired.png", user.ID),
		OriginalFilename: "reference-expired.png",
		ContentType:      "image/png",
		ByteSize:         12,
		ExpiresAt:        &expiredAt,
	}
	require.NoError(t, repo.CreateReference(ctx, reference))

	refs, err := repo.ListReferencesByID(ctx, user.ID, []string{reference.ID})

	require.NoError(t, err)
	require.Empty(t, refs)
}

func TestImageStudioReferenceQuotaIsAtomicUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-quota-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	expiresAt := time.Now().UTC().Add(time.Hour)
	start := make(chan struct{})
	errs := make(chan error, service.ImageStudioReferenceMaxPendingCount+8)
	var wg sync.WaitGroup
	for i := 0; i < service.ImageStudioReferenceMaxPendingCount+8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs <- repo.CreateReference(ctx, &service.ImageStudioReference{
				ID:               uuid.NewString(),
				UserID:           user.ID,
				StorageKey:       fmt.Sprintf("%d/reference-%d.png", user.ID, i),
				OriginalFilename: fmt.Sprintf("reference-%d.png", i),
				ContentType:      "image/png",
				ByteSize:         1,
				ExpiresAt:        &expiresAt,
			})
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, limited int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, service.ErrImageStudioReferenceQuota):
			limited++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, service.ImageStudioReferenceMaxPendingCount, success)
	require.Equal(t, 8, limited)
}

func TestImageStudioDeleteReferenceIsOwnerScoped(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	userA := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-delete-a-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	userB := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-reference-delete-b-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	expiresAt := time.Now().UTC().Add(time.Hour)
	reference := &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           userA.ID,
		StorageKey:       fmt.Sprintf("%d/reference-delete.png", userA.ID),
		OriginalFilename: "reference-delete.png",
		ContentType:      "image/png",
		ByteSize:         1,
		ExpiresAt:        &expiresAt,
	}
	require.NoError(t, repo.CreateReference(ctx, reference))
	lifecycleRepo := repo.(service.ImageStudioReferenceLifecycleRepository)

	_, err := lifecycleRepo.GetReferenceForDelete(ctx, userB.ID, reference.ID)
	require.ErrorIs(t, err, service.ErrImageStudioReferenceNotFound)

	got, err := lifecycleRepo.GetReferenceForDelete(ctx, userA.ID, reference.ID)
	require.NoError(t, err)
	require.Equal(t, reference.StorageKey, got.StorageKey)
	require.NoError(t, lifecycleRepo.DeleteReferenceForUser(ctx, userA.ID, reference.ID))
	_, err = lifecycleRepo.GetReferenceForDelete(ctx, userA.ID, reference.ID)
	require.ErrorIs(t, err, service.ErrImageStudioReferenceNotFound)
}

func TestImageStudioDeleteJobRetainsObjectOutboxWhenObjectDeleteFails(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-delete-outbox-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 30)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	require.NoError(t, repo.UpdateJobStatus(ctx, job.ID, service.ImageStudioJobStatusCompleted))
	require.NoError(t, repo.InsertAssets(ctx, job.ID, []service.ImageStudioAssetRecord{{
		ID:          uuid.NewString(),
		StorageKey:  "../invalid-object.png",
		ContentType: "image/png",
		ByteSize:    12,
		SortOrder:   0,
	}}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_object_deletions WHERE job_id = $1::uuid",
			job.ID)
	})
	studio := service.NewImageStudioService(
		repo,
		service.NewImageStudioAssetStore(t.TempDir()),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	err := studio.DeleteJob(ctx, user.ID, job.ID)

	require.Error(t, err)
	var jobCount, outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*)::int FROM image_studio_jobs WHERE id = $1::uuid",
		job.ID,
	).Scan(&jobCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*)::int FROM image_studio_object_deletions WHERE job_id = $1::uuid",
		job.ID,
	).Scan(&outboxCount))
	require.Zero(t, jobCount)
	require.Equal(t, 1, outboxCount)
}

func TestImageStudioPurgeRetriesPendingObjectDeletionOutbox(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-delete-outbox-retry-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	store := service.NewImageStudioAssetStore(t.TempDir())
	key, err := store.Save(user.ID, "retry-object", "image/png", []byte("private image"))
	require.NoError(t, err)
	jobID := uuid.NewString()
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO image_studio_object_deletions (user_id, job_id, storage_key)
		VALUES ($1, $2::uuid, $3)`, user.ID, jobID, key)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_object_deletions WHERE job_id = $1::uuid",
			jobID)
	})
	studio := service.NewImageStudioService(
		repo,
		store,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err = studio.PurgeExpiredJobs(ctx, time.Now().UTC())

	require.NoError(t, err)
	_, err = store.Read(key)
	require.Error(t, err)
	var outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*)::int FROM image_studio_object_deletions WHERE job_id = $1::uuid",
		jobID,
	).Scan(&outboxCount))
	require.Zero(t, outboxCount)
}

func TestImageStudioPurgeExpiredJobsDeletesMetadataBeforeRetryingObjects(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-expired-outbox-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 31)
	expiredAt := time.Now().UTC().Add(-time.Hour)
	job.ExpiresAt = &expiredAt
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	require.NoError(t, repo.UpdateJobStatus(ctx, job.ID, service.ImageStudioJobStatusCompleted))
	require.NoError(t, repo.InsertAssets(ctx, job.ID, []service.ImageStudioAssetRecord{{
		ID:          uuid.NewString(),
		StorageKey:  "../invalid-expired-object.png",
		ContentType: "image/png",
		ByteSize:    12,
		SortOrder:   0,
	}}))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			"DELETE FROM image_studio_object_deletions WHERE job_id = $1::uuid",
			job.ID)
	})
	studio := service.NewImageStudioService(
		repo,
		service.NewImageStudioAssetStore(t.TempDir()),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	deleted, err := studio.PurgeExpiredJobs(ctx, time.Now().UTC())

	require.Error(t, err)
	require.Equal(t, int64(1), deleted)
	var jobCount, outboxCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*)::int FROM image_studio_jobs WHERE id = $1::uuid",
		job.ID,
	).Scan(&jobCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*)::int FROM image_studio_object_deletions WHERE job_id = $1::uuid",
		job.ID,
	).Scan(&outboxCount))
	require.Zero(t, jobCount)
	require.Equal(t, 1, outboxCount)
}

func TestImageStudioFilterTrackedObjectStorageKeysCoversAllPrivateMetadata(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-object-tracking-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	referenceKey := fmt.Sprintf("%d/upload-reference.png", user.ID)
	expiresAt := time.Now().UTC().Add(time.Hour)
	require.NoError(t, repo.CreateReference(ctx, &service.ImageStudioReference{
		ID:               uuid.NewString(),
		UserID:           user.ID,
		StorageKey:       referenceKey,
		OriginalFilename: "upload-reference.png",
		ContentType:      "image/png",
		ByteSize:         64,
		ExpiresAt:        &expiresAt,
	}))

	job := integrationImageStudioJob(user.ID, 40)
	jobReferenceKey := fmt.Sprintf("%d/job-reference.png", user.ID)
	job.JobReferences = []service.ImageStudioJobReference{{
		ID:          uuid.NewString(),
		JobID:       job.ID,
		StorageKey:  jobReferenceKey,
		ContentType: "image/png",
		ByteSize:    64,
		SortOrder:   0,
	}}
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	require.NoError(t, repo.UpdateJobStatus(ctx, job.ID, service.ImageStudioJobStatusCompleted))

	originalKey := fmt.Sprintf("%d/original.png", user.ID)
	thumbnailKey := fmt.Sprintf("%d/thumbnail.png", user.ID)
	require.NoError(t, repo.InsertAssets(ctx, job.ID, []service.ImageStudioAssetRecord{{
		ID:                   uuid.NewString(),
		StorageKey:           originalKey,
		ContentType:          "image/png",
		ByteSize:             256,
		SortOrder:            0,
		Width:                1024,
		Height:               1024,
		ThumbnailStorageKey:  thumbnailKey,
		ThumbnailContentType: "image/png",
		ThumbnailByteSize:    64,
	}}))

	orphanKey := fmt.Sprintf("%d/orphan.png", user.ID)
	tracked, err := repo.(service.ImageStudioObjectReconciliationRepository).
		FilterTrackedObjectStorageKeys(ctx, []string{
			referenceKey,
			jobReferenceKey,
			originalKey,
			thumbnailKey,
			orphanKey,
		})

	require.NoError(t, err)
	require.Equal(t, map[string]struct{}{
		referenceKey:    {},
		jobReferenceKey: {},
		originalKey:     {},
		thumbnailKey:    {},
	}, tracked)
	require.NotContains(t, tracked, orphanKey)
}

func TestImageStudioCreatePendingJobPersistsJobReferencesAtomically(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-job-reference-atomic-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	jobReference := service.ImageStudioJobReference{
		ID:          uuid.NewString(),
		JobID:       job.ID,
		StorageKey:  fmt.Sprintf("%d/%s-reference.png", user.ID, job.ID),
		ContentType: "image/png",
		ByteSize:    64,
		SortOrder:   0,
	}
	job.JobReferences = []service.ImageStudioJobReference{jobReference}
	duplicateItemID := uuid.NewString()
	items := []service.ImageStudioItem{
		{ID: duplicateItemID, JobID: job.ID, SortOrder: 0, Status: service.ImageStudioItemStatusPending},
		{ID: duplicateItemID, JobID: job.ID, SortOrder: 1, Status: service.ImageStudioItemStatusPending},
	}

	err := repo.CreatePendingJob(ctx, job, items, nil)

	require.Error(t, err)
	var jobCount, referenceCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*)::int FROM image_studio_jobs WHERE id = $1::uuid`, job.ID,
	).Scan(&jobCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*)::int FROM image_studio_job_references WHERE id = $1::uuid`, jobReference.ID,
	).Scan(&referenceCount))
	require.Zero(t, jobCount)
	require.Zero(t, referenceCount)
}

func TestImageStudioAcceptedEditJobRecoversAfterUploadReferenceExpiresAndIsDeleted(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-job-reference-recovery-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	store := service.NewImageStudioAssetStore(t.TempDir())
	imageData := integrationImageStudioReferencePNG(t)
	uploadReferenceID := uuid.NewString()
	uploadKey, err := store.Save(user.ID, uploadReferenceID, "image/png", imageData)
	require.NoError(t, err)
	expiredAt := time.Now().UTC().Add(-time.Hour)
	uploadReference := &service.ImageStudioReference{
		ID:               uploadReferenceID,
		UserID:           user.ID,
		StorageKey:       uploadKey,
		OriginalFilename: "upload.png",
		ContentType:      "image/png",
		ByteSize:         int64(len(imageData)),
		ExpiresAt:        &expiredAt,
	}
	require.NoError(t, repo.CreateReference(ctx, uploadReference))

	job := integrationImageStudioJob(user.ID, 2)
	jobReferenceID := uuid.NewString()
	jobReferenceKey, err := store.Save(user.ID, job.ID+"-reference-"+jobReferenceID, "image/png", imageData)
	require.NoError(t, err)
	job.JobReferences = []service.ImageStudioJobReference{{
		ID:          jobReferenceID,
		JobID:       job.ID,
		StorageKey:  jobReferenceKey,
		ContentType: "image/png",
		ByteSize:    int64(len(imageData)),
		SortOrder:   0,
	}}
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	studio := service.NewImageStudioService(
		repo,
		store,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	deletedJobs, err := studio.PurgeExpiredJobs(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, deletedJobs)
	_, err = store.Read(uploadKey)
	require.Error(t, err)
	var uploadCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*)::int FROM image_studio_references WHERE id = $1::uuid`, uploadReferenceID,
	).Scan(&uploadCount))
	require.Zero(t, uploadCount)
	jobReferenceKeys, err := repo.(service.ImageStudioJobReferenceStorageRepository).
		ListJobReferenceStorageKeysForJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, []string{jobReferenceKey}, jobReferenceKeys)

	decrypted := fmt.Sprintf(
		`{"endpoint":"/v1/images/edits","body":{"model":"gpt-image-1","prompt":"edit","n":1,"image_studio_job_reference_ids":[%q]}}`,
		jobReferenceID,
	)

	workerRequest, err := studio.BuildWorkerRequest(ctx, job, decrypted)

	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(workerRequest.ContentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(bytes.NewReader(workerRequest.Body), params["boundary"])
	var recovered []byte
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(part)
		require.NoError(t, err)
		if part.FileName() != "" {
			recovered = data
		}
	}
	require.Equal(t, imageData, recovered)
}

func TestImageStudioJobsPagePersistsAssetDimensionsAndPaginates(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-page-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	for i := 0; i < 3; i++ {
		job := integrationImageStudioJob(user.ID, i)
		job.Status = service.ImageStudioJobStatusCompleted
		job.RequestPayloadEncrypted = ""
		require.NoError(t, repo.InsertJob(ctx, job))
		if i == 2 {
			require.NoError(t, repo.InsertAssets(ctx, job.ID, []service.ImageStudioAssetRecord{{
				ID:                   uuid.NewString(),
				StorageKey:           fmt.Sprintf("%d/original.png", user.ID),
				ContentType:          "image/png",
				ByteSize:             100,
				SortOrder:            0,
				Width:                800,
				Height:               400,
				ThumbnailStorageKey:  fmt.Sprintf("%d/thumbnail.png", user.ID),
				ThumbnailContentType: "image/png",
				ThumbnailByteSize:    25,
			}}))
		}
	}

	first, total, err := repo.ListJobsPage(ctx, user.ID, 1, 2)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, first, 2)
	require.Len(t, first[0].Assets, 1)
	require.Equal(t, 800, first[0].Assets[0].Width)
	require.Equal(t, 400, first[0].Assets[0].Height)
	require.Equal(t, "2:1", first[0].Assets[0].AspectRatio)
	require.NotEmpty(t, first[0].Assets[0].ThumbnailStorageKey)

	second, total, err := repo.ListJobsPage(ctx, user.ID, 2, 2)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, second, 1)
	require.NotEqual(t, first[0].ID, second[0].ID)
	require.NotEqual(t, first[1].ID, second[0].ID)
}

func TestImageStudioJobsPageDoesNotLetActiveJobsConsumeHistorySlots(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-history-page-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	for i := 0; i < 12; i++ {
		job := integrationImageStudioJob(user.ID, i)
		job.Status = service.ImageStudioJobStatusCompleted
		job.RequestPayloadEncrypted = ""
		require.NoError(t, repo.InsertJob(ctx, job))
	}
	for i := 0; i < 2; i++ {
		job := integrationImageStudioJob(user.ID, 100+i)
		require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	}

	jobs, total, err := repo.ListJobsPage(ctx, user.ID, 1, 12)

	require.NoError(t, err)
	require.Equal(t, 12, total)
	require.Len(t, jobs, 12)
	for _, job := range jobs {
		require.Equal(t, service.ImageStudioJobStatusCompleted, job.Status)
	}
}

func TestImageStudioClaimRecoversExpiredLease(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-lease-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), func(context.Context) error {
		return nil
	}))

	now := time.Now().UTC()
	first, err := repo.ClaimNextJob(ctx, "worker-a", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, first.ID)

	none, err := repo.ClaimNextJob(ctx, "worker-b", now.Add(30*time.Second), time.Minute)
	require.NoError(t, err)
	require.Nil(t, none)

	recovered, err := repo.ClaimNextJob(ctx, "worker-b", now.Add(2*time.Minute), time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, recovered.ID)
	require.Equal(t, "worker-b", recovered.LeaseOwner)
}

func TestImageStudioCheckpointSurvivesLeaseRecoveryWithoutProviderRetry(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-checkpoint-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))

	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-checkpoint-a", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-checkpoint-a", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	checkpoint := service.ImageStudioImagePayload{
		Data:        []byte("durable-provider-output"),
		ContentType: "image/png",
	}
	require.NoError(t, repo.CheckpointItem(
		ctx,
		job.ID,
		item.ID,
		"worker-checkpoint-a",
		checkpoint,
		0.07,
		now.Add(time.Second),
	))

	recovered, err := repo.ClaimNextJob(ctx, "worker-checkpoint-b", now.Add(2*time.Minute), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	resumed, err := repo.ClaimNextItem(ctx, job.ID, "worker-checkpoint-b", now.Add(2*time.Minute))
	require.NoError(t, err)
	require.NotNil(t, resumed)
	require.Equal(t, service.ImageStudioItemStatusPersisting, resumed.Status)
	require.Equal(t, checkpoint.Data, resumed.CheckpointData)
	require.Equal(t, checkpoint.ContentType, resumed.CheckpointContentType)
	require.NotNil(t, resumed.CheckpointActualCost)
	require.InDelta(t, 0.07, *resumed.CheckpointActualCost, 0.000001)

	asset := &service.ImageStudioAssetRecord{
		ID:          resumed.ID,
		URL:         "https://example.test/checkpoint.png",
		ContentType: checkpoint.ContentType,
		ByteSize:    int64(len(checkpoint.Data)),
		SortOrder:   resumed.SortOrder,
	}
	require.NoError(t, repo.CompleteItem(
		ctx,
		job.ID,
		resumed.ID,
		"worker-checkpoint-b",
		service.ImageStudioItemStatusSuccess,
		resumed.CheckpointActualCost,
		"",
		asset,
		now.Add(2*time.Minute+time.Second),
	))

	completed, err := repo.GetJob(ctx, user.ID, job.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioItemStatusSuccess, completed.Items[0].Status)
	require.Nil(t, completed.Items[0].CheckpointActualCost)
	require.Empty(t, completed.Items[0].CheckpointData)
	require.Equal(t, resumed.ID, *completed.Items[0].AssetID)
}

func TestImageStudioAssetsRemainPrivateUntilJobSettlementCommits(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-asset-publish-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	store := service.NewImageStudioAssetStore(t.TempDir())
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))

	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-publish", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-publish", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	imageData := integrationImageStudioReferencePNG(t)
	storageKey, err := store.Save(user.ID, item.ID, "image/png", imageData)
	require.NoError(t, err)
	cost := 0.04
	require.NoError(t, repo.CompleteItem(
		ctx,
		job.ID,
		item.ID,
		"worker-publish",
		service.ImageStudioItemStatusSuccess,
		&cost,
		"",
		&service.ImageStudioAssetRecord{
			ID:          item.ID,
			StorageKey:  storageKey,
			ContentType: "image/png",
			ByteSize:    int64(len(imageData)),
			SortOrder:   item.SortOrder,
		},
		now.Add(time.Second),
	))

	runningJob, err := repo.GetJob(ctx, user.ID, job.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusRunning, runningJob.Status)
	require.Empty(t, runningJob.Assets)
	activeJobs, err := repo.ListActiveJobs(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, activeJobs, 1)
	require.Empty(t, activeJobs[0].Assets)

	studio := service.NewImageStudioService(
		repo,
		store,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	_, _, err = studio.OpenAssetContent(ctx, user.ID, item.ID)
	require.ErrorIs(t, err, service.ErrImageStudioAssetNotFound)

	settled, err := repo.SettleJob(ctx, job.ID, "worker-publish", now.Add(2*time.Second), nil)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCompleted, settled.Status)
	require.Len(t, settled.Assets, 1)

	data, contentType, err := studio.OpenAssetContent(ctx, user.ID, item.ID)
	require.NoError(t, err)
	require.Equal(t, imageData, data)
	require.Equal(t, "image/png", contentType)
}

func TestImageStudioPersistingCheckpointStopsAfterThreeClaims(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-persisting-retry-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))

	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-persisting-0", now, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-persisting-0", now)
	require.NoError(t, err)
	require.NotNil(t, item)
	require.NoError(t, repo.CheckpointItem(
		ctx,
		job.ID,
		item.ID,
		"worker-persisting-0",
		service.ImageStudioImagePayload{Data: []byte("checkpoint"), ContentType: "image/png"},
		0.09,
		now.Add(time.Second),
	))

	for attempt := 1; attempt <= 2; attempt++ {
		at := now.Add(time.Duration(attempt) * 2 * time.Minute)
		owner := fmt.Sprintf("worker-persisting-%d", attempt)
		recovered, err := repo.ClaimNextJob(ctx, owner, at, time.Minute)
		require.NoError(t, err)
		require.NotNil(t, recovered)
		resumed, err := repo.ClaimNextItem(ctx, job.ID, owner, at)
		require.NoError(t, err)
		require.NotNil(t, resumed)
		require.Equal(t, service.ImageStudioItemStatusPersisting, resumed.Status)
		require.Equal(t, attempt+1, resumed.AttemptCount)
		require.Equal(t, []byte("checkpoint"), resumed.CheckpointData)
		require.NotNil(t, resumed.CheckpointActualCost)
		require.InDelta(t, 0.09, *resumed.CheckpointActualCost, 0.000001)
	}

	recoveryTime := now.Add(6 * time.Minute)
	recovered, err := repo.ClaimNextJob(ctx, "worker-persisting-final", recoveryTime, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	resumed, err := repo.ClaimNextItem(ctx, job.ID, "worker-persisting-final", recoveryTime)
	require.NoError(t, err)
	require.Nil(t, resumed)

	var settledActualCost float64
	settled, err := repo.SettleJob(
		ctx,
		job.ID,
		"worker-persisting-final",
		recoveryTime,
		func(_ context.Context, _ *service.ImageStudioJob, actualCost float64) error {
			settledActualCost = actualCost
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusFailed, settled.Status)
	require.Equal(t, 1, settled.FailCount)
	require.Equal(t, 3, settled.Items[0].AttemptCount)
	require.Contains(t, settled.Items[0].Error, "persistence retry limit")
	require.NotNil(t, settled.Items[0].ActualCost)
	require.InDelta(t, 0.09, *settled.Items[0].ActualCost, 0.000001)
	require.InDelta(t, 0.09, settledActualCost, 0.000001)
	require.NotNil(t, settled.ActualCost)
	require.InDelta(t, 0.09, *settled.ActualCost, 0.000001)
	require.Empty(t, settled.Items[0].CheckpointData)
	require.Nil(t, settled.Items[0].CheckpointActualCost)
}

func TestImageStudioConcurrentClaimsRespectPerUserRunningLimit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-claim-limit-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	for i := 0; i < 2; i++ {
		job := integrationImageStudioJob(user.ID, i)
		require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	}
	third := integrationImageStudioJob(user.ID, 3)
	require.NoError(t, insertImageStudioJobBypassingUserLimit(ctx, third, integrationImageStudioItems(third)))

	triggerName := "image_studio_claim_pause_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	functionName := triggerName + "_fn"
	require.NoError(t, installImageStudioClaimPauseTrigger(ctx, triggerName, functionName))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON image_studio_jobs", pq.QuoteIdentifier(triggerName)))
		_, _ = integrationDB.ExecContext(context.Background(),
			fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", pq.QuoteIdentifier(functionName)))
	})

	now := time.Now().UTC()
	start := make(chan struct{})
	results := make(chan *service.ImageStudioJob, 3)
	errs := make(chan error, 3)
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			claimed, err := repo.ClaimNextJob(ctx, fmt.Sprintf("worker-claim-%d", index), now, time.Minute)
			results <- claimed
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	claimedCount := 0
	for claimed := range results {
		if claimed != nil {
			claimedCount++
		}
	}
	require.LessOrEqual(t, claimedCount, 2)

	var running int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM image_studio_jobs
		WHERE user_id = $1 AND status = 'running'`, user.ID).Scan(&running))
	require.LessOrEqual(t, running, 2)
}

func TestImageStudioUploadSlotsAreSharedAcrossRepositoryInstances(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repoA := NewImageStudioRepository(client, integrationDB)
	repoB := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-upload-slots-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(
			context.Background(),
			`DELETE FROM image_studio_upload_slots WHERE user_id = $1`,
			user.ID,
		)
	})
	now := time.Now().UTC()
	acquire := func(repo service.ImageStudioRepository) (string, bool, error) {
		limiter, ok := repo.(service.ImageStudioUploadLimitRepository)
		require.True(t, ok)
		return limiter.AcquireImageStudioUploadSlot(
			ctx,
			user.ID,
			now,
			10*time.Minute,
			2,
			20,
			time.Minute,
		)
	}

	tokenA, acquired, err := acquire(repoA)
	require.NoError(t, err)
	require.True(t, acquired)
	tokenB, acquired, err := acquire(repoB)
	require.NoError(t, err)
	require.True(t, acquired)
	_, acquired, err = acquire(repoA)
	require.NoError(t, err)
	require.False(t, acquired)

	limiterA := repoA.(service.ImageStudioUploadLimitRepository)
	limiterB := repoB.(service.ImageStudioUploadLimitRepository)
	require.NoError(t, limiterA.ReleaseImageStudioUploadSlot(ctx, user.ID, tokenA, now.Add(time.Second)))
	tokenC, acquired, err := acquire(repoB)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, limiterB.ReleaseImageStudioUploadSlot(ctx, user.ID, tokenB, now.Add(2*time.Second)))
	require.NoError(t, limiterB.ReleaseImageStudioUploadSlot(ctx, user.ID, tokenC, now.Add(2*time.Second)))

	var attempts int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM image_studio_upload_slots
		WHERE user_id = $1 AND started_at >= $2`, user.ID, now.Add(-time.Minute)).
		Scan(&attempts))
	require.Equal(t, 3, attempts)
}

func TestImageStudioCompleteItemRejectsExpiredLeaseWithoutInsertingAsset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-expired-complete-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))

	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-expired-complete", now, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-expired-complete", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	cost := 0.04
	asset := &service.ImageStudioAssetRecord{
		ID:        item.ID,
		URL:       "https://example.test/expired.png",
		SortOrder: item.SortOrder,
	}
	err = repo.CompleteItem(
		ctx,
		job.ID,
		item.ID,
		"worker-expired-complete",
		service.ImageStudioItemStatusSuccess,
		&cost,
		"",
		asset,
		now.Add(2*time.Minute),
	)
	require.ErrorIs(t, err, service.ErrImageStudioLeaseLost)

	var assetCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM image_studio_assets WHERE id = $1::uuid`, asset.ID).Scan(&assetCount))
	require.Zero(t, assetCount)
	persisted, err := repo.GetItem(ctx, job.ID, item.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioItemStatusRunning, persisted.Status)
	require.Nil(t, persisted.AssetID)
}

func TestImageStudioRecoveryStopsRetryingItemAfterThreeClaims(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-retry-limit-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))

	now := time.Now().UTC()
	for attempt := 0; attempt < 3; attempt++ {
		owner := fmt.Sprintf("worker-retry-%d", attempt)
		claimed, err := repo.ClaimNextJob(ctx, owner, now.Add(time.Duration(attempt)*2*time.Minute), time.Minute)
		require.NoError(t, err)
		require.NotNil(t, claimed)
		item, err := repo.ClaimNextItem(ctx, job.ID, owner, now.Add(time.Duration(attempt)*2*time.Minute))
		require.NoError(t, err)
		require.NotNil(t, item)
		require.Equal(t, attempt+1, item.AttemptCount)
	}

	recoveryTime := now.Add(6 * time.Minute)
	recovered, err := repo.ClaimNextJob(ctx, "worker-final", recoveryTime, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-final", recoveryTime)
	require.NoError(t, err)
	require.Nil(t, item)

	settled, err := repo.SettleJob(ctx, job.ID, "worker-final", recoveryTime, nil)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusFailed, settled.Status)
	require.Equal(t, 1, settled.FailCount)
	require.Equal(t, 3, settled.Items[0].AttemptCount)
	require.Contains(t, settled.Items[0].Error, "retry limit")
}

func TestImageStudioCancelledRunningJobRecoversAfterWorkerCrash(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-cancel-recovery-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-crashed", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-crashed", now)
	require.NoError(t, err)
	require.NotNil(t, item)
	_, err = repo.RequestCancel(ctx, user.ID, job.ID, now.Add(10*time.Second), nil)
	require.NoError(t, err)

	recovered, err := repo.ClaimNextJob(ctx, "worker-recovery", now.Add(2*time.Minute), time.Minute)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.Equal(t, job.ID, recovered.ID)

	settled, err := repo.SettleJob(ctx, job.ID, "worker-recovery", now.Add(2*time.Minute), func(_ context.Context, _ *service.ImageStudioJob, actual float64) error {
		require.Zero(t, actual)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCancelled, settled.Status)
	require.Len(t, settled.Items, 1)
	require.Equal(t, service.ImageStudioItemStatusCancelled, settled.Items[0].Status)
}

func TestImageStudioHoldFailureDoesNotCreateRunnableJob(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-hold-failure-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	holdErr := service.ErrImageStudioInsufficientBalance

	err := repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), func(context.Context) error {
		return holdErr
	})
	require.ErrorIs(t, err, holdErr)

	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int FROM image_studio_jobs WHERE id = $1::uuid`, job.ID).Scan(&count))
	require.Zero(t, count)
}

func TestImageStudioCreateRollbackAlsoRollsBackBalanceHold(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	studioRepo := NewImageStudioRepository(client, integrationDB)
	billingRepo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-atomic-hold-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-atomic-" + uuid.NewString(),
		Name:   "image-studio-atomic",
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	existing := integrationImageStudioJob(user.ID, 1)
	existing.Status = service.ImageStudioJobStatusFailed
	require.NoError(t, studioRepo.InsertJob(ctx, existing))

	duplicate := integrationImageStudioJob(user.ID, 2)
	duplicate.ID = existing.ID
	duplicate.APIKeyID = &apiKey.ID
	holdAmount := 12.0
	duplicate.EstimatedCost = holdAmount
	duplicate.HoldAmount = &holdAmount
	duplicate.HoldID = service.ImageStudioHoldRequestID(duplicate.ID)
	err := studioRepo.CreatePendingJob(ctx, duplicate, integrationImageStudioItems(duplicate), func(reserveCtx context.Context) error {
		_, reserveErr := billingRepo.ReserveBatchImageBalance(reserveCtx, &service.BatchImageBalanceHoldCommand{
			RequestID:     duplicate.HoldID,
			HoldRequestID: duplicate.HoldID,
			APIKeyID:      apiKey.ID,
			UserID:        user.ID,
			BatchID:       duplicate.ID,
			HoldAmount:    holdAmount,
		})
		return reserveErr
	})
	require.Error(t, err)

	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance, frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&balance, &frozen))
	require.InDelta(t, 100, balance, 0.000001)
	require.Zero(t, frozen)

	var dedupCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2`, duplicate.HoldID, apiKey.ID).Scan(&dedupCount))
	require.Zero(t, dedupCount)
}

func TestImageStudioPendingCancelReleasesHoldInSameTransaction(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	studioRepo := NewImageStudioRepository(client, integrationDB)
	billingRepo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-cancel-hold-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-cancel-" + uuid.NewString(),
		Name:   "image-studio-cancel",
	})
	cleanupImageStudioJobsForUser(t, user.ID)

	job := integrationImageStudioJob(user.ID, 1)
	job.APIKeyID = &apiKey.ID
	holdAmount := 9.0
	job.EstimatedCost = holdAmount
	job.HoldAmount = &holdAmount
	job.HoldID = service.ImageStudioHoldRequestID(job.ID)
	require.NoError(t, studioRepo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), func(reserveCtx context.Context) error {
		_, err := billingRepo.ReserveBatchImageBalance(reserveCtx, &service.BatchImageBalanceHoldCommand{
			RequestID:     job.HoldID,
			HoldRequestID: job.HoldID,
			APIKeyID:      apiKey.ID,
			UserID:        user.ID,
			BatchID:       job.ID,
			HoldAmount:    holdAmount,
		})
		return err
	}))

	releaseErr := errors.New("release unavailable")
	_, err := studioRepo.RequestCancel(ctx, user.ID, job.ID, time.Now().UTC(), func(context.Context, *service.ImageStudioJob) error {
		return releaseErr
	})
	require.ErrorIs(t, err, releaseErr)
	stillPending, err := studioRepo.GetJob(ctx, user.ID, job.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusPending, stillPending.Status)

	cancelled, err := studioRepo.RequestCancel(ctx, user.ID, job.ID, time.Now().UTC(), func(releaseCtx context.Context, locked *service.ImageStudioJob) error {
		_, releaseErr := billingRepo.ReleaseBatchImageBalance(releaseCtx, &service.BatchImageBalanceHoldCommand{
			RequestID:     service.ImageStudioReleaseRequestID(locked.ID),
			HoldRequestID: locked.HoldID,
			APIKeyID:      *locked.APIKeyID,
			UserID:        locked.UserID,
			BatchID:       locked.ID,
			HoldAmount:    *locked.HoldAmount,
		})
		return releaseErr
	})
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCancelled, cancelled.Status)

	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance, frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&balance, &frozen))
	require.InDelta(t, 100, balance, 0.000001)
	require.Zero(t, frozen)
}

func TestImageStudioCreatePendingJobDoesNotBlockDifferentUsers(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	userA := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-parallel-a-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	userB := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-parallel-b-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, userA.ID)
	cleanupImageStudioJobsForUser(t, userB.ID)

	holdEntered := make(chan struct{})
	releaseHold := make(chan struct{})
	firstDone := make(chan error, 1)
	jobA := integrationImageStudioJob(userA.ID, 1)
	go func() {
		firstDone <- repo.CreatePendingJob(ctx, jobA, integrationImageStudioItems(jobA), func(context.Context) error {
			close(holdEntered)
			<-releaseHold
			return nil
		})
	}()
	<-holdEntered

	jobB := integrationImageStudioJob(userB.ID, 2)
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- repo.CreatePendingJob(ctx, jobB, integrationImageStudioItems(jobB), func(context.Context) error {
			return nil
		})
	}()

	select {
	case err := <-secondDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("different user's create was blocked by another user's advisory lock")
	}
	close(releaseHold)
	require.NoError(t, <-firstDone)
}

func TestImageStudioBalanceHoldCaptureReleaseAreIdempotentAndValidateHoldID(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewUsageBillingRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-billing-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID: user.ID,
		Key:    "sk-image-studio-" + uuid.NewString(),
		Name:   "image-studio",
	})

	jobID := uuid.NewString()
	holdID := service.ImageStudioHoldRequestID(jobID)
	reserve := &service.BatchImageBalanceHoldCommand{
		RequestID:     holdID,
		HoldRequestID: holdID,
		APIKeyID:      apiKey.ID,
		UserID:        user.ID,
		BatchID:       jobID,
		HoldAmount:    10,
	}
	first, err := repo.ReserveBatchImageBalance(ctx, reserve)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := repo.ReserveBatchImageBalance(ctx, reserve)
	require.NoError(t, err)
	require.False(t, second.Applied)

	capture := &service.BatchImageBalanceHoldCommand{
		RequestID:     service.ImageStudioCaptureRequestID(jobID),
		HoldRequestID: holdID,
		APIKeyID:      apiKey.ID,
		UserID:        user.ID,
		BatchID:       jobID,
		HoldAmount:    10,
		ActualAmount:  6,
	}
	first, err = repo.CaptureBatchImageBalance(ctx, capture)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err = repo.CaptureBatchImageBalance(ctx, capture)
	require.NoError(t, err)
	require.False(t, second.Applied)

	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance, frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&balance, &frozen))
	require.InDelta(t, 94, balance, 0.000001)
	require.Zero(t, frozen)

	releaseJobID := uuid.NewString()
	releaseHoldID := service.ImageStudioHoldRequestID(releaseJobID)
	releaseReserve := &service.BatchImageBalanceHoldCommand{
		RequestID:     releaseHoldID,
		HoldRequestID: releaseHoldID,
		APIKeyID:      apiKey.ID,
		UserID:        user.ID,
		BatchID:       releaseJobID,
		HoldAmount:    8,
	}
	_, err = repo.ReserveBatchImageBalance(ctx, releaseReserve)
	require.NoError(t, err)
	release := &service.BatchImageBalanceHoldCommand{
		RequestID:     service.ImageStudioReleaseRequestID(releaseJobID),
		HoldRequestID: releaseHoldID,
		APIKeyID:      apiKey.ID,
		UserID:        user.ID,
		BatchID:       releaseJobID,
		HoldAmount:    8,
	}
	first, err = repo.ReleaseBatchImageBalance(ctx, release)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err = repo.ReleaseBatchImageBalance(ctx, release)
	require.NoError(t, err)
	require.False(t, second.Applied)

	bad := *release
	bad.RequestID = service.ImageStudioReleaseRequestID(uuid.NewString())
	bad.HoldRequestID = service.ImageStudioHoldRequestID(uuid.NewString())
	_, err = repo.ReleaseBatchImageBalance(ctx, &bad)
	require.ErrorIs(t, err, service.ErrUsageBillingHoldNotFound)

	overageJobID := uuid.NewString()
	overageHoldID := service.ImageStudioHoldRequestID(overageJobID)
	_, err = repo.ReserveBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:     overageHoldID,
		HoldRequestID: overageHoldID,
		APIKeyID:      apiKey.ID,
		UserID:        user.ID,
		BatchID:       overageJobID,
		HoldAmount:    10,
	})
	require.NoError(t, err)
	_, err = repo.CaptureBatchImageBalance(ctx, &service.BatchImageBalanceHoldCommand{
		RequestID:           service.ImageStudioCaptureRequestID(overageJobID),
		HoldRequestID:       overageHoldID,
		APIKeyID:            apiKey.ID,
		UserID:              user.ID,
		BatchID:             overageJobID,
		HoldAmount:          10,
		ActualAmount:        12,
		AllowBalanceOverage: true,
	})
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance, frozen_balance FROM users WHERE id = $1`, user.ID).Scan(&balance, &frozen))
	require.InDelta(t, 82, balance, 0.000001)
	require.Zero(t, frozen)
}

func TestImageStudioCompletionCommittedBeforeCancelKeepsCompletedAsset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-race-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-race", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-race", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	cost := 0.04
	asset := &service.ImageStudioAssetRecord{
		ID:        uuid.NewString(),
		URL:       "https://example.test/race.png",
		SortOrder: item.SortOrder,
	}
	require.NoError(t, repo.CompleteItem(
		ctx,
		job.ID,
		item.ID,
		"worker-race",
		service.ImageStudioItemStatusSuccess,
		&cost,
		"",
		asset,
		now.Add(time.Second),
	))
	_, err = repo.RequestCancel(ctx, user.ID, job.ID, now.Add(2*time.Second), nil)
	require.NoError(t, err)

	settled, err := repo.SettleJob(ctx, job.ID, "worker-race", now.Add(3*time.Second), func(context.Context, *service.ImageStudioJob, float64) error {
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCompleted, settled.Status)
	require.Len(t, settled.Assets, 1)
	require.Equal(t, asset.ID, settled.Assets[0].ID)
}

func TestImageStudioCheckpointCommittedBeforeCancelKeepsCompletedAsset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-checkpoint-first-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-checkpoint-first", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-checkpoint-first", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	checkpoint := service.ImageStudioImagePayload{
		Data:        integrationImageStudioReferencePNG(t),
		ContentType: "image/png",
	}
	cost := 0.04
	require.NoError(t, repo.CheckpointItem(
		ctx,
		job.ID,
		item.ID,
		"worker-checkpoint-first",
		checkpoint,
		cost,
		now.Add(time.Second),
	))
	_, err = repo.RequestCancel(ctx, user.ID, job.ID, now.Add(2*time.Second), nil)
	require.NoError(t, err)

	asset := &service.ImageStudioAssetRecord{
		ID:          item.ID,
		StorageKey:  fmt.Sprintf("%d/checkpoint-first.png", user.ID),
		ContentType: checkpoint.ContentType,
		ByteSize:    int64(len(checkpoint.Data)),
		SortOrder:   item.SortOrder,
	}
	require.NoError(t, repo.CompleteItem(
		ctx,
		job.ID,
		item.ID,
		"worker-checkpoint-first",
		service.ImageStudioItemStatusSuccess,
		&cost,
		"",
		asset,
		now.Add(3*time.Second),
	))

	settled, err := repo.SettleJob(
		ctx,
		job.ID,
		"worker-checkpoint-first",
		now.Add(4*time.Second),
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCompleted, settled.Status)
	require.Len(t, settled.Assets, 1)
	require.Equal(t, item.ID, settled.Assets[0].ID)
	require.Equal(t, service.ImageStudioItemStatusSuccess, settled.Items[0].Status)
}

func TestImageStudioCancelCommittedBeforeCheckpointRejectsAssetAndBillsConfirmedCost(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-cancel-first-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-cancel-first", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-cancel-first", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	_, err = repo.RequestCancel(ctx, user.ID, job.ID, now.Add(time.Second), nil)
	require.NoError(t, err)
	checkpoint := service.ImageStudioImagePayload{Data: []byte("generated"), ContentType: "image/png"}
	require.ErrorIs(t, repo.CheckpointItem(
		ctx,
		job.ID,
		item.ID,
		"worker-cancel-first",
		checkpoint,
		0.04,
		now.Add(2*time.Second),
	), service.ErrImageStudioCheckpointCancelled)
	persisted, err := repo.GetItem(ctx, job.ID, item.ID)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioItemStatusCancelled, persisted.Status)
	require.NotNil(t, persisted.ActualCost)
	require.InDelta(t, 0.04, *persisted.ActualCost, 0.000001)
	require.Empty(t, persisted.CheckpointData)
	cost := 0.04
	require.ErrorIs(t, repo.CompleteItem(
		ctx,
		job.ID,
		item.ID,
		"worker-cancel-first",
		service.ImageStudioItemStatusSuccess,
		&cost,
		"",
		&service.ImageStudioAssetRecord{
			ID:        uuid.NewString(),
			URL:       "https://example.test/cancelled.png",
			SortOrder: item.SortOrder,
		},
		now.Add(2*time.Second),
	), service.ErrImageStudioLeaseLost)

	recoveryTime := now.Add(2 * time.Minute)
	recovered, err := repo.ClaimNextJob(ctx, "worker-cancel-recovery", recoveryTime, time.Minute)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	var settledActualCost float64
	settled, err := repo.SettleJob(
		ctx,
		job.ID,
		"worker-cancel-recovery",
		recoveryTime,
		func(_ context.Context, _ *service.ImageStudioJob, actualCost float64) error {
			settledActualCost = actualCost
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusCancelled, settled.Status)
	require.Empty(t, settled.Assets)
	require.Equal(t, service.ImageStudioItemStatusCancelled, settled.Items[0].Status)
	require.InDelta(t, 0.04, settledActualCost, 0.000001)
	require.NotNil(t, settled.ActualCost)
	require.InDelta(t, 0.04, *settled.ActualCost, 0.000001)
}

func TestImageStudioCheckpointAndCancelSerializeByJobLock(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-checkpoint-cancel-race-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-checkpoint-cancel-race", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-checkpoint-cancel-race", now)
	require.NoError(t, err)
	require.NotNil(t, item)

	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback() })
	_, err = blocker.ExecContext(ctx, `
		SELECT id
		FROM image_studio_items
		WHERE id = $1::uuid
		FOR UPDATE`, item.ID)
	require.NoError(t, err)

	checkpointStarted := make(chan struct{})
	checkpointDone := make(chan error, 1)
	go func() {
		close(checkpointStarted)
		checkpointDone <- repo.CheckpointItem(
			ctx,
			job.ID,
			item.ID,
			"worker-checkpoint-cancel-race",
			service.ImageStudioImagePayload{Data: []byte("generated"), ContentType: "image/png"},
			0.04,
			now.Add(time.Second),
		)
	}()
	<-checkpointStarted
	select {
	case err := <-checkpointDone:
		require.Failf(t, "checkpoint did not wait for the locked item", "returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancelDone := make(chan error, 1)
	go func() {
		_, cancelErr := repo.RequestCancel(ctx, user.ID, job.ID, now.Add(2*time.Second), nil)
		cancelDone <- cancelErr
	}()
	select {
	case err := <-cancelDone:
		require.Failf(t, "cancel bypassed checkpoint job lock", "returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, blocker.Commit())

	select {
	case err := <-checkpointDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "checkpoint did not finish after releasing the item lock")
	}
	select {
	case err := <-cancelDone:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.Fail(t, "cancel did not finish after checkpoint committed")
	}
	var itemStatus string
	var checkpointCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status,
		       COUNT(checkpoint_data) OVER ()::int
		FROM image_studio_items
		WHERE id = $1::uuid`, item.ID).Scan(&itemStatus, &checkpointCount))
	require.Equal(t, service.ImageStudioItemStatusPersisting, itemStatus)
	require.Equal(t, 1, checkpointCount)
}

func TestImageStudioRetryItemPersistsAttemptsAndRejectsCancel(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-provider-retry-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	require.NoError(t, repo.CreatePendingJob(ctx, job, integrationImageStudioItems(job), nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-provider-retry", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	item, err := repo.ClaimNextItem(ctx, job.ID, "worker-provider-retry", now)
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, 1, item.AttemptCount)

	require.NoError(t, repo.RetryItem(
		ctx,
		job.ID,
		item.ID,
		"worker-provider-retry",
		now.Add(time.Second),
	))
	item, err = repo.ClaimNextItem(ctx, job.ID, "worker-provider-retry", now.Add(2*time.Second))
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, 2, item.AttemptCount)

	_, err = repo.RequestCancel(ctx, user.ID, job.ID, now.Add(3*time.Second), nil)
	require.NoError(t, err)
	require.ErrorIs(t, repo.RetryItem(
		ctx,
		job.ID,
		item.ID,
		"worker-provider-retry",
		now.Add(4*time.Second),
	), service.ErrImageStudioLeaseLost)
}

func TestImageStudioPartialSuccessRetainsSuccessfulItemAndAsset(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewImageStudioRepository(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("image-studio-partial-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
		Balance:      100,
	})
	cleanupImageStudioJobsForUser(t, user.ID)
	job := integrationImageStudioJob(user.ID, 1)
	job.Count = 2
	items := []service.ImageStudioItem{
		{ID: uuid.NewString(), JobID: job.ID, SortOrder: 0, Status: service.ImageStudioItemStatusPending},
		{ID: uuid.NewString(), JobID: job.ID, SortOrder: 1, Status: service.ImageStudioItemStatusPending},
	}
	require.NoError(t, repo.CreatePendingJob(ctx, job, items, nil))
	now := time.Now().UTC()
	claimed, err := repo.ClaimNextJob(ctx, "worker-partial", now, time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)

	first, err := repo.ClaimNextItem(ctx, job.ID, "worker-partial", now)
	require.NoError(t, err)
	cost := 0.04
	asset := &service.ImageStudioAssetRecord{
		ID:        uuid.NewString(),
		URL:       "https://example.test/partial.png",
		SortOrder: first.SortOrder,
	}
	require.NoError(t, repo.CompleteItem(ctx, job.ID, first.ID, "worker-partial", service.ImageStudioItemStatusSuccess, &cost, "", asset, now.Add(time.Second)))
	second, err := repo.ClaimNextItem(ctx, job.ID, "worker-partial", now.Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, repo.CompleteItem(ctx, job.ID, second.ID, "worker-partial", service.ImageStudioItemStatusFailed, nil, "upstream failed", nil, now.Add(2*time.Second)))

	settled, err := repo.SettleJob(ctx, job.ID, "worker-partial", now.Add(3*time.Second), func(_ context.Context, _ *service.ImageStudioJob, actual float64) error {
		require.InDelta(t, cost, actual, 0.000001)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, service.ImageStudioJobStatusPartial, settled.Status)
	require.Equal(t, 1, settled.SuccessCount)
	require.Equal(t, 1, settled.FailCount)
	require.Len(t, settled.Assets, 1)
	require.Len(t, settled.Items, 2)
	require.Equal(t, service.ImageStudioItemStatusSuccess, settled.Items[0].Status)
	require.Equal(t, service.ImageStudioItemStatusFailed, settled.Items[1].Status)
}

func integrationImageStudioJob(userID int64, n int) *service.ImageStudioJob {
	hold := 0.08
	return &service.ImageStudioJob{
		ID:                      uuid.NewString(),
		UserID:                  userID,
		TemplateID:              "editorial-poster",
		PromptHash:              fmt.Sprintf("hash-%d", n),
		RequestPayloadEncrypted: fmt.Sprintf("encrypted-%d", n),
		Model:                   "gpt-image-1",
		Quality:                 "standard",
		Size:                    "1024x1024",
		Count:                   1,
		Status:                  service.ImageStudioJobStatusPending,
		EstimatedCost:           hold,
		HoldAmount:              &hold,
		HoldID:                  service.ImageStudioHoldRequestID(fmt.Sprintf("job-%d", n)),
	}
}

func integrationImageStudioItems(job *service.ImageStudioJob) []service.ImageStudioItem {
	return []service.ImageStudioItem{{
		ID:        uuid.NewString(),
		JobID:     job.ID,
		SortOrder: 0,
		Status:    service.ImageStudioItemStatusPending,
	}}
}

func insertImageStudioJobBypassingUserLimit(
	ctx context.Context,
	job *service.ImageStudioJob,
	items []service.ImageStudioItem,
) error {
	if _, err := integrationDB.ExecContext(ctx, `
		INSERT INTO image_studio_jobs (
			id, user_id, template_id, prompt_hash, request_payload_encrypted,
			model, quality, size, count, status, estimated_cost, actual_cost,
			api_key_id, hold_amount, hold_id, success_count, fail_count, expires_at
		)
		VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18
		)`,
		job.ID,
		job.UserID,
		job.TemplateID,
		job.PromptHash,
		job.RequestPayloadEncrypted,
		job.Model,
		job.Quality,
		job.Size,
		job.Count,
		job.Status,
		job.EstimatedCost,
		job.ActualCost,
		job.APIKeyID,
		job.HoldAmount,
		job.HoldID,
		job.SuccessCount,
		job.FailCount,
		job.ExpiresAt,
	); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := integrationDB.ExecContext(ctx, `
			INSERT INTO image_studio_items (id, job_id, sort_order, status)
			VALUES ($1::uuid, $2::uuid, $3, $4)`,
			item.ID, job.ID, item.SortOrder, item.Status,
		); err != nil {
			return err
		}
	}
	return nil
}

func installImageStudioClaimPauseTrigger(ctx context.Context, triggerName, functionName string) error {
	quotedTrigger := pq.QuoteIdentifier(triggerName)
	quotedFunction := pq.QuoteIdentifier(functionName)
	if _, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`
		CREATE FUNCTION %s() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.status = 'running' AND OLD.status IS DISTINCT FROM NEW.status THEN
				PERFORM pg_sleep(0.25);
			END IF;
			RETURN NEW;
		END
		$$`, quotedFunction)); err != nil {
		return err
	}
	_, err := integrationDB.ExecContext(ctx, fmt.Sprintf(`
		CREATE TRIGGER %s
		BEFORE UPDATE ON image_studio_jobs
		FOR EACH ROW EXECUTE FUNCTION %s()`,
		quotedTrigger,
		quotedFunction,
	))
	return err
}

func integrationImageStudioReferencePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 220, G: 40, B: 80, A: 255})
	img.Set(1, 0, color.NRGBA{R: 30, G: 150, B: 230, A: 255})
	img.Set(0, 1, color.NRGBA{R: 80, G: 210, B: 70, A: 255})
	img.Set(1, 1, color.NRGBA{R: 240, G: 190, B: 50, A: 255})
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func cleanupImageStudioJobsForUser(t *testing.T, userID int64) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM image_studio_jobs WHERE user_id = $1`, userID)
	})
}
