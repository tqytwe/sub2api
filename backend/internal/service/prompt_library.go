package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const PromptContentNotice = "收录于极速蹬提示词库\n由极速蹬整理、翻译并完成模型适配"

type PromptStatus string

const (
	PromptStatusDraft         PromptStatus = "draft"
	PromptStatusPendingReview PromptStatus = "pending_review"
	PromptStatusPublished     PromptStatus = "published"
	PromptStatusOffline       PromptStatus = "offline"
)

type PromptBrand string

const (
	PromptBrandOriginal   PromptBrand = "original"
	PromptBrandAuthorized PromptBrand = "authorized"
	PromptBrandCurated    PromptBrand = "curated"
	PromptBrandCommunity  PromptBrand = "community"
)

func (b PromptBrand) Label() string {
	switch b {
	case PromptBrandOriginal:
		return "极速蹬原创"
	case PromptBrandAuthorized:
		return "极速蹬授权"
	case PromptBrandCommunity:
		return "极速蹬社区精选"
	default:
		return "极速蹬精选"
	}
}

type PromptProvenance string

const (
	PromptProvenanceInternal  PromptProvenance = "internal"
	PromptProvenanceExternal  PromptProvenance = "external"
	PromptProvenanceCommunity PromptProvenance = "community"
)

type PromptAuthorization string

const (
	PromptAuthorizationUnknown    PromptAuthorization = "unknown"
	PromptAuthorizationOriginal   PromptAuthorization = "original"
	PromptAuthorizationAuthorized PromptAuthorization = "authorized"
	PromptAuthorizationCurated    PromptAuthorization = "curated"
	PromptAuthorizationCommunity  PromptAuthorization = "community"
	PromptAuthorizationRejected   PromptAuthorization = "rejected"
)

type PromptReferenceRequirement string

const (
	PromptReferenceNone     PromptReferenceRequirement = "none"
	PromptReferenceOptional PromptReferenceRequirement = "optional"
	PromptReferenceRequired PromptReferenceRequirement = "required"
)

type PromptReviewDecision string

const (
	PromptReviewApprove PromptReviewDecision = "approve"
	PromptReviewReject  PromptReviewDecision = "reject"
)

type PromptImportStatus string

const (
	PromptImportStatusPendingReview PromptImportStatus = "pending_review"
	PromptImportStatusCompleted     PromptImportStatus = "completed"
	PromptImportStatusFailed        PromptImportStatus = "failed"
)

type PromptImportItemStatus string

const (
	PromptImportItemPendingReview PromptImportItemStatus = "pending_review"
	PromptImportItemApproved      PromptImportItemStatus = "approved"
	PromptImportItemRejected      PromptImportItemStatus = "rejected"
	PromptImportItemDuplicate     PromptImportItemStatus = "duplicate"
)

var (
	ErrPromptOriginalEvidenceRequired = errors.New("original prompt requires verified source evidence")
	ErrPromptApprovalRequired         = errors.New("approved review record required")
	ErrPromptChineseContentRequired   = errors.New("chinese title and description required")
	ErrPromptPublishContentIncomplete = errors.New("prompt publish content incomplete")
	ErrPromptRollbackVersionInvalid   = errors.New("rollback target must be an older version")
	ErrPromptVersionConflict          = errors.New("prompt version conflict")
)

var (
	promptOpenAISecretPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])sk-[a-z0-9_-]{16,}`)
	promptGitHubSecretPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])ghp_[a-z0-9]{20,}`)
	promptJWTSecretPattern    = regexp.MustCompile(`(?:^|[^A-Za-z0-9_-])eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)
)

type Prompt struct {
	ID                     int64                      `json:"id"`
	Status                 PromptStatus               `json:"status"`
	BrandType              PromptBrand                `json:"brand_type"`
	ProvenanceType         PromptProvenance           `json:"provenance_type"`
	AuthorizationStatus    PromptAuthorization        `json:"authorization_status"`
	SourceEvidenceVerified bool                       `json:"source_evidence_verified"`
	TitleZH                string                     `json:"title_zh"`
	DescriptionZH          string                     `json:"description_zh"`
	Purpose                string                     `json:"purpose"`
	Style                  string                     `json:"style"`
	Subject                string                     `json:"subject"`
	Featured               bool                       `json:"featured"`
	CurrentVersion         int                        `json:"current_version"`
	ExpectedVersion        int                        `json:"expected_version,omitempty"`
	PublishedVersion       int                        `json:"published_version"`
	PromptText             string                     `json:"prompt_text"`
	Variables              map[string]any             `json:"variables"`
	Models                 []string                   `json:"models"`
	Sizes                  []string                   `json:"sizes"`
	ReferenceRequirement   PromptReferenceRequirement `json:"reference_requirement"`
	ReferenceInstructions  string                     `json:"reference_instructions,omitempty"`
	RequiresReference      bool                       `json:"requires_reference"`
	PublicAttributionNote  string                     `json:"public_attribution_note,omitempty"`
	UseCount               int64                      `json:"use_count"`
	FavoriteCount          int64                      `json:"favorite_count"`
	Favorited              bool                       `json:"favorited"`
	CategoryIDs            []int64                    `json:"category_ids"`
	Media                  []PromptMedia              `json:"media"`
	Sources                []PromptSource             `json:"sources,omitempty"`
	Reviews                []PromptReviewRecord       `json:"reviews,omitempty"`
	PublishedAt            *time.Time                 `json:"published_at,omitempty"`
	CreatedAt              time.Time                  `json:"created_at"`
	UpdatedAt              time.Time                  `json:"updated_at"`
}

type PublicPrompt struct {
	ID                    int64                      `json:"id"`
	Title                 string                     `json:"title"`
	Description           string                     `json:"description"`
	Purpose               string                     `json:"purpose"`
	Style                 string                     `json:"style"`
	Subject               string                     `json:"subject"`
	Featured              bool                       `json:"featured"`
	Version               int                        `json:"version"`
	PromptText            string                     `json:"prompt_text,omitempty"`
	Variables             map[string]any             `json:"variables,omitempty"`
	Models                []string                   `json:"models"`
	Sizes                 []string                   `json:"sizes"`
	ReferenceRequirement  PromptReferenceRequirement `json:"reference_requirement"`
	ReferenceInstructions string                     `json:"reference_instructions,omitempty"`
	RequiresReference     bool                       `json:"requires_reference"`
	BrandLabel            string                     `json:"brand_label"`
	ContentNotice         string                     `json:"content_notice"`
	PublicAttributionNote string                     `json:"public_attribution_note,omitempty"`
	UseCount              int64                      `json:"use_count"`
	FavoriteCount         int64                      `json:"favorite_count"`
	Favorited             bool                       `json:"favorited"`
	Media                 []PromptMedia              `json:"media"`
	PublishedAt           *time.Time                 `json:"published_at,omitempty"`
}

type PromptUseResult struct {
	PromptID              int64                      `json:"prompt_id"`
	Version               int                        `json:"version"`
	Title                 string                     `json:"title"`
	PromptText            string                     `json:"prompt_text"`
	Variables             map[string]any             `json:"variables"`
	Models                []string                   `json:"models"`
	Sizes                 []string                   `json:"sizes"`
	ReferenceRequirement  PromptReferenceRequirement `json:"reference_requirement"`
	ReferenceInstructions string                     `json:"reference_instructions,omitempty"`
	RequiresReference     bool                       `json:"requires_reference"`
}

type PromptCategory struct {
	ID            int64     `json:"id"`
	Slug          string    `json:"slug"`
	NameZH        string    `json:"name_zh"`
	DescriptionZH string    `json:"description_zh"`
	Dimension     string    `json:"dimension"`
	SortOrder     int       `json:"sort_order"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type PromptMedia struct {
	ID        int64  `json:"id"`
	MediaType string `json:"media_type"`
	URL       string `json:"url"`
	AltZH     string `json:"alt_zh"`
	SortOrder int    `json:"sort_order"`
}

type PromptSource struct {
	ID                  int64               `json:"id"`
	PromptID            int64               `json:"prompt_id"`
	Version             int                 `json:"version"`
	SourceKey           string              `json:"source_key"`
	ExternalID          string              `json:"external_id"`
	SourceURL           string              `json:"source_url"`
	OriginalAuthor      string              `json:"original_author"`
	SourcePayload       map[string]any      `json:"source_payload"`
	Evidence            map[string]any      `json:"evidence"`
	AuthorizationStatus PromptAuthorization `json:"authorization_status"`
	EvidenceVerified    bool                `json:"evidence_verified"`
	CreatedAt           time.Time           `json:"created_at"`
}

type PromptReviewRecord struct {
	ID         int64                `json:"id"`
	PromptID   int64                `json:"prompt_id"`
	Version    int                  `json:"version"`
	Decision   PromptReviewDecision `json:"decision"`
	Note       string               `json:"note"`
	ReviewerID int64                `json:"reviewer_id"`
	CreatedAt  time.Time            `json:"created_at"`
}

type PromptReport struct {
	ID           int64      `json:"id"`
	PromptID     int64      `json:"prompt_id"`
	PromptTitle  string     `json:"prompt_title,omitempty"`
	ReporterID   *int64     `json:"reporter_id,omitempty"`
	ReporterName string     `json:"reporter_name,omitempty"`
	Reason       string     `json:"reason"`
	Detail       string     `json:"detail"`
	Status       string     `json:"status"`
	Resolution   string     `json:"resolution"`
	ResolvedBy   *int64     `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type PromptListFilter struct {
	Query                string
	Purpose              string
	Style                string
	Subject              string
	Model                string
	Size                 string
	ReferenceRequirement PromptReferenceRequirement
	Featured             *bool
	FavoritedOnly        bool
	Status               PromptStatus
	Sort                 string
	Pagination           pagination.PaginationParams
}

type PromptImportItemInput struct {
	ExternalID            string                     `json:"external_id"`
	NormalizedHash        string                     `json:"normalized_hash"`
	TitleZH               string                     `json:"title_zh"`
	DescriptionZH         string                     `json:"description_zh"`
	PromptText            string                     `json:"prompt_text"`
	Variables             map[string]any             `json:"variables"`
	Models                []string                   `json:"models"`
	Sizes                 []string                   `json:"sizes"`
	ReferenceRequirement  PromptReferenceRequirement `json:"reference_requirement"`
	ReferenceInstructions string                     `json:"reference_instructions"`
	RequiresReference     bool                       `json:"requires_reference"`
	Media                 []PromptMedia              `json:"media"`
	Purpose               string                     `json:"purpose"`
	Style                 string                     `json:"style"`
	Subject               string                     `json:"subject"`
	BrandType             PromptBrand                `json:"brand_type"`
	AuthorizationStatus   PromptAuthorization        `json:"authorization_status"`
	EvidenceVerified      bool                       `json:"evidence_verified"`
	SourceURL             string                     `json:"source_url"`
	OriginalAuthor        string                     `json:"original_author"`
	SourcePayload         map[string]any             `json:"source_payload"`
	Evidence              map[string]any             `json:"evidence"`
}

type PromptImportJobInput struct {
	SourceKey  string                  `json:"source_key"`
	Status     PromptImportStatus      `json:"status,omitempty"`
	RawPayload map[string]any          `json:"raw_payload"`
	Items      []PromptImportItemInput `json:"items"`
}

type PromptImportJob struct {
	ID         int64              `json:"id"`
	SourceKey  string             `json:"source_key"`
	Status     PromptImportStatus `json:"status"`
	RawPayload map[string]any     `json:"raw_payload,omitempty"`
	ItemCount  int                `json:"item_count"`
	CreatedBy  int64              `json:"created_by"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
}

type PromptImportItem struct {
	ID                  int64                  `json:"id"`
	JobID               int64                  `json:"job_id"`
	SourceKey           string                 `json:"source_key"`
	ExternalID          string                 `json:"external_id"`
	NormalizedHash      string                 `json:"normalized_hash"`
	Status              PromptImportItemStatus `json:"status"`
	NormalizedPayload   map[string]any         `json:"normalized_payload"`
	AuthorizationStatus PromptAuthorization    `json:"authorization_status"`
	PromptID            *int64                 `json:"prompt_id,omitempty"`
	RejectionReason     string                 `json:"rejection_reason,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
}

type PromptLibraryRepository interface {
	GetPrompt(ctx context.Context, id int64, userID *int64, publicOnly bool) (*Prompt, error)
	SavePrompt(ctx context.Context, prompt *Prompt, actorID int64) (*Prompt, error)
	ListPromptSources(ctx context.Context, promptID int64) ([]PromptSource, error)
	ListPromptReviews(ctx context.Context, promptID int64, version int) ([]PromptReviewRecord, error)
	SetPromptStatus(ctx context.Context, id int64, version int, status PromptStatus, actorID int64) (*Prompt, error)
	RollbackPrompt(ctx context.Context, id int64, version int, actorID int64) (*Prompt, error)
	CreateImportJob(ctx context.Context, input PromptImportJobInput, actorID int64) (*PromptImportJob, error)
}

type promptPublicRepository interface {
	ListPrompts(ctx context.Context, filter PromptListFilter, userID *int64, publicOnly bool) ([]Prompt, *pagination.PaginationResult, error)
	ListCategories(ctx context.Context, publicOnly bool) ([]PromptCategory, error)
	SetFavorite(ctx context.Context, promptID, userID int64, favorite bool) (bool, error)
	UsePrompt(ctx context.Context, promptID, userID int64) (*Prompt, error)
	CreateReport(ctx context.Context, report PromptReport) (*PromptReport, error)
}

type promptAdminRepository interface {
	SubmitPromptReview(ctx context.Context, promptID int64, actorID int64) (*Prompt, error)
	AddPromptReview(ctx context.Context, record PromptReviewRecord) error
	ApprovePromptVersion(ctx context.Context, promptID int64, version int, actorID int64, note string) (*Prompt, error)
	SaveCategory(ctx context.Context, category *PromptCategory) (*PromptCategory, error)
	DeleteCategory(ctx context.Context, id int64) error
	ListImportJobs(ctx context.Context, pagination pagination.PaginationParams) ([]PromptImportJob, *pagination.PaginationResult, error)
	GetImportJob(ctx context.Context, id int64) (*PromptImportJob, error)
	ListImportItems(ctx context.Context, filter PromptImportItemListFilter) ([]PromptImportItem, *pagination.PaginationResult, error)
	ReviewImportItem(ctx context.Context, id, actorID int64, approve bool, reason string) (*PromptImportItem, error)
	ListReports(ctx context.Context, filter PromptReportListFilter) ([]PromptReport, *pagination.PaginationResult, error)
	ResolveReport(ctx context.Context, id, actorID int64, status, resolution string) (*PromptReport, error)
}

type PromptImportItemListFilter struct {
	JobID      int64
	Status     PromptImportItemStatus
	Pagination pagination.PaginationParams
}

type PromptReportListFilter struct {
	Status     string
	Pagination pagination.PaginationParams
}

type PromptLibraryService struct {
	repo PromptLibraryRepository
}

func NewPromptLibraryService(repo PromptLibraryRepository) *PromptLibraryService {
	return &PromptLibraryService{repo: repo}
}

func ValidatePromptProvenance(prompt Prompt) error {
	if prompt.BrandType != PromptBrandOriginal {
		return nil
	}
	if !prompt.SourceEvidenceVerified ||
		prompt.AuthorizationStatus != PromptAuthorizationOriginal {
		return ErrPromptOriginalEvidenceRequired
	}
	return nil
}

func NormalizeImportedBrand(requested PromptBrand, authorization PromptAuthorization, verified bool) PromptBrand {
	if !verified {
		return PromptBrandCurated
	}
	switch {
	case requested == PromptBrandOriginal && authorization == PromptAuthorizationOriginal:
		return PromptBrandOriginal
	case requested == PromptBrandAuthorized && authorization == PromptAuthorizationAuthorized:
		return PromptBrandAuthorized
	case requested == PromptBrandCommunity && authorization == PromptAuthorizationCommunity:
		return PromptBrandCommunity
	default:
		return PromptBrandCurated
	}
}

func (s *PromptLibraryService) ListPublic(ctx context.Context, filter PromptListFilter, userID *int64) ([]PublicPrompt, *pagination.PaginationResult, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, nil, errors.New("prompt public repository unavailable")
	}
	filter.Status = PromptStatusPublished
	rows, page, err := repo.ListPrompts(ctx, filter, userID, true)
	if err != nil {
		return nil, nil, err
	}
	out := make([]PublicPrompt, 0, len(rows))
	for i := range rows {
		out = append(out, toPublicPrompt(&rows[i], false))
	}
	return out, page, nil
}

func (s *PromptLibraryService) GetPublic(ctx context.Context, id int64, userID *int64) (*PublicPrompt, error) {
	prompt, err := s.repo.GetPrompt(ctx, id, userID, true)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	out := toPublicPrompt(prompt, true)
	return &out, nil
}

func (s *PromptLibraryService) ListCategories(ctx context.Context) ([]PromptCategory, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, errors.New("prompt public repository unavailable")
	}
	return repo.ListCategories(ctx, true)
}

func (s *PromptLibraryService) SetFavorite(ctx context.Context, promptID, userID int64, favorite bool) (bool, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return false, errors.New("prompt public repository unavailable")
	}
	return repo.SetFavorite(ctx, promptID, userID, favorite)
}

func (s *PromptLibraryService) UsePrompt(ctx context.Context, promptID, userID int64) (*PromptUseResult, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, errors.New("prompt public repository unavailable")
	}
	prompt, err := repo.UsePrompt(ctx, promptID, userID)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	return &PromptUseResult{
		PromptID:              prompt.ID,
		Version:               prompt.PublishedVersion,
		Title:                 prompt.TitleZH,
		PromptText:            prompt.PromptText,
		Variables:             prompt.Variables,
		Models:                prompt.Models,
		Sizes:                 prompt.Sizes,
		ReferenceRequirement:  prompt.ReferenceRequirement,
		ReferenceInstructions: prompt.ReferenceInstructions,
		RequiresReference:     prompt.RequiresReference,
	}, nil
}

func (s *PromptLibraryService) ReportPrompt(
	ctx context.Context,
	promptID, userID int64,
	reason, detail string,
) (*PromptReport, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, errors.New("prompt public repository unavailable")
	}
	reason = strings.TrimSpace(reason)
	detail = strings.TrimSpace(detail)
	if reason == "" {
		return nil, apperrors.BadRequest("PROMPT_REPORT_REASON_REQUIRED", "report reason is required")
	}
	if len([]rune(reason)) > 96 || len([]rune(detail)) > 2000 {
		return nil, apperrors.BadRequest("PROMPT_REPORT_TOO_LONG", "report content is too long")
	}
	return repo.CreateReport(ctx, PromptReport{
		PromptID:   promptID,
		ReporterID: &userID,
		Reason:     reason,
		Detail:     detail,
		Status:     "open",
	})
}

func (s *PromptLibraryService) SavePrompt(ctx context.Context, prompt *Prompt, actorID int64) (*Prompt, error) {
	if prompt == nil {
		return nil, apperrors.BadRequest("PROMPT_REQUIRED", "prompt is required")
	}
	if err := ValidatePromptProvenance(*prompt); err != nil {
		return nil, apperrors.BadRequest("PROMPT_ORIGINAL_EVIDENCE_REQUIRED", err.Error())
	}
	if !hasValidPromptSource(prompt.BrandType, prompt.CurrentVersion, prompt.Sources) {
		return nil, apperrors.BadRequest(
			"PROMPT_SOURCE_EVIDENCE_REQUIRED",
			"source evidence and authorization are required",
		)
	}
	saved, err := s.repo.SavePrompt(ctx, prompt, actorID)
	if errors.Is(err, ErrPromptVersionConflict) {
		return nil, apperrors.Conflict(
			"PROMPT_VERSION_CONFLICT",
			"prompt was updated by another request",
		).WithCause(err)
	}
	return saved, err
}

func (s *PromptLibraryService) SubmitForReview(ctx context.Context, id, actorID int64) (*Prompt, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt admin repository unavailable")
	}
	return repo.SubmitPromptReview(ctx, id, actorID)
}

func (s *PromptLibraryService) ApproveAndPublish(ctx context.Context, id, actorID int64) (*Prompt, error) {
	prompt, err := s.repo.GetPrompt(ctx, id, nil, false)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	if err := validatePublishContent(prompt); err != nil {
		return nil, err
	}
	sources, err := s.repo.ListPromptSources(ctx, id)
	if err != nil {
		return nil, err
	}
	if !validPublishSource(prompt, sources) {
		return nil, apperrors.BadRequest("PROMPT_SOURCE_EVIDENCE_REQUIRED", "source evidence and authorization are required")
	}
	reviews, err := s.repo.ListPromptReviews(ctx, id, prompt.CurrentVersion)
	if err != nil {
		return nil, err
	}
	approved := false
	for _, review := range reviews {
		if review.Decision == PromptReviewApprove {
			approved = true
			break
		}
	}
	if !approved {
		return nil, apperrors.BadRequest(
			"PROMPT_APPROVAL_REQUIRED",
			ErrPromptApprovalRequired.Error(),
		).WithCause(ErrPromptApprovalRequired)
	}
	return s.repo.SetPromptStatus(ctx, id, prompt.CurrentVersion, PromptStatusPublished, actorID)
}

func (s *PromptLibraryService) ReviewAndPublish(ctx context.Context, id, actorID int64, note string) (*Prompt, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt admin repository unavailable")
	}
	prompt, err := s.repo.GetPrompt(ctx, id, nil, false)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	if err := validatePublishContent(prompt); err != nil {
		return nil, err
	}
	sources, err := s.repo.ListPromptSources(ctx, id)
	if err != nil {
		return nil, err
	}
	if !validPublishSource(prompt, sources) {
		return nil, apperrors.BadRequest("PROMPT_SOURCE_EVIDENCE_REQUIRED", "source evidence and authorization are required")
	}
	return repo.ApprovePromptVersion(ctx, id, prompt.CurrentVersion, actorID, note)
}

func (s *PromptLibraryService) Offline(ctx context.Context, id, actorID int64) (*Prompt, error) {
	prompt, err := s.repo.GetPrompt(ctx, id, nil, false)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	return s.repo.SetPromptStatus(ctx, id, prompt.CurrentVersion, PromptStatusOffline, actorID)
}

func (s *PromptLibraryService) RollbackVersion(ctx context.Context, id int64, version int, actorID int64) (*Prompt, error) {
	prompt, err := s.repo.GetPrompt(ctx, id, nil, false)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	if version <= 0 || version >= prompt.CurrentVersion {
		return nil, apperrors.BadRequest(
			"PROMPT_ROLLBACK_VERSION_INVALID",
			ErrPromptRollbackVersionInvalid.Error(),
		).WithCause(ErrPromptRollbackVersionInvalid)
	}
	return s.repo.RollbackPrompt(ctx, id, version, actorID)
}

func (s *PromptLibraryService) CreateImportJob(ctx context.Context, input PromptImportJobInput, actorID int64) (*PromptImportJob, error) {
	input.SourceKey = strings.TrimSpace(input.SourceKey)
	if input.SourceKey == "" || len(input.Items) == 0 {
		return nil, apperrors.BadRequest(
			"PROMPT_IMPORT_INVALID",
			"source key and at least one item are required",
		)
	}
	input.Status = PromptImportStatusPendingReview
	if containsPromptImportSecret(input.RawPayload) {
		return nil, apperrors.BadRequest("PROMPT_IMPORT_SECRET_REJECTED", "导入内容包含疑似密钥、令牌或 Cookie 字段")
	}
	for i := range input.Items {
		if containsPromptImportSecret(input.Items[i]) {
			return nil, apperrors.BadRequest("PROMPT_IMPORT_SECRET_REJECTED", "导入内容包含疑似密钥、令牌或 Cookie 字段")
		}
		input.Items[i].NormalizedHash = promptImportContentHash(input.Items[i])
		input.Items[i].ExternalID = strings.TrimSpace(input.Items[i].ExternalID)
		if input.Items[i].ExternalID == "" {
			input.Items[i].ExternalID = input.Items[i].NormalizedHash
		}
		input.Items[i].BrandType = NormalizeImportedBrand(
			input.Items[i].BrandType,
			input.Items[i].AuthorizationStatus,
			input.Items[i].EvidenceVerified,
		)
	}
	return s.repo.CreateImportJob(ctx, input, actorID)
}

func containsPromptImportSecret(value any) bool {
	return containsPromptImportSecretValue(reflect.ValueOf(value))
}

func containsPromptImportSecretValue(value reflect.Value) bool {
	if !value.IsValid() {
		return false
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return false
		}
		return containsPromptImportSecretValue(value.Elem())
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if containsPromptImportSecretValue(value.Field(i)) {
				return true
			}
		}
	case reflect.Map:
		if value.IsNil() {
			return false
		}
		iter := value.MapRange()
		for iter.Next() {
			key := iter.Key()
			if key.Kind() == reflect.String && isPromptSecretKey(key.String()) {
				return true
			}
			if containsPromptImportSecretValue(iter.Value()) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			if containsPromptImportSecretValue(value.Index(i)) {
				return true
			}
		}
	case reflect.String:
		return isPromptSecretString(value.String())
	}
	return false
}

func isPromptSecretKey(key string) bool {
	var normalized strings.Builder
	for _, r := range strings.ToLower(key) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			_, _ = normalized.WriteRune(r)
		}
	}
	switch normalized.String() {
	case "apikey", "xapikey", "accesstoken", "refreshtoken", "authorization",
		"authorizationheader", "proxyauthorization", "cookie", "cookies", "setcookie",
		"password", "passwd", "privatekey", "clientsecret", "secretkey",
		"sessiontoken", "sessioncookie":
		return true
	default:
		return false
	}
}

func isPromptSecretString(value string) bool {
	trimmed := strings.TrimSpace(value)
	normalized := strings.ToLower(trimmed)
	return strings.HasPrefix(normalized, "bearer ") ||
		strings.HasPrefix(normalized, "cookie:") ||
		strings.HasPrefix(normalized, "set-cookie:") ||
		(strings.Contains(normalized, "-----begin ") && strings.Contains(normalized, "private key-----")) ||
		promptOpenAISecretPattern.MatchString(trimmed) ||
		promptGitHubSecretPattern.MatchString(trimmed) ||
		promptJWTSecretPattern.MatchString(trimmed)
}

func promptImportContentHash(input PromptImportItemInput) string {
	normalized, _ := json.Marshal(struct {
		TitleZH               string                     `json:"title_zh"`
		DescriptionZH         string                     `json:"description_zh"`
		PromptText            string                     `json:"prompt_text"`
		Variables             map[string]any             `json:"variables"`
		Models                []string                   `json:"models"`
		Sizes                 []string                   `json:"sizes"`
		ReferenceRequirement  PromptReferenceRequirement `json:"reference_requirement"`
		ReferenceInstructions string                     `json:"reference_instructions"`
		RequiresReference     bool                       `json:"requires_reference"`
		Media                 []PromptMedia              `json:"media"`
		Purpose               string                     `json:"purpose"`
		Style                 string                     `json:"style"`
		Subject               string                     `json:"subject"`
	}{
		TitleZH:               strings.TrimSpace(input.TitleZH),
		DescriptionZH:         strings.TrimSpace(input.DescriptionZH),
		PromptText:            strings.TrimSpace(input.PromptText),
		Variables:             input.Variables,
		Models:                input.Models,
		Sizes:                 input.Sizes,
		ReferenceRequirement:  input.ReferenceRequirement,
		ReferenceInstructions: strings.TrimSpace(input.ReferenceInstructions),
		RequiresReference:     input.RequiresReference,
		Media:                 input.Media,
		Purpose:               strings.TrimSpace(input.Purpose),
		Style:                 strings.TrimSpace(input.Style),
		Subject:               strings.TrimSpace(input.Subject),
	})
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func (s *PromptLibraryService) ListAdmin(
	ctx context.Context,
	filter PromptListFilter,
) ([]Prompt, *pagination.PaginationResult, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, nil, errors.New("prompt admin list repository unavailable")
	}
	return repo.ListPrompts(ctx, filter, nil, false)
}

func (s *PromptLibraryService) GetAdmin(ctx context.Context, id int64) (*Prompt, error) {
	prompt, err := s.repo.GetPrompt(ctx, id, nil, false)
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, apperrors.NotFound("PROMPT_NOT_FOUND", "prompt not found")
	}
	return prompt, nil
}

func (s *PromptLibraryService) ListAdminCategories(ctx context.Context) ([]PromptCategory, error) {
	repo, ok := s.repo.(promptPublicRepository)
	if !ok {
		return nil, errors.New("prompt category repository unavailable")
	}
	return repo.ListCategories(ctx, false)
}

func (s *PromptLibraryService) SaveCategory(ctx context.Context, category *PromptCategory) (*PromptCategory, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt category repository unavailable")
	}
	if category == nil || strings.TrimSpace(category.Slug) == "" || strings.TrimSpace(category.NameZH) == "" {
		return nil, apperrors.BadRequest("PROMPT_CATEGORY_INVALID", "category slug and Chinese name are required")
	}
	return repo.SaveCategory(ctx, category)
}

func (s *PromptLibraryService) DeleteCategory(ctx context.Context, id int64) error {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return errors.New("prompt category repository unavailable")
	}
	return repo.DeleteCategory(ctx, id)
}

func (s *PromptLibraryService) GetImportJob(ctx context.Context, id int64) (*PromptImportJob, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt import repository unavailable")
	}
	job, err := repo.GetImportJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, apperrors.NotFound("PROMPT_IMPORT_JOB_NOT_FOUND", "prompt import job not found")
	}
	return job, nil
}

func (s *PromptLibraryService) ListImportJobs(
	ctx context.Context,
	params pagination.PaginationParams,
) ([]PromptImportJob, *pagination.PaginationResult, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, nil, errors.New("prompt import repository unavailable")
	}
	return repo.ListImportJobs(ctx, params)
}

func (s *PromptLibraryService) ListImportItems(
	ctx context.Context,
	filter PromptImportItemListFilter,
) ([]PromptImportItem, *pagination.PaginationResult, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, nil, errors.New("prompt import repository unavailable")
	}
	return repo.ListImportItems(ctx, filter)
}

func (s *PromptLibraryService) ReviewImportItem(
	ctx context.Context,
	id, actorID int64,
	approve bool,
	reason string,
) (*PromptImportItem, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt import repository unavailable")
	}
	if !approve && strings.TrimSpace(reason) == "" {
		return nil, apperrors.BadRequest("PROMPT_IMPORT_REJECTION_REASON_REQUIRED", "rejection reason is required")
	}
	return repo.ReviewImportItem(ctx, id, actorID, approve, reason)
}

func (s *PromptLibraryService) ListReports(
	ctx context.Context,
	filter PromptReportListFilter,
) ([]PromptReport, *pagination.PaginationResult, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, nil, errors.New("prompt report repository unavailable")
	}
	return repo.ListReports(ctx, filter)
}

func (s *PromptLibraryService) ResolveReport(
	ctx context.Context,
	id, actorID int64,
	status, resolution string,
) (*PromptReport, error) {
	repo, ok := s.repo.(promptAdminRepository)
	if !ok {
		return nil, errors.New("prompt report repository unavailable")
	}
	switch status {
	case "resolved", "dismissed":
	default:
		return nil, apperrors.BadRequest("PROMPT_REPORT_STATUS_INVALID", "report status must be resolved or dismissed")
	}
	if strings.TrimSpace(resolution) == "" {
		return nil, apperrors.BadRequest("PROMPT_REPORT_RESOLUTION_REQUIRED", "resolution is required")
	}
	return repo.ResolveReport(ctx, id, actorID, status, resolution)
}

func toPublicPrompt(prompt *Prompt, includeText bool) PublicPrompt {
	version := prompt.PublishedVersion
	out := PublicPrompt{
		ID:                    prompt.ID,
		Title:                 prompt.TitleZH,
		Description:           prompt.DescriptionZH,
		Purpose:               prompt.Purpose,
		Style:                 prompt.Style,
		Subject:               prompt.Subject,
		Featured:              prompt.Featured,
		Version:               version,
		Models:                prompt.Models,
		Sizes:                 prompt.Sizes,
		ReferenceRequirement:  prompt.ReferenceRequirement,
		ReferenceInstructions: prompt.ReferenceInstructions,
		RequiresReference:     prompt.RequiresReference,
		BrandLabel:            prompt.BrandType.Label(),
		ContentNotice:         PromptContentNotice,
		PublicAttributionNote: prompt.PublicAttributionNote,
		UseCount:              prompt.UseCount,
		FavoriteCount:         prompt.FavoriteCount,
		Favorited:             prompt.Favorited,
		Media:                 prompt.Media,
		PublishedAt:           prompt.PublishedAt,
	}
	if includeText {
		out.PromptText = prompt.PromptText
		out.Variables = prompt.Variables
	}
	return out
}

func validatePublishContent(prompt *Prompt) error {
	if !containsHan(prompt.TitleZH) || !containsHan(prompt.DescriptionZH) {
		return apperrors.BadRequest(
			"PROMPT_CHINESE_CONTENT_REQUIRED",
			ErrPromptChineseContentRequired.Error(),
		).WithCause(ErrPromptChineseContentRequired)
	}
	if strings.TrimSpace(prompt.PromptText) == "" || len(prompt.Models) == 0 {
		return apperrors.BadRequest(
			"PROMPT_PUBLISH_CONTENT_INCOMPLETE",
			ErrPromptPublishContentIncomplete.Error(),
		).WithCause(ErrPromptPublishContentIncomplete)
	}
	return nil
}

func validPublishSource(prompt *Prompt, sources []PromptSource) bool {
	return hasValidPromptSource(prompt.BrandType, prompt.CurrentVersion, sources)
}

func hasValidPromptSource(brand PromptBrand, version int, sources []PromptSource) bool {
	for _, source := range sources {
		if source.Version != version ||
			strings.TrimSpace(source.SourceKey) == "" ||
			!source.EvidenceVerified ||
			!validPromptEvidence(source.Evidence) ||
			source.AuthorizationStatus != requiredPromptAuthorization(brand) {
			continue
		}
		if brand == PromptBrandOriginal &&
			(strings.TrimSpace(source.ExternalID) == "" ||
				strings.TrimSpace(source.OriginalAuthor) == "" ||
				promptEvidenceString(source.Evidence, "proof_type") == "") {
			continue
		}
		return true
	}
	return false
}

func requiredPromptAuthorization(brand PromptBrand) PromptAuthorization {
	switch brand {
	case PromptBrandOriginal:
		return PromptAuthorizationOriginal
	case PromptBrandAuthorized:
		return PromptAuthorizationAuthorized
	case PromptBrandCurated:
		return PromptAuthorizationCurated
	case PromptBrandCommunity:
		return PromptAuthorizationCommunity
	default:
		return ""
	}
}

func validPromptEvidence(evidence map[string]any) bool {
	if promptEvidenceString(evidence, "summary") == "" {
		return false
	}
	switch capturedAt := evidence["captured_at"].(type) {
	case string:
		_, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(capturedAt))
		return err == nil
	case time.Time:
		return !capturedAt.IsZero()
	default:
		return false
	}
}

func promptEvidenceString(evidence map[string]any, key string) string {
	value, ok := evidence[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func containsHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
