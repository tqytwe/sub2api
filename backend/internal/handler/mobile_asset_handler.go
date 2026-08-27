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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	mobileAssetDefaultPageSize = 20
	mobileAssetMaxPageSize     = 100
	mobileAssetMaxUploadBytes  = 200 << 20
	mobileAssetMaxImageBytes   = 30 << 20
	mobileAssetMaxAudioBytes   = 15 << 20
	mobileAssetMaxResultBytes  = 200 << 20
	// Inline reference fallback is deliberately small. Larger media is handed
	// to a provider using a short-lived object-storage URL instead of being
	// expanded to base64 in the video worker.
	mobileVideoInlineReferenceMaxBytes = 20 << 20
	mobileVideoReferenceURLExpiry      = time.Hour
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

type mobileAssetRenameInput struct {
	OriginalName string `json:"original_name"`
}

type mobileAssetListFilter struct {
	Kind     string
	Status   string
	Page     int
	PageSize int
}

type mobileAssetSyncResult struct {
	Version    string              `json:"version"`
	ETag       string              `json:"etag"`
	UpdatedAt  *time.Time          `json:"updated_at,omitempty"`
	Items      []mobileAssetRecord `json:"items"`
	DeletedIDs []string            `json:"deleted_ids"`
}

type mobileAssetStore interface {
	Create(context.Context, int64, mobileAssetCreateInput) (*mobileAssetRecord, error)
	List(context.Context, int64, mobileAssetListFilter) ([]mobileAssetRecord, int64, error)
	Get(context.Context, int64, string) (*mobileAssetRecord, error)
	UpdateOriginalName(context.Context, int64, string, string) (*mobileAssetRecord, error)
	SoftDelete(context.Context, int64, string) error
	Sync(context.Context, int64, *time.Time) (*mobileAssetSyncResult, error)
}

type MobileAssetHandler struct {
	store   mobileAssetStore
	storage service.ImageStorage
}

// ReadAssetForVideo returns a user-owned reference file for the server-side
// video adapter. It deliberately does not expose the storage key or a public
// URL to the provider/client boundary.
func (h *MobileAssetHandler) ReadAssetForVideo(ctx context.Context, userID int64, id string) ([]byte, string, error) {
	if h == nil || h.store == nil || h.storage == nil || userID <= 0 {
		return nil, "", errMobileAssetNotFound
	}
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return nil, "", errMobileAssetNotFound
	}
	record, err := h.store.Get(ctx, userID, strings.TrimSpace(id))
	if err != nil {
		return nil, "", err
	}
	if record.Status != "ready" || !mobileAssetAllowed(record.Kind, "image", "video", "audio") {
		return nil, "", errMobileAssetNotFound
	}
	reader, ok := h.storage.(service.ImageAssetReader)
	if !ok {
		return nil, "", errMobileAssetNotFound
	}
	body, storedType, err := reader.Open(ctx, record.StorageKey)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = body.Close() }()
	if record.ByteSize > mobileVideoInlineReferenceMaxBytes {
		return nil, "", fmt.Errorf("video reference is too large")
	}
	data, err := io.ReadAll(io.LimitReader(body, mobileVideoInlineReferenceMaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || int64(len(data)) > mobileVideoInlineReferenceMaxBytes {
		return nil, "", fmt.Errorf("video reference is too large")
	}
	contentType := strings.TrimSpace(record.ContentType)
	if contentType == "" {
		contentType = strings.TrimSpace(storedType)
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	return data, contentType, nil
}

// ReferenceURLForVideo returns an owner-scoped, short-lived URL for an
// upstream provider. It never exposes a storage key or relies on a public
// bucket. Local storage intentionally has no such URL and callers may only
// fall back to the small inline reader above.
func (h *MobileAssetHandler) ReferenceURLForVideo(ctx context.Context, userID int64, id string) (string, string, error) {
	if h == nil || h.store == nil || h.storage == nil || userID <= 0 {
		return "", "", errMobileAssetNotFound
	}
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return "", "", errMobileAssetNotFound
	}
	record, err := h.store.Get(ctx, userID, strings.TrimSpace(id))
	if err != nil {
		return "", "", err
	}
	if record.Status != "ready" || !mobileAssetAllowed(record.Kind, "image", "video", "audio") {
		return "", "", errMobileAssetNotFound
	}
	provider, ok := h.storage.(service.ImageAssetURLProvider)
	if !ok {
		return "", "", errors.New("private reference urls are unavailable")
	}
	url, err := provider.PresignGet(ctx, record.StorageKey, mobileVideoReferenceURLExpiry)
	if err != nil {
		return "", "", err
	}
	contentType := strings.TrimSpace(record.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return url, contentType, nil
}

// VideoReferenceKind returns only the user-owned reference type. It is used
// before a task is queued so capability checks cannot be bypassed by sending a
// video or audio asset through the generic reference ID array.
func (h *MobileAssetHandler) VideoReferenceKind(ctx context.Context, userID int64, id string) (string, error) {
	if h == nil || h.store == nil || userID <= 0 {
		return "", errMobileAssetNotFound
	}
	if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
		return "", errMobileAssetNotFound
	}
	record, err := h.store.Get(ctx, userID, strings.TrimSpace(id))
	if err != nil {
		return "", err
	}
	if record.Status != "ready" || !mobileAssetAllowed(record.Kind, "image", "video", "audio") {
		return "", errMobileAssetNotFound
	}
	// Uploads derive kind from the detected content type, but older records and
	// the metadata endpoint can still contain an inconsistent kind. Never let a
	// claimed image turn a video or audio blob into an image-to-video reference.
	kind := mobileAssetKindFromContentType(record.ContentType, record.OriginalName)
	if kind != record.Kind || !mobileAssetAllowed(kind, "image", "video", "audio") {
		return "", errMobileAssetNotFound
	}
	return kind, nil
}

// CreateFromStored copies a server-owned completed result into the user's
// material namespace. The copy is deliberate: deleting the resulting material
// must never remove the video task artifact that task history still references.
func (h *MobileAssetHandler) CreateFromStored(ctx context.Context, userID int64, sourceKey, taskID, originalName, contentType string) (*mobileAssetRecord, error) {
	if h == nil || h.store == nil || h.storage == nil || userID <= 0 || strings.TrimSpace(sourceKey) == "" {
		return nil, errMobileAssetNotFound
	}
	reader, ok := h.storage.(service.ImageAssetReader)
	if !ok {
		return nil, errors.New("mobile asset storage cannot read results")
	}
	body, storedType, err := reader.Open(ctx, sourceKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, mobileAssetMaxResultBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > mobileAssetMaxResultBytes {
		return nil, fmt.Errorf("stored result is too large")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = strings.TrimSpace(storedType)
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(contentType, "video/") {
		return nil, fmt.Errorf("stored result is not a video")
	}
	name := strings.TrimSpace(originalName)
	if name == "" {
		name = "video.mp4"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	assetID := uuid.NewString()
	key := fmt.Sprintf("mobile-assets/%d/%s.mp4", userID, assetID)
	if _, err := h.storage.Save(ctx, key, contentType, data); err != nil {
		return nil, err
	}
	sourceType := "mobile_video_job"
	sourceID := strings.TrimSpace(taskID)
	record, err := h.store.Create(ctx, userID, mobileAssetCreateInput{
		Kind: "video", Source: "video_result", StorageKey: key,
		OriginalName: name, ContentType: contentType, ByteSize: int64(len(data)),
		SHA256: &hash, Status: "ready", SourceType: &sourceType, SourceID: &sourceID,
		Metadata: map[string]any{},
	})
	if err != nil {
		if deleter, ok := h.storage.(service.ImageAssetDeleter); ok {
			_ = deleter.Delete(context.WithoutCancel(ctx), key)
		}
		return nil, err
	}
	setMobileAssetContentURL(record)
	return record, nil
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
	// Multipart parsing otherwise has no request-wide safety boundary. A small
	// allowance covers form headers in addition to the largest 200 MiB video.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileAssetMaxUploadBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择需要上传的素材")
		return
	}
	defer func() { _ = file.Close() }()
	contentType := strings.ToLower(strings.TrimSpace(header.Header.Get("Content-Type")))
	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	if kind == "" {
		kind = mobileAssetKindFromContentType(contentType, header.Filename)
	}
	maxBytes := mobileAssetUploadLimit(kind)
	staged, err := os.CreateTemp("", "sub2api-mobile-asset-*")
	if err != nil {
		response.InternalError(c, "素材暂存失败")
		return
	}
	stagedPath := staged.Name()
	defer func() { _ = os.Remove(stagedPath) }()
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(staged, hasher), io.LimitReader(file, maxBytes+1))
	closeErr := staged.Close()
	if copyErr != nil || closeErr != nil || written == 0 {
		response.BadRequest(c, "素材文件无法读取")
		return
	}
	if written > maxBytes {
		response.BadRequest(c, mobileAssetUploadLimitMessage(kind))
		return
	}
	if contentType == "" || contentType == "application/octet-stream" {
		sniffFile, openErr := os.Open(stagedPath)
		if openErr != nil {
			response.BadRequest(c, "素材文件无法读取")
			return
		}
		sniff := make([]byte, 512)
		sniffLen, readErr := sniffFile.Read(sniff)
		_ = sniffFile.Close()
		if readErr != nil && readErr != io.EOF || sniffLen == 0 {
			response.BadRequest(c, "素材文件无法读取")
			return
		}
		sniff = sniff[:sniffLen]
		contentType = strings.ToLower(http.DetectContentType(sniff))
		if kind == "" {
			kind = mobileAssetKindFromContentType(contentType, header.Filename)
		}
	}
	if kind == "" {
		kind = mobileAssetKindFromContentType(contentType, header.Filename)
	}
	if !mobileAssetContentTypeAllowed(kind, contentType, header.Filename) {
		response.BadRequest(c, "素材文件类型不支持")
		return
	}
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	source := strings.ToLower(strings.TrimSpace(c.PostForm("source")))
	switch source {
	case "share":
	case "voice":
	default:
		source = "upload"
	}
	input := mobileAssetCreateInput{
		Kind: kind, Source: source,
		OriginalName: strings.TrimSpace(header.Filename), ContentType: contentType,
		ByteSize: written, SHA256: &hash, Status: "ready", Metadata: map[string]any{},
	}

	// The request fingerprint includes the content digest and all immutable
	// metadata. A replay with the same user-scoped key returns the original
	// asset record without saving the bytes or creating a second database row.
	fingerprint := struct {
		Kind         string `json:"kind"`
		Source       string `json:"source"`
		OriginalName string `json:"original_name"`
		ContentType  string `json:"content_type"`
		ByteSize     int64  `json:"byte_size"`
		SHA256       string `json:"sha256"`
	}{
		Kind: input.Kind, Source: input.Source, OriginalName: input.OriginalName,
		ContentType: input.ContentType, ByteSize: input.ByteSize, SHA256: hash,
	}

	c.Header("Cache-Control", "private, no-store")
	executeUserIdempotentCreated(
		c,
		mobileUserIdempotencyScope(c, "mobile.asset.upload"),
		fingerprint,
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			id := uuid.NewString()
			ext := strings.ToLower(filepath.Ext(header.Filename))
			if len(ext) > 12 || strings.ContainsAny(ext, "/\\") {
				ext = ""
			}
			key := fmt.Sprintf("mobile-assets/%d/%s%s", userID, id, ext)
			if err := saveStagedMobileAsset(ctx, h.storage, key, contentType, stagedPath, written); err != nil {
				return nil, infraerrors.ServiceUnavailable(
					"MOBILE_ASSET_STORAGE_UNAVAILABLE",
					"上传素材失败",
				).WithCause(err)
			}
			createInput := input
			createInput.StorageKey = key
			record, err := h.store.Create(ctx, userID, createInput)
			if err != nil {
				if deleter, ok := h.storage.(service.ImageAssetDeleter); ok {
					_ = deleter.Delete(context.WithoutCancel(ctx), key)
				}
				return nil, infraerrors.InternalServer(
					"MOBILE_ASSET_CREATE_FAILED",
					"创建素材失败",
				).WithCause(err)
			}
			setMobileAssetContentURL(record)
			return record, nil
		},
	)
}

func mobileAssetUploadLimit(kind string) int64 {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image":
		return mobileAssetMaxImageBytes
	case "audio":
		return mobileAssetMaxAudioBytes
	case "video":
		return mobileAssetMaxUploadBytes
	default:
		return mobileAssetMaxImageBytes
	}
}

func mobileAssetUploadLimitMessage(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "video":
		return "视频素材不能超过 200 MB"
	case "audio":
		return "音频素材不能超过 15 MB"
	default:
		return "图片素材不能超过 30 MB"
	}
}

func saveStagedMobileAsset(ctx context.Context, storage service.ImageStorage, key, contentType, stagedPath string, size int64) error {
	file, err := os.Open(stagedPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if writer, ok := storage.(service.ImageAssetStreamWriter); ok {
		_, err = writer.SaveReader(ctx, key, contentType, file, size)
		return err
	}
	// The in-memory fallback is only retained for older test/local adapters;
	// production-scale references require a streaming object-storage backend.
	if size > mobileVideoInlineReferenceMaxBytes {
		return errors.New("素材存储不支持大文件流式上传")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	_, err = storage.Save(ctx, key, contentType, data)
	return err
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

// Sync returns the user's material metadata delta. The client downloads bytes
// only for new or changed items; deleted IDs are tombstones for local cleanup.
// An unchanged ETag is answered with 304 so reopening the app is cheap.
func (h *MobileAssetHandler) Sync(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}
	var since *time.Time
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			response.BadRequest(c, "同步版本不正确")
			return
		}
		since = &parsed
	}
	result, err := h.store.Sync(c.Request.Context(), userID, since)
	if err != nil {
		response.InternalError(c, "同步素材失败")
		return
	}
	if etag := strings.TrimSpace(c.GetHeader("If-None-Match")); etag != "" && etag == result.ETag {
		c.Header("ETag", result.ETag)
		c.AbortWithStatus(http.StatusNotModified)
		return
	}
	for index := range result.Items {
		setMobileAssetContentURL(&result.Items[index])
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("ETag", result.ETag)
	response.Success(c, result)
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

// Rename changes only the user-visible material name. The storage key stays
// immutable, so a rename cannot move bytes across accounts or namespaces.
func (h *MobileAssetHandler) Rename(c *gin.Context) {
	userID, ok := mobileAssetUserID(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		response.NotFound(c, "素材不存在")
		return
	}
	var input mobileAssetRenameInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "素材名称格式不正确")
		return
	}
	input.OriginalName = strings.TrimSpace(input.OriginalName)
	if input.OriginalName == "" || len(input.OriginalName) > 255 || strings.ContainsAny(input.OriginalName, "\r\n\x00") {
		response.BadRequest(c, "素材名称不正确")
		return
	}
	if h == nil || h.store == nil {
		response.InternalError(c, "素材库暂不可用")
		return
	}

	payload := struct {
		ID           string `json:"id"`
		OriginalName string `json:"original_name"`
	}{ID: id, OriginalName: input.OriginalName}
	c.Header("Cache-Control", "private, no-store")
	executeUserIdempotentJSON(
		c,
		mobileUserIdempotencyScope(c, "mobile.asset.rename"),
		payload,
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			record, err := h.store.UpdateOriginalName(ctx, userID, id, input.OriginalName)
			if errors.Is(err, errMobileAssetNotFound) {
				return nil, infraerrors.NotFound("MOBILE_ASSET_NOT_FOUND", "素材不存在")
			}
			if err != nil {
				return nil, infraerrors.InternalServer("MOBILE_ASSET_RENAME_FAILED", "重命名素材失败").WithCause(err)
			}
			setMobileAssetContentURL(record)
			return record, nil
		},
	)
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
	if !mobileAssetAllowed(input.Source, "upload", "share", "image_result", "video_result", "chat_export", "voice") {
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

func (s *sqlMobileAssetStore) Sync(ctx context.Context, userID int64, since *time.Time) (*mobileAssetSyncResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("mobile asset database is unavailable")
	}
	var maxUpdated time.Time
	var total int64
	var revisionSignature string
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(MAX(updated_at), TIMESTAMPTZ '1970-01-01 00:00:00+00'),
			COUNT(*),
			COALESCE(
				string_agg(
					jsonb_build_array(
						id::text,
						kind,
						source,
						storage_key,
						original_name,
						content_type,
						byte_size,
						COALESCE(sha256, ''),
						status,
						COALESCE(source_type, ''),
						COALESCE(source_id, ''),
						COALESCE(metadata, '{}'::jsonb),
						to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
						to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
						COALESCE(to_char(deleted_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), '')
					)::text,
					'|' ORDER BY id
				),
				''
			)
		FROM mobile_assets WHERE user_id = $1`, userID).Scan(&maxUpdated, &total, &revisionSignature); err != nil {
		return nil, err
	}
	version := maxUpdated.UTC().Format(time.RFC3339Nano)
	etag := mobileAssetSyncETag(version, total, revisionSignature)
	result := &mobileAssetSyncResult{
		Version: version, ETag: etag, UpdatedAt: &maxUpdated,
		Items: []mobileAssetRecord{}, DeletedIDs: []string{},
	}
	args := []any{userID}
	activeWhere := "user_id = $1 AND deleted_at IS NULL"
	deletedWhere := "user_id = $1 AND deleted_at IS NOT NULL"
	if since != nil {
		// The cursor is a timestamp for wire compatibility. Include the
		// preceding database tick so rows sharing the previous max timestamp
		// cannot be skipped; the client de-duplicates by asset id/hash.
		args = append(args, since.UTC().Add(-time.Microsecond))
		activeWhere += fmt.Sprintf(" AND updated_at > $%d", len(args))
		deletedWhere += fmt.Sprintf(" AND updated_at > $%d", len(args))
	}
	rows, err := s.db.QueryContext(ctx, "SELECT "+mobileAssetSelectColumns+" FROM mobile_assets WHERE "+activeWhere+" ORDER BY updated_at ASC, id ASC", args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		record, scanErr := scanMobileAsset(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		result.Items = append(result.Items, *record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	deletedRows, err := s.db.QueryContext(ctx, "SELECT id::text FROM mobile_assets WHERE "+deletedWhere+" ORDER BY updated_at ASC, id ASC", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = deletedRows.Close() }()
	for deletedRows.Next() {
		var id string
		if err := deletedRows.Scan(&id); err != nil {
			return nil, err
		}
		result.DeletedIDs = append(result.DeletedIDs, id)
	}
	return result, deletedRows.Err()
}

// mobileAssetSyncETag is a digest of the complete user-scoped material state,
// including tombstones. A max-updated-at/count pair alone can collide when two
// writes share a database timestamp, causing a client to incorrectly accept a
// 304 response and miss a delta.
func mobileAssetSyncETag(version string, total int64, revisionSignature string) string {
	payload := fmt.Sprintf("%s\x00%d\x00%s", version, total, revisionSignature)
	return fmt.Sprintf("\"%x\"", sha256.Sum256([]byte(payload)))
}

func (s *sqlMobileAssetStore) Get(ctx context.Context, userID int64, id string) (*mobileAssetRecord, error) {
	record, err := scanMobileAsset(s.db.QueryRowContext(ctx, "SELECT "+mobileAssetSelectColumns+" FROM mobile_assets WHERE id = $1::uuid AND user_id = $2 AND deleted_at IS NULL", id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errMobileAssetNotFound
	}
	return record, err
}

func (s *sqlMobileAssetStore) UpdateOriginalName(ctx context.Context, userID int64, id, originalName string) (*mobileAssetRecord, error) {
	record, err := scanMobileAsset(s.db.QueryRowContext(ctx, `
		UPDATE mobile_assets
		SET original_name = $1, updated_at = NOW()
		WHERE id = $2::uuid AND user_id = $3 AND deleted_at IS NULL
		RETURNING `+mobileAssetSelectColumns, originalName, id, userID))
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
