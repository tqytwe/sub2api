package handler

import (
	"context"
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
	record, err := h.service.CreateMobileFeedback(c.Request.Context(), subject.UserID, service.MobileFeedbackInput{
		Title: req.Title, Category: req.Category, Content: req.Content,
		AppVersion: req.AppVersion, Platform: req.Platform, DeviceModel: req.DeviceModel,
		AndroidVersion: req.AndroidVersion, SystemVersion: req.SystemVersion,
		GroupName: req.GroupName, GroupID: req.GroupID, BackendURL: req.BackendURL,
		LastError: req.LastError, CrashLog: req.CrashLog, DeviceInfo: req.DeviceInfo,
		Screenshots: req.Screenshots,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, record)
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
		return h.parseMultipartCreateRequest(c)
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req mobileSupportCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, err
	}
	return req, nil
}

func (h *MobileSupportHandler) parseMultipartCreateRequest(c *gin.Context) (mobileSupportCreateRequest, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileFeedbackMaxRequestBytes)
	var req mobileSupportCreateRequest
	if err := c.Request.ParseMultipartForm(mobileFeedbackMaxRequestBytes); err != nil {
		return req, err
	}
	req.Title = c.PostForm("title")
	req.Category = c.PostForm("category")
	req.Content = c.PostForm("content")
	req.AppVersion = c.PostForm("app_version")
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
			return req, err
		}
		req.GroupID = &parsed
	}
	if raw := strings.TrimSpace(c.PostForm("device_info")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req.DeviceInfo); err != nil {
			return req, err
		}
	}
	screenshots, err := h.uploadScreenshots(c)
	if err != nil {
		if errors.Is(err, service.ErrAnnouncementAssetStorageUnavailable) {
			req.LastError = strings.TrimSpace(strings.Join([]string{req.LastError, "截图上传失败：反馈附件存储暂不可用"}, "；"))
			return req, nil
		}
		return req, err
	}
	req.Screenshots = screenshots
	return req, nil
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
