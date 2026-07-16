package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const (
	ImageStudioJobStatusPending   = "pending"
	ImageStudioJobStatusRunning   = "running"
	ImageStudioJobStatusCompleted = "completed"
	ImageStudioJobStatusFailed    = "failed"

	defaultImageStudioSize = "1024x1024"
	maxImageStudioCount    = 4
)

var (
	ErrImageStudioDisabled       = infraerrors.BadRequest("IMAGE_STUDIO_DISABLED", "image studio is disabled")
	ErrImageStudioJobNotFound    = infraerrors.NotFound("IMAGE_STUDIO_JOB_NOT_FOUND", "image studio job not found")
	ErrImageStudioTemplate       = infraerrors.BadRequest("IMAGE_STUDIO_TEMPLATE_INVALID", "invalid template")
	ErrImageStudioPromptRequired = infraerrors.BadRequest("IMAGE_STUDIO_PROMPT_REQUIRED", "image description is required")
	ErrImageStudioAPIKey         = infraerrors.BadRequest("IMAGE_STUDIO_API_KEY_REQUIRED", "valid API key is required")
	ErrImageStudioAssetNotFound  = infraerrors.NotFound("IMAGE_STUDIO_ASSET_NOT_FOUND", "image studio asset not found")
)

// ValidateImageStudioAPIKey rejects keys that cannot authorize a generation request.
func ValidateImageStudioAPIKey(apiKey *APIKey) error {
	if apiKey == nil || !apiKey.IsActive() || apiKey.IsExpired() || apiKey.IsQuotaExhausted() {
		return ErrImageStudioAPIKey
	}
	return nil
}

type ImageStudioAsset struct {
	ID          string `json:"id"`
	URL         string `json:"url,omitempty"`
	SortOrder   int    `json:"sort_order"`
	ContentType string `json:"content_type,omitempty"`
	ByteSize    int64  `json:"byte_size,omitempty"`
	PreviewURL  string `json:"preview_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	StorageKey  string `json:"-"`
}

type ImageStudioAssetRecord struct {
	ID          string
	StorageKey  string
	ContentType string
	ByteSize    int64
	SortOrder   int
	URL         string
}

type ImageStudioImagePayload struct {
	Data        []byte
	ContentType string
}

type ImageStudioJob struct {
	ID            string             `json:"id"`
	UserID        int64              `json:"user_id"`
	TemplateID    string             `json:"template_id"`
	PromptHash    string             `json:"-"`
	Size          string             `json:"size"`
	Count         int                `json:"count"`
	Status        string             `json:"status"`
	EstimatedCost float64            `json:"estimated_cost"`
	ActualCost    *float64           `json:"actual_cost,omitempty"`
	APIKeyID      *int64             `json:"api_key_id,omitempty"`
	ErrorMessage  string             `json:"error_message,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	ExpiresAt     *time.Time         `json:"expires_at,omitempty"`
	Assets        []ImageStudioAsset `json:"assets,omitempty"`
}

type ImageStudioGenerateRequest struct {
	TemplateID   string  `json:"template_id"`
	UserPrompt   string  `json:"user_prompt"`
	AccentColor  string  `json:"accent_color"`
	Size         string  `json:"size"`
	Aspect       string  `json:"aspect"`
	Tier         string  `json:"tier"`
	Quality      string  `json:"quality"`
	Count        int     `json:"count"`
	Model        string  `json:"model"`
	ExpertPrompt *string `json:"expert_prompt"`
	APIKeyID     int64   `json:"api_key_id"`
	RetainDays   *int    `json:"retain_days,omitempty"`
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
	UpdateJobResult(ctx context.Context, jobID string, status string, actualCost *float64, errMsg string) error
	InsertAssets(ctx context.Context, jobID string, assets []ImageStudioAssetRecord) error
	GetJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error)
	GetActiveJob(ctx context.Context, userID int64) (*ImageStudioJob, error)
	GetAsset(ctx context.Context, userID int64, assetID string) (*ImageStudioAsset, error)
	ListJobs(ctx context.Context, userID int64, limit int) ([]ImageStudioJob, error)
	ListAssetStorageKeysForJob(ctx context.Context, jobID string) ([]string, error)
	ListExpiredJobIDs(ctx context.Context, before time.Time) ([]string, error)
	DeleteJob(ctx context.Context, userID int64, jobID string) error
	CountCompletedToday(ctx context.Context, userID int64, dayStart time.Time) (int, error)
	UpdateJobStatus(ctx context.Context, jobID string, status string) error
	DeleteExpiredJobsBefore(ctx context.Context, before time.Time) (int64, error)
	HasCompletedJob(ctx context.Context, userID int64) (bool, error)
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
	capabilityCache *ImageStudioCapabilityCache
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
	}
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

func (s *ImageStudioService) Estimate(ctx context.Context, userID int64, templateID string, size string, count int, apiKeyID int64, model string) (*ImageStudioEstimate, error) {
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
		if p, err := s.pricing.BatchImageUnitPrice(ctx, &BatchImageJob{
			Provider: PlatformOpenAI,
			Model:    model,
		}); err == nil && p > 0 {
			unit = p
		}
	}
	if apiKey.Group != nil {
		if configured := apiKey.Group.GetImagePrice(normalizeStudioImageSize(size)); configured != nil && *configured > 0 {
			unit = *configured
		}
		mult := apiKey.Group.RateMultiplier
		if apiKey.Group.ImageRateIndependent {
			mult = apiKey.Group.ImageRateMultiplier
		}
		if mult > 0 {
			unit *= mult
		}
	}
	return unit * float64(count), nil
}

func (s *ImageStudioService) CreatePendingJob(ctx context.Context, userID int64, req ImageStudioGenerateRequest) (*ImageStudioJob, string, error) {
	if !s.IsEnabled(ctx) {
		return nil, "", ErrImageStudioDisabled
	}
	if err := validateImageStudioPrompt(req.UserPrompt); err != nil {
		return nil, "", err
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
	resolvedModel, err := s.resolveImageModel(ctx, apiKey, req.Model)
	if err != nil {
		return nil, "", err
	}
	size, err := s.resolveGenerateSize(apiKey, resolvedModel, req, tpl)
	if err != nil {
		return nil, "", err
	}
	if err := s.ValidateQualityForModel(resolvedModel, req.Quality); err != nil {
		return nil, "", err
	}
	prompt := buildImageStudioPrompt(tpl, req)
	est, err := s.estimateCost(ctx, apiKey, resolvedModel, size, count)
	if err != nil {
		return nil, "", err
	}
	if user.Balance < est {
		return nil, "", infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance for image generation")
	}
	var expires *time.Time
	if req.RetainDays != nil && *req.RetainDays == 0 {
		expires = nil
	} else {
		days := 7
		if req.RetainDays != nil && *req.RetainDays > 0 {
			days = *req.RetainDays
		}
		t := time.Now().AddDate(0, 0, days)
		expires = &t
	}
	job := &ImageStudioJob{
		ID:            uuid.NewString(),
		UserID:        userID,
		TemplateID:    req.TemplateID,
		PromptHash:    hashPrompt(prompt),
		Size:          size,
		Count:         count,
		Status:        ImageStudioJobStatusPending,
		EstimatedCost: est,
		APIKeyID:      &apiKey.ID,
		ExpiresAt:     expires,
	}
	if err := s.repo.InsertJob(ctx, job); err != nil {
		return nil, "", err
	}
	payload := map[string]any{
		"model":  resolvedModel,
		"prompt": prompt,
		"n":      count,
		"size":   size,
		// Prefer inline bytes so CompleteJob can persist without a second remote fetch.
		"response_format": "b64_json",
	}
	if quality := strings.TrimSpace(strings.ToLower(req.Quality)); quality != "" {
		payload["quality"] = quality
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	return job, string(body), nil
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

func (s *ImageStudioService) OpenAssetContent(ctx context.Context, userID int64, assetID string) ([]byte, string, error) {
	asset, err := s.repo.GetAsset(ctx, userID, assetID)
	if err != nil {
		return nil, "", err
	}
	if asset.StorageKey != "" && s.assetStore != nil {
		data, err := s.assetStore.Read(asset.StorageKey)
		if err != nil {
			return nil, "", err
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
	if s.assetStore != nil {
		keys, err := s.repo.ListAssetStorageKeysForJob(ctx, jobID)
		if err == nil {
			for _, key := range keys {
				_ = s.assetStore.Delete(key)
			}
		}
	}
	return s.repo.DeleteJob(ctx, userID, jobID)
}

func (s *ImageStudioService) MarkJobRunning(ctx context.Context, jobID string) error {
	return s.repo.UpdateJobStatus(ctx, jobID, ImageStudioJobStatusRunning)
}

func (s *ImageStudioService) PurgeExpiredJobs(ctx context.Context, now time.Time) (int64, error) {
	if s.assetStore != nil {
		jobIDs, err := s.repo.ListExpiredJobIDs(ctx, now)
		if err == nil {
			for _, jobID := range jobIDs {
				keys, kerr := s.repo.ListAssetStorageKeysForJob(ctx, jobID)
				if kerr != nil {
					continue
				}
				for _, key := range keys {
					_ = s.assetStore.Delete(key)
				}
			}
		}
	}
	return s.repo.DeleteExpiredJobsBefore(ctx, now)
}

func (s *ImageStudioService) enrichJobAssets(job *ImageStudioJob) {
	if job == nil {
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
		return key, nil
	}
	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1}, APIKeyListFilters{Status: StatusAPIKeyActive})
	if err != nil || len(keys) == 0 {
		return nil, ErrImageStudioAPIKey
	}
	if ValidateImageStudioAPIKey(&keys[0]) != nil {
		return nil, ErrImageStudioAPIKey
	}
	return &keys[0], nil
}

func buildImageStudioPrompt(tpl ImageStudioTemplate, req ImageStudioGenerateRequest) string {
	if req.ExpertPrompt != nil && strings.TrimSpace(*req.ExpertPrompt) != "" {
		return strings.TrimSpace(*req.ExpertPrompt)
	}
	subject := strings.TrimSpace(req.UserPrompt)
	prompt := strings.ReplaceAll(tpl.PromptTemplate, "{{subject}}", subject)
	if accent := strings.TrimSpace(req.AccentColor); accent != "" {
		prompt += ", accent color " + accent
	}
	return prompt
}

func validateImageStudioPrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return ErrImageStudioPromptRequired
	}
	return nil
}

func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
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
