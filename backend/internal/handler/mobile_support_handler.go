package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const mobileSupportMessageMaxBytes = 8 << 10

type mobileSupportCreateRequest struct {
	Title          string                             `json:"title"`
	Category       string                             `json:"category"`
	Content        string                             `json:"content"`
	AppVersion     string                             `json:"app_version"`
	InstallationID string                             `json:"installation_id"`
	Channel        string                             `json:"channel"`
	Referrer       string                             `json:"referrer"`
	Platform       string                             `json:"platform"`
	DeviceModel    string                             `json:"device_model"`
	AndroidVersion string                             `json:"android_version"`
	SystemVersion  string                             `json:"system_version"`
	GroupName      string                             `json:"group_name"`
	GroupID        *int64                             `json:"group_id"`
	BackendURL     string                             `json:"backend_url"`
	LastError      string                             `json:"last_error"`
	CrashLog       string                             `json:"crash_log"`
	DeviceInfo     map[string]any                     `json:"device_info"`
	Screenshots    []service.MobileFeedbackScreenshot `json:"-"`
}

type mobileSupportService interface {
	CreateMobileFeedback(ctx context.Context, userID int64, input service.MobileFeedbackInput) (*service.MobileFeedbackRecord, error)
	ListUserMobileFeedback(ctx context.Context, userID int64, filter service.MobileFeedbackListFilter) (*service.MobileSupportTicketList, error)
	GetUserMobileFeedback(ctx context.Context, userID, id int64) (*service.MobileSupportTicket, error)
	AddUserMobileFeedbackMessage(ctx context.Context, userID, id int64, content string) (*service.MobileSupportTicket, error)
	CloseUserMobileFeedback(ctx context.Context, userID, id int64) (*service.MobileSupportTicket, error)
}

func (h *MobileSupportHandler) Create(c *gin.Context) {
	subject, ok := mobileSupportSubject(c)
	if !ok {
		return
	}
	req, err := h.parseCreateRequest(c)
	if err != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		response.BadRequest(c, "工单内容不正确")
		return
	}
	fingerprint, err := mobileFeedbackCreateFingerprint(c, req)
	if err != nil {
		response.BadRequest(c, "工单附件不正确")
		return
	}

	// Parsing multipart data is intentionally outside the idempotent executor,
	// but screenshot storage and ticket creation are inside it. A successful
	// replay therefore returns the original ticket without re-uploading its
	// screenshots or creating another feedback record.
	executeUserIdempotentCreated(
		c,
		mobileUserIdempotencyScope(c, "mobile.support.ticket.create"),
		fingerprint,
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			request := req
			if err := h.populateScreenshots(c, &request); err != nil {
				return nil, err
			}
			return h.service.CreateMobileFeedback(ctx, subject.UserID, mobileFeedbackInput(request))
		},
	)
}

type MobileSupportHandler struct {
	service              mobileSupportService
	feedbackAssetService *service.AnnouncementAssetService
}

func NewMobileSupportHandler(svc mobileSupportService, feedbackAssetService ...*service.AnnouncementAssetService) *MobileSupportHandler {
	var assetService *service.AnnouncementAssetService
	if len(feedbackAssetService) > 0 {
		assetService = feedbackAssetService[0]
	}
	return &MobileSupportHandler{service: svc, feedbackAssetService: assetService}
}

func (h *MobileSupportHandler) parseCreateRequest(c *gin.Context) (mobileSupportCreateRequest, error) {
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.Contains(contentType, "multipart/form-data") {
		return parseMobileFeedbackMultipartCreateRequest(c)
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req mobileSupportCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, err
	}
	return req, nil
}

// parseMobileFeedbackMultipartCreateRequest is shared by the canonical support
// route and its legacy fallback. Keeping field parsing identical means a
// fallback cannot silently alter ticket content or the idempotency fingerprint.
func parseMobileFeedbackMultipartCreateRequest(c *gin.Context) (mobileSupportCreateRequest, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileFeedbackMaxRequestBytes)
	var req mobileSupportCreateRequest
	if err := c.Request.ParseMultipartForm(mobileFeedbackMaxRequestBytes); err != nil {
		return req, err
	}
	req.Title = c.PostForm("title")
	req.Category = c.PostForm("category")
	req.Content = c.PostForm("content")
	req.AppVersion = c.PostForm("app_version")
	req.InstallationID = c.PostForm("installation_id")
	req.Channel = c.PostForm("channel")
	req.Referrer = c.PostForm("referrer")
	req.Platform = c.PostForm("platform")
	req.DeviceModel = c.PostForm("device_model")
	req.AndroidVersion = c.PostForm("android_version")
	req.SystemVersion = c.PostForm("system_version")
	req.GroupName = c.PostForm("group_name")
	req.BackendURL = c.PostForm("backend_url")
	req.LastError = c.PostForm("last_error")
	req.CrashLog = c.PostForm("crash_log")
	if raw := strings.TrimSpace(c.PostForm("group_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			if err != nil {
				return req, err
			}
			return req, errors.New("group id must be positive")
		}
		req.GroupID = &parsed
	}
	if raw := strings.TrimSpace(c.PostForm("device_info")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.DeviceInfo); err != nil {
			return req, err
		}
	}
	return req, nil
}

func mobileFeedbackInput(req mobileSupportCreateRequest) service.MobileFeedbackInput {
	return service.MobileFeedbackInput{
		Title: req.Title, Category: req.Category, Content: req.Content,
		AppVersion: req.AppVersion, Platform: req.Platform, DeviceModel: req.DeviceModel,
		InstallationID: req.InstallationID, Channel: req.Channel, Referrer: req.Referrer,
		AndroidVersion: req.AndroidVersion, SystemVersion: req.SystemVersion,
		GroupName: req.GroupName, GroupID: req.GroupID, BackendURL: req.BackendURL,
		LastError: req.LastError, CrashLog: req.CrashLog, DeviceInfo: req.DeviceInfo,
		Screenshots: req.Screenshots,
	}
}

type mobileFeedbackCreateIdempotencyFingerprint struct {
	RequestSHA256 string                                       `json:"request_sha256"`
	Screenshots   []mobileFeedbackScreenshotIdempotencySummary `json:"screenshots"`
}

type mobileFeedbackScreenshotIdempotencySummary struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
	SHA256      string `json:"sha256"`
}

// mobileFeedbackCreateFingerprint includes the stable form/JSON fields plus
// digests of multipart screenshots. It deliberately hashes the bytes before
// the executor so a successful replay can skip object storage entirely.
func mobileFeedbackCreateFingerprint(c *gin.Context, payload any) (mobileFeedbackCreateIdempotencyFingerprint, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return mobileFeedbackCreateIdempotencyFingerprint{}, err
	}
	requestDigest := sha256.Sum256(raw)
	fingerprint := mobileFeedbackCreateIdempotencyFingerprint{
		RequestSHA256: hex.EncodeToString(requestDigest[:]),
		Screenshots:   make([]mobileFeedbackScreenshotIdempotencySummary, 0),
	}
	if c == nil || c.Request == nil || c.Request.MultipartForm == nil {
		return fingerprint, nil
	}
	files := c.Request.MultipartForm.File["screenshots"]
	if len(files) > mobileFeedbackMaxScreenshots {
		return mobileFeedbackCreateIdempotencyFingerprint{}, errors.New("too many screenshots")
	}
	for _, header := range files {
		if header == nil {
			continue
		}
		if header.Size > mobileFeedbackMaxFileBytes {
			return mobileFeedbackCreateIdempotencyFingerprint{}, service.ErrAnnouncementAssetTooLarge
		}
		file, err := header.Open()
		if err != nil {
			return mobileFeedbackCreateIdempotencyFingerprint{}, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, mobileFeedbackMaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return mobileFeedbackCreateIdempotencyFingerprint{}, readErr
		}
		if closeErr != nil {
			return mobileFeedbackCreateIdempotencyFingerprint{}, closeErr
		}
		if int64(len(data)) > mobileFeedbackMaxFileBytes {
			return mobileFeedbackCreateIdempotencyFingerprint{}, service.ErrAnnouncementAssetTooLarge
		}
		digest := sha256.Sum256(data)
		fingerprint.Screenshots = append(fingerprint.Screenshots, mobileFeedbackScreenshotIdempotencySummary{
			FileName:    header.Filename,
			ContentType: header.Header.Get("Content-Type"),
			ByteSize:    int64(len(data)),
			SHA256:      hex.EncodeToString(digest[:]),
		})
	}
	return fingerprint, nil
}

func (h *MobileSupportHandler) populateScreenshots(c *gin.Context, req *mobileSupportCreateRequest) error {
	if req == nil {
		return errors.New("support request is required")
	}
	screenshots, err := h.uploadScreenshots(c)
	if err != nil {
		if errors.Is(err, service.ErrAnnouncementAssetStorageUnavailable) {
			req.LastError = strings.TrimSpace(strings.Join([]string{req.LastError, "截图上传失败：反馈附件存储暂不可用"}, "；"))
			return nil
		}
		return err
	}
	req.Screenshots = screenshots
	return nil
}

func (h *MobileSupportHandler) uploadScreenshots(c *gin.Context) ([]service.MobileFeedbackScreenshot, error) {
	if c.Request.MultipartForm == nil || c.Request.MultipartForm.File == nil {
		return []service.MobileFeedbackScreenshot{}, nil
	}
	files := c.Request.MultipartForm.File["screenshots"]
	if len(files) == 0 {
		return []service.MobileFeedbackScreenshot{}, nil
	}
	if len(files) > mobileFeedbackMaxScreenshots {
		return nil, errors.New("too many screenshots")
	}
	if h.feedbackAssetService == nil {
		return nil, service.ErrAnnouncementAssetStorageUnavailable
	}
	out := make([]service.MobileFeedbackScreenshot, 0, len(files))
	for _, header := range files {
		if header == nil {
			continue
		}
		if header.Size > mobileFeedbackMaxFileBytes {
			return nil, service.ErrAnnouncementAssetTooLarge
		}
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, mobileFeedbackMaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(data)) > mobileFeedbackMaxFileBytes {
			return nil, service.ErrAnnouncementAssetTooLarge
		}
		asset, err := h.feedbackAssetService.Upload(c.Request.Context(), header.Filename, header.Header.Get("Content-Type"), data)
		if err != nil {
			return nil, err
		}
		out = append(out, service.MobileFeedbackScreenshot{
			URL:         asset.URL,
			FileName:    header.Filename,
			ContentType: asset.ContentType,
			ByteSize:    asset.ByteSize,
		})
	}
	return out, nil
}

func (h *MobileSupportHandler) List(c *gin.Context) {
	subject, ok := mobileSupportSubject(c)
	if !ok {
		return
	}
	page, err := mobileSupportPositiveInt(c.DefaultQuery("page", "1"))
	if err != nil {
		response.BadRequest(c, "分页参数不正确")
		return
	}
	pageSize, err := mobileSupportPositiveInt(c.DefaultQuery("page_size", "20"))
	if err != nil {
		response.BadRequest(c, "分页参数不正确")
		return
	}
	result, err := h.service.ListUserMobileFeedback(c.Request.Context(), subject.UserID, service.MobileFeedbackListFilter{
		Status: c.Query("status"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *MobileSupportHandler) Detail(c *gin.Context) {
	subject, id, ok := h.subjectAndTicketID(c)
	if !ok {
		return
	}
	ticket, err := h.service.GetUserMobileFeedback(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *MobileSupportHandler) AddMessage(c *gin.Context) {
	subject, id, ok := h.subjectAndTicketID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileSupportMessageMaxBytes)
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		response.BadRequest(c, "回复内容不能为空")
		return
	}
	ticket, err := h.service.AddUserMobileFeedbackMessage(c.Request.Context(), subject.UserID, id, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *MobileSupportHandler) Close(c *gin.Context) {
	subject, id, ok := h.subjectAndTicketID(c)
	if !ok {
		return
	}
	ticket, err := h.service.CloseUserMobileFeedback(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, ticket)
}

func (h *MobileSupportHandler) subjectAndTicketID(c *gin.Context) (middleware.AuthSubject, int64, bool) {
	subject, ok := mobileSupportSubject(c)
	if !ok {
		return middleware.AuthSubject{}, 0, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "工单编号不正确")
		return middleware.AuthSubject{}, 0, false
	}
	return subject, id, true
}

func mobileSupportSubject(c *gin.Context) (middleware.AuthSubject, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return middleware.AuthSubject{}, false
	}
	return subject, true
}

func mobileSupportPositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}
