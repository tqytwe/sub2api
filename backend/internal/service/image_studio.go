package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	ImageStudioJobStatusPending   = "pending"
	ImageStudioJobStatusRunning   = "running"
	ImageStudioJobStatusCompleted = "completed"
	ImageStudioJobStatusFailed    = "failed"
	ImageStudioJobStatusCancelled = "cancelled"
	ImageStudioJobStatusPartial   = "partial"

	ImageStudioItemStatusPending    = "pending"
	ImageStudioItemStatusRunning    = "running"
	ImageStudioItemStatusPersisting = "persisting"
	ImageStudioItemStatusSuccess    = "success"
	ImageStudioItemStatusFailed     = "failed"
	ImageStudioItemStatusCancelled  = "cancelled"

	ImageStudioMaxItemAttempts = 3

	defaultImageStudioSize    = "1024x1024"
	maxImageStudioCount       = 4
	maxImageStudioPromptChars = 8000

	ImageStudioReferenceUploadMaxBytes  int64 = 20 << 20
	ImageStudioReferenceMaxPendingCount       = 20
	ImageStudioReferenceMaxPendingBytes int64 = 80 << 20

	imageStudioUntrackedObjectGrace = time.Hour
	imageStudioReferenceDefaultTTL  = 7 * 24 * time.Hour
	imageStudioAssetDefaultTTL      = 24 * time.Hour
	imageStudioAssetPurgeBatchSize  = 100
)

type imageStudioReferenceTTLContextKey struct{}

var (
	ErrImageStudioDisabled                     = infraerrors.BadRequest("IMAGE_STUDIO_DISABLED", "image studio is disabled")
	ErrImageStudioInvalidID                    = infraerrors.BadRequest("IMAGE_STUDIO_INVALID_ID", "invalid image studio identifier")
	ErrImageStudioJobNotFound                  = infraerrors.NotFound("IMAGE_STUDIO_JOB_NOT_FOUND", "image studio job not found")
	ErrImageStudioTemplate                     = infraerrors.BadRequest("IMAGE_STUDIO_TEMPLATE_INVALID", "invalid template")
	ErrImageStudioPromptRequired               = infraerrors.BadRequest("IMAGE_STUDIO_PROMPT_REQUIRED", "image description is required")
	ErrImageStudioPromptTooLong                = infraerrors.BadRequest("IMAGE_STUDIO_PROMPT_TOO_LONG", "image description exceeds 8000 characters")
	ErrImageStudioPromptRef                    = infraerrors.BadRequest("IMAGE_STUDIO_PROMPT_REFERENCE_INVALID", "prompt id and version must be provided together")
	ErrImageStudioAPIKey                       = infraerrors.BadRequest("IMAGE_STUDIO_API_KEY_REQUIRED", "valid API key is required")
	ErrImageStudioAssetNotFound                = infraerrors.NotFound("IMAGE_STUDIO_ASSET_NOT_FOUND", "image studio asset not found")
	ErrImageStudioAssetExpired                 = infraerrors.New(http.StatusGone, "IMAGE_STUDIO_ASSET_EXPIRED", "image studio asset has expired")
	ErrImageStudioAssetUnavailable             = infraerrors.New(http.StatusServiceUnavailable, "IMAGE_STUDIO_ASSET_UNAVAILABLE", "image studio asset is temporarily unavailable")
	ErrImageStudioConcurrentJobLimit           = infraerrors.New(http.StatusConflict, "IMAGE_STUDIO_CONCURRENT_JOB_LIMIT", "at most two image studio jobs may be active")
	ErrImageStudioBillingFailed                = infraerrors.New(http.StatusBadGateway, "IMAGE_STUDIO_BILLING_FAILED", "image studio billing failed")
	ErrImageStudioInsufficientBalance          = infraerrors.New(http.StatusPaymentRequired, "IMAGE_STUDIO_INSUFFICIENT_BALANCE", "insufficient balance for image studio hold")
	ErrImageStudioJobRunning                   = infraerrors.New(http.StatusConflict, "IMAGE_STUDIO_JOB_RUNNING", "running image studio job must be cancelled before deletion")
	ErrImageStudioLeaseLost                    = infraerrors.New(http.StatusConflict, "IMAGE_STUDIO_LEASE_LOST", "image studio job lease is no longer owned by this worker")
	ErrImageStudioCheckpointCancelled          = infraerrors.New(http.StatusConflict, "IMAGE_STUDIO_CHECKPOINT_CANCELLED", "image studio item was cancelled after provider cost was confirmed")
	ErrImageStudioReferenceNotFound            = infraerrors.NotFound("IMAGE_STUDIO_REFERENCE_NOT_FOUND", "image studio reference image not found")
	ErrImageStudioReferenceInvalid             = infraerrors.BadRequest("IMAGE_STUDIO_REFERENCE_INVALID", "invalid image studio reference image")
	ErrImageStudioReferenceTooLarge            = infraerrors.New(http.StatusRequestEntityTooLarge, "IMAGE_STUDIO_REFERENCE_TOO_LARGE", "image studio reference image exceeds 20 MiB")
	ErrImageStudioReferenceLimit               = infraerrors.BadRequest("IMAGE_STUDIO_REFERENCE_LIMIT", "image studio edit accepts at most four reference images")
	ErrImageStudioReferenceQuota               = infraerrors.New(http.StatusTooManyRequests, "IMAGE_STUDIO_REFERENCE_QUOTA", "image studio reference upload quota exceeded")
	ErrImageStudioReferenceRateLimit           = infraerrors.New(http.StatusTooManyRequests, "IMAGE_STUDIO_REFERENCE_RATE_LIMIT", "too many image studio reference uploads")
	ErrImageStudioArchiveUnavailable           = infraerrors.BadRequest("IMAGE_STUDIO_ARCHIVE_UNAVAILABLE", "image studio job archive is unavailable")
	ErrImageStudioSubscriptionGroupUnsupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_SUBSCRIPTION_GROUP_UNSUPPORTED",
		"Image Studio currently supports wallet-billed API key groups only",
	)
	ErrImageStudioProviderNotSupported = infraerrors.BadRequest(
		"IMAGE_STUDIO_PROVIDER_NOT_SUPPORTED",
		"image provider is not supported by Image Studio",
	)
	ErrImageStudioCapabilityProfileChanged = infraerrors.New(
		http.StatusConflict,
		"IMAGE_STUDIO_CAPABILITY_PROFILE_CHANGED",
		"image capability profile changed after the job was created",
	)
)

// ValidateImageStudioAPIKey rejects keys that cannot authorize a generation request.
func ValidateImageStudioAPIKey(apiKey *APIKey) error {
	if apiKey == nil || !apiKey.IsActive() || apiKey.IsExpired() || apiKey.IsQuotaExhausted() {
		return ErrImageStudioAPIKey
	}
	return nil
}

func WithImageStudioReferenceTTL(ctx context.Context, ttl time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		return ctx
	}
	return context.WithValue(ctx, imageStudioReferenceTTLContextKey{}, ttl)
}

func ImageStudioReferenceTTL(ctx context.Context) time.Duration {
	if ctx != nil {
		if ttl, ok := ctx.Value(imageStudioReferenceTTLContextKey{}).(time.Duration); ok && ttl > 0 {
			return ttl
		}
	}
	return imageStudioReferenceDefaultTTL
}

type ImageStudioAsset struct {
	ID           string     `json:"id"`
	URL          string     `json:"url,omitempty"`
	SortOrder    int        `json:"sort_order"`
	ContentType  string     `json:"content_type,omitempty"`
	ByteSize     int64      `json:"byte_size,omitempty"`
	Filename     string     `json:"filename,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	PurgedAt     *time.Time `json:"purged_at,omitempty"`
	Availability string     `json:"availability,omitempty"`
	PreviewURL   string     `json:"preview_url,omitempty"`
	DownloadURL  string     `json:"download_url,omitempty"`
	StorageKey   string     `json:"-"`
	Width        int        `json:"width,omitempty"`
	Height       int        `json:"height,omitempty"`
	AspectRatio  string     `json:"aspect_ratio,omitempty"`

	ThumbnailURL         string `json:"thumbnail_url,omitempty"`
	ThumbnailStorageKey  string `json:"-"`
	ThumbnailContentType string `json:"-"`
	ThumbnailByteSize    int64  `json:"-"`
}

type ImageStudioAssetRecord struct {
	ID          string
	StorageKey  string
	ContentType string
	ByteSize    int64
	SortOrder   int
	URL         string
	Width       int
	Height      int
	Filename    string
	ExpiresAt   *time.Time

	ThumbnailStorageKey  string
	ThumbnailContentType string
	ThumbnailByteSize    int64
}

type ImageStudioImagePayload struct {
	Data        []byte
	ContentType string
}

type ImageStudioJob struct {
	ID                      string                    `json:"id"`
	UserID                  int64                     `json:"user_id"`
	TemplateID              string                    `json:"template_id"`
	PromptID                *int64                    `json:"prompt_id,omitempty"`
	PromptVersion           *int                      `json:"prompt_version,omitempty"`
	PromptHash              string                    `json:"-"`
	RequestPayloadEncrypted string                    `json:"-"`
	Model                   string                    `json:"model,omitempty"`
	Quality                 string                    `json:"quality,omitempty"`
	Size                    string                    `json:"size"`
	Count                   int                       `json:"count"`
	Status                  string                    `json:"status"`
	EstimatedCost           float64                   `json:"estimated_cost"`
	ActualCost              *float64                  `json:"actual_cost,omitempty"`
	APIKeyID                *int64                    `json:"api_key_id,omitempty"`
	GroupID                 *int64                    `json:"group_id,omitempty"`
	Platform                string                    `json:"platform,omitempty"`
	CapabilityProfileID     string                    `json:"capability_profile_id,omitempty"`
	CapabilityRevision      string                    `json:"capability_revision,omitempty"`
	HoldAmount              *float64                  `json:"hold_amount,omitempty"`
	HoldID                  string                    `json:"-"`
	SuccessCount            int                       `json:"success_count"`
	FailCount               int                       `json:"fail_count"`
	ErrorMessage            string                    `json:"error_message,omitempty"`
	CreatedAt               time.Time                 `json:"created_at"`
	ExpiresAt               *time.Time                `json:"expires_at,omitempty"`
	CancelRequestedAt       *time.Time                `json:"cancel_requested_at,omitempty"`
	StartedAt               *time.Time                `json:"started_at,omitempty"`
	FinishedAt              *time.Time                `json:"finished_at,omitempty"`
	HeartbeatAt             *time.Time                `json:"heartbeat_at,omitempty"`
	LeaseOwner              string                    `json:"-"`
	LeaseExpiresAt          *time.Time                `json:"-"`
	IdempotencyKeyHash      string                    `json:"-"`
	IdempotencyFingerprint  string                    `json:"-"`
	Assets                  []ImageStudioAsset        `json:"assets,omitempty"`
	Items                   []ImageStudioItem         `json:"items,omitempty"`
	JobReferences           []ImageStudioJobReference `json:"-"`
}

type ImageStudioItem struct {
	ID                    string     `json:"id"`
	JobID                 string     `json:"job_id,omitempty"`
	SortOrder             int        `json:"sort_order"`
	Status                string     `json:"status"`
	ActualCost            *float64   `json:"actual_cost,omitempty"`
	Error                 string     `json:"error,omitempty"`
	AssetID               *string    `json:"asset_id,omitempty"`
	AttemptCount          int        `json:"attempt_count"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	FinishedAt            *time.Time `json:"finished_at,omitempty"`
	CheckpointData        []byte     `json:"-"`
	CheckpointContentType string     `json:"-"`
	CheckpointActualCost  *float64   `json:"-"`
}

type ImageStudioReference struct {
	ID               string     `json:"id"`
	UserID           int64      `json:"-"`
	StorageKey       string     `json:"-"`
	OriginalFilename string     `json:"filename,omitempty"`
	ContentType      string     `json:"content_type,omitempty"`
	ByteSize         int64      `json:"byte_size,omitempty"`
	CreatedAt        time.Time  `json:"created_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

type ImageStudioJobReference struct {
	ID          string
	JobID       string
	StorageKey  string
	ContentType string
	ByteSize    int64
	SortOrder   int
	CreatedAt   time.Time
}

type ImageStudioWorkerRequest struct {
	Platform    string
	Operation   string
	Endpoint    string
	ContentType string
	Body        []byte
}

type imageStudioWorkerEnvelope struct {
	Platform            string          `json:"platform,omitempty"`
	Operation           string          `json:"operation,omitempty"`
	CapabilityProfileID string          `json:"capability_profile_id,omitempty"`
	CapabilityRevision  string          `json:"capability_revision,omitempty"`
	Endpoint            string          `json:"endpoint,omitempty"`
	Body                json.RawMessage `json:"body"`
}

type ImageStudioGenerateRequest struct {
	TemplateID             string   `json:"template_id"`
	PromptID               *int64   `json:"prompt_id,omitempty"`
	PromptVersion          *int     `json:"prompt_version,omitempty"`
	UserPrompt             string   `json:"user_prompt"`
	AccentColor            string   `json:"accent_color"`
	Size                   string   `json:"size"`
	Aspect                 string   `json:"aspect"`
	Tier                   string   `json:"tier"`
	Quality                string   `json:"quality"`
	Count                  int      `json:"count"`
	Model                  string   `json:"model"`
	ExpertPrompt           *string  `json:"expert_prompt"`
	APIKeyID               int64    `json:"api_key_id"`
	RetainDays             *int     `json:"retain_days,omitempty"`
	Mode                   string   `json:"mode,omitempty"`
	ReferenceIDs           []string `json:"reference_ids,omitempty"`
	Background             string   `json:"background,omitempty"`
	OutputFormat           string   `json:"output_format,omitempty"`
	InputFidelity          string   `json:"input_fidelity,omitempty"`
	Style                  string   `json:"style,omitempty"`
	OutputCompression      *int     `json:"output_compression,omitempty"`
	IdempotencyKeyHash     string   `json:"-"`
	IdempotencyFingerprint string   `json:"-"`
}

type ImageStudioGenerateResult struct {
	Job           ImageStudioJob  `json:"job"`
	QuestProgress *PlayQuestToday `json:"quest_progress,omitempty"`
}

type ImageStudioEstimate struct {
	EstimatedCost float64 `json:"estimated_cost"`
	Balance       float64 `json:"balance"`
	Sufficient    bool    `json:"sufficient"`
	Model         string  `json:"model"`
	Count         int     `json:"count"`
	Size          string  `json:"size"`
}

type ImageStudioRepository interface {
	InsertJob(ctx context.Context, job *ImageStudioJob) error
	CreatePendingJob(ctx context.Context, job *ImageStudioJob, items []ImageStudioItem, reserve func(context.Context) error) error
	UpdateJobResult(ctx context.Context, jobID string, status string, actualCost *float64, errMsg string) error
	InsertAssets(ctx context.Context, jobID string, assets []ImageStudioAssetRecord) error
	GetJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error)
	GetActiveJob(ctx context.Context, userID int64) (*ImageStudioJob, error)
	ListActiveJobs(ctx context.Context, userID int64) ([]ImageStudioJob, error)
	GetAsset(ctx context.Context, userID int64, assetID string) (*ImageStudioAsset, error)
	ListJobs(ctx context.Context, userID int64, limit int) ([]ImageStudioJob, error)
	ListJobsPage(ctx context.Context, userID int64, page, pageSize int) ([]ImageStudioJob, int, error)
	ListAssetStorageKeysForJob(ctx context.Context, jobID string) ([]string, error)
	ListExpiredJobIDs(ctx context.Context, before time.Time) ([]string, error)
	DeleteJobWithStorageKeys(ctx context.Context, userID int64, jobID string) ([]string, error)
	CountCompletedToday(ctx context.Context, userID int64, dayStart time.Time) (int, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string) error
	DeleteExpiredJobsBefore(ctx context.Context, before time.Time) (int64, error)
	HasCompletedJob(ctx context.Context, userID int64) (bool, error)
	CreateReference(ctx context.Context, reference *ImageStudioReference) error
	ListReferencesByID(ctx context.Context, userID int64, ids []string) ([]ImageStudioReference, error)
	ClaimNextJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*ImageStudioJob, error)
	HeartbeatJob(ctx context.Context, jobID, leaseOwner string, now time.Time, leaseDuration time.Duration) error
	ClaimNextItem(ctx context.Context, jobID, leaseOwner string, now time.Time) (*ImageStudioItem, error)
	RetryItem(ctx context.Context, jobID, itemID, leaseOwner string, now time.Time) error
	CheckpointItem(ctx context.Context, jobID, itemID, leaseOwner string, image ImageStudioImagePayload, actualCost float64, now time.Time) error
	GetItem(ctx context.Context, jobID, itemID string) (*ImageStudioItem, error)
	CompleteItem(ctx context.Context, jobID, itemID, leaseOwner string, status string, actualCost *float64, errMsg string, asset *ImageStudioAssetRecord, now time.Time) error
	RequestCancel(
		ctx context.Context,
		userID int64,
		jobID string,
		now time.Time,
		release func(context.Context, *ImageStudioJob) error,
	) (*ImageStudioJob, error)
	SettleJob(ctx context.Context, jobID, leaseOwner string, now time.Time, settle func(context.Context, *ImageStudioJob, float64) error) (*ImageStudioJob, error)
}

type ImageStudioAssetPurgeCandidate struct {
	ID          string
	StorageKeys []string
}

type ImageStudioAssetPurgeRepository interface {
	ListExpiredAssetsForPurge(ctx context.Context, before time.Time, limit int) ([]ImageStudioAssetPurgeCandidate, error)
	MarkAssetsPurged(ctx context.Context, assetIDs []string, purgedAt time.Time) (int64, error)
}

type ImageStudioIdempotentJobRepository interface {
	FindJobByIdempotency(
		ctx context.Context,
		userID int64,
		keyHash string,
		fingerprint string,
	) (jobID string, found bool, err error)
	CreatePendingJobIdempotent(
		ctx context.Context,
		job *ImageStudioJob,
		items []ImageStudioItem,
		reserve func(context.Context) error,
	) (existingJobID string, created bool, err error)
}

type ImageStudioObjectDeletionRepository interface {
	ListPendingObjectDeletions(ctx context.Context, limit int) ([]string, error)
	AcknowledgeObjectDeletion(ctx context.Context, storageKey string) error
	RecordObjectDeletionFailure(ctx context.Context, storageKey string, deleteErr error) error
}

type ImageStudioObjectReconciliationRepository interface {
	FilterTrackedObjectStorageKeys(ctx context.Context, storageKeys []string) (map[string]struct{}, error)
}

type ImageStudioHubStatus struct {
	Enabled         bool `json:"enabled"`
	ImagesToday     int  `json:"images_today"`
	HasCompletedJob bool `json:"has_completed_job"`
}

type ImageStudioService struct {
	repo            ImageStudioRepository
	assetStore      *ImageStudioAssetStore
	apiKeyService   *APIKeyService
	userRepo        UserRepository
	settingService  *SettingService
	playService     *PlayService
	pricing         *BatchImageModelPricingResolver
	gateway         ImageStudioModelResolver
	promptRepo      PromptLibraryRepository
	capabilityCache *ImageStudioCapabilityCache
	encryptor       SecretEncryptor
	billingRepo     UsageBillingRepository
	billingCache    *BillingCacheService
}

func NewImageStudioService(
	repo ImageStudioRepository,
	assetStore *ImageStudioAssetStore,
	apiKeyService *APIKeyService,
	userRepo UserRepository,
	settingService *SettingService,
	playService *PlayService,
	pricing *BatchImageModelPricingResolver,
	gateway ImageStudioModelResolver,
	encryptor SecretEncryptor,
	billingRepo UsageBillingRepository,
	billingCache *BillingCacheService,
) *ImageStudioService {
	return &ImageStudioService{
		repo:            repo,
		assetStore:      assetStore,
		apiKeyService:   apiKeyService,
		userRepo:        userRepo,
		settingService:  settingService,
		playService:     playService,
		pricing:         pricing,
		gateway:         gateway,
		capabilityCache: NewImageStudioCapabilityCache(),
		encryptor:       encryptor,
		billingRepo:     billingRepo,
		billingCache:    billingCache,
	}
}

func (s *ImageStudioService) StorageHealth(ctx context.Context) error {
	if s == nil || s.assetStore == nil {
		return errors.New("image studio asset storage is unavailable")
	}
	return s.assetStore.StorageHealth(ctx)
}

func (s *ImageStudioService) IsEnabled(ctx context.Context) bool {
	if s.playService == nil {
		return false
	}
	return s.playService.GetRuntime(ctx).ImageStudioEnabled
}

func (s *ImageStudioService) ListTemplates() ImageStudioCatalog {
	return defaultImageStudioCatalog()
}

func (s *ImageStudioService) ListCapabilities() ImageStudioCapabilities {
	return ListImageStudioCapabilities()
}

func (s *ImageStudioService) CreateReference(
	ctx context.Context,
	userID int64,
	filename string,
	contentType string,
	data []byte,
) (*ImageStudioReference, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrImageStudioDisabled
	}
	if userID <= 0 || s.repo == nil || s.assetStore == nil || len(data) == 0 {
		return nil, ErrImageStudioReferenceInvalid
	}
	if int64(len(data)) > ImageStudioReferenceUploadMaxBytes {
		return nil, ErrImageStudioReferenceTooLarge
	}
	contentType, err := validateImageStudioReference(contentType, data)
	if err != nil {
		return nil, err
	}
	referenceID := uuid.NewString()
	storageKey, err := s.assetStore.Save(userID, referenceID, contentType, data)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(ImageStudioReferenceTTL(ctx))
	reference := &ImageStudioReference{
		ID:               referenceID,
		UserID:           userID,
		StorageKey:       storageKey,
		OriginalFilename: filepath.Base(strings.TrimSpace(filename)),
		ContentType:      contentType,
		ByteSize:         int64(len(data)),
		ExpiresAt:        &expiresAt,
	}
	if err := s.repo.CreateReference(ctx, reference); err != nil {
		refs, verifyErr := s.repo.ListReferencesByID(
			context.WithoutCancel(ctx),
			userID,
			[]string{reference.ID},
		)
		if verifyErr == nil {
			if len(refs) == 1 && refs[0].ID == reference.ID && refs[0].StorageKey == storageKey {
				reference.CreatedAt = refs[0].CreatedAt
				reference.ExpiresAt = refs[0].ExpiresAt
				return reference, nil
			}
			_ = s.assetStore.Delete(storageKey)
		}
		return nil, err
	}
	return reference, nil
}

func (s *ImageStudioService) resolveGenerateSize(apiKey *APIKey, model string, req ImageStudioGenerateRequest, tpl ImageStudioTemplate) (string, error) {
	size := strings.TrimSpace(req.Size)
	if size == "" {
		aspect, tier := strings.TrimSpace(req.Aspect), strings.TrimSpace(req.Tier)
		if aspect == "" && tier == "" {
			size = tpl.Defaults.Size
			if size == "" {
				size = defaultImageStudioSize
			}
		} else {
			resolved, err := ResolveImageStudioSize(aspect, tier, "")
			if err != nil {
				return "", err
			}
			size = resolved
		}
	} else {
		resolved, err := ResolveImageStudioSize("", "", size)
		if err != nil {
			return "", err
		}
		size = resolved
	}
	if err := s.ValidateSizeForModel(apiKey, model, size); err != nil {
		return "", err
	}
	return size, nil
}

func (s *ImageStudioService) Estimate(
	ctx context.Context,
	userID int64,
	templateID string,
	size string,
	count int,
	apiKeyID int64,
	model string,
	referenceIDs []string,
) (*ImageStudioEstimate, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrImageStudioDisabled
	}
	tpl, ok := findImageStudioTemplate(templateID)
	if !ok {
		return nil, ErrImageStudioTemplate
	}
	if count <= 0 {
		count = tpl.Defaults.Count
	}
	if count > maxImageStudioCount {
		count = maxImageStudioCount
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.TotalRecharged <= 0 && count > 1 {
		count = 1
	}
	apiKey, err := s.resolveAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	resolvedModel, err := s.resolveImageModel(ctx, apiKey, model)
	if err != nil {
		return nil, err
	}
	if size == "" {
		size = tpl.Defaults.Size
		if size == "" {
			size = defaultImageStudioSize
		}
	}
	resolvedSize, err := ResolveImageStudioSize("", "", size)
	if err != nil {
		return nil, err
	}
	if err := s.ValidateSizeForModel(apiKey, resolvedModel, resolvedSize); err != nil {
		return nil, err
	}
	size = resolvedSize
	cost, err := s.estimateCost(ctx, apiKey, resolvedModel, size, count)
	if err != nil {
		return nil, err
	}
	providedReferenceCount := 0
	for _, id := range referenceIDs {
		if strings.TrimSpace(id) != "" {
			providedReferenceCount++
		}
	}
	if providedReferenceCount > maxImageStudioReferences {
		return nil, ErrImageStudioReferenceLimit
	}
	referenceIDs = normalizeImageStudioReferenceIDs(referenceIDs)
	if len(referenceIDs) > 0 {
		capability, ok := resolveImageStudioCapabilitiesForAPIKey(apiKey, resolvedModel)
		if !ok {
			return nil, ErrImageStudioProviderNotSupported
		}
		if err := ValidateImageStudioProviderOptions(
			capability,
			"edit",
			ImageStudioGenerateRequest{ReferenceIDs: referenceIDs},
		); err != nil {
			return nil, err
		}
		references, err := s.repo.ListReferencesByID(ctx, userID, referenceIDs)
		if err != nil {
			return nil, err
		}
		if len(references) != len(referenceIDs) {
			return nil, ErrImageStudioReferenceNotFound
		}
		inputCost, err := s.estimateReferenceInputCost(
			ctx,
			apiKey,
			resolvedModel,
			references,
			count,
		)
		if err != nil {
			return nil, err
		}
		cost += inputCost
	}
	return &ImageStudioEstimate{
		EstimatedCost: cost,
		Balance:       user.Balance,
		Sufficient:    user.Balance >= cost,
		Model:         resolvedModel,
		Count:         count,
		Size:          size,
	}, nil
}

func (s *ImageStudioService) estimateCost(ctx context.Context, apiKey *APIKey, model, size string, count int) (float64, error) {
	if apiKey == nil {
		return 0, ErrImageStudioAPIKey
	}
	unit := 0.04
	if s.pricing != nil && apiKey.GroupID != nil {
		platform := PlatformOpenAI
		if apiKey.Group != nil && strings.TrimSpace(apiKey.Group.Platform) != "" {
			platform = strings.ToLower(strings.TrimSpace(apiKey.Group.Platform))
		}
		if p, err := s.pricing.BatchImageUnitPrice(ctx, &BatchImageJob{
			Provider: platform,
			Model:    model,
		}); err == nil && p > 0 {
			unit = p
		}
	}
	if apiKey.Group != nil {
		if configured := apiKey.Group.GetImagePrice(normalizeStudioImageSize(size)); configured != nil && *configured > 0 {
			unit = *configured
		}
		effectiveGroupMultiplier := apiKey.Group.RateMultiplier
		multiplierResolved := false
		if apiKey.GroupID != nil && apiKey.UserID > 0 {
			if resolver, ok := s.gateway.(ImageStudioRateMultiplierResolver); ok {
				effectiveGroupMultiplier = resolver.ResolveUserGroupRateMultiplier(
					ctx,
					apiKey.UserID,
					*apiKey.GroupID,
					apiKey.Group.RateMultiplier,
				)
				multiplierResolved = true
			}
		}
		mult := effectiveGroupMultiplier
		if apiKey.Group.ImageRateIndependent {
			mult = apiKey.Group.ImageRateMultiplier
			multiplierResolved = true
		}
		if mult > 0 || multiplierResolved {
			if mult < 0 {
				mult = 0
			}
			unit *= mult
		}
	}
	return unit * float64(count), nil
}

func (s *ImageStudioService) estimateReferenceInputCost(
	ctx context.Context,
	apiKey *APIKey,
	model string,
	references []ImageStudioReference,
	outputCount int,
) (float64, error) {
	if len(references) == 0 {
		return 0, nil
	}
	estimator, ok := s.gateway.(ImageStudioInputCostEstimator)
	if !ok || s.assetStore == nil {
		return 0, ErrImageStudioBillingFailed.WithCause(errors.New("image input pricing is unavailable"))
	}
	if outputCount <= 0 {
		outputCount = 1
	}
	tokensPerOutput := 0
	for _, reference := range references {
		data, err := s.assetStore.Read(reference.StorageKey)
		if err != nil {
			return 0, ErrImageStudioReferenceNotFound.WithCause(err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
			return 0, ErrImageStudioReferenceInvalid
		}
		tokensPerOutput += imageStudioReferenceInputTokenUpperBound(cfg.Width, cfg.Height)
	}
	imageInputTokens := tokensPerOutput * outputCount
	cost, err := estimator.EstimateImageStudioInputCost(ctx, model, apiKey, imageInputTokens)
	if err != nil || cost < 0 {
		if err == nil {
			err = errors.New("negative image input cost")
		}
		return 0, ErrImageStudioBillingFailed.WithCause(err)
	}
	return cost, nil
}

func imageStudioReferenceInputTokenUpperBound(width, height int) int {
	const (
		patchSize       = 16
		baseTokens      = 1024
		fidelityReserve = 2
	)
	patchesWide := (width + patchSize - 1) / patchSize
	patchesHigh := (height + patchSize - 1) / patchSize
	return (patchesWide*patchesHigh + baseTokens) * fidelityReserve
}

func imageStudioPlatformForAPIKey(apiKey *APIKey) (string, error) {
	if apiKey == nil || apiKey.Group == nil {
		return "", ErrImageStudioProviderNotSupported
	}
	platform := strings.ToLower(strings.TrimSpace(apiKey.Group.Platform))
	switch platform {
	case PlatformOpenAI, PlatformGemini, PlatformGrok:
		return platform, nil
	default:
		return "", ErrImageStudioProviderNotSupported
	}
}

func inferImageStudioProviderFromModel(model string) (string, error) {
	capability, ok := ResolveImageStudioModelCapability(model)
	if !ok || strings.TrimSpace(capability.Platform) == "" {
		return "", ErrImageStudioProviderNotSupported
	}
	return capability.Platform, nil
}

func ImageStudioProviderSupportsAutomaticRetry(model string) bool {
	platform, err := inferImageStudioProviderFromModel(model)
	return err == nil && platform != PlatformGrok
}

func buildImageStudioProviderPayload(
	platform, operation, model, prompt, size string,
	count int,
	req ImageStudioGenerateRequest,
	referenceIDs []string,
) (string, []byte, error) {
	endpoint := openAIImagesGenerationsEndpoint
	if operation == "edit" {
		endpoint = openAIImagesEditsEndpoint
	}
	payload := map[string]any{
		"model":  model,
		"prompt": prompt,
		"n":      count,
	}
	switch platform {
	case PlatformOpenAI:
		if endpoint, body, handled, err := buildAdaptedImageStudioProviderPayload(
			operation,
			model,
			prompt,
			size,
			count,
			req,
			referenceIDs,
		); handled || err != nil {
			return endpoint, body, err
		}
		payload["size"] = size
		if IsGPTImageGenerationModel(model) {
			// Inline output lets the durable worker checkpoint before object storage.
			payload["response_format"] = "b64_json"
			if quality := strings.ToLower(strings.TrimSpace(req.Quality)); quality != "" {
				payload["quality"] = quality
			}
			for _, field := range []struct {
				key   string
				value string
			}{
				{key: "background", value: req.Background},
				{key: "output_format", value: normalizeImageStudioOutputFormat(req.OutputFormat)},
				{key: "input_fidelity", value: req.InputFidelity},
				{key: "style", value: req.Style},
			} {
				if value := strings.ToLower(strings.TrimSpace(field.value)); value != "" {
					payload[field.key] = value
				}
			}
			if req.OutputCompression != nil {
				payload["output_compression"] = *req.OutputCompression
			}
		}
	case PlatformGemini:
		endpoint = "/v1beta/models/" + model + ":generateContent"
		geminiBody := map[string]any{
			"contents": []geminiContent{{
				Parts: []geminiPart{{Text: prompt}},
			}},
			"generationConfig": geminiGenerationConfig{
				ResponseModalities: []string{"TEXT", "IMAGE"},
			},
		}
		if operation == "edit" {
			geminiBody["image_studio_job_reference_ids"] = referenceIDs
		}
		body, err := json.Marshal(geminiBody)
		if err != nil {
			return "", nil, err
		}
		return endpoint, body, nil
	case PlatformGrok:
		aspect, tier := InferImageStudioAspectTier(size)
		payload["aspect_ratio"] = aspect
		payload["resolution"] = strings.ToLower(tier)
	default:
		return "", nil, ErrImageStudioProviderNotSupported
	}
	if operation == "edit" {
		payload["image_studio_job_reference_ids"] = referenceIDs
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, err
	}
	return endpoint, body, nil
}

func (s *ImageStudioService) CreatePendingJob(ctx context.Context, userID int64, req ImageStudioGenerateRequest) (*ImageStudioJob, string, error) {
	var idempotentRepo ImageStudioIdempotentJobRepository
	if req.IdempotencyKeyHash != "" {
		var ok bool
		idempotentRepo, ok = s.repo.(ImageStudioIdempotentJobRepository)
		if !ok {
			return nil, "", ErrImageStudioBillingFailed.WithCause(errors.New("image studio idempotent repository is not configured"))
		}
		existingJobID, found, err := idempotentRepo.FindJobByIdempotency(
			ctx,
			userID,
			req.IdempotencyKeyHash,
			req.IdempotencyFingerprint,
		)
		if err != nil {
			return nil, "", err
		}
		if found {
			existingJob, err := s.repo.GetJob(ctx, userID, existingJobID)
			if err != nil {
				return nil, "", err
			}
			return existingJob, "", nil
		}
	}
	if !s.IsEnabled(ctx) {
		return nil, "", ErrImageStudioDisabled
	}
	promptID, promptVersion, err := normalizeImageStudioPromptReference(req.PromptID, req.PromptVersion)
	if err != nil {
		return nil, "", err
	}
	if err := s.validatePromptLibraryReference(ctx, userID, promptID, promptVersion); err != nil {
		return nil, "", err
	}
	if err := validateImageStudioPrompt(req.UserPrompt); err != nil {
		return nil, "", err
	}
	if req.ExpertPrompt != nil && strings.TrimSpace(*req.ExpertPrompt) != "" {
		if err := validateImageStudioPrompt(*req.ExpertPrompt); err != nil {
			return nil, "", err
		}
	}
	tpl, ok := findImageStudioTemplate(req.TemplateID)
	if !ok {
		return nil, "", ErrImageStudioTemplate
	}
	count := req.Count
	if count <= 0 {
		count = tpl.Defaults.Count
	}
	if count > maxImageStudioCount {
		count = maxImageStudioCount
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if user.TotalRecharged <= 0 && count > 1 {
		count = 1
	}
	if count <= 0 {
		count = 1
	}
	apiKey, err := s.resolveAPIKey(ctx, userID, req.APIKeyID)
	if err != nil {
		return nil, "", err
	}
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		return nil, "", ErrImageStudioSubscriptionGroupUnsupported
	}
	resolvedModel, err := s.resolveImageModel(ctx, apiKey, req.Model)
	if err != nil {
		return nil, "", err
	}
	platform, err := imageStudioPlatformForAPIKey(apiKey)
	if err != nil {
		return nil, "", err
	}
	capability, ok := ResolveImageStudioProviderCapability(platform, resolvedModel)
	if !ok {
		return nil, "", ErrImageStudioProviderNotSupported
	}
	size, err := s.resolveGenerateSize(apiKey, resolvedModel, req, tpl)
	if err != nil {
		return nil, "", err
	}
	if err := s.ValidateQualityForModel(apiKey, resolvedModel, req.Quality); err != nil {
		return nil, "", err
	}
	providedReferenceCount := 0
	for _, id := range req.ReferenceIDs {
		if strings.TrimSpace(id) != "" {
			providedReferenceCount++
		}
	}
	if providedReferenceCount > maxImageStudioReferences {
		return nil, "", ErrImageStudioReferenceLimit
	}
	referenceIDs := normalizeImageStudioReferenceIDs(req.ReferenceIDs)
	if len(referenceIDs) > maxImageStudioReferences {
		return nil, "", ErrImageStudioReferenceLimit
	}
	mode := strings.TrimSpace(strings.ToLower(req.Mode))
	editMode := mode == "edit" || len(referenceIDs) > 0
	operation := "create"
	if editMode {
		operation = "edit"
	}
	if err := ValidateImageStudioProviderOptions(capability, operation, req); err != nil {
		return nil, "", err
	}
	var uploadReferences []ImageStudioReference
	if editMode {
		if len(referenceIDs) == 0 {
			return nil, "", ErrImageStudioReferenceNotFound
		}
		refs, err := s.repo.ListReferencesByID(ctx, userID, referenceIDs)
		if err != nil {
			return nil, "", err
		}
		if len(refs) != len(referenceIDs) {
			return nil, "", ErrImageStudioReferenceNotFound
		}
		uploadReferences = refs
	}
	prompt := buildImageStudioPrompt(tpl, req)
	est, err := s.estimateCost(ctx, apiKey, resolvedModel, size, count)
	if err != nil {
		return nil, "", err
	}
	if editMode {
		inputCost, err := s.estimateReferenceInputCost(ctx, apiKey, resolvedModel, uploadReferences, count)
		if err != nil {
			return nil, "", err
		}
		est += inputCost
	}
	job := &ImageStudioJob{
		ID:                     uuid.NewString(),
		UserID:                 userID,
		TemplateID:             req.TemplateID,
		PromptID:               promptID,
		PromptVersion:          promptVersion,
		PromptHash:             hashPrompt(prompt),
		Size:                   size,
		Count:                  count,
		Status:                 ImageStudioJobStatusPending,
		EstimatedCost:          est,
		APIKeyID:               &apiKey.ID,
		GroupID:                apiKey.GroupID,
		Platform:               platform,
		CapabilityProfileID:    capability.ProfileID,
		CapabilityRevision:     capability.Revision,
		IdempotencyKeyHash:     req.IdempotencyKeyHash,
		IdempotencyFingerprint: req.IdempotencyFingerprint,
	}
	referencesPersisted := false
	if editMode {
		jobReferences, err := s.copyImageStudioJobReferences(job, uploadReferences)
		if err != nil {
			return nil, "", err
		}
		job.JobReferences = jobReferences
		defer func() {
			if !referencesPersisted {
				_ = s.deleteImageStudioJobReferenceObjects(jobReferences)
			}
		}()
		referenceIDs = make([]string, len(jobReferences))
		for i := range jobReferences {
			referenceIDs[i] = jobReferences[i].ID
		}
	}
	endpoint, providerBody, err := buildImageStudioProviderPayload(
		platform,
		operation,
		resolvedModel,
		prompt,
		size,
		count,
		req,
		referenceIDs,
	)
	if err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(imageStudioWorkerEnvelope{
		Platform:            platform,
		Operation:           operation,
		CapabilityProfileID: capability.ProfileID,
		CapabilityRevision:  capability.Revision,
		Endpoint:            endpoint,
		Body:                providerBody,
	})
	if err != nil {
		return nil, "", err
	}
	if s.encryptor == nil {
		return nil, "", ErrImageStudioBillingFailed.WithCause(errors.New("image studio request encryptor is not configured"))
	}
	encryptedPayload, err := s.encryptor.Encrypt(string(body))
	if err != nil {
		return nil, "", err
	}
	holdAmount := est
	job.RequestPayloadEncrypted = encryptedPayload
	job.Model = resolvedModel
	job.Quality = strings.TrimSpace(strings.ToLower(req.Quality))
	job.HoldAmount = &holdAmount
	job.HoldID = ImageStudioHoldRequestID(job.ID)
	items := make([]ImageStudioItem, count)
	for i := range items {
		items[i] = ImageStudioItem{
			ID:        uuid.NewString(),
			JobID:     job.ID,
			SortOrder: i,
			Status:    ImageStudioItemStatusPending,
		}
	}
	reserve := func(reserveCtx context.Context) error {
		return reserveImageStudioBalance(reserveCtx, s.billingRepo, job)
	}
	if job.IdempotencyKeyHash != "" {
		existingJobID, created, err := idempotentRepo.CreatePendingJobIdempotent(ctx, job, items, reserve)
		if err != nil {
			committed, definitive := s.confirmImageStudioJobCommit(
				context.WithoutCancel(ctx),
				userID,
				job,
				idempotentRepo,
			)
			if committed != nil {
				referencesPersisted = true
				return committed, "", nil
			}
			if !definitive {
				referencesPersisted = true
			}
			return nil, "", err
		}
		if !created {
			existingJob, err := s.repo.GetJob(ctx, userID, existingJobID)
			if err != nil {
				return nil, "", err
			}
			return existingJob, "", nil
		}
	} else if err := s.repo.CreatePendingJob(ctx, job, items, reserve); err != nil {
		if errors.Is(err, ErrImageStudioInsufficientBalance) {
			return nil, "", err
		}
		committed, definitive := s.confirmImageStudioJobCommit(
			context.WithoutCancel(ctx),
			userID,
			job,
			nil,
		)
		if committed != nil {
			referencesPersisted = true
			return committed, "", nil
		}
		if !definitive {
			referencesPersisted = true
		}
		return nil, "", err
	}
	referencesPersisted = true
	s.invalidateImageStudioBalance(ctx, userID)
	job.Items = items
	return job, string(body), nil
}

func (s *ImageStudioService) confirmImageStudioJobCommit(
	ctx context.Context,
	userID int64,
	job *ImageStudioJob,
	idempotentRepo ImageStudioIdempotentJobRepository,
) (*ImageStudioJob, bool) {
	if job == nil {
		return nil, true
	}
	jobID := job.ID
	if idempotentRepo != nil && job.IdempotencyKeyHash != "" {
		existingJobID, found, err := idempotentRepo.FindJobByIdempotency(
			ctx,
			userID,
			job.IdempotencyKeyHash,
			job.IdempotencyFingerprint,
		)
		if err != nil {
			return nil, false
		}
		if !found {
			return nil, true
		}
		jobID = existingJobID
	}
	committed, err := s.repo.GetJob(ctx, userID, jobID)
	if err == nil {
		return committed, true
	}
	if errors.Is(err, ErrImageStudioJobNotFound) {
		return nil, true
	}
	return nil, false
}

func (s *ImageStudioService) CompleteJob(ctx context.Context, userID int64, jobID string, images []ImageStudioImagePayload, actualCost float64, errMsg string) (*ImageStudioGenerateResult, error) {
	status := ImageStudioJobStatusCompleted
	if errMsg != "" || len(images) == 0 {
		status = ImageStudioJobStatusFailed
	}
	job, err := s.repo.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}

	// Persist assets BEFORE marking completed. Previously UpdateJobResult ran first;
	// Save/Insert failures left jobs stuck as "completed" with zero assets (blank gallery).
	if status == ImageStudioJobStatusCompleted {
		if s.assetStore == nil {
			status = ImageStudioJobStatusFailed
			errMsg = "image asset store unavailable"
		} else {
			records := make([]ImageStudioAssetRecord, 0, len(images))
			for i, img := range images {
				assetID := uuid.NewString()
				assetExpiresAt := time.Now().UTC().Add(imageStudioAssetDefaultTTL)
				storageKey, saveErr := s.assetStore.Save(userID, assetID, img.ContentType, img.Data)
				if saveErr != nil {
					status = ImageStudioJobStatusFailed
					errMsg = "failed to persist generated image: " + saveErr.Error()
					records = nil
					break
				}
				records = append(records, ImageStudioAssetRecord{
					ID:          assetID,
					StorageKey:  storageKey,
					ContentType: img.ContentType,
					ByteSize:    int64(len(img.Data)),
					SortOrder:   i,
					Filename:    imageStudioAssetFilename(assetID, img.ContentType),
					ExpiresAt:   &assetExpiresAt,
				})
			}
			if status == ImageStudioJobStatusCompleted {
				if err := s.repo.InsertAssets(ctx, jobID, records); err != nil {
					status = ImageStudioJobStatusFailed
					errMsg = "failed to save image assets: " + err.Error()
					for _, rec := range records {
						_ = s.assetStore.Delete(rec.StorageKey)
					}
				}
			}
		}
	}

	cost := actualCost
	if cost <= 0 && status == ImageStudioJobStatusCompleted {
		cost = job.EstimatedCost
	}
	var costPtr *float64
	if cost > 0 {
		costPtr = &cost
	}
	if err := s.repo.UpdateJobResult(ctx, jobID, status, costPtr, errMsg); err != nil {
		return nil, err
	}
	if status == ImageStudioJobStatusCompleted && s.playService != nil {
		_ = s.playService.MarkQuestCompleted(ctx, userID, PlayQuestKeyImageGenerate)
	}

	outJob, err := s.repo.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	s.enrichJobAssets(outJob)
	out := &ImageStudioGenerateResult{Job: *outJob}
	if s.playService != nil {
		quests, err := s.playService.GetQuestsToday(ctx, userID)
		if err == nil {
			out.QuestProgress = quests
		}
	}
	return out, nil
}

func (s *ImageStudioService) ListJobs(ctx context.Context, userID int64, limit int) ([]ImageStudioJob, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrImageStudioDisabled
	}
	jobs, err := s.repo.ListJobs(ctx, userID, limit)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		s.enrichJobAssets(&jobs[i])
	}
	return jobs, nil
}

func (s *ImageStudioService) GetJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error) {
	job, err := s.repo.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	s.invalidateImageStudioBalance(ctx, userID)
	s.enrichJobAssets(job)
	return job, nil
}

func (s *ImageStudioService) GetActiveJob(ctx context.Context, userID int64) (*ImageStudioJob, error) {
	if !s.IsEnabled(ctx) || userID <= 0 {
		return nil, nil
	}
	job, err := s.repo.GetActiveJob(ctx, userID)
	if err != nil || job == nil {
		return job, err
	}
	s.enrichJobAssets(job)
	return job, nil
}

func (s *ImageStudioService) ListActiveJobs(ctx context.Context, userID int64) ([]ImageStudioJob, error) {
	if !s.IsEnabled(ctx) || userID <= 0 {
		return []ImageStudioJob{}, nil
	}
	jobs, err := s.repo.ListActiveJobs(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		s.enrichJobAssets(&jobs[i])
	}
	return jobs, nil
}

func (s *ImageStudioService) CancelJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error) {
	job, err := s.repo.RequestCancel(
		ctx,
		userID,
		jobID,
		time.Now().UTC(),
		func(releaseCtx context.Context, job *ImageStudioJob) error {
			return settleImageStudioBalance(releaseCtx, s.billingRepo, job, 0)
		},
	)
	if err != nil {
		return nil, err
	}
	s.invalidateImageStudioBalance(ctx, userID)
	s.enrichJobAssets(job)
	return job, nil
}

func (s *ImageStudioService) ClaimNextJob(ctx context.Context, leaseOwner string, now time.Time, leaseDuration time.Duration) (*ImageStudioJob, error) {
	return s.repo.ClaimNextJob(ctx, leaseOwner, now, leaseDuration)
}

func (s *ImageStudioService) HeartbeatJob(ctx context.Context, jobID, leaseOwner string, now time.Time, leaseDuration time.Duration) error {
	return s.repo.HeartbeatJob(ctx, jobID, leaseOwner, now, leaseDuration)
}

func (s *ImageStudioService) DecryptJobRequest(job *ImageStudioJob) (string, error) {
	if job == nil || strings.TrimSpace(job.RequestPayloadEncrypted) == "" || s.encryptor == nil {
		return "", errors.New("image studio encrypted request is unavailable")
	}
	return s.encryptor.Decrypt(job.RequestPayloadEncrypted)
}

func (s *ImageStudioService) BuildWorkerRequest(ctx context.Context, job *ImageStudioJob, decrypted string) (*ImageStudioWorkerRequest, error) {
	if strings.TrimSpace(decrypted) == "" {
		return nil, errors.New("image studio worker request is empty")
	}
	envelope, err := decodeImageStudioWorkerEnvelope([]byte(decrypted))
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(gjson.GetBytes(envelope.Body, "model").String())
	if model == "" && job != nil {
		model = strings.TrimSpace(job.Model)
	}
	platform := strings.ToLower(strings.TrimSpace(envelope.Platform))
	if platform == "" {
		platform, err = inferImageStudioProviderFromModel(model)
		if err != nil {
			return nil, err
		}
	}
	capability, ok := ResolveImageStudioProviderCapability(platform, model)
	if !ok {
		return nil, ErrImageStudioProviderNotSupported
	}
	if envelope.CapabilityProfileID != "" || envelope.CapabilityRevision != "" {
		if envelope.CapabilityProfileID != capability.ProfileID ||
			envelope.CapabilityRevision != capability.Revision {
			return nil, ErrImageStudioCapabilityProfileChanged
		}
	}
	operation := strings.ToLower(strings.TrimSpace(envelope.Operation))
	if operation == "" {
		operation = "create"
		if envelope.Endpoint == openAIImagesEditsEndpoint {
			operation = "edit"
		}
	}
	if !imageStudioStringAllowed(capability.Operations, operation) {
		return nil, ErrImageStudioOperationNotSupported
	}
	endpoint := strings.TrimSpace(envelope.Endpoint)
	if endpoint == "" {
		endpoint = openAIImagesGenerationsEndpoint
		if operation == "edit" {
			endpoint = openAIImagesEditsEndpoint
		}
	}

	switch platform {
	case PlatformOpenAI:
		return s.buildOpenAIImageStudioWorkerRequest(ctx, job, operation, endpoint, envelope.Body)
	case PlatformGemini:
		return s.buildGeminiImageStudioWorkerRequest(ctx, job, operation, endpoint, envelope.Body)
	case PlatformGrok:
		return s.buildGrokImageStudioWorkerRequest(ctx, job, operation, endpoint, envelope.Body)
	default:
		return nil, ErrImageStudioProviderNotSupported
	}
}

func (s *ImageStudioService) buildOpenAIImageStudioWorkerRequest(
	ctx context.Context,
	job *ImageStudioJob,
	operation, endpoint string,
	body []byte,
) (*ImageStudioWorkerRequest, error) {
	if operation == "edit" {
		if endpoint != openAIImagesEditsEndpoint {
			return nil, ErrImageStudioOperationNotSupported
		}
		contentType, multipartBody, err := s.buildImageStudioEditMultipart(ctx, job, body)
		if err != nil {
			return nil, err
		}
		return &ImageStudioWorkerRequest{
			Platform:    PlatformOpenAI,
			Operation:   operation,
			Endpoint:    endpoint,
			ContentType: contentType,
			Body:        multipartBody,
		}, nil
	}
	if endpoint != openAIImagesGenerationsEndpoint {
		return nil, ErrImageStudioOperationNotSupported
	}
	single, err := forceImageStudioSingleOutputJSON(body)
	if err != nil {
		return nil, err
	}
	return &ImageStudioWorkerRequest{
		Platform:    PlatformOpenAI,
		Operation:   operation,
		Endpoint:    endpoint,
		ContentType: "application/json",
		Body:        single,
	}, nil
}

func (s *ImageStudioService) buildGeminiImageStudioWorkerRequest(
	ctx context.Context,
	job *ImageStudioJob,
	operation, endpoint string,
	body []byte,
) (*ImageStudioWorkerRequest, error) {
	if operation == "edit" {
		editBody, err := s.buildGeminiImageStudioEditJSON(ctx, job, body)
		if err != nil {
			return nil, err
		}
		return &ImageStudioWorkerRequest{
			Platform:    PlatformGemini,
			Operation:   operation,
			Endpoint:    endpoint,
			ContentType: "application/json",
			Body:        editBody,
		}, nil
	}
	return &ImageStudioWorkerRequest{
		Platform:    PlatformGemini,
		Operation:   operation,
		Endpoint:    endpoint,
		ContentType: "application/json",
		Body:        body,
	}, nil
}

func (s *ImageStudioService) buildGrokImageStudioWorkerRequest(
	ctx context.Context,
	job *ImageStudioJob,
	operation, endpoint string,
	body []byte,
) (*ImageStudioWorkerRequest, error) {
	if operation == "edit" {
		if endpoint != openAIImagesEditsEndpoint {
			return nil, ErrImageStudioOperationNotSupported
		}
		editBody, err := s.buildGrokImageStudioEditJSON(ctx, job, body)
		if err != nil {
			return nil, err
		}
		return &ImageStudioWorkerRequest{
			Platform:    PlatformGrok,
			Operation:   operation,
			Endpoint:    endpoint,
			ContentType: "application/json",
			Body:        editBody,
		}, nil
	}
	if endpoint != openAIImagesGenerationsEndpoint {
		return nil, ErrImageStudioOperationNotSupported
	}
	single, err := forceImageStudioSingleOutputJSON(body)
	if err != nil {
		return nil, err
	}
	return &ImageStudioWorkerRequest{
		Platform:    PlatformGrok,
		Operation:   operation,
		Endpoint:    endpoint,
		ContentType: "application/json",
		Body:        single,
	}, nil
}

func decodeImageStudioWorkerEnvelope(raw []byte) (imageStudioWorkerEnvelope, error) {
	var envelope imageStudioWorkerEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Body) > 0 {
		return envelope, nil
	}
	if !json.Valid(raw) {
		return imageStudioWorkerEnvelope{}, errors.New("image studio worker request is invalid JSON")
	}
	return imageStudioWorkerEnvelope{
		Endpoint: openAIImagesGenerationsEndpoint,
		Body:     append(json.RawMessage(nil), raw...),
	}, nil
}

func (s *ImageStudioService) buildGrokImageStudioEditJSON(
	ctx context.Context,
	job *ImageStudioJob,
	body []byte,
) ([]byte, error) {
	if s.assetStore == nil {
		return nil, errors.New("image studio asset store unavailable")
	}
	if job == nil || job.ID == "" || job.UserID <= 0 {
		return nil, errors.New("image studio job identity is required")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	referenceIDs := referenceIDsFromImageStudioPayload(payload["image_studio_job_reference_ids"])
	if len(referenceIDs) == 0 {
		return nil, ErrImageStudioReferenceNotFound
	}
	repo, ok := s.repo.(ImageStudioJobReferenceReader)
	if !ok {
		return nil, ErrImageStudioReferenceNotFound
	}
	refs, err := repo.ListJobReferencesByID(ctx, job.ID, referenceIDs)
	if err != nil {
		return nil, err
	}
	if len(refs) != len(referenceIDs) {
		return nil, ErrImageStudioReferenceNotFound
	}
	images := make([]map[string]string, 0, len(refs))
	for _, ref := range refs {
		data, err := s.assetStore.Read(ref.StorageKey)
		if err != nil {
			return nil, err
		}
		contentType := strings.TrimSpace(ref.ContentType)
		if contentType == "" {
			return nil, ErrImageStudioReferenceInvalid
		}
		images = append(images, map[string]string{
			"type": "image_url",
			"url":  "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data),
		})
	}
	delete(payload, "image_studio_job_reference_ids")
	payload["images"] = images
	payload["n"] = 1
	return json.Marshal(payload)
}

func (s *ImageStudioService) buildGeminiImageStudioEditJSON(
	ctx context.Context,
	job *ImageStudioJob,
	body []byte,
) ([]byte, error) {
	if s.assetStore == nil {
		return nil, errors.New("image studio asset store unavailable")
	}
	if job == nil || job.ID == "" || job.UserID <= 0 {
		return nil, errors.New("image studio job identity is required")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	referenceIDs := referenceIDsFromImageStudioPayload(payload["image_studio_job_reference_ids"])
	if len(referenceIDs) == 0 {
		return nil, ErrImageStudioReferenceNotFound
	}
	repo, ok := s.repo.(ImageStudioJobReferenceReader)
	if !ok {
		return nil, ErrImageStudioReferenceNotFound
	}
	refs, err := repo.ListJobReferencesByID(ctx, job.ID, referenceIDs)
	if err != nil {
		return nil, err
	}
	if len(refs) != len(referenceIDs) {
		return nil, ErrImageStudioReferenceNotFound
	}
	contents, _ := payload["contents"].([]any)
	if len(contents) == 0 {
		return nil, ErrImageStudioReferenceInvalid
	}
	firstContent, _ := contents[0].(map[string]any)
	if firstContent == nil {
		return nil, ErrImageStudioReferenceInvalid
	}
	parts, _ := firstContent["parts"].([]any)
	if len(parts) == 0 {
		return nil, ErrImageStudioReferenceInvalid
	}
	for _, ref := range refs {
		data, err := s.assetStore.Read(ref.StorageKey)
		if err != nil {
			return nil, err
		}
		contentType := strings.TrimSpace(ref.ContentType)
		if contentType == "" {
			return nil, ErrImageStudioReferenceInvalid
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]string{
				"mimeType": contentType,
				"data":     base64.StdEncoding.EncodeToString(data),
			},
		})
	}
	firstContent["parts"] = parts
	contents[0] = firstContent
	payload["contents"] = contents
	delete(payload, "image_studio_job_reference_ids")
	return json.Marshal(payload)
}

func (s *ImageStudioService) buildImageStudioEditMultipart(ctx context.Context, job *ImageStudioJob, body []byte) (string, []byte, error) {
	if s.assetStore == nil {
		return "", nil, errors.New("image studio asset store unavailable")
	}
	if job == nil || job.ID == "" || job.UserID <= 0 {
		return "", nil, errors.New("image studio job identity is required")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", nil, err
	}
	referenceIDs := referenceIDsFromImageStudioPayload(payload["image_studio_job_reference_ids"])
	if len(referenceIDs) == 0 {
		return "", nil, ErrImageStudioReferenceNotFound
	}
	if len(referenceIDs) > maxImageStudioReferences {
		return "", nil, ErrImageStudioReferenceLimit
	}
	repo, ok := s.repo.(ImageStudioJobReferenceReader)
	if !ok {
		return "", nil, ErrImageStudioReferenceNotFound
	}
	refs, err := repo.ListJobReferencesByID(ctx, job.ID, referenceIDs)
	if err != nil {
		return "", nil, err
	}
	if len(refs) != len(referenceIDs) {
		return "", nil, ErrImageStudioReferenceNotFound
	}

	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	writeField := func(name string, value any) error {
		text := imageStudioMultipartFieldValue(value)
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return writer.WriteField(name, text)
	}
	for _, name := range []string{
		"model", "prompt", "size", "response_format", "quality", "background",
		"output_format", "moderation", "input_fidelity", "style", "output_compression",
		"partial_images",
	} {
		if err := writeField(name, payload[name]); err != nil {
			return "", nil, err
		}
	}
	if err := writer.WriteField("n", "1"); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(imageStudioMultipartFieldValue(payload["response_format"])) == "" {
		if err := writer.WriteField("response_format", "b64_json"); err != nil {
			return "", nil, err
		}
	}
	for _, ref := range refs {
		data, err := s.assetStore.Read(ref.StorageKey)
		if err != nil {
			return "", nil, err
		}
		contentType := strings.TrimSpace(ref.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s%s"`, ref.ID, extensionForContentType(contentType)))
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			return "", nil, err
		}
		if _, err := part.Write(data); err != nil {
			return "", nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return writer.FormDataContentType(), buffer.Bytes(), nil
}

func forceImageStudioSingleOutputJSON(body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	payload["n"] = 1
	return json.Marshal(payload)
}

func imageStudioMultipartFieldValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func referenceIDsFromImageStudioPayload(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(raw))
	for _, item := range raw {
		id := strings.TrimSpace(fmt.Sprint(item))
		if id != "" {
			ids = append(ids, id)
		}
	}
	return normalizeImageStudioReferenceIDs(ids)
}

func normalizeImageStudioReferenceIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *ImageStudioService) ClaimNextItem(ctx context.Context, jobID, leaseOwner string, now time.Time) (*ImageStudioItem, error) {
	return s.repo.ClaimNextItem(ctx, jobID, leaseOwner, now)
}

func (s *ImageStudioService) RetryWorkerItem(
	ctx context.Context,
	job *ImageStudioJob,
	item *ImageStudioItem,
	leaseOwner string,
	_ error,
	now time.Time,
) error {
	if job == nil || item == nil {
		return errors.New("image studio job and item are required")
	}
	if err := s.repo.RetryItem(ctx, job.ID, item.ID, leaseOwner, now); err != nil {
		return err
	}
	item.Status = ImageStudioItemStatusPending
	item.ActualCost = nil
	item.Error = ""
	item.AssetID = nil
	item.FinishedAt = nil
	item.CheckpointData = nil
	item.CheckpointContentType = ""
	item.CheckpointActualCost = nil
	return nil
}

func (s *ImageStudioService) CheckpointWorkerItem(
	ctx context.Context,
	job *ImageStudioJob,
	item *ImageStudioItem,
	leaseOwner string,
	image *ImageStudioImagePayload,
	actualCost float64,
	now time.Time,
) error {
	if job == nil || item == nil || image == nil || len(image.Data) == 0 {
		return errors.New("image studio checkpoint requires generated image data")
	}
	actualCost = imageStudioActualCostWithinHold(job, actualCost)
	if err := s.repo.CheckpointItem(ctx, job.ID, item.ID, leaseOwner, *image, actualCost, now); err != nil {
		if errors.Is(err, ErrImageStudioCheckpointCancelled) {
			item.Status = ImageStudioItemStatusCancelled
			item.ActualCost = &actualCost
			item.CheckpointData = nil
			item.CheckpointContentType = ""
			item.CheckpointActualCost = nil
		}
		return err
	}
	item.Status = ImageStudioItemStatusPersisting
	item.CheckpointData = append([]byte(nil), image.Data...)
	item.CheckpointContentType = image.ContentType
	item.CheckpointActualCost = &actualCost
	return nil
}

func (s *ImageStudioService) CompleteWorkerItem(
	ctx context.Context,
	job *ImageStudioJob,
	item *ImageStudioItem,
	leaseOwner string,
	image *ImageStudioImagePayload,
	actualCost float64,
	itemErr error,
	now time.Time,
) error {
	if job == nil || item == nil {
		return errors.New("image studio job and item are required")
	}
	actualCost = imageStudioActualCostWithinHold(job, actualCost)
	status := ImageStudioItemStatusFailed
	errMsg := errStringForImageStudio(itemErr)
	var cost *float64
	var asset *ImageStudioAssetRecord
	if itemErr == nil && image != nil {
		if s.assetStore == nil {
			return errors.New("image studio asset store unavailable")
		}
		assetID := item.ID
		assetExpiresAt := now.UTC().Add(imageStudioAssetDefaultTTL)
		derivative, err := buildImageStudioAssetDerivative(image.Data)
		if err != nil {
			return fmt.Errorf("failed to inspect generated image: %w", err)
		}
		originalObjectID := assetID + "-original"
		thumbnailObjectID := assetID + "-thumbnail"
		storageKey, err := s.assetStore.Save(job.UserID, originalObjectID, image.ContentType, image.Data)
		if err != nil {
			return fmt.Errorf("failed to persist generated image: %w", err)
		}
		thumbnailStorageKey, err := s.assetStore.Save(
			job.UserID,
			thumbnailObjectID,
			derivative.ThumbnailContentType,
			derivative.ThumbnailData,
		)
		if err != nil {
			_ = s.assetStore.Delete(storageKey)
			return fmt.Errorf("failed to persist generated image thumbnail: %w", err)
		}
		asset = &ImageStudioAssetRecord{
			ID:                   assetID,
			StorageKey:           storageKey,
			ContentType:          image.ContentType,
			ByteSize:             int64(len(image.Data)),
			SortOrder:            item.SortOrder,
			Width:                derivative.Width,
			Height:               derivative.Height,
			Filename:             imageStudioAssetFilename(assetID, image.ContentType),
			ExpiresAt:            &assetExpiresAt,
			ThumbnailStorageKey:  thumbnailStorageKey,
			ThumbnailContentType: derivative.ThumbnailContentType,
			ThumbnailByteSize:    int64(len(derivative.ThumbnailData)),
		}
		status = ImageStudioItemStatusSuccess
		cost = &actualCost
		if err := s.repo.CompleteItem(ctx, job.ID, item.ID, leaseOwner, status, cost, "", asset, now); err != nil {
			persisted, verifyErr := s.repo.GetItem(context.WithoutCancel(ctx), job.ID, item.ID)
			if verifyErr == nil && persisted.Status == ImageStudioItemStatusSuccess &&
				persisted.AssetID != nil && *persisted.AssetID == assetID {
				return nil
			}
			if verifyErr == nil {
				_ = s.assetStore.Delete(storageKey)
				_ = s.assetStore.Delete(thumbnailStorageKey)
			}
			return err
		}
		return nil
	}
	return s.repo.CompleteItem(ctx, job.ID, item.ID, leaseOwner, status, nil, errMsg, nil, now)
}

func imageStudioActualCostWithinHold(job *ImageStudioJob, actualCost float64) float64 {
	if actualCost < 0 {
		actualCost = 0
	}
	if job == nil {
		return actualCost
	}
	perItemCap := ImageStudioPerItemBillingCap(job)
	if actualCost > perItemCap {
		return perItemCap
	}
	return actualCost
}

func imageStudioAssetFilename(assetID, contentType string) string {
	trimmed := strings.TrimSpace(assetID)
	if len(trimmed) > 8 {
		trimmed = trimmed[:8]
	}
	if trimmed == "" {
		trimmed = "image"
	}
	return "image-studio-" + trimmed + extensionForContentType(contentType)
}

func (s *ImageStudioService) SettleJob(ctx context.Context, jobID, leaseOwner string, now time.Time) (*ImageStudioJob, error) {
	job, err := s.repo.SettleJob(ctx, jobID, leaseOwner, now, func(settleCtx context.Context, job *ImageStudioJob, actualCost float64) error {
		return settleImageStudioBalance(settleCtx, s.billingRepo, job, actualCost)
	})
	if err != nil {
		return nil, err
	}
	if job != nil {
		s.invalidateImageStudioBalance(ctx, job.UserID)
		s.enrichJobAssets(job)
		if job.SuccessCount > 0 && s.playService != nil {
			_ = s.playService.MarkQuestCompleted(ctx, job.UserID, PlayQuestKeyImageGenerate)
		}
	}
	return job, nil
}

func (s *ImageStudioService) invalidateImageStudioBalance(ctx context.Context, userID int64) {
	if s.billingCache == nil || userID <= 0 {
		return
	}
	_ = s.billingCache.InvalidateUserBalance(context.WithoutCancel(ctx), userID)
}

func errStringForImageStudio(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *ImageStudioService) OpenAssetContent(ctx context.Context, userID int64, assetID string) ([]byte, string, error) {
	asset, err := s.repo.GetAsset(ctx, userID, assetID)
	if err != nil {
		return nil, "", err
	}
	if imageStudioAssetExpired(asset, time.Now().UTC()) {
		return nil, "", ErrImageStudioAssetExpired
	}
	if asset.StorageKey != "" && s.assetStore != nil {
		data, err := s.assetStore.Read(asset.StorageKey)
		if err != nil {
			return nil, "", ErrImageStudioAssetUnavailable.WithCause(err)
		}
		ct := asset.ContentType
		if ct == "" {
			ct = "image/png"
		}
		return data, ct, nil
	}
	if asset.URL != "" {
		return nil, asset.URL, nil
	}
	return nil, "", ErrImageStudioAssetNotFound
}

func (s *ImageStudioService) DeleteJob(ctx context.Context, userID int64, jobID string) error {
	if s.assetStore == nil {
		_, err := s.repo.DeleteJobWithStorageKeys(ctx, userID, jobID)
		return err
	}

	referenceKeys, err := s.listJobReferenceStorageKeys(ctx, jobID)
	if err != nil {
		return err
	}
	keys, err := s.repo.DeleteJobWithStorageKeys(ctx, userID, jobID)
	if err != nil {
		return err
	}
	keys = append(keys, referenceKeys...)
	var deleteErr error
	for _, key := range keys {
		err := s.assetStore.Delete(key)
		if err != nil {
			deleteErr = errors.Join(deleteErr, err)
			if deletionRepo, ok := s.repo.(ImageStudioObjectDeletionRepository); ok {
				deleteErr = errors.Join(deleteErr, deletionRepo.RecordObjectDeletionFailure(ctx, key, err))
			}
			continue
		}
		if deletionRepo, ok := s.repo.(ImageStudioObjectDeletionRepository); ok {
			deleteErr = errors.Join(deleteErr, deletionRepo.AcknowledgeObjectDeletion(ctx, key))
		}
	}
	return deleteErr
}

func (s *ImageStudioService) MarkJobRunning(ctx context.Context, jobID string) error {
	return s.repo.UpdateJobStatus(ctx, jobID, ImageStudioJobStatusRunning)
}

func (s *ImageStudioService) PurgeExpiredJobs(ctx context.Context, now time.Time) (int64, error) {
	retryErr := s.retryPendingObjectDeletions(ctx)
	retryErr = errors.Join(retryErr, s.reconcileUntrackedObjects(ctx, now))
	if err := s.purgeExpiredUploadReferences(ctx, now); err != nil {
		return 0, errors.Join(retryErr, err)
	}
	if s.settingService != nil && s.settingService.IsImageStudioAssetPurgeEnabled(ctx) {
		_, err := s.PurgeExpiredAssets(ctx, now)
		retryErr = errors.Join(retryErr, err)
	}
	deleted, err := s.repo.DeleteExpiredJobsBefore(ctx, now)
	if err != nil {
		return 0, errors.Join(retryErr, err)
	}
	retryErr = errors.Join(retryErr, s.retryPendingObjectDeletions(ctx))
	return deleted, retryErr
}

func (s *ImageStudioService) PurgeExpiredAssets(ctx context.Context, now time.Time) (int64, error) {
	repo, ok := s.repo.(ImageStudioAssetPurgeRepository)
	if !ok || s.assetStore == nil {
		return 0, nil
	}
	candidates, err := repo.ListExpiredAssetsForPurge(ctx, now, imageStudioAssetPurgeBatchSize)
	if err != nil {
		return 0, err
	}
	purgedIDs := make([]string, 0, len(candidates))
	var purgeErr error
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.ID) == "" {
			continue
		}
		var candidateErr error
		for _, key := range candidate.StorageKeys {
			if err := s.assetStore.Delete(key); err != nil {
				candidateErr = errors.Join(candidateErr, err)
			}
		}
		if candidateErr != nil {
			purgeErr = errors.Join(purgeErr, candidateErr)
			continue
		}
		purgedIDs = append(purgedIDs, candidate.ID)
	}
	if len(purgedIDs) == 0 {
		return 0, purgeErr
	}
	n, err := repo.MarkAssetsPurged(ctx, purgedIDs, now.UTC())
	return n, errors.Join(purgeErr, err)
}

func (s *ImageStudioService) retryPendingObjectDeletions(ctx context.Context) error {
	repo, ok := s.repo.(ImageStudioObjectDeletionRepository)
	if !ok || s.assetStore == nil {
		return nil
	}
	keys, err := repo.ListPendingObjectDeletions(ctx, 100)
	if err != nil {
		return err
	}
	var retryErr error
	for _, key := range keys {
		if err := s.assetStore.Delete(key); err != nil {
			retryErr = errors.Join(retryErr, err, repo.RecordObjectDeletionFailure(ctx, key, err))
			continue
		}
		retryErr = errors.Join(retryErr, repo.AcknowledgeObjectDeletion(ctx, key))
	}
	return retryErr
}

func (s *ImageStudioService) reconcileUntrackedObjects(ctx context.Context, now time.Time) error {
	repo, ok := s.repo.(ImageStudioObjectReconciliationRepository)
	if !ok || s.assetStore == nil {
		return nil
	}
	keys, err := s.assetStore.ListStorageKeysBefore(now.Add(-imageStudioUntrackedObjectGrace), 100)
	if err != nil || len(keys) == 0 {
		return err
	}
	tracked, err := repo.FilterTrackedObjectStorageKeys(ctx, keys)
	if err != nil {
		return err
	}
	var reconcileErr error
	for _, key := range keys {
		if _, ok := tracked[key]; ok {
			continue
		}
		reconcileErr = errors.Join(reconcileErr, s.assetStore.Delete(key))
	}
	return reconcileErr
}

func (s *ImageStudioService) enrichJobAssets(job *ImageStudioJob) {
	if job == nil {
		return
	}
	if job.Status != ImageStudioJobStatusCompleted && job.Status != ImageStudioJobStatusPartial {
		job.Assets = nil
		return
	}
	for i := range job.Assets {
		s.enrichAsset(&job.Assets[i])
	}
}

func (s *ImageStudioService) enrichAsset(asset *ImageStudioAsset) {
	if asset == nil {
		return
	}
	if asset.Filename == "" {
		asset.Filename = imageStudioAssetFilename(asset.ID, asset.ContentType)
	}
	asset.Availability = imageStudioAssetAvailability(asset, time.Now().UTC())
	if imageStudioAssetExpired(asset, time.Now().UTC()) {
		asset.URL = ""
		asset.PreviewURL = ""
		asset.DownloadURL = ""
		asset.ThumbnailURL = ""
		return
	}
	if strings.TrimSpace(asset.ThumbnailStorageKey) != "" {
		asset.ThumbnailURL = "/api/v1/image-studio/assets/" + asset.ID + "/thumbnail"
	}
	if asset.StorageKey != "" {
		asset.PreviewURL = "/api/v1/image-studio/assets/" + asset.ID + "/content"
		asset.DownloadURL = "/api/v1/image-studio/assets/" + asset.ID + "/download"
		asset.URL = asset.PreviewURL
		return
	}
	if asset.URL != "" {
		asset.PreviewURL = asset.URL
		asset.DownloadURL = asset.URL
	}
}

func imageStudioAssetExpired(asset *ImageStudioAsset, now time.Time) bool {
	if asset == nil {
		return false
	}
	if asset.PurgedAt != nil {
		return true
	}
	return asset.ExpiresAt != nil && !asset.ExpiresAt.After(now)
}

func imageStudioAssetAvailability(asset *ImageStudioAsset, now time.Time) string {
	if asset == nil {
		return ""
	}
	if imageStudioAssetExpired(asset, now) {
		return "expired"
	}
	if strings.TrimSpace(asset.StorageKey) == "" && strings.TrimSpace(asset.URL) == "" {
		return "unavailable"
	}
	return "available"
}

func (s *ImageStudioService) GetHubStatus(ctx context.Context, userID int64) (*ImageStudioHubStatus, error) {
	out := &ImageStudioHubStatus{Enabled: s.IsEnabled(ctx)}
	if !out.Enabled || userID <= 0 || s.playService == nil {
		return out, nil
	}
	now := s.playService.serverNow()
	dayStart := s.playService.serverDate(now)
	count, err := s.repo.CountCompletedToday(ctx, userID, dayStart)
	if err != nil {
		return nil, err
	}
	out.ImagesToday = count
	hasJob, err := s.repo.HasCompletedJob(ctx, userID)
	if err != nil {
		return nil, err
	}
	out.HasCompletedJob = hasJob || count > 0
	return out, nil
}

func (s *ImageStudioService) resolveAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	if apiKeyID > 0 {
		key, err := s.apiKeyService.GetByID(ctx, apiKeyID)
		if err != nil {
			return nil, ErrImageStudioAPIKey
		}
		if key.UserID != userID || ValidateImageStudioAPIKey(key) != nil {
			return nil, ErrImageStudioAPIKey
		}
		if key.Group != nil && key.Group.IsSubscriptionType() {
			return nil, ErrImageStudioSubscriptionGroupUnsupported
		}
		return key, nil
	}
	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1}, APIKeyListFilters{Status: StatusAPIKeyActive})
	if err != nil || len(keys) == 0 {
		return nil, ErrImageStudioAPIKey
	}
	if ValidateImageStudioAPIKey(&keys[0]) != nil {
		return nil, ErrImageStudioAPIKey
	}
	if keys[0].Group != nil && keys[0].Group.IsSubscriptionType() {
		return nil, ErrImageStudioSubscriptionGroupUnsupported
	}
	return &keys[0], nil
}

func buildImageStudioPrompt(tpl ImageStudioTemplate, req ImageStudioGenerateRequest) string {
	if req.ExpertPrompt != nil && strings.TrimSpace(*req.ExpertPrompt) != "" {
		return strings.TrimSpace(*req.ExpertPrompt)
	}
	subject := strings.TrimSpace(req.UserPrompt)
	templatePrompt := strings.ReplaceAll(tpl.PromptTemplate, "{{subject}}", subject)
	templatePrompt = softenImageStudioTextConstraint(templatePrompt)
	prompt := strings.Join([]string{
		"Primary subject: " + subject + ". The image must be about this subject, not a generic substitute. If the subject includes a brand name, product name, title, UI text, or membership wording, preserve that text only where it belongs to the subject.",
		"Template direction: " + templatePrompt,
	}, " ")
	if accent := strings.TrimSpace(req.AccentColor); accent != "" {
		prompt += ", accent color " + accent
	}
	return prompt
}

func softenImageStudioTextConstraint(prompt string) string {
	replacer := strings.NewReplacer(
		"no text", "no unrelated text",
		"No text", "No unrelated text",
		"NO TEXT", "NO UNRELATED TEXT",
	)
	return replacer.Replace(prompt)
}

func validateImageStudioPrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return ErrImageStudioPromptRequired
	}
	if utf8.RuneCountInString(prompt) > maxImageStudioPromptChars {
		return ErrImageStudioPromptTooLong
	}
	return nil
}

func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

func normalizeImageStudioPromptReference(promptID *int64, promptVersion *int) (*int64, *int, error) {
	if promptID == nil && promptVersion == nil {
		return nil, nil, nil
	}
	if promptID == nil || promptVersion == nil || *promptID <= 0 || *promptVersion <= 0 {
		return nil, nil, ErrImageStudioPromptRef
	}
	return promptID, promptVersion, nil
}

func (s *ImageStudioService) validatePromptLibraryReference(
	ctx context.Context,
	userID int64,
	promptID *int64,
	promptVersion *int,
) error {
	if promptID == nil {
		return nil
	}
	if s.promptRepo == nil || promptVersion == nil {
		return ErrImageStudioPromptRef
	}
	prompt, err := s.promptRepo.GetPrompt(ctx, *promptID, &userID, true)
	if err != nil {
		return err
	}
	if prompt == nil || prompt.PublishedVersion != *promptVersion {
		return ErrImageStudioPromptRef
	}
	return nil
}

func findImageStudioTemplate(id string) (ImageStudioTemplate, bool) {
	for _, intent := range defaultImageStudioCatalog().Intents {
		for _, tpl := range intent.Templates {
			if tpl.ID == id {
				return tpl, true
			}
		}
	}
	return ImageStudioTemplate{}, false
}
