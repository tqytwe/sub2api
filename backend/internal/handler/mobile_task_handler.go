package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileTaskStore interface {
	Create(context.Context, int64, service.MobileTaskCreateInput) (*service.MobileTask, error)
	List(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error)
	Get(context.Context, int64, string) (*service.MobileTask, error)
	Delete(context.Context, int64, string) (*service.MobileTaskDeleteResult, error)
	Cancel(context.Context, int64, string) (*service.MobileTask, error)
	Retry(context.Context, int64, string, string) (*service.MobileTask, error)
	Transition(context.Context, int64, string, service.MobileTaskTransitionInput) (*service.MobileTask, error)
}

type MobileTaskHandler struct {
	store mobileTaskStore
	push  *service.MobilePushService
}

type mobileTaskCreateRequest struct {
	Kind            service.MobileTaskKind      `json:"kind"`
	Operation       string                      `json:"operation"`
	ClientRequestID string                      `json:"client_request_id"`
	ParentTaskID    string                      `json:"parent_task_id"`
	Resource        *service.MobileTaskResource `json:"resource"`
}

type mobileTaskRetryRequest struct {
	ClientRequestID string `json:"client_request_id"`
}

type mobileTaskTransitionRequest struct {
	Status    service.MobileTaskStatus     `json:"status"`
	Progress  *int                         `json:"progress"`
	Resource  *service.MobileTaskResource  `json:"resource"`
	Artifacts []service.MobileTaskArtifact `json:"artifacts"`
	Error     *service.MobileTaskError     `json:"error"`
}

func NewMobileTaskHandler(taskService *service.MobileTaskService) *MobileTaskHandler {
	return &MobileTaskHandler{store: taskService}
}

func NewMobileTaskHandlerWithPush(taskService *service.MobileTaskService, push *service.MobilePushService) *MobileTaskHandler {
	return &MobileTaskHandler{store: taskService, push: push}
}

func newMobileTaskHandlerWithStore(store mobileTaskStore) *MobileTaskHandler {
	return &MobileTaskHandler{store: store}
}

func (h *MobileTaskHandler) Create(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	var request mobileTaskCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "任务参数不正确")
		return
	}
	request.Operation = strings.TrimSpace(request.Operation)
	request.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	if !service.IsValidMobileTaskKind(request.Kind) || request.Operation == "" || request.ClientRequestID == "" {
		response.BadRequest(c, "任务类型、操作和请求标识不能为空")
		return
	}
	task, err := h.store.Create(c.Request.Context(), userID, service.MobileTaskCreateInput{
		Kind:            request.Kind,
		Operation:       request.Operation,
		ClientRequestID: request.ClientRequestID,
		ParentTaskID:    strings.TrimSpace(request.ParentTaskID),
		Resource:        request.Resource,
	})
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, task)
}

func (h *MobileTaskHandler) List(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	filter, err := parseMobileTaskListFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page, err := h.store.List(c.Request.Context(), userID, filter)
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, page)
}

func (h *MobileTaskHandler) Get(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	task, err := h.store.Get(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, task)
}

func (h *MobileTaskHandler) Delete(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	result, err := h.store.Delete(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, result)
}

func (h *MobileTaskHandler) ImageHistory(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	filter, err := parseMobileTaskListFilter(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	filter.Kind = service.MobileTaskKindImage
	page, err := h.store.List(c.Request.Context(), userID, filter)
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, page)
}

func (h *MobileTaskHandler) DeleteImageHistory(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	task, err := h.store.Get(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindImage {
		response.NotFound(c, "生图历史不存在")
		return
	}
	result, err := h.store.Delete(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, result)
}

func (h *MobileTaskHandler) RetryImageHistory(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	task, err := h.store.Get(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindImage {
		response.NotFound(c, "生图历史不存在")
		return
	}
	var request mobileTaskRetryRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.ClientRequestID) == "" {
		response.BadRequest(c, "重试请求标识不能为空")
		return
	}
	retry, err := h.store.Retry(c.Request.Context(), userID, id, strings.TrimSpace(request.ClientRequestID))
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, retry)
}

func (h *MobileTaskHandler) Cancel(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	task, err := h.store.Cancel(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, task)
}

func (h *MobileTaskHandler) Retry(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok {
		return
	}
	if !h.available(c) {
		return
	}
	var request mobileTaskRetryRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.ClientRequestID) == "" {
		response.BadRequest(c, "重试请求标识不能为空")
		return
	}
	task, err := h.store.Retry(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")), strings.TrimSpace(request.ClientRequestID))
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, task)
}

func (h *MobileTaskHandler) Transition(c *gin.Context) {
	userID, ok := mobileTaskUserID(c)
	if !ok || !h.available(c) {
		return
	}
	var request mobileTaskTransitionRequest
	if err := c.ShouldBindJSON(&request); err != nil || !service.IsValidMobileTaskStatus(request.Status) {
		response.BadRequest(c, "任务状态不正确")
		return
	}
	task, err := h.store.Transition(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")), service.MobileTaskTransitionInput{
		Status: request.Status, Progress: request.Progress, Resource: request.Resource,
		Artifacts: request.Artifacts, Error: request.Error,
	})
	if writeMobileTaskError(c, err) {
		return
	}
	h.enqueueTerminalPush(c.Request.Context(), userID, task)
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, task)
}

func (h *MobileTaskHandler) enqueueTerminalPush(ctx context.Context, userID int64, task *service.MobileTask) {
	if h == nil || h.push == nil || task == nil || !service.IsTerminalMobileTaskStatus(task.Status) || task.Status == service.MobileTaskStatusCancelled {
		return
	}
	title, body := "任务已完成", "你在极速蹬发起的任务已处理完成"
	switch task.Status {
	case service.MobileTaskStatusFailed:
		title, body = "任务处理失败", "任务未能完成，可回到极速蹬查看并重试"
	case service.MobileTaskStatusPartial:
		title, body = "任务部分完成", "任务已有部分结果，可回到极速蹬继续处理"
	}
	_, _, _ = h.push.Enqueue(ctx, service.MobilePushEvent{
		UserID: userID, IdempotencyKey: "mobile-task:" + task.ID + ":" + string(task.Status),
		EventType: "task." + string(task.Status), SourceType: "mobile_task", SourceID: task.ID,
		TitleZh: title, BodyZh: body, Data: map[string]string{"kind": string(task.Kind), "status": string(task.Status)},
	})
}

func (h *MobileTaskHandler) available(c *gin.Context) bool {
	if h != nil && h.store != nil {
		return true
	}
	response.Error(c, http.StatusServiceUnavailable, "任务服务暂时不可用")
	return false
}

func mobileTaskUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return subject.UserID, true
}

func parseMobileTaskListFilter(c *gin.Context) (service.MobileTaskListFilter, error) {
	filter := service.MobileTaskListFilter{
		Kind:   service.MobileTaskKind(strings.TrimSpace(c.Query("kind"))),
		Status: service.MobileTaskStatus(strings.TrimSpace(c.Query("status"))),
		Page:   1,
	}
	if filter.Kind != "" && !service.IsValidMobileTaskKind(filter.Kind) {
		return filter, errors.New("任务类型不正确")
	}
	if filter.Status != "" && !service.IsValidMobileTaskStatus(filter.Status) {
		return filter, errors.New("任务状态不正确")
	}
	if value := c.Query("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return filter, errors.New("页码不正确")
		}
		filter.Page = page
	}
	if value := c.Query("page_size"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			return filter, errors.New("每页数量不正确")
		}
		filter.PageSize = pageSize
	}
	return filter, nil
}

func writeMobileTaskError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrMobileTaskNotFound):
		response.NotFound(c, "任务不存在")
	case errors.Is(err, service.ErrMobileTaskClientRequestConflict):
		response.Error(c, http.StatusConflict, "请求标识已被其他任务使用")
	case errors.Is(err, service.ErrMobileTaskNotCancellable):
		response.Error(c, http.StatusConflict, "当前任务状态不能取消")
	case errors.Is(err, service.ErrMobileTaskNotRetryable):
		response.Error(c, http.StatusConflict, "当前任务状态不能重试")
	case errors.Is(err, service.ErrMobileTaskInvalidTransition):
		response.Error(c, http.StatusConflict, "任务状态已经变化，请刷新后重试")
	case errors.Is(err, service.ErrMobileTaskInvalidKind),
		errors.Is(err, service.ErrMobileTaskInvalidStatus),
		errors.Is(err, service.ErrMobileTaskInvalidID),
		errors.Is(err, service.ErrMobileTaskInvalidOperation),
		errors.Is(err, service.ErrMobileTaskInvalidRequestID),
		errors.Is(err, service.ErrMobileTaskInvalidProgress),
		errors.Is(err, service.ErrMobileTaskInvalidResource),
		errors.Is(err, service.ErrMobileTaskInvalidArtifact):
		response.BadRequest(c, "任务参数不正确")
	case errors.Is(err, service.ErrMobileTaskStoreUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "任务服务暂时不可用")
	default:
		response.InternalError(c, "任务操作失败")
	}
	return true
}
