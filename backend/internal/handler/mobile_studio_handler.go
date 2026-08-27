package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	mobileStudioDefaultPageSize = 20
	mobileStudioMaxPageSize     = 100
	mobileStudioMaxDocumentSize = 1 << 20
)

var (
	errMobileStudioNotFound     = errors.New("mobile studio project not found")
	errMobileStudioConflict     = errors.New("mobile studio version conflict")
	errMobileStudioInvalidAsset = errors.New("mobile studio asset is unavailable")
)

// mobileStudioProject is the project directory analogue. Every other studio
// record is reached through this owner-scoped root; no project identity is
// written into generic asset metadata.
type mobileStudioProject struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	AspectRatio  string     `json:"aspect_ratio"`
	Language     string     `json:"language"`
	Status       string     `json:"status"`
	CoverAssetID *string    `json:"cover_asset_id,omitempty"`
	Version      int        `json:"version"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	ArchivedAt   *time.Time `json:"archived_at,omitempty"`
}

type mobileStudioEpisode struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	StableID  string    `json:"stable_id"`
	Title     string    `json:"title"`
	Sequence  int       `json:"sequence"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type mobileStudioDocument struct {
	ID           string          `json:"id"`
	ProjectID    string          `json:"project_id"`
	EpisodeID    *string         `json:"episode_id,omitempty"`
	DocumentType string          `json:"document_type"`
	Content      json.RawMessage `json:"content"`
	Version      int             `json:"version"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type mobileStudioAssetLink struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	EpisodeID *string         `json:"episode_id,omitempty"`
	AssetID   string          `json:"asset_id"`
	LinkType  string          `json:"link_type"`
	StableRef *string         `json:"stable_ref,omitempty"`
	Position  *int            `json:"position,omitempty"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type mobileStudioProjectInput struct {
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	AspectRatio  string  `json:"aspect_ratio"`
	Language     string  `json:"language"`
	Status       string  `json:"status"`
	CoverAssetID *string `json:"cover_asset_id"`
	Version      int     `json:"version"`
}

type mobileStudioEpisodeInput struct {
	StableID string `json:"stable_id"`
	Title    string `json:"title"`
	Sequence int    `json:"sequence"`
	Status   string `json:"status"`
}

type mobileStudioDocumentInput struct {
	EpisodeID       *string         `json:"episode_id"`
	Content         json.RawMessage `json:"content"`
	ExpectedVersion int             `json:"expected_version"`
}

type mobileStudioAssetLinkInput struct {
	EpisodeID *string         `json:"episode_id"`
	AssetID   string          `json:"asset_id"`
	LinkType  string          `json:"link_type"`
	StableRef *string         `json:"stable_ref"`
	Position  *int            `json:"position"`
	Metadata  json.RawMessage `json:"metadata"`
}

type mobileStudioListFilter struct{ Page, PageSize int }

type mobileStudioStore interface {
	CreateProject(context.Context, int64, mobileStudioProjectInput) (*mobileStudioProject, error)
	ListProjects(context.Context, int64, mobileStudioListFilter) ([]mobileStudioProject, int64, error)
	GetProject(context.Context, int64, string) (*mobileStudioProject, error)
	UpdateProject(context.Context, int64, string, mobileStudioProjectInput) (*mobileStudioProject, error)
	ArchiveProject(context.Context, int64, string) error
	ListEpisodes(context.Context, int64, string) ([]mobileStudioEpisode, error)
	CreateEpisode(context.Context, int64, string, mobileStudioEpisodeInput) (*mobileStudioEpisode, error)
	UpdateEpisode(context.Context, int64, string, string, mobileStudioEpisodeInput) (*mobileStudioEpisode, error)
	ArchiveEpisode(context.Context, int64, string, string) error
	ListDocuments(context.Context, int64, string, *string) ([]mobileStudioDocument, error)
	PutDocument(context.Context, int64, string, string, mobileStudioDocumentInput) (*mobileStudioDocument, error)
	ListAssetLinks(context.Context, int64, string, *string) ([]mobileStudioAssetLink, error)
	LinkAsset(context.Context, int64, string, mobileStudioAssetLinkInput) (*mobileStudioAssetLink, error)
	DeleteAssetLink(context.Context, int64, string, string) error
}

type MobileStudioHandler struct{ store mobileStudioStore }

func NewMobileStudioHandler(db *sql.DB) *MobileStudioHandler {
	return &MobileStudioHandler{store: &sqlMobileStudioStore{db: db}}
}

func newMobileStudioHandlerWithStore(store mobileStudioStore) *MobileStudioHandler {
	return &MobileStudioHandler{store: store}
}

func (h *MobileStudioHandler) ListProjects(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	filter, err := parseMobileStudioListFilter(c)
	if err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	items, total, err := h.store.ListProjects(c, userID, filter)
	if err != nil {
		mobileStudioInternal(c, err)
		return
	}
	mobileStudioPrivate(c)
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *MobileStudioHandler) CreateProject(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	var input mobileStudioProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("项目参数不正确"))
		return
	}
	if err := normalizeMobileStudioProjectInput(&input, true); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.CreateProject(c, userID, input)
	if err != nil {
		mobileStudioInternal(c, err)
		return
	}
	mobileStudioPrivate(c)
	response.Created(c, item)
}

func (h *MobileStudioHandler) GetProject(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	id, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	item, err := h.store.GetProject(c, userID, id)
	if errors.Is(err, errMobileStudioNotFound) {
		response.NotFound(c, "项目不存在")
		return
	}
	if err != nil {
		mobileStudioInternal(c, err)
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, item)
}

func (h *MobileStudioHandler) UpdateProject(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	id, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	var input mobileStudioProjectInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("项目参数不正确"))
		return
	}
	if err := normalizeMobileStudioProjectInput(&input, false); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.UpdateProject(c, userID, id, input)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, item)
}

func (h *MobileStudioHandler) ArchiveProject(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	id, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	if mobileStudioWriteError(c, h.store.ArchiveProject(c, userID, id)) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"archived": true, "id": id})
}

func (h *MobileStudioHandler) ListEpisodes(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.store.ListEpisodes(c, userID, projectID)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"items": items})
}

func (h *MobileStudioHandler) CreateEpisode(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	var input mobileStudioEpisodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("分集参数不正确"))
		return
	}
	if err := normalizeMobileStudioEpisodeInput(&input); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.CreateEpisode(c, userID, projectID, input)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Created(c, item)
}

func (h *MobileStudioHandler) UpdateEpisode(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	episodeID, ok := mobileStudioUUIDParam(c, "episodeID")
	if !ok {
		return
	}
	var input mobileStudioEpisodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("分集参数不正确"))
		return
	}
	if err := normalizeMobileStudioEpisodeInput(&input); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.UpdateEpisode(c, userID, projectID, episodeID, input)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, item)
}

func (h *MobileStudioHandler) ArchiveEpisode(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	episodeID, ok := mobileStudioUUIDParam(c, "episodeID")
	if !ok {
		return
	}
	if mobileStudioWriteError(c, h.store.ArchiveEpisode(c, userID, projectID, episodeID)) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"archived": true, "id": episodeID})
}

func (h *MobileStudioHandler) ListDocuments(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	episodeID, ok := mobileStudioOptionalUUID(c, "episode_id")
	if !ok {
		return
	}
	items, err := h.store.ListDocuments(c, userID, projectID, episodeID)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"items": items})
}

func (h *MobileStudioHandler) PutDocument(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	documentType := strings.TrimSpace(c.Param("documentType"))
	if !mobileStudioAllowed(documentType, mobileStudioDocumentTypes...) {
		mobileStudioBadRequest(c, errors.New("文档类型不支持"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileStudioMaxDocumentSize+(1<<10))
	var input mobileStudioDocumentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("文档参数不正确"))
		return
	}
	if err := normalizeMobileStudioDocumentInput(&input); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.PutDocument(c, userID, projectID, documentType, input)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, item)
}

func (h *MobileStudioHandler) ListAssetLinks(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	episodeID, ok := mobileStudioOptionalUUID(c, "episode_id")
	if !ok {
		return
	}
	items, err := h.store.ListAssetLinks(c, userID, projectID, episodeID)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"items": items})
}

func (h *MobileStudioHandler) LinkAsset(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	var input mobileStudioAssetLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		mobileStudioBadRequest(c, errors.New("素材关联参数不正确"))
		return
	}
	if err := normalizeMobileStudioAssetLinkInput(&input); err != nil {
		mobileStudioBadRequest(c, err)
		return
	}
	item, err := h.store.LinkAsset(c, userID, projectID, input)
	if mobileStudioWriteError(c, err) {
		return
	}
	mobileStudioPrivate(c)
	response.Created(c, item)
}

func (h *MobileStudioHandler) DeleteAssetLink(c *gin.Context) {
	userID, ok := mobileStudioUserID(c)
	if !ok {
		return
	}
	projectID, ok := mobileStudioUUIDParam(c, "id")
	if !ok {
		return
	}
	linkID, ok := mobileStudioUUIDParam(c, "linkID")
	if !ok {
		return
	}
	if mobileStudioWriteError(c, h.store.DeleteAssetLink(c, userID, projectID, linkID)) {
		return
	}
	mobileStudioPrivate(c)
	response.Success(c, gin.H{"deleted": true, "id": linkID})
}

var mobileStudioDocumentTypes = []string{"script", "visual_bible", "storyboard", "image_prompts", "video_prompts"}
var mobileStudioLinkTypes = []string{"reference", "character", "scene", "prop", "keyframe", "shot", "video", "audio", "final"}

func mobileStudioUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return subject.UserID, true
}
func mobileStudioPrivate(c *gin.Context) { c.Header("Cache-Control", "private, no-store") }
func mobileStudioUUIDParam(c *gin.Context, name string) (string, bool) {
	id := strings.TrimSpace(c.Param(name))
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(c, "资源不存在")
		return "", false
	}
	return id, true
}
func mobileStudioOptionalUUID(c *gin.Context, name string) (*string, bool) {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return nil, true
	}
	if _, err := uuid.Parse(value); err != nil {
		mobileStudioBadRequest(c, errors.New("分集标识不正确"))
		return nil, false
	}
	return &value, true
}
func mobileStudioBadRequest(c *gin.Context, err error) {
	response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "STUDIO_REQUEST_INVALID", nil)
}
func mobileStudioInternal(c *gin.Context, err error) {
	_ = err
	response.InternalError(c, "创作项目服务暂不可用")
}
func mobileStudioWriteError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, errMobileStudioNotFound):
		response.NotFound(c, "项目或资源不存在")
	case errors.Is(err, errMobileStudioConflict):
		response.ErrorWithDetails(c, http.StatusConflict, "内容已被其他修改覆盖，请刷新后重试", "STUDIO_VERSION_CONFLICT", nil)
	case errors.Is(err, errMobileStudioInvalidAsset):
		response.ErrorWithDetails(c, http.StatusForbidden, "素材不可用于该项目", "STUDIO_ASSET_UNAVAILABLE", nil)
	default:
		mobileStudioInternal(c, err)
	}
	return true
}
func mobileStudioAllowed(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}
func mobileStudioClean(value string, maximum int) string {
	value = strings.TrimSpace(value)
	if len(value) > maximum {
		return ""
	}
	return value
}
func parseMobileStudioListFilter(c *gin.Context) (mobileStudioListFilter, error) {
	filter := mobileStudioListFilter{Page: mobileStudioDefaultPageSize / mobileStudioDefaultPageSize, PageSize: mobileStudioDefaultPageSize}
	for _, item := range []struct {
		key     string
		into    *int
		maximum int
	}{{"page", &filter.Page, 0}, {"page_size", &filter.PageSize, mobileStudioMaxPageSize}} {
		if raw := strings.TrimSpace(c.Query(item.key)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || (item.maximum > 0 && value > item.maximum) {
				return filter, errors.New("分页参数不正确")
			}
			*item.into = value
		}
	}
	return filter, nil
}

func normalizeMobileStudioProjectInput(input *mobileStudioProjectInput, create bool) error {
	input.Title = mobileStudioClean(input.Title, 200)
	input.Description = mobileStudioClean(input.Description, 20_000)
	if input.Title == "" {
		return errors.New("项目名称不正确")
	}
	if create {
		if input.AspectRatio == "" {
			input.AspectRatio = "9:16"
		}
		if input.Language == "" {
			input.Language = "zh-CN"
		}
		if input.Status == "" {
			input.Status = "draft"
		}
	}
	if !mobileStudioAllowed(input.AspectRatio, "9:16", "16:9", "1:1") || input.Language == "" || len(input.Language) > 16 || !mobileStudioAllowed(input.Status, "draft", "active") || input.Version < 0 {
		return errors.New("项目参数不正确")
	}
	if input.CoverAssetID != nil {
		id := strings.TrimSpace(*input.CoverAssetID)
		if _, err := uuid.Parse(id); err != nil {
			return errors.New("封面素材标识不正确")
		}
		input.CoverAssetID = &id
	}
	return nil
}
func normalizeMobileStudioEpisodeInput(input *mobileStudioEpisodeInput) error {
	input.StableID = mobileStudioClean(input.StableID, 64)
	input.Title = mobileStudioClean(input.Title, 200)
	if !mobileStudioAllowed(input.Status, "draft", "ready", "archived") || input.StableID == "" || input.Title == "" || input.Sequence < 1 {
		return errors.New("分集参数不正确")
	}
	return nil
}
func normalizeMobileStudioDocumentInput(input *mobileStudioDocumentInput) error {
	if input.ExpectedVersion < 0 || len(input.Content) == 0 || len(input.Content) > mobileStudioMaxDocumentSize {
		return errors.New("文档参数不正确")
	}
	var content any
	if json.Unmarshal(input.Content, &content) != nil {
		return errors.New("文档内容必须是 JSON")
	}
	if input.EpisodeID != nil {
		id := strings.TrimSpace(*input.EpisodeID)
		if _, err := uuid.Parse(id); err != nil {
			return errors.New("分集标识不正确")
		}
		input.EpisodeID = &id
	}
	return nil
}
func normalizeMobileStudioAssetLinkInput(input *mobileStudioAssetLinkInput) error {
	input.AssetID = strings.TrimSpace(input.AssetID)
	input.LinkType = strings.TrimSpace(input.LinkType)
	if _, err := uuid.Parse(input.AssetID); err != nil || !mobileStudioAllowed(input.LinkType, mobileStudioLinkTypes...) || (input.Position != nil && *input.Position < 0) {
		return errors.New("素材关联参数不正确")
	}
	if input.EpisodeID != nil {
		id := strings.TrimSpace(*input.EpisodeID)
		if _, err := uuid.Parse(id); err != nil {
			return errors.New("分集标识不正确")
		}
		input.EpisodeID = &id
	}
	if input.StableRef != nil {
		value := mobileStudioClean(*input.StableRef, 64)
		if value == "" {
			return errors.New("素材关联标识不正确")
		}
		input.StableRef = &value
	}
	if len(input.Metadata) == 0 {
		input.Metadata = json.RawMessage(`{}`)
	}
	if len(input.Metadata) > 64<<10 || !json.Valid(input.Metadata) {
		return errors.New("素材关联元数据不正确")
	}
	return nil
}

type sqlMobileStudioStore struct{ db *sql.DB }

const mobileStudioProjectColumns = "id::text, title, description, aspect_ratio, language, status, cover_asset_id::text, version, created_at, updated_at, archived_at"
const mobileStudioEpisodeColumns = "id::text, project_id::text, stable_id, title, sequence, status, created_at, updated_at"
const mobileStudioDocumentColumns = "id::text, project_id::text, episode_id::text, document_type, content, version, created_at, updated_at"
const mobileStudioAssetLinkColumns = "id::text, project_id::text, episode_id::text, asset_id::text, link_type, stable_ref, position, metadata, created_at, updated_at"

func scanMobileStudioProject(row interface{ Scan(...any) error }) (*mobileStudioProject, error) {
	item := &mobileStudioProject{}
	if err := row.Scan(&item.ID, &item.Title, &item.Description, &item.AspectRatio, &item.Language, &item.Status, &item.CoverAssetID, &item.Version, &item.CreatedAt, &item.UpdatedAt, &item.ArchivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errMobileStudioNotFound
		}
		return nil, err
	}
	return item, nil
}
func scanMobileStudioEpisode(row interface{ Scan(...any) error }) (*mobileStudioEpisode, error) {
	item := &mobileStudioEpisode{}
	if err := row.Scan(&item.ID, &item.ProjectID, &item.StableID, &item.Title, &item.Sequence, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errMobileStudioNotFound
		}
		return nil, err
	}
	return item, nil
}
func scanMobileStudioDocument(row interface{ Scan(...any) error }) (*mobileStudioDocument, error) {
	item := &mobileStudioDocument{}
	if err := row.Scan(&item.ID, &item.ProjectID, &item.EpisodeID, &item.DocumentType, &item.Content, &item.Version, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errMobileStudioNotFound
		}
		return nil, err
	}
	return item, nil
}
func scanMobileStudioAssetLink(row interface{ Scan(...any) error }) (*mobileStudioAssetLink, error) {
	item := &mobileStudioAssetLink{}
	if err := row.Scan(&item.ID, &item.ProjectID, &item.EpisodeID, &item.AssetID, &item.LinkType, &item.StableRef, &item.Position, &item.Metadata, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errMobileStudioNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *sqlMobileStudioStore) CreateProject(ctx context.Context, userID int64, input mobileStudioProjectInput) (*mobileStudioProject, error) {
	if err := s.requireOwnedStudioAsset(ctx, userID, input.CoverAssetID); err != nil {
		return nil, err
	}
	return scanMobileStudioProject(s.db.QueryRowContext(ctx, `INSERT INTO studio_projects (id,user_id,title,description,aspect_ratio,language,status,cover_asset_id) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8::uuid) RETURNING `+mobileStudioProjectColumns, uuid.NewString(), userID, input.Title, input.Description, input.AspectRatio, input.Language, input.Status, input.CoverAssetID))
}
func (s *sqlMobileStudioStore) ListProjects(ctx context.Context, userID int64, filter mobileStudioListFilter) ([]mobileStudioProject, int64, error) {
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM studio_projects WHERE user_id=$1 AND archived_at IS NULL`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+mobileStudioProjectColumns+` FROM studio_projects WHERE user_id=$1 AND archived_at IS NULL ORDER BY updated_at DESC,id DESC LIMIT $2 OFFSET $3`, userID, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []mobileStudioProject{}
	for rows.Next() {
		item, err := scanMobileStudioProject(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, rows.Err()
}
func (s *sqlMobileStudioStore) GetProject(ctx context.Context, userID int64, id string) (*mobileStudioProject, error) {
	return scanMobileStudioProject(s.db.QueryRowContext(ctx, `SELECT `+mobileStudioProjectColumns+` FROM studio_projects WHERE id=$1::uuid AND user_id=$2 AND archived_at IS NULL`, id, userID))
}
func (s *sqlMobileStudioStore) UpdateProject(ctx context.Context, userID int64, id string, input mobileStudioProjectInput) (*mobileStudioProject, error) {
	if err := s.requireOwnedStudioAsset(ctx, userID, input.CoverAssetID); err != nil {
		return nil, err
	}
	item, err := scanMobileStudioProject(s.db.QueryRowContext(ctx, `UPDATE studio_projects SET title=$3,description=$4,aspect_ratio=$5,language=$6,status=$7,cover_asset_id=$8::uuid,version=version+1,updated_at=NOW() WHERE id=$1::uuid AND user_id=$2 AND archived_at IS NULL AND version=$9 RETURNING `+mobileStudioProjectColumns, id, userID, input.Title, input.Description, input.AspectRatio, input.Language, input.Status, input.CoverAssetID, input.Version))
	if errors.Is(err, errMobileStudioNotFound) {
		exists, checkErr := s.projectExists(ctx, userID, id)
		if checkErr != nil {
			return nil, checkErr
		}
		if exists {
			return nil, errMobileStudioConflict
		}
	}
	return item, err
}
func (s *sqlMobileStudioStore) ArchiveProject(ctx context.Context, userID int64, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE studio_projects SET status='archived',archived_at=NOW(),updated_at=NOW(),version=version+1 WHERE id=$1::uuid AND user_id=$2 AND archived_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return errMobileStudioNotFound
	}
	return nil
}
func (s *sqlMobileStudioStore) projectExists(ctx context.Context, userID int64, id string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM studio_projects WHERE id=$1::uuid AND user_id=$2 AND archived_at IS NULL)`, id, userID).Scan(&exists)
	return exists, err
}
func (s *sqlMobileStudioStore) ListEpisodes(ctx context.Context, userID int64, projectID string) ([]mobileStudioEpisode, error) {
	if ok, err := s.projectExists(ctx, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errMobileStudioNotFound
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+mobileStudioEpisodeColumns+` FROM studio_episodes WHERE project_id=$1::uuid AND user_id=$2 ORDER BY sequence,id`, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []mobileStudioEpisode{}
	for rows.Next() {
		item, err := scanMobileStudioEpisode(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}
func (s *sqlMobileStudioStore) CreateEpisode(ctx context.Context, userID int64, projectID string, input mobileStudioEpisodeInput) (*mobileStudioEpisode, error) {
	return scanMobileStudioEpisode(s.db.QueryRowContext(ctx, `INSERT INTO studio_episodes (id,project_id,user_id,stable_id,title,sequence,status) SELECT $1::uuid,p.id,$2,$3,$4,$5,$6 FROM studio_projects p WHERE p.id=$7::uuid AND p.user_id=$2 AND p.archived_at IS NULL RETURNING `+mobileStudioEpisodeColumns, uuid.NewString(), userID, input.StableID, input.Title, input.Sequence, input.Status, projectID))
}
func (s *sqlMobileStudioStore) UpdateEpisode(ctx context.Context, userID int64, projectID, episodeID string, input mobileStudioEpisodeInput) (*mobileStudioEpisode, error) {
	return scanMobileStudioEpisode(s.db.QueryRowContext(ctx, `UPDATE studio_episodes SET stable_id=$4,title=$5,sequence=$6,status=$7,updated_at=NOW() WHERE id=$1::uuid AND project_id=$2::uuid AND user_id=$3 RETURNING `+mobileStudioEpisodeColumns, episodeID, projectID, userID, input.StableID, input.Title, input.Sequence, input.Status))
}
func (s *sqlMobileStudioStore) ArchiveEpisode(ctx context.Context, userID int64, projectID, episodeID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE studio_episodes SET status='archived',updated_at=NOW() WHERE id=$1::uuid AND project_id=$2::uuid AND user_id=$3`, episodeID, projectID, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errMobileStudioNotFound
	}
	return nil
}
func (s *sqlMobileStudioStore) ListDocuments(ctx context.Context, userID int64, projectID string, episodeID *string) ([]mobileStudioDocument, error) {
	if ok, err := s.projectExists(ctx, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errMobileStudioNotFound
	}
	query := `SELECT ` + mobileStudioDocumentColumns + ` FROM studio_documents WHERE project_id=$1::uuid AND user_id=$2 AND archived_at IS NULL`
	args := []any{projectID, userID}
	if episodeID == nil {
		query += ` AND episode_id IS NULL`
	} else {
		query += ` AND episode_id=$3::uuid`
		args = append(args, *episodeID)
	}
	query += ` ORDER BY document_type,version DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []mobileStudioDocument{}
	for rows.Next() {
		item, err := scanMobileStudioDocument(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}
func (s *sqlMobileStudioStore) PutDocument(ctx context.Context, userID int64, projectID, documentType string, input mobileStudioDocumentInput) (*mobileStudioDocument, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM studio_projects WHERE id=$1::uuid AND user_id=$2 AND archived_at IS NULL)`, projectID, userID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, errMobileStudioNotFound
	}
	if input.EpisodeID != nil {
		var episodeOK bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM studio_episodes WHERE id=$1::uuid AND project_id=$2::uuid AND user_id=$3)`, *input.EpisodeID, projectID, userID).Scan(&episodeOK); err != nil {
			return nil, err
		}
		if !episodeOK {
			return nil, errMobileStudioNotFound
		}
	}
	var current int
	query := `SELECT COALESCE(MAX(version),0) FROM studio_documents WHERE project_id=$1::uuid AND user_id=$2 AND document_type=$3 AND archived_at IS NULL`
	args := []any{projectID, userID, documentType}
	if input.EpisodeID == nil {
		query += ` AND episode_id IS NULL`
	} else {
		query += ` AND episode_id=$4::uuid`
		args = append(args, *input.EpisodeID)
	}
	if err = tx.QueryRowContext(ctx, query, args...).Scan(&current); err != nil {
		return nil, err
	}
	if current != input.ExpectedVersion {
		return nil, errMobileStudioConflict
	}
	item, err := scanMobileStudioDocument(tx.QueryRowContext(ctx, `INSERT INTO studio_documents (id,project_id,episode_id,user_id,document_type,content,version) VALUES ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::jsonb,$7) RETURNING `+mobileStudioDocumentColumns, uuid.NewString(), projectID, input.EpisodeID, userID, documentType, input.Content, current+1))
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE studio_projects SET updated_at=NOW() WHERE id=$1::uuid`, projectID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}
func (s *sqlMobileStudioStore) ListAssetLinks(ctx context.Context, userID int64, projectID string, episodeID *string) ([]mobileStudioAssetLink, error) {
	if ok, err := s.projectExists(ctx, userID, projectID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errMobileStudioNotFound
	}
	query := `SELECT ` + mobileStudioAssetLinkColumns + ` FROM studio_asset_links WHERE project_id=$1::uuid AND user_id=$2`
	args := []any{projectID, userID}
	if episodeID == nil {
		query += ` AND episode_id IS NULL`
	} else {
		query += ` AND episode_id=$3::uuid`
		args = append(args, *episodeID)
	}
	query += ` ORDER BY position NULLS LAST,created_at`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []mobileStudioAssetLink{}
	for rows.Next() {
		item, err := scanMobileStudioAssetLink(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}
func (s *sqlMobileStudioStore) LinkAsset(ctx context.Context, userID int64, projectID string, input mobileStudioAssetLinkInput) (*mobileStudioAssetLink, error) {
	if input.EpisodeID != nil {
		var ok bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM studio_episodes WHERE id=$1::uuid AND project_id=$2::uuid AND user_id=$3)`, *input.EpisodeID, projectID, userID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, errMobileStudioNotFound
		}
	}
	item, err := scanMobileStudioAssetLink(s.db.QueryRowContext(ctx, `INSERT INTO studio_asset_links (id,project_id,episode_id,user_id,asset_id,link_type,stable_ref,position,metadata) SELECT $1::uuid,p.id,$2::uuid,$3,a.id,$5,$6,$7,$8::jsonb FROM studio_projects p JOIN mobile_assets a ON a.id=$4::uuid AND a.user_id=$3 AND a.deleted_at IS NULL AND a.status='ready' WHERE p.id=$9::uuid AND p.user_id=$3 AND p.archived_at IS NULL RETURNING `+mobileStudioAssetLinkColumns, uuid.NewString(), input.EpisodeID, userID, input.AssetID, input.LinkType, input.StableRef, input.Position, input.Metadata, projectID))
	if errors.Is(err, errMobileStudioNotFound) {
		return nil, errMobileStudioInvalidAsset
	}
	return item, err
}
func (s *sqlMobileStudioStore) DeleteAssetLink(ctx context.Context, userID int64, projectID, linkID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM studio_asset_links WHERE id=$1::uuid AND project_id=$2::uuid AND user_id=$3`, linkID, projectID, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errMobileStudioNotFound
	}
	return nil
}

func (s *sqlMobileStudioStore) requireOwnedStudioAsset(ctx context.Context, userID int64, assetID *string) error {
	if assetID == nil {
		return nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM mobile_assets WHERE id=$1::uuid AND user_id=$2 AND deleted_at IS NULL AND status='ready')`, *assetID, userID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return errMobileStudioInvalidAsset
	}
	return nil
}

func (s *sqlMobileStudioStore) String() string { return fmt.Sprintf("sqlMobileStudioStore(%p)", s.db) }
