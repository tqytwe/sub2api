package handler

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	mobileAssetDefaultPageSize = 20
	mobileAssetMaxPageSize     = 100
	mobileAssetMaxUploadBytes  = 25 << 20
)

var errMobileAssetNotFound = errors.New("mobile asset not found")

type mobileAssetRecord struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	Source       string         `json:"source"`
	StorageKey   string         `json:"-"`
	ContentURL   string         `json:"content_url,omitempty"`
	OriginalName string         `json:"original_name"`
	ContentType  string         `json:"content_type"`
	ByteSize     int64          `json:"byte_size"`
	SHA256       *string        `json:"sha256,omitempty"`
	Status       string         `json:"status"`
	SourceType   *string        `json:"source_type,omitempty"`
	SourceID     *string        `json:"source_id,omitempty"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type mobileAssetCreateInput struct {
	Kind         string         `json:"kind" binding:"required"`
	Source       string         `json:"source" binding:"required"`
	StorageKey   string         `json:"storage_key" binding:"required"`
	OriginalName string         `json:"original_name" binding:"required"`
	ContentType  string         `json:"content_type" binding:"required"`
	ByteSize     int64          `json:"byte_size"`
	SHA256       *string        `json:"sha256"`
	Status       string         `json:"status"`
	SourceType   *string        `json:"source_type"`
	SourceID     *string        `json:"source_id"`
	Metadata     map[string]any `json:"metadata"`
}

type mobileAssetListFilter struct {
	Kind     string
	Status   string
	Page     int
	PageSize int
}

type mobileAssetStore interface {
	Create(context.Context, int64, mobileAssetCreateInput) (*mobileAssetRecord, error)
	List(context.Context, int64, mobileAssetListFilter) ([]mobileAssetRecord, int64, error)
	Get(context.Context, int64, string) (*mobileAssetRecord, error)
	SoftDelete(context.Context, int64, string) error
}

type MobileAssetHandler struct {
	store   mobileAssetStore
	storage service.ImageStorage
}

func NewMobileAssetHandler(db *sql.DB) *MobileAssetHandler {
	return NewMobileAssetHandlerWithStorage(db, nil)
}

func NewMobileAssetHandlerWithStorage(db *sql.DB, storage service.ImageStorage) *MobileAssetHandler {
	if db == nil {
		return &MobileAssetHandler{storage: storage}
	}
	return &MobileAssetHandler{store: &sqlMobileAssetStore{db: db}, storage: storage}
}

// NewMobileAssetHandlerFromAuth reuses the Ent driver owned by the existing
// application lifecycle, avoiding another connection pool or wire dependency.
func NewMobileAssetHandlerFromAuth(auth *AuthHandler) *MobileAssetHandler {
	if auth == nil || auth.authService == nil || auth.authService.EntClient() == nil {
		return NewMobileAssetHandler(nil)
	}
	driver, ok := auth.authService.EntClient().Driver().(*entsql.Driver)
	if !ok {
		return NewMobileAssetHandler(nil)
	}
	return NewMobileAssetHandler(driver.DB())
}

func newMobileAssetHandlerWithStore(store mobileAssetStore) *MobileAssetHandler {
	return &MobileAssetHandler{store: store}
}

func newMobileAssetHandlerWithStoreAndStorage(store mobileAssetStore, storage service.ImageStorage) *MobileAssetHandler {
	return &MobileAssetHandler{store: store, storage: storage}
}

func (h *MobileAssetHandler) Upload(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	if h == nil || h.store == nil || h.storage == nil {
		response.InternalError(c, "素材存储暂不可用")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择需要上传的素材")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, mobileAssetMaxUploadBytes+1))
	if err != nil || len(data) == 0 {
		response.BadRequest(c, "素材文件无法读取")
		return
	}
	if len(data) > mobileAssetMaxUploadBytes {
		response.BadRequest(c, "单个素材不能超过 25 MB")
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = strings.ToLower(http.DetectContentType(data))
	}
	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	if kind == "" {
		kind = mobileAssetKindFromContentType(contentType, header.Filename)
	}
	if !mobileAssetContentTypeAllowed(kind, contentType, header.Filename) {
		response.BadRequest(c, "素材文件类型不支持")
		return
	}
	id := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if len(ext) > 12 || strings.ContainsAny(ext, "/\\") {
		ext = ""
	}
	key := fmt.Sprintf("mobile-assets/%d/%s%s", userID, id, ext)
	if _, err := h.storage.Save(c.Request.Context(), key, contentType, data); err != nil {
		response.InternalError(c, "上传素材失败")
		return
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	source := strings.ToLower(strings.TrimSpace(c.PostForm("source")))
	switch source {
	case "share":
	case "voice":
	default:
		source = "upload"
	}
	input := mobileAssetCreateInput{
		Kind: kind, Source: source, StorageKey: key,
		OriginalName: strings.TrimSpace(header.Filename), ContentType: contentType,
		ByteSize: int64(len(data)), SHA256: &hash, Status: "ready", Metadata: map[string]any{},
	}
	record, err := h.store.Create(c.Request.Context(), userID, input)
	if err != nil {
		if deleter, ok := h.storage.(service.ImageAssetDeleter); ok {
			_ = deleter.Delete(context.WithoutCancel(c.Request.Context()), key)
		}
		response.InternalError(c, "创建素材失败")
		return
	}
	setMobileAssetContentURL(record)
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, record)
}

func (h *MobileAssetHandler) Create(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	var input mobileAssetCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "素材信息格式不正确")
		return
	}
	if err := normalizeMobileAssetInput(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}
	record, err := h.store.Create(c.Request.Context(), userID, input)
	if err != nil {
		response.InternalError(c, "创建素材失败")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, record)
}

func (h *MobileAssetHandler) List(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	filter, err := parseMobileAssetListFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}
	items, total, err := h.store.List(c.Request.Context(), userID, filter)
	if err != nil {
		response.InternalError(c, "读取素材失败")
		return
	}
	for index := range items {
		setMobileAssetContentURL(&items[index])
	}
	c.Header("Cache-Control", "private, no-store")
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

func (h *MobileAssetHandler) Get(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(c, "素材不存在")
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}
	record, err := h.store.Get(c.Request.Context(), userID, id)
	if errors.Is(err, errMobileAssetNotFound) {
		response.NotFound(c, "素材不存在")
		return
	}
	if err != nil {
		response.InternalError(c, "读取素材失败")
		return
	}
	setMobileAssetContentURL(record)
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, record)
}

func (h *MobileAssetHandler) Delete(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(c, "素材不存在")
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}
	record, err := h.store.Get(c.Request.Context(), userID, id)
	if errors.Is(err, errMobileAssetNotFound) {
		response.NotFound(c, "素材不存在")
		return
	} else if err != nil {
		response.InternalError(c, "删除素材失败")
		return
	}
	if err := h.store.SoftDelete(c.Request.Context(), userID, id); errors.Is(err, errMobileAssetNotFound) {
		response.NotFound(c, "素材不存在")
		return
	} else if err != nil {
		response.InternalError(c, "删除素材失败")
		return
	}
	cleanupPending := false
	if record.StorageKey != "" {
		deleter, ok := h.storage.(service.ImageAssetDeleter)
		if !ok || deleter.Delete(context.WithoutCancel(c.Request.Context()), record.StorageKey) != nil {
			cleanupPending = true
		}
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"deleted": true, "id": id, "cleanup_pending": cleanupPending})
}

func (h *MobileAssetHandler) Content(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(c, "素材不存在")
		return
	}
	if h == nil || h.store == nil || h.storage == nil {
		response.InternalError(c, "素材存储暂不可用")
		return
	}
	reader, readerOK := h.storage.(service.ImageAssetReader)
	if !readerOK {
		response.InternalError(c, "素材存储暂不可用")
		return
	}
	record, err := h.store.Get(c.Request.Context(), userID, id)
	if errors.Is(err, errMobileAssetNotFound) {
		response.NotFound(c, "素材不存在")
		return
	} else if err != nil {
		response.InternalError(c, "读取素材失败")
		return
	}
	body, storedType, err := reader.Open(c.Request.Context(), record.StorageKey)
	if err != nil {
		response.NotFound(c, "素材文件不存在")
		return
	}
	defer func() { _ = body.Close() }()
	contentType := record.ContentType
	if contentType == "" {
		contentType = storedType
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", strings.ReplaceAll(record.OriginalName, "\"", "")))
	c.DataFromReader(http.StatusOK, record.ByteSize, contentType, body, nil)
}

func setMobileAssetContentURL(record *mobileAssetRecord) {
	if record != nil && record.ID != "" {
		record.ContentURL = "/api/v1/mobile/assets/" + record.ID + "/content"
	}
}

func mobileAssetKindFromContentType(contentType, name string) string {
	value := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(value, "image/"):
		return "image"
	case strings.HasPrefix(value, "audio/"):
		return "audio"
	case strings.HasPrefix(value, "video/"):
		return "video"
	case value == "application/pdf" || strings.EqualFold(filepath.Ext(name), ".pdf"):
		return "pdf"
	case strings.HasPrefix(value, "text/"):
		return "document"
	default:
		return "file"
	}
}

func mobileAssetContentTypeAllowed(kind, contentType, name string) bool {
	if !mobileAssetAllowed(kind, "image", "audio", "video", "pdf", "document", "file") {
		return false
	}
	detectedKind := mobileAssetKindFromContentType(contentType, name)
	return kind == detectedKind || (kind == "file" && detectedKind == "document")
}

func mobileAssetUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "用户未登录")
		return 0, false
	}
	return subject.UserID, true
}

func normalizeMobileAssetInput(input *mobileAssetCreateInput) error {
	input.Kind = strings.TrimSpace(input.Kind)
	input.Source = strings.TrimSpace(input.Source)
	input.StorageKey = strings.TrimSpace(input.StorageKey)
	input.OriginalName = strings.TrimSpace(input.OriginalName)
	input.ContentType = strings.TrimSpace(input.ContentType)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = "uploading"
	}
	if !mobileAssetAllowed(input.Kind, "image", "audio", "video", "pdf", "document", "file") {
		return errors.New("素材类型不支持")
	}
	if !mobileAssetAllowed(input.Source, "upload", "share", "image_result", "chat_export", "voice") {
		return errors.New("素材来源不支持")
	}
	if !mobileAssetAllowed(input.Status, "uploading", "ready", "failed") {
		return errors.New("素材状态不正确")
	}
	if input.StorageKey == "" || len(input.StorageKey) > 1024 || input.OriginalName == "" || len(input.OriginalName) > 255 || input.ContentType == "" || len(input.ContentType) > 255 || input.ByteSize < 0 {
		return errors.New("素材文件信息不正确")
	}
	if input.SHA256 != nil {
		hash := strings.ToLower(strings.TrimSpace(*input.SHA256))
		if len(hash) != 64 {
			return errors.New("素材校验值不正确")
		}
		for _, char := range hash {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return errors.New("素材校验值不正确")
			}
		}
		input.SHA256 = &hash
	}
	input.SourceType = normalizeOptionalMobileAssetString(input.SourceType, 64)
	input.SourceID = normalizeOptionalMobileAssetString(input.SourceID, 128)
	input.Metadata = sanitizeMobileAssetMetadata(input.Metadata)
	return nil
}

func normalizeOptionalMobileAssetString(value *string, maxLen int) *string {
	if value == nil {
		return nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" || len(normalized) > maxLen {
		return nil
	}
	return &normalized
}

func sanitizeMobileAssetMetadata(input map[string]any) map[string]any {
	allowed := map[string]bool{
		"width": true, "height": true, "duration_ms": true, "page_count": true,
		"orientation": true, "has_thumbnail": true, "reference_recommended": true,
	}
	clean := make(map[string]any)
	for key, value := range input {
		if allowed[key] {
			clean[key] = value
		}
	}
	return clean
}

func parseMobileAssetListFilter(c *gin.Context) (mobileAssetListFilter, error) {
	filter := mobileAssetListFilter{Kind: strings.TrimSpace(c.Query("kind")), Status: strings.TrimSpace(c.Query("status")), Page: 1, PageSize: mobileAssetDefaultPageSize}
	if value := c.Query("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return filter, errors.New("页码不正确")
		}
		filter.Page = parsed
	}
	if value := c.Query("page_size"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > mobileAssetMaxPageSize {
			return filter, errors.New("每页数量不正确")
		}
		filter.PageSize = parsed
	}
	if filter.Kind != "" && !mobileAssetAllowed(filter.Kind, "image", "audio", "video", "pdf", "document", "file") {
		return filter, errors.New("素材类型不支持")
	}
	if filter.Status != "" && !mobileAssetAllowed(filter.Status, "uploading", "ready", "failed") {
		return filter, errors.New("素材状态不正确")
	}
	return filter, nil
}

func mobileAssetAllowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

type sqlMobileAssetStore struct {
	db *sql.DB
}

const mobileAssetSelectColumns = `id::text, kind, source, storage_key, original_name, content_type, byte_size, sha256, status, source_type, source_id, metadata, created_at, updated_at`

func (s *sqlMobileAssetStore) Create(ctx context.Context, userID int64, input mobileAssetCreateInput) (*mobileAssetRecord, error) {
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO mobile_assets (id, user_id, kind, source, storage_key, original_name, content_type, byte_size, sha256, status, source_type, source_id, metadata, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, NOW(), NOW())
		RETURNING `+mobileAssetSelectColumns, id, userID, input.Kind, input.Source, input.StorageKey, input.OriginalName, input.ContentType, input.ByteSize, input.SHA256, input.Status, input.SourceType, input.SourceID, metadata)
	return scanMobileAsset(row)
}

func (s *sqlMobileAssetStore) List(ctx context.Context, userID int64, filter mobileAssetListFilter) ([]mobileAssetRecord, int64, error) {
	where := []string{"user_id = $1", "deleted_at IS NULL"}
	args := []any{userID}
	if filter.Kind != "" {
		args = append(args, filter.Kind)
		where = append(where, fmt.Sprintf("kind = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	var total int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mobile_assets WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := fmt.Sprintf("SELECT %s FROM mobile_assets WHERE %s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", mobileAssetSelectColumns, whereSQL, len(args)-1, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]mobileAssetRecord, 0)
	for rows.Next() {
		record, err := scanMobileAsset(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *record)
	}
	return items, total, rows.Err()
}

func (s *sqlMobileAssetStore) Get(ctx context.Context, userID int64, id string) (*mobileAssetRecord, error) {
	record, err := scanMobileAsset(s.db.QueryRowContext(ctx, "SELECT "+mobileAssetSelectColumns+" FROM mobile_assets WHERE id = $1::uuid AND user_id = $2 AND deleted_at IS NULL", id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errMobileAssetNotFound
	}
	return record, err
}

func (s *sqlMobileAssetStore) SoftDelete(ctx context.Context, userID int64, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE mobile_assets SET status = 'deleted', deleted_at = NOW(), updated_at = NOW() WHERE id = $1::uuid AND user_id = $2 AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errMobileAssetNotFound
	}
	return nil
}

type mobileAssetScanner interface {
	Scan(...any) error
}

func scanMobileAsset(scanner mobileAssetScanner) (*mobileAssetRecord, error) {
	var record mobileAssetRecord
	var metadata []byte
	if err := scanner.Scan(&record.ID, &record.Kind, &record.Source, &record.StorageKey, &record.OriginalName, &record.ContentType, &record.ByteSize, &record.SHA256, &record.Status, &record.SourceType, &record.SourceID, &metadata, &record.CreatedAt, &record.UpdatedAt); err != nil {
		return nil, err
	}
	record.Metadata = map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &record.Metadata); err != nil {
			return nil, err
		}
	}
	record.Metadata = sanitizeMobileAssetMetadata(record.Metadata)
	return &record, nil
}
