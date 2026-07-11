package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ImageStudioJobStatusPending   = "pending"
	ImageStudioJobStatusRunning   = "running"
	ImageStudioJobStatusCompleted = "completed"
	ImageStudioJobStatusFailed    = "failed"

	defaultImageStudioModel = "gpt-image-2"
	defaultImageStudioSize  = "1024x1024"
)

var (
	ErrImageStudioDisabled    = infraerrors.BadRequest("IMAGE_STUDIO_DISABLED", "image studio is disabled")
	ErrImageStudioJobNotFound = infraerrors.NotFound("IMAGE_STUDIO_JOB_NOT_FOUND", "image studio job not found")
	ErrImageStudioTemplate    = infraerrors.BadRequest("IMAGE_STUDIO_TEMPLATE_INVALID", "invalid template")
	ErrImageStudioAPIKey      = infraerrors.BadRequest("IMAGE_STUDIO_API_KEY_REQUIRED", "valid API key is required")
)

type ImageStudioAsset struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
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
	Count        int     `json:"count"`
	ExpertPrompt *string `json:"expert_prompt"`
	APIKeyID     int64   `json:"api_key_id"`
}

type ImageStudioGenerateResult struct {
	Job            ImageStudioJob     `json:"job"`
	QuestProgress  *PlayQuestToday    `json:"quest_progress,omitempty"`
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
	InsertAssets(ctx context.Context, jobID string, urls []string) error
	GetJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error)
	ListJobs(ctx context.Context, userID int64, limit int) ([]ImageStudioJob, error)
	DeleteJob(ctx context.Context, userID int64, jobID string) error
	CountCompletedToday(ctx context.Context, userID int64, dayStart time.Time) (int, error)
}

type ImageStudioHubStatus struct {
	Enabled         bool `json:"enabled"`
	ImagesToday     int  `json:"images_today"`
	HasCompletedJob bool `json:"has_completed_job"`
}

type ImageStudioService struct {
	repo           ImageStudioRepository
	apiKeyService  *APIKeyService
	userRepo       UserRepository
	settingService *SettingService
	playService    *PlayService
	pricing        *BatchImageModelPricingResolver
}

func NewImageStudioService(
	repo ImageStudioRepository,
	apiKeyService *APIKeyService,
	userRepo UserRepository,
	settingService *SettingService,
	playService *PlayService,
	pricing *BatchImageModelPricingResolver,
) *ImageStudioService {
	return &ImageStudioService{
		repo:           repo,
		apiKeyService:  apiKeyService,
		userRepo:       userRepo,
		settingService: settingService,
		playService:    playService,
		pricing:        pricing,
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

func (s *ImageStudioService) Estimate(ctx context.Context, userID int64, templateID string, size string, count int, apiKeyID int64) (*ImageStudioEstimate, error) {
	if !s.IsEnabled(ctx) {
		return nil, ErrImageStudioDisabled
	}
	tpl, ok := findImageStudioTemplate(templateID)
	if !ok {
		return nil, ErrImageStudioTemplate
	}
	if size == "" {
		size = tpl.Defaults.Size
	}
	if count <= 0 {
		count = tpl.Defaults.Count
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	cost, err := s.estimateCost(ctx, userID, apiKeyID, size, count)
	if err != nil {
		return nil, err
	}
	return &ImageStudioEstimate{
		EstimatedCost: cost,
		Balance:       user.Balance,
		Sufficient:    user.Balance >= cost,
		Model:         defaultImageStudioModel,
		Count:         count,
		Size:          size,
	}, nil
}

func (s *ImageStudioService) estimateCost(ctx context.Context, userID, apiKeyID int64, size string, count int) (float64, error) {
	apiKey, err := s.resolveAPIKey(ctx, userID, apiKeyID)
	if err != nil {
		return 0, err
	}
	unit := 0.04
	if s.pricing != nil && apiKey.GroupID != nil {
		if p, err := s.pricing.BatchImageUnitPrice(ctx, &BatchImageJob{
			Provider: PlatformOpenAI,
			Model:    defaultImageStudioModel,
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

func normalizeStudioImageSize(size string) string {
	switch strings.TrimSpace(size) {
	case "1024x1536", "1536x1024":
		return "2K"
	default:
		return "1K"
	}
}

func (s *ImageStudioService) CreatePendingJob(ctx context.Context, userID int64, req ImageStudioGenerateRequest) (*ImageStudioJob, string, error) {
	if !s.IsEnabled(ctx) {
		return nil, "", ErrImageStudioDisabled
	}
	tpl, ok := findImageStudioTemplate(req.TemplateID)
	if !ok {
		return nil, "", ErrImageStudioTemplate
	}
	size := req.Size
	if size == "" {
		size = tpl.Defaults.Size
	}
	count := req.Count
	if count <= 0 {
		count = tpl.Defaults.Count
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if user.TotalRecharged <= 0 && count > 1 {
		count = 1
	}
	apiKey, err := s.resolveAPIKey(ctx, userID, req.APIKeyID)
	if err != nil {
		return nil, "", err
	}
	prompt := buildImageStudioPrompt(tpl, req)
	est, err := s.estimateCost(ctx, userID, apiKey.ID, size, count)
	if err != nil {
		return nil, "", err
	}
	if user.Balance < est {
		return nil, "", infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance for image generation")
	}
	expires := time.Now().AddDate(0, 0, 7)
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
		ExpiresAt:     &expires,
	}
	if err := s.repo.InsertJob(ctx, job); err != nil {
		return nil, "", err
	}
	body, err := json.Marshal(map[string]any{
		"model":           defaultImageStudioModel,
		"prompt":          prompt,
		"n":               count,
		"size":            size,
		"response_format": "url",
	})
	if err != nil {
		return nil, "", err
	}
	return job, string(body), nil
}

func (s *ImageStudioService) CompleteJob(ctx context.Context, userID int64, jobID string, imageURLs []string, actualCost float64, errMsg string) (*ImageStudioGenerateResult, error) {
	status := ImageStudioJobStatusCompleted
	if errMsg != "" || len(imageURLs) == 0 {
		status = ImageStudioJobStatusFailed
	}
	var costPtr *float64
	if actualCost > 0 {
		costPtr = &actualCost
	}
	if err := s.repo.UpdateJobResult(ctx, jobID, status, costPtr, errMsg); err != nil {
		return nil, err
	}
	if status == ImageStudioJobStatusCompleted {
		if err := s.repo.InsertAssets(ctx, jobID, imageURLs); err != nil {
			return nil, err
		}
		if s.playService != nil {
			_ = s.playService.MarkQuestCompleted(ctx, userID, PlayQuestKeyImageGenerate)
		}
	}
	job, err := s.repo.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, err
	}
	out := &ImageStudioGenerateResult{Job: *job}
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
	return s.repo.ListJobs(ctx, userID, limit)
}

func (s *ImageStudioService) GetJob(ctx context.Context, userID int64, jobID string) (*ImageStudioJob, error) {
	return s.repo.GetJob(ctx, userID, jobID)
}

func (s *ImageStudioService) DeleteJob(ctx context.Context, userID int64, jobID string) error {
	return s.repo.DeleteJob(ctx, userID, jobID)
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
	out.HasCompletedJob = count > 0
	return out, nil
}

func (s *ImageStudioService) resolveAPIKey(ctx context.Context, userID, apiKeyID int64) (*APIKey, error) {
	if apiKeyID > 0 {
		key, err := s.apiKeyService.GetByID(ctx, apiKeyID)
		if err != nil {
			return nil, ErrImageStudioAPIKey
		}
		if key.UserID != userID {
			return nil, ErrImageStudioAPIKey
		}
		return key, nil
	}
	keys, _, err := s.apiKeyService.List(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 1}, APIKeyListFilters{})
	if err != nil || len(keys) == 0 {
		return nil, ErrImageStudioAPIKey
	}
	return &keys[0], nil
}

func buildImageStudioPrompt(tpl ImageStudioTemplate, req ImageStudioGenerateRequest) string {
	if req.ExpertPrompt != nil && strings.TrimSpace(*req.ExpertPrompt) != "" {
		return strings.TrimSpace(*req.ExpertPrompt)
	}
	subject := strings.TrimSpace(req.UserPrompt)
	if subject == "" {
		subject = "product"
	}
	prompt := strings.ReplaceAll(tpl.PromptTemplate, "{{subject}}", subject)
	if accent := strings.TrimSpace(req.AccentColor); accent != "" {
		prompt += ", accent color " + accent
	}
	return prompt
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
