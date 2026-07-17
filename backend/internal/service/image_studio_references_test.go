package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type imageStudioReferenceRepoStub struct {
	ImageStudioRepository
	created            *ImageStudioJob
	items              []ImageStudioItem
	refs               []ImageStudioReference
	jobRefs            []ImageStudioJobReference
	createdRef         *ImageStudioReference
	createRefErr       error
	createJobErr       error
	createRefCommitted bool
	createJobCommitted bool
	getJobErr          error
	listRefsErr        error
}

type imageStudioInputCostEstimatorStub struct {
	imageStudioModelResolverStub
	imageInputTokens int
	pricePerToken    float64
}

func (s *imageStudioInputCostEstimatorStub) EstimateImageStudioInputCost(
	_ context.Context,
	_ string,
	_ *APIKey,
	imageInputTokens int,
) (float64, error) {
	s.imageInputTokens = imageInputTokens
	return float64(imageInputTokens) * s.pricePerToken, nil
}

func (s *imageStudioReferenceRepoStub) CreatePendingJob(
	ctx context.Context,
	job *ImageStudioJob,
	items []ImageStudioItem,
	reserve func(context.Context) error,
) error {
	if reserve != nil {
		if err := reserve(ctx); err != nil {
			return err
		}
	}
	copyJob := *job
	copyJob.JobReferences = append([]ImageStudioJobReference(nil), job.JobReferences...)
	s.created = &copyJob
	s.items = append([]ImageStudioItem(nil), items...)
	s.jobRefs = append([]ImageStudioJobReference(nil), job.JobReferences...)
	return s.createJobErr
}

func (s *imageStudioReferenceRepoStub) ListReferencesByID(_ context.Context, userID int64, ids []string) ([]ImageStudioReference, error) {
	if s.listRefsErr != nil {
		return nil, s.listRefsErr
	}
	out := make([]ImageStudioReference, 0, len(ids))
	for _, id := range ids {
		for _, ref := range s.refs {
			if ref.ID == id && ref.UserID == userID {
				out = append(out, ref)
			}
		}
	}
	return out, nil
}

func (s *imageStudioReferenceRepoStub) ListJobReferencesByID(
	_ context.Context,
	jobID string,
	ids []string,
) ([]ImageStudioJobReference, error) {
	out := make([]ImageStudioJobReference, 0, len(ids))
	for _, id := range ids {
		for _, ref := range s.jobRefs {
			if ref.JobID == jobID && ref.ID == id {
				out = append(out, ref)
			}
		}
	}
	return out, nil
}

func (s *imageStudioReferenceRepoStub) CreateReference(_ context.Context, reference *ImageStudioReference) error {
	copyReference := *reference
	s.createdRef = &copyReference
	if s.createRefCommitted {
		s.refs = append(s.refs, copyReference)
	}
	return s.createRefErr
}

func (s *imageStudioReferenceRepoStub) GetJob(_ context.Context, userID int64, jobID string) (*ImageStudioJob, error) {
	if s.getJobErr != nil {
		return nil, s.getJobErr
	}
	if s.createJobCommitted && s.created != nil && s.created.UserID == userID && s.created.ID == jobID {
		copyJob := *s.created
		return &copyJob, nil
	}
	return nil, ErrImageStudioJobNotFound
}

func TestImageStudioCreateReferencePersistsPrivateMetadata(t *testing.T) {
	repo := &imageStudioReferenceRepoStub{}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store
	imageData := encodeImageStudioReferencePNG(t, 2, 2)

	reference, err := svc.CreateReference(context.Background(), 10, "../reference.png", "", imageData)

	require.NoError(t, err)
	require.NotNil(t, reference)
	require.NotNil(t, repo.createdRef)
	require.Equal(t, "reference.png", reference.OriginalFilename)
	require.Equal(t, "image/png", reference.ContentType)
	require.NotEmpty(t, reference.StorageKey)
	require.NotContains(t, reference.StorageKey, "..")
	stored, err := store.Read(reference.StorageKey)
	require.NoError(t, err)
	require.Equal(t, imageData, stored)
}

func TestImageStudioCreateReferenceDeletesPrivateObjectWhenMetadataWriteFails(t *testing.T) {
	repo := &imageStudioReferenceRepoStub{createRefErr: errors.New("reference metadata unavailable")}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store
	imageData := encodeImageStudioReferencePNG(t, 2, 2)

	reference, err := svc.CreateReference(context.Background(), 10, "reference.png", "image/png", imageData)

	require.Nil(t, reference)
	require.ErrorIs(t, err, repo.createRefErr)
	require.NotNil(t, repo.createdRef)
	_, readErr := store.Read(repo.createdRef.StorageKey)
	require.Error(t, readErr)
}

func TestImageStudioCreateReferenceReturnsCommittedMetadataAfterUnknownCommitResult(t *testing.T) {
	commitErr := errors.New("commit response unavailable")
	repo := &imageStudioReferenceRepoStub{
		createRefErr:       commitErr,
		createRefCommitted: true,
	}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	reference, err := svc.CreateReference(
		context.Background(),
		10,
		"reference.png",
		"image/png",
		encodeImageStudioReferencePNG(t, 2, 2),
	)

	require.NoError(t, err)
	require.NotNil(t, reference)
	_, readErr := store.Read(reference.StorageKey)
	require.NoError(t, readErr)
}

func TestImageStudioCreateReferenceKeepsObjectWhenCommitCannotBeVerified(t *testing.T) {
	commitErr := errors.New("commit response unavailable")
	repo := &imageStudioReferenceRepoStub{
		createRefErr: commitErr,
		listRefsErr:  errors.New("database unavailable"),
	}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	reference, err := svc.CreateReference(
		context.Background(),
		10,
		"reference.png",
		"image/png",
		encodeImageStudioReferencePNG(t, 2, 2),
	)

	require.Nil(t, reference)
	require.ErrorIs(t, err, commitErr)
	require.NotNil(t, repo.createdRef)
	_, readErr := store.Read(repo.createdRef.StorageKey)
	require.NoError(t, readErr)
}

func TestImageStudioCreateReferenceRejectsSpoofedImageContentType(t *testing.T) {
	repo := &imageStudioReferenceRepoStub{}
	store := NewImageStudioAssetStore(t.TempDir())
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	reference, err := svc.CreateReference(
		context.Background(),
		10,
		"not-an-image.png",
		"image/png",
		[]byte("plain text disguised as an image"),
	)

	require.Nil(t, reference)
	require.ErrorIs(t, err, ErrImageStudioReferenceInvalid)
	require.Nil(t, repo.createdRef)
}

func TestImageStudioCreateReferenceDecodesSupportedFormats(t *testing.T) {
	testCases := []struct {
		name        string
		contentType string
		data        func(*testing.T) []byte
	}{
		{name: "png", contentType: "image/png", data: func(t *testing.T) []byte {
			return encodeImageStudioReferencePNG(t, 2, 2)
		}},
		{name: "jpeg", contentType: "image/jpeg", data: func(t *testing.T) []byte {
			return encodeImageStudioReferenceJPEG(t, 2, 2)
		}},
		{name: "webp", contentType: "image/webp", data: decodeImageStudioReferenceWebP},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &imageStudioReferenceRepoStub{}
			svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
			svc.assetStore = NewImageStudioAssetStore(t.TempDir())

			reference, err := svc.CreateReference(
				context.Background(),
				10,
				"reference."+tc.name,
				tc.contentType,
				tc.data(t),
			)

			require.NoError(t, err)
			require.Equal(t, tc.contentType, reference.ContentType)
		})
	}
}

func TestImageStudioCreateReferenceRejectsTruncatedOrCorruptImages(t *testing.T) {
	valid := encodeImageStudioReferencePNG(t, 4, 4)
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)-8] ^= 0xff
	testCases := map[string][]byte{
		"truncated": valid[:len(valid)-12],
		"corrupt":   corrupt,
	}
	for name, data := range testCases {
		t.Run(name, func(t *testing.T) {
			repo := &imageStudioReferenceRepoStub{}
			svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
			svc.assetStore = NewImageStudioAssetStore(t.TempDir())

			reference, err := svc.CreateReference(context.Background(), 10, "reference.png", "image/png", data)

			require.Nil(t, reference)
			require.ErrorIs(t, err, ErrImageStudioReferenceInvalid)
			require.Nil(t, repo.createdRef)
		})
	}
}

func TestImageStudioCreateReferenceRejectsUnsafePixelCountBeforeDecode(t *testing.T) {
	repo := &imageStudioReferenceRepoStub{}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = NewImageStudioAssetStore(t.TempDir())

	reference, err := svc.CreateReference(
		context.Background(),
		10,
		"oversized.png",
		"image/png",
		encodeImageStudioReferencePNGHeader(t, 10000, 10000),
	)

	require.Nil(t, reference)
	require.ErrorIs(t, err, ErrImageStudioReferenceInvalid)
	require.Nil(t, repo.createdRef)
}

func TestImageStudioCreatePendingEditJobStoresOnlyJobReferenceIDs(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 2, 2)
	sourceKey, err := store.Save(10, "ref-private-1", "image/png", sourceData)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{{
			ID:          "ref-private-1",
			UserID:      10,
			StorageKey:  sourceKey,
			ContentType: "image/png",
			ByteSize:    int64(len(sourceData)),
		}},
	}
	encryptor := &imageStudioEncryptorStub{}
	svc := newImageStudioReferenceServiceForTest(repo, encryptor, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:         "edit",
		TemplateID:   "free-create",
		UserPrompt:   "replace the background",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-1",
		APIKeyID:     20,
		ReferenceIDs: []string{"ref-private-1"},
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Len(t, repo.jobRefs, 1)
	require.Equal(t, job.ID, repo.jobRefs[0].JobID)
	require.NotEqual(t, "ref-private-1", repo.jobRefs[0].ID)
	require.NotEqual(t, sourceKey, repo.jobRefs[0].StorageKey)
	require.NotContains(t, encryptor.plaintext, "ref-private-1")
	require.NotContains(t, encryptor.plaintext, sourceKey)
	require.Contains(t, encryptor.plaintext, repo.jobRefs[0].ID)
	require.Contains(t, encryptor.plaintext, `"/v1/images/edits"`)
	stored, err := store.Read(repo.jobRefs[0].StorageKey)
	require.NoError(t, err)
	require.Equal(t, sourceData, stored)
}

func TestImageStudioCreatePendingEditJobReservesConservativeReferenceInputCost(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 1024, 512)
	sourceKey, err := store.Save(10, "ref-priced-1", "image/png", sourceData)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{{
			ID:          "ref-priced-1",
			UserID:      10,
			StorageKey:  sourceKey,
			ContentType: "image/png",
			ByteSize:    int64(len(sourceData)),
		}},
	}
	billing := &imageStudioCreateBillingStub{}
	estimator := &imageStudioInputCostEstimatorStub{
		imageStudioModelResolverStub: imageStudioModelResolverStub{models: []string{"gpt-image-1"}},
		pricePerToken:                0.00001,
	}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, billing)
	svc.assetStore = store
	svc.gateway = estimator

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:          "edit",
		TemplateID:    "free-create",
		UserPrompt:    "replace the background",
		Size:          "1024x1024",
		Count:         2,
		Model:         "gpt-image-1",
		APIKeyID:      20,
		InputFidelity: "high",
		ReferenceIDs:  []string{"ref-priced-1"},
	})

	require.NoError(t, err)
	require.Positive(t, estimator.imageInputTokens)
	require.NotNil(t, billing.lastReserve)
	require.Greater(t, billing.lastReserve.HoldAmount, 0.08)
	require.InDelta(
		t,
		0.08+float64(estimator.imageInputTokens)*estimator.pricePerToken,
		billing.lastReserve.HoldAmount,
		0.000001,
	)
	require.InDelta(t, billing.lastReserve.HoldAmount, job.EstimatedCost, 0.000001)
}

func TestImageStudioEstimateEditIncludesOwnerScopedReferenceInputCost(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 1024, 512)
	sourceKey, err := store.Save(10, "ref-estimate-priced", "image/png", sourceData)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{
			{
				ID:          "ref-estimate-priced",
				UserID:      10,
				StorageKey:  sourceKey,
				ContentType: "image/png",
				ByteSize:    int64(len(sourceData)),
			},
			{
				ID:          "ref-other-owner",
				UserID:      11,
				StorageKey:  sourceKey,
				ContentType: "image/png",
				ByteSize:    int64(len(sourceData)),
			},
		},
	}
	estimator := &imageStudioInputCostEstimatorStub{
		imageStudioModelResolverStub: imageStudioModelResolverStub{models: []string{"gpt-image-1"}},
		pricePerToken:                0.00001,
	}
	svc := newImageStudioReferenceServiceForTest(
		repo,
		&imageStudioEncryptorStub{},
		&imageStudioCreateBillingStub{},
	)
	svc.assetStore = store
	svc.gateway = estimator

	estimate, err := svc.Estimate(
		context.Background(),
		10,
		"free-create",
		"1024x1024",
		2,
		20,
		"gpt-image-1",
		[]string{"ref-estimate-priced"},
	)

	require.NoError(t, err)
	require.Positive(t, estimator.imageInputTokens)
	require.InDelta(
		t,
		0.08+float64(estimator.imageInputTokens)*estimator.pricePerToken,
		estimate.EstimatedCost,
		0.000001,
	)
	require.Equal(t, 100.0, estimate.Balance)
	require.True(t, estimate.Sufficient)

	_, err = svc.Estimate(
		context.Background(),
		10,
		"free-create",
		"1024x1024",
		1,
		20,
		"gpt-image-1",
		[]string{"ref-other-owner"},
	)
	require.ErrorIs(t, err, ErrImageStudioReferenceNotFound)
}

func TestImageStudioCreatePendingEditJobRejectsFifthReference(t *testing.T) {
	repo := &imageStudioReferenceRepoStub{}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:         "edit",
		TemplateID:   "free-create",
		UserPrompt:   "replace the background",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-1",
		APIKeyID:     20,
		ReferenceIDs: []string{"ref-1", "ref-2", "ref-3", "ref-4", "ref-5"},
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, ErrImageStudioReferenceLimit)
	require.Nil(t, repo.created)
}

func TestImageStudioCreatePendingEditJobDeletesJobCopyWhenPersistenceFails(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 2, 2)
	sourceKey, err := store.Save(10, "ref-private-1", "image/png", sourceData)
	require.NoError(t, err)
	persistErr := errors.New("job persistence failed")
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{{
			ID:          "ref-private-1",
			UserID:      10,
			StorageKey:  sourceKey,
			ContentType: "image/png",
			ByteSize:    int64(len(sourceData)),
		}},
		createJobErr: persistErr,
	}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:         "edit",
		TemplateID:   "free-create",
		UserPrompt:   "replace the background",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-1",
		APIKeyID:     20,
		ReferenceIDs: []string{"ref-private-1"},
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, persistErr)
	require.Len(t, repo.jobRefs, 1)
	_, err = store.Read(repo.jobRefs[0].StorageKey)
	require.Error(t, err)
	source, err := store.Read(sourceKey)
	require.NoError(t, err)
	require.Equal(t, sourceData, source)
}

func TestImageStudioCreatePendingEditJobReturnsCommittedJobAfterUnknownCommitResult(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 2, 2)
	sourceKey, err := store.Save(10, "ref-private-committed", "image/png", sourceData)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{{
			ID:          "ref-private-committed",
			UserID:      10,
			StorageKey:  sourceKey,
			ContentType: "image/png",
			ByteSize:    int64(len(sourceData)),
		}},
		createJobErr:       errors.New("commit response unavailable"),
		createJobCommitted: true,
	}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:         "edit",
		TemplateID:   "free-create",
		UserPrompt:   "replace the background",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-1",
		APIKeyID:     20,
		ReferenceIDs: []string{"ref-private-committed"},
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Len(t, repo.jobRefs, 1)
	_, readErr := store.Read(repo.jobRefs[0].StorageKey)
	require.NoError(t, readErr)
}

func TestImageStudioCreatePendingEditJobKeepsCopyWhenCommitCannotBeVerified(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	sourceData := encodeImageStudioReferencePNG(t, 2, 2)
	sourceKey, err := store.Save(10, "ref-private-uncertain", "image/png", sourceData)
	require.NoError(t, err)
	commitErr := errors.New("commit response unavailable")
	repo := &imageStudioReferenceRepoStub{
		refs: []ImageStudioReference{{
			ID:          "ref-private-uncertain",
			UserID:      10,
			StorageKey:  sourceKey,
			ContentType: "image/png",
			ByteSize:    int64(len(sourceData)),
		}},
		createJobErr: commitErr,
		getJobErr:    errors.New("database unavailable"),
	}
	svc := newImageStudioReferenceServiceForTest(repo, &imageStudioEncryptorStub{}, &imageStudioCreateBillingStub{})
	svc.assetStore = store

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:         "edit",
		TemplateID:   "free-create",
		UserPrompt:   "replace the background",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-1",
		APIKeyID:     20,
		ReferenceIDs: []string{"ref-private-uncertain"},
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, commitErr)
	require.Len(t, repo.jobRefs, 1)
	_, readErr := store.Read(repo.jobRefs[0].StorageKey)
	require.NoError(t, readErr)
}

func TestImageStudioBuildWorkerRequestReadsJobOwnedReferencesForEdits(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	imageData := encodeImageStudioReferencePNG(t, 2, 2)
	key, err := store.Save(10, "job-ref-private-1", "image/png", imageData)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		jobRefs: []ImageStudioJobReference{{
			ID:          "job-ref-private-1",
			JobID:       "job-private-1",
			StorageKey:  key,
			ContentType: "image/png",
			ByteSize:    int64(len(imageData)),
		}},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}
	decrypted := `{"endpoint":"/v1/images/edits","body":{"model":"gpt-image-1","prompt":"replace the background","n":4,"size":"1024x1024","response_format":"b64_json","image_studio_job_reference_ids":["job-ref-private-1"],"input_fidelity":"high"}}`

	req, err := svc.BuildWorkerRequest(context.Background(), &ImageStudioJob{ID: "job-private-1", UserID: 10}, decrypted)

	require.NoError(t, err)
	require.Equal(t, "/v1/images/edits", req.Endpoint)
	require.Contains(t, req.ContentType, "multipart/form-data")
	mediaType, params, err := mime.ParseMediaType(req.ContentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	reader := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
	fields := map[string]string{}
	var imageBytes []byte
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(part)
		require.NoError(t, err)
		if part.FileName() != "" {
			require.Equal(t, "image", part.FormName())
			require.Equal(t, "image/png", part.Header.Get("Content-Type"))
			imageBytes = data
			continue
		}
		fields[part.FormName()] = string(data)
	}
	require.Equal(t, imageData, imageBytes)
	require.Equal(t, "1", fields["n"])
	require.Equal(t, "replace the background", fields["prompt"])
	require.Equal(t, "high", fields["input_fidelity"])
	require.NotContains(t, strings.Join(imageStudioStringMapValues(fields), "\n"), "job-ref-private-1")
}

func newImageStudioReferenceServiceForTest(
	repo ImageStudioRepository,
	encryptor SecretEncryptor,
	billing UsageBillingRepository,
) *ImageStudioService {
	settingService := NewSettingService(&imageStudioSettingRepoStub{values: map[string]string{
		SettingKeyImageStudioEnabled: "true",
	}}, &config.Config{})
	playService := NewPlayService(nil, nil, nil, settingService, nil, nil)
	userRepo := &imageStudioCreateUserRepoStub{user: &User{
		ID:             10,
		Balance:        100,
		TotalRecharged: 100,
	}}
	groupID := int64(30)
	apiKey := &APIKey{
		ID:      20,
		UserID:  10,
		GroupID: &groupID,
		Status:  StatusAPIKeyActive,
		Group: &Group{
			ID:                   groupID,
			Platform:             PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}
	apiKeyService := NewAPIKeyService(
		&imageStudioCreateAPIKeyRepoStub{key: apiKey},
		userRepo,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)
	return NewImageStudioService(
		repo,
		nil,
		apiKeyService,
		userRepo,
		settingService,
		playService,
		nil,
		&imageStudioInputCostEstimatorStub{
			imageStudioModelResolverStub: imageStudioModelResolverStub{models: []string{"gpt-image-1"}},
		},
		encryptor,
		billing,
		nil,
	)
}

func imageStudioStringMapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func encodeImageStudioReferencePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x + 20), G: uint8(y + 40), B: 120, A: 255})
		}
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}

func encodeImageStudioReferenceJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: uint8(x + y), B: 160, A: 255})
		}
	}
	var out bytes.Buffer
	require.NoError(t, jpeg.Encode(&out, img, nil))
	return out.Bytes()
}

func decodeImageStudioReferenceWebP(t *testing.T) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(
		"UklGRhwAAABXRUJQVlA4TA8AAAAvAUAAAAcQ/Y/+ByKi/wEA",
	)
	require.NoError(t, err)
	return data
}

func encodeImageStudioReferencePNGHeader(t *testing.T, width, height uint32) []byte {
	t.Helper()
	var out bytes.Buffer
	_, err := out.Write([]byte("\x89PNG\r\n\x1a\n"))
	require.NoError(t, err)
	require.NoError(t, binary.Write(&out, binary.BigEndian, uint32(13)))
	_, err = out.WriteString("IHDR")
	require.NoError(t, err)
	var header bytes.Buffer
	require.NoError(t, binary.Write(&header, binary.BigEndian, width))
	require.NoError(t, binary.Write(&header, binary.BigEndian, height))
	_, err = header.Write([]byte{8, 6, 0, 0, 0})
	require.NoError(t, err)
	_, err = out.Write(header.Bytes())
	require.NoError(t, err)
	require.NoError(t, binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(append([]byte("IHDR"), header.Bytes()...))))
	return out.Bytes()
}
