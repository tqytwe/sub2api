package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const mobileSupportMessageMaxBytes = 8 << 10

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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req struct {
		Title      string         `json:"title"`
		Category   string         `json:"category"`
		Content    string         `json:"content"`
		AppVersion string         `json:"app_version"`
		Platform   string         `json:"platform"`
		DeviceInfo map[string]any `json:"device_info"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		response.BadRequest(c, "工单内容不正确")
		return
	}
	record, err := h.service.CreateMobileFeedback(c.Request.Context(), subject.UserID, service.MobileFeedbackInput{
		Title: req.Title, Category: req.Category, Content: req.Content,
		AppVersion: req.AppVersion, Platform: req.Platform, DeviceInfo: req.DeviceInfo,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, record)
}

type MobileSupportHandler struct {
	service mobileSupportService
}

func NewMobileSupportHandler(service mobileSupportService) *MobileSupportHandler {
	return &MobileSupportHandler{service: service}
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
