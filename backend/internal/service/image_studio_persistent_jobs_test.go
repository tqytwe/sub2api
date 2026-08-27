package service

import (
	"context"
	"encoding/base64"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageStudioManagedBillingContext(t *testing.T) {
	ctx := WithImageStudioManagedBilling(context.Background())
	require.True(t, IsImageStudioManagedBilling(ctx))
	require.False(t, IsImageStudioManagedBilling(context.Background()))
}

func TestImageStudioBillingRequestIDsAreIndependentFromBatchImage(t *testing.T) {
	const jobID = "job-123"
	require.Equal(t, "image_studio_hold:"+jobID, ImageStudioHoldRequestID(jobID))
	require.Equal(t, "image_studio_capture:"+jobID, ImageStudioCaptureRequestID(jobID))
	require.Equal(t, "image_studio_release:"+jobID, ImageStudioReleaseRequestID(jobID))
	require.NotEqual(t, BatchImageHoldRequestID(jobID), ImageStudioHoldRequestID(jobID))
}

type imageStudioEffectiveRateResolverStub struct {
	rate float64
}

func (s *imageStudioEffectiveRateResolverStub) GetAvailableModels(context.Context, *int64, string) []string {
	return []string{"gpt-image-1"}
}

func (s *imageStudioEffectiveRateResolverStub) ResolveUserGroupRateMultiplier(
	context.Context,
	int64,
	int64,
	float64,
) float64 {
	return s.rate
}

func TestImageStudioEstimateCostUsesEffectiveUserGroupRate(t *testing.T) {
	groupID := int64(30)
	price := 0.04
	svc := &ImageStudioService{
		gateway: &imageStudioEffectiveRateResolverStub{rate: 2},
	}
	apiKey := &APIKey{
		UserID:  10,
		GroupID: &groupID,
		Group: &Group{
			ID:             groupID,
			RateMultiplier: 1,
			ImagePrice1K:   &price,
		},
	}

	cost, err := svc.estimateCost(context.Background(), apiKey, "gpt-image-1", "1024x1024", 1)

	require.NoError(t, err)
	require.InDelta(t, 0.08, cost, 0.000001)
}

func TestImageStudioEstimateCostKeepsIndependentImageRate(t *testing.T) {
	groupID := int64(30)
	price := 0.04
	svc := &ImageStudioService{
		gateway: &imageStudioEffectiveRateResolverStub{rate: 2},
	}
	apiKey := &APIKey{
		UserID:  10,
		GroupID: &groupID,
		Group: &Group{
			ID:                   groupID,
			RateMultiplier:       1,
			ImageRateIndependent: true,
			ImageRateMultiplier:  0.5,
			ImagePrice1K:         &price,
		},
	}

	cost, err := svc.estimateCost(context.Background(), apiKey, "gpt-image-1", "1024x1024", 1)

	require.NoError(t, err)
	require.InDelta(t, 0.02, cost, 0.000001)
}

func TestBuildUsageBillingCommandKeepsQuotaAccountingForImageStudioManagedBilling(t *testing.T) {
	ctx := WithImageStudioManagedBilling(context.Background())
	cmd := buildUsageBillingCommandForContext(ctx, "request-1", nil, &postUsageBillingParams{
		Cost: &CostBreakdown{
			TotalCost:  2.5,
			ActualCost: 3.0,
		},
		User: &User{ID: 10},
		APIKey: &APIKey{
			ID:    20,
			Quota: 100,
		},
		Account: &Account{
			ID:   30,
			Type: AccountTypeAPIKey,
		},
		AccountRateMultiplier: 1,
		APIKeyService:         &imageStudioAPIKeyQuotaUpdater{},
	})

	require.NotNil(t, cmd)
	require.InDelta(t, 3.0, cmd.ActualCost, 0.000001)
	require.Zero(t, cmd.BalanceCost)
	require.Zero(t, cmd.SubscriptionCost)
	require.InDelta(t, 3.0, cmd.APIKeyQuotaCost, 0.000001)
	require.Zero(t, cmd.APIKeyRateLimitCost)
	require.Zero(t, cmd.AccountQuotaCost)
}

func TestOpenAIGatewayManagedImageStudioUsagePersistsCostWithoutOrdinaryBilling(t *testing.T) {
	imagePrice := 0.25
	groupID := int64(77)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)
	capture := NewImageStudioBillingCapture()
	ctx := WithImageStudioBillingCapture(context.Background(), capture)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:  "image-studio-item-job-1-0",
			Model:      "gpt-image-2",
			ImageCount: 1,
			ImageSize:  "1K",
			Duration:   time.Second,
		},
		APIKey: &APIKey{
			ID:          20,
			UserID:      10,
			Quota:       100,
			RateLimit5h: 100,
			GroupID:     &groupID,
			Group: &Group{
				ID:             groupID,
				Platform:       PlatformOpenAI,
				RateMultiplier: 1,
				ImagePrice1K:   &imagePrice,
			},
		},
		User: &User{ID: 10},
		Account: &Account{
			ID:    30,
			Type:  AccountTypeAPIKey,
			Extra: map[string]any{"quota_limit": 100.0},
		},
		APIKeyService: quotaSvc,
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, imagePrice, usageRepo.lastLog.ActualCost, 0.000001)
	require.Equal(t, 1, billingRepo.calls)
	require.NotNil(t, billingRepo.lastCmd)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.Zero(t, billingRepo.lastCmd.SubscriptionCost)
	require.InDelta(t, imagePrice, billingRepo.lastCmd.APIKeyQuotaCost, 0.000001)
	require.InDelta(t, imagePrice, billingRepo.lastCmd.APIKeyRateLimitCost, 0.000001)
	require.InDelta(t, imagePrice, billingRepo.lastCmd.AccountQuotaCost, 0.000001)
	require.Zero(t, userRepo.deductCalls)
	require.Zero(t, subRepo.incrementCalls)
	require.Zero(t, quotaSvc.quotaCalls)
	require.Zero(t, quotaSvc.rateLimitCalls)
	capturedCost, ok := capture.Wait(context.Background())
	require.True(t, ok)
	require.InDelta(t, imagePrice, capturedCost, 0.000001)
}

func TestOpenAIGatewayManagedImageStudioUsageUsesHeldActualCostSnapshot(t *testing.T) {
	imagePrice := 0.25
	heldPerItem := 0.10
	groupID := int64(79)
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		usageRepo,
		billingRepo,
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	capture := NewImageStudioBillingCapture()
	ctx := WithImageStudioBillingActualCostCap(
		WithImageStudioBillingCapture(context.Background(), capture),
		heldPerItem,
	)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:  "image-studio-item-held-snapshot",
			Model:      "gpt-image-2",
			ImageCount: 1,
			ImageSize:  "1K",
			Duration:   time.Second,
		},
		APIKey: &APIKey{
			ID:          22,
			UserID:      12,
			Quota:       100,
			RateLimit5h: 100,
			GroupID:     &groupID,
			Group: &Group{
				ID:             groupID,
				Platform:       PlatformOpenAI,
				RateMultiplier: 1,
				ImagePrice1K:   &imagePrice,
			},
		},
		User: &User{ID: 12},
		Account: &Account{
			ID:    32,
			Type:  AccountTypeAPIKey,
			Extra: map[string]any{"quota_limit": 100.0},
		},
		APIKeyService: &imageStudioAPIKeyQuotaUpdater{},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, imagePrice, usageRepo.lastLog.TotalCost, 0.000001)
	require.InDelta(t, heldPerItem, usageRepo.lastLog.ActualCost, 0.000001)
	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, heldPerItem, billingRepo.lastCmd.ActualCost, 0.000001)
	require.InDelta(t, heldPerItem, billingRepo.lastCmd.APIKeyQuotaCost, 0.000001)
	capturedCost, ok := capture.Wait(context.Background())
	require.True(t, ok)
	require.InDelta(t, heldPerItem, capturedCost, 0.000001)
}

func TestOpenAIGatewayManagedImageStudioUsageCapturesCostWhenBillingPersistenceFails(t *testing.T) {
	imagePrice := 0.25
	heldPerItem := 0.10
	groupID := int64(78)
	persistErr := errors.New("usage billing persistence unavailable")
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{err: persistErr}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)
	capture := NewImageStudioBillingCapture()
	ctx := WithImageStudioBillingActualCostCap(
		WithImageStudioBillingCapture(context.Background(), capture),
		heldPerItem,
	)

	err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:  "image-studio-item-job-1-1",
			Model:      "gpt-image-2",
			ImageCount: 1,
			ImageSize:  "1K",
			Duration:   time.Second,
		},
		APIKey: &APIKey{
			ID:          21,
			UserID:      11,
			Quota:       100,
			RateLimit5h: 100,
			GroupID:     &groupID,
			Group: &Group{
				ID:             groupID,
				Platform:       PlatformOpenAI,
				RateMultiplier: 1,
				ImagePrice1K:   &imagePrice,
			},
		},
		User: &User{ID: 11},
		Account: &Account{
			ID:    31,
			Type:  AccountTypeAPIKey,
			Extra: map[string]any{"quota_limit": 100.0},
		},
		APIKeyService: &imageStudioAPIKeyQuotaUpdater{},
	})

	require.ErrorIs(t, err, persistErr)
	require.Equal(t, 1, billingRepo.calls)
	require.Equal(t, 1, usageRepo.calls)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, imagePrice, usageRepo.lastLog.TotalCost, 0.000001)
	require.InDelta(t, heldPerItem, usageRepo.lastLog.ActualCost, 0.000001)
	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, heldPerItem, billingRepo.lastCmd.ActualCost, 0.000001)
	costCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	capturedCost, ok := capture.Wait(costCtx)
	require.True(t, ok)
	require.InDelta(t, heldPerItem, capturedCost, 0.000001)
}

func TestGrokImageStudioManagedEditUsageCapturesFallbackInputCost(t *testing.T) {
	reference := encodeImageStudioReferencePNG(t, 1024, 512)
	requestInfo, err := ParseGrokMediaRequestWithError(
		"application/json",
		[]byte(`{
			"model":"grok-image-edit-priced",
			"prompt":"edit",
			"images":[{"type":"image_url","url":"data:image/png;base64,`+
			base64.StdEncoding.EncodeToString(reference)+`"}]
		}`),
	)
	require.NoError(t, err)
	usage := grokMediaUsageFromResponse(
		GrokMediaEndpointImagesEdits,
		requestInfo,
		[]byte(`{"data":[{"url":"data:image/png;base64,QQ=="}]}`),
	)

	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{
		result: &UsageBillingApplyResult{Applied: true},
	}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(
		usageRepo,
		billingRepo,
		&openAIRecordUsageUserRepoStub{},
		&openAIRecordUsageSubRepoStub{},
		nil,
	)
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"grok-image-edit-priced": {
			InputCostPerToken:      0.000001,
			InputCostPerImageToken: 0.00001,
		},
	}}
	svc.billingService = NewBillingService(&config.Config{}, pricing)

	groupID := int64(30)
	outputPrice := 0.04
	capture := NewImageStudioBillingCapture()
	ctx := WithImageStudioBillingCapture(context.Background(), capture)
	err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:    "image-studio-grok-edit-cost",
			Model:        "grok-image-edit-priced",
			BillingModel: "grok-image-edit-priced",
			Usage:        usage.Usage,
			ImageCount:   1,
			ImageSize:    "1K",
			Duration:     time.Second,
		},
		APIKey: &APIKey{
			ID:      21,
			UserID:  11,
			GroupID: &groupID,
			Group: &Group{
				ID:                   groupID,
				Platform:             PlatformGrok,
				RateMultiplier:       1,
				AllowImageGeneration: true,
				ImagePrice1K:         &outputPrice,
			},
		},
		User:    &User{ID: 11},
		Account: &Account{ID: 31, Type: AccountTypeAPIKey},
	})

	require.NoError(t, err)
	costCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	capturedCost, ok := capture.Wait(costCtx)
	require.True(t, ok)
	expected := outputPrice + float64(usage.Usage.ImageInputTokens)*0.00001
	require.InDelta(t, expected, capturedCost, 0.000001)
	require.NotNil(t, billingRepo.lastCmd)
	require.InDelta(t, expected, billingRepo.lastCmd.ActualCost, 0.000001)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, usage.Usage.ImageInputTokens, usageRepo.lastLog.ImageInputTokens)
}

type imageStudioEncryptorStub struct {
	plaintext string
}

func (s *imageStudioEncryptorStub) Encrypt(plaintext string) (string, error) {
	s.plaintext = plaintext
	return "ciphertext-only", nil
}

func (*imageStudioEncryptorStub) Decrypt(string) (string, error) {
	return "", nil
}

type imageStudioCreateRepoStub struct {
	ImageStudioRepository
	created *ImageStudioJob
	items   []ImageStudioItem
}

func (s *imageStudioCreateRepoStub) CreatePendingJob(
	ctx context.Context,
	job *ImageStudioJob,
	items []ImageStudioItem,
	reserve func(context.Context) error,
) error {
	if err := reserve(ctx); err != nil {
		return err
	}
	copyJob := *job
	s.created = &copyJob
	s.items = append([]ImageStudioItem(nil), items...)
	return nil
}

type imageStudioIdempotentReplayRepoStub struct {
	ImageStudioRepository
	job *ImageStudioJob
}

func (s *imageStudioIdempotentReplayRepoStub) FindJobByIdempotency(
	_ context.Context,
	userID int64,
	keyHash string,
	fingerprint string,
) (string, bool, error) {
	if s.job == nil {
		return "", false, nil
	}
	if s.job.UserID != userID || s.job.IdempotencyKeyHash != keyHash {
		return "", false, nil
	}
	if s.job.IdempotencyFingerprint != fingerprint {
		return "", false, ErrIdempotencyKeyConflict
	}
	return s.job.ID, true, nil
}

func (s *imageStudioIdempotentReplayRepoStub) CreatePendingJobIdempotent(
	context.Context,
	*ImageStudioJob,
	[]ImageStudioItem,
	func(context.Context) error,
) (string, bool, error) {
	panic("CreatePendingJobIdempotent must not run for an existing idempotent job")
}

func (s *imageStudioIdempotentReplayRepoStub) GetJob(
	_ context.Context,
	userID int64,
	jobID string,
) (*ImageStudioJob, error) {
	if s.job != nil && s.job.UserID == userID && s.job.ID == jobID {
		copyJob := *s.job
		return &copyJob, nil
	}
	return nil, ErrImageStudioJobNotFound
}

type imageStudioCreateUserRepoStub struct {
	UserRepository
	user *User
}

func (s *imageStudioCreateUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

type imageStudioCreateAPIKeyRepoStub struct {
	APIKeyRepository
	key *APIKey
}

func (s *imageStudioCreateAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	return s.key, nil
}

type imageStudioCreateBillingStub struct {
	UsageBillingRepository
	reserveErr   error
	reserveCalls int
	releaseCalls int
	lastReserve  *BatchImageBalanceHoldCommand
}

type imageStudioCancelRepoStub struct {
	ImageStudioRepository
}

func (s *imageStudioCancelRepoStub) RequestCancel(
	ctx context.Context,
	userID int64,
	jobID string,
	_ time.Time,
	release func(context.Context, *ImageStudioJob) error,
) (*ImageStudioJob, error) {
	apiKeyID := int64(20)
	holdAmount := 1.25
	job := &ImageStudioJob{
		ID:         jobID,
		UserID:     userID,
		Status:     ImageStudioJobStatusCancelled,
		APIKeyID:   &apiKeyID,
		HoldAmount: &holdAmount,
		HoldID:     ImageStudioHoldRequestID(jobID),
	}
	if release != nil {
		if err := release(ctx, job); err != nil {
			return nil, err
		}
	}
	return job, nil
}

type imageStudioCancelBillingStub struct {
	UsageBillingRepository
	releaseCalls atomic.Int64
}

func (s *imageStudioCancelBillingStub) ReleaseBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.releaseCalls.Add(1)
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

type imageStudioBalanceInvalidateCacheStub struct {
	BillingCache
	calls  atomic.Int64
	userID atomic.Int64
}

func (s *imageStudioBalanceInvalidateCacheStub) InvalidateUserBalance(_ context.Context, userID int64) error {
	s.calls.Add(1)
	s.userID.Store(userID)
	return nil
}

func TestImageStudioCancelJobInvalidatesBalanceCache(t *testing.T) {
	cache := &imageStudioBalanceInvalidateCacheStub{}
	billingCache := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(billingCache.Stop)
	billingRepo := &imageStudioCancelBillingStub{}
	svc := NewImageStudioService(
		&imageStudioCancelRepoStub{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		billingRepo,
		billingCache,
	)

	job, err := svc.CancelJob(context.Background(), 10, "job-cancel-cache")

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, int64(1), billingRepo.releaseCalls.Load())
	require.Equal(t, int64(1), cache.calls.Load())
	require.Equal(t, int64(10), cache.userID.Load())
}

type imageStudioSettingRepoStub struct {
	SettingRepository
	values map[string]string
}

func (s *imageStudioSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *imageStudioCreateBillingStub) ReserveBatchImageBalance(_ context.Context, cmd *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.reserveCalls++
	s.lastReserve = cmd
	if s.reserveErr != nil {
		return nil, s.reserveErr
	}
	return &BatchImageBalanceHoldResult{Applied: true}, nil
}

func (s *imageStudioCreateBillingStub) ReleaseBatchImageBalance(context.Context, *BatchImageBalanceHoldCommand) (*BatchImageBalanceHoldResult, error) {
	s.releaseCalls++
	return &BatchImageBalanceHoldResult{}, ErrUsageBillingHoldNotFound
}

func TestImageStudioCreatePendingJobEncryptsPromptAndReservesBeforePersist(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	encryptor := &imageStudioEncryptorStub{}
	billing := &imageStudioCreateBillingStub{}
	svc := newImageStudioCreateServiceForTest(repo, encryptor, billing)
	const prompt = "private launch artwork"

	job, body, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID: "free-create",
		UserPrompt: prompt,
		Size:       "1024x1024",
		Count:      2,
		Model:      "gpt-image-1",
		APIKeyID:   20,
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.NotEmpty(t, body)
	require.Contains(t, encryptor.plaintext, prompt)
	require.NotNil(t, repo.created)
	require.Equal(t, "ciphertext-only", repo.created.RequestPayloadEncrypted)
	require.NotContains(t, repo.created.RequestPayloadEncrypted, prompt)
	require.NotContains(t, repo.created.PromptHash, prompt)
	require.Len(t, repo.items, 2)
	require.Equal(t, 1, billing.reserveCalls)
	require.NotNil(t, billing.lastReserve)
	require.Equal(t, ImageStudioHoldRequestID(job.ID), billing.lastReserve.RequestID)
	require.Equal(t, ImageStudioHoldRequestID(job.ID), billing.lastReserve.HoldRequestID)
	require.False(t, billing.lastReserve.AllowBalanceOverage)
}

func TestImageStudioCreatePendingJobHoldFailureDoesNotPersist(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	billing := &imageStudioCreateBillingStub{reserveErr: ErrBatchImageInsufficientBalance}
	svc := newImageStudioCreateServiceForTest(repo, &imageStudioEncryptorStub{}, billing)

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID: "free-create",
		UserPrompt: "private prompt",
		Size:       "1024x1024",
		Count:      1,
		Model:      "gpt-image-1",
		APIKeyID:   20,
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, ErrImageStudioInsufficientBalance)
	require.Nil(t, repo.created)
	require.Equal(t, 1, billing.reserveCalls)
	require.Zero(t, billing.releaseCalls)
}

func TestImageStudioCreatePendingJobRejectsSubscriptionGroupBeforeHold(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	billing := &imageStudioCreateBillingStub{}
	svc := newImageStudioCreateServiceForTest(repo, &imageStudioEncryptorStub{}, billing)
	key, err := svc.apiKeyService.GetByID(context.Background(), 20)
	require.NoError(t, err)
	key.Group.SubscriptionType = SubscriptionTypeSubscription

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID: "free-create",
		UserPrompt: "private prompt",
		Size:       "1024x1024",
		Count:      1,
		Model:      "gpt-image-1",
		APIKeyID:   20,
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, ErrImageStudioSubscriptionGroupUnsupported)
	require.Nil(t, repo.created)
	require.Zero(t, billing.reserveCalls)
}

func TestImageStudioCreatePendingJobReplaysCommittedJobBeforeReferenceValidation(t *testing.T) {
	existing := &ImageStudioJob{
		ID:                     "existing-job",
		UserID:                 10,
		Status:                 ImageStudioJobStatusPending,
		IdempotencyKeyHash:     HashIdempotencyKey("same-edit-submit"),
		IdempotencyFingerprint: "same-edit-fingerprint",
	}
	repo := &imageStudioIdempotentReplayRepoStub{job: existing}
	svc := &ImageStudioService{repo: repo}

	job, body, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		Mode:                   "edit",
		ReferenceIDs:           []string{"already-expired-reference"},
		IdempotencyKeyHash:     existing.IdempotencyKeyHash,
		IdempotencyFingerprint: existing.IdempotencyFingerprint,
	})

	require.NoError(t, err)
	require.Equal(t, existing.ID, job.ID)
	require.Empty(t, body)
}

func newImageStudioCreateServiceForTest(
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
		nil,
		encryptor,
		billing,
		nil,
	)
}

type imageStudioAPIKeyQuotaUpdater struct{}

func (*imageStudioAPIKeyQuotaUpdater) UpdateQuotaUsed(context.Context, int64, float64) error {
	return nil
}

func (*imageStudioAPIKeyQuotaUpdater) UpdateRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
