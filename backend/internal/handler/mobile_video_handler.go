package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileVideoTaskStore interface {
	Create(context.Context, int64, service.MobileTaskCreateInput) (*service.MobileTask, error)
	List(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error)
	Get(context.Context, int64, string) (*service.MobileTask, error)
	Cancel(context.Context, int64, string) (*service.MobileTask, error)
	Retry(context.Context, int64, string, string) (*service.MobileTask, error)
}

type mobileVideoGroupStore interface {
	GetNextChatSelectableGroups(context.Context, int64) ([]service.Group, error)
}

type MobileVideoHandler struct {
	tasks  mobileVideoTaskStore
	groups mobileVideoGroupStore
	jobs   *service.MobileVideoJobService
}

type mobileVideoJobRequest struct {
	GroupID           int64    `json:"group_id"`
	Model             string   `json:"model"`
	Prompt            string   `json:"prompt"`
	Resolution        string   `json:"resolution"`
	Ratio             string   `json:"ratio"`
	DurationSeconds   int      `json:"duration_seconds"`
	GenerateAudio     bool     `json:"generate_audio"`
	Watermark         bool     `json:"watermark"`
	ReferenceAssetIDs []string `json:"reference_asset_ids"`
	ClientRequestID   string   `json:"client_request_id"`
}

type mobileVideoRetryRequest struct {
	ClientRequestID string `json:"client_request_id"`
}

type mobileVideoGroup struct {
	ID                   int64                            `json:"id"`
	Name                 string                           `json:"name"`
	Platform             string                           `json:"platform"`
	VideoAvailable       bool                             `json:"video_available"`
	VideoUnavailableCode string                           `json:"video_unavailable_code,omitempty"`
	Models               []string                         `json:"models"`
	Capabilities         *service.MobileVideoCapabilities `json:"capabilities,omitempty"`
}

type mobileVideoBootstrap struct {
	ProtocolVersion     int                `json:"protocol_version"`
	CapabilitiesVersion string             `json:"capabilities_version"`
	Groups              []mobileVideoGroup `json:"groups"`
}

type mobileVideoEstimate struct {
	GroupID          int64   `json:"group_id"`
	Model            string  `json:"model"`
	Resolution       string  `json:"resolution"`
	DurationSeconds  int     `json:"duration_seconds"`
	UnitPriceUSD     float64 `json:"unit_price_usd"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Currency         string  `json:"currency"`
}

func NewMobileVideoHandler(taskService *service.MobileTaskService, groups *service.APIKeyService, jobs ...*service.MobileVideoJobService) *MobileVideoHandler {
	var jobService *service.MobileVideoJobService
	if len(jobs) > 0 {
		jobService = jobs[0]
	}
	return &MobileVideoHandler{tasks: taskService, groups: groups, jobs: jobService}
}

func newMobileVideoHandlerWithDependencies(tasks mobileVideoTaskStore, groups mobileVideoGroupStore) *MobileVideoHandler {
	return &MobileVideoHandler{tasks: tasks, groups: groups}
}

func (h *MobileVideoHandler) Bootstrap(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	groups, err := h.groups.GetNextChatSelectableGroups(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "读取视频分组失败")
		return
	}
	items := make([]mobileVideoGroup, 0, len(groups))
	for _, group := range groups {
		items = append(items, h.videoGroupSummary(group))
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, mobileVideoBootstrap{
		ProtocolVersion:     mobileProtocolVersion,
		CapabilitiesVersion: service.NextChatVideoCapabilitiesVersion,
		Groups:              items,
	})
}

func (h *MobileVideoHandler) Models(c *gin.Context) { h.Bootstrap(c) }

func (h *MobileVideoHandler) Estimate(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	input, err := parseMobileVideoJobRequest(c, false)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	group, err := h.authorizedVideoGroup(c.Request.Context(), userID, input.GroupID, input.Model)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	price := group.GetVideoPriceForModel(input.Model, input.Resolution)
	if price == nil {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	resolution, _ := service.LookupVideoBillingResolution(input.Resolution)
	response.Success(c, mobileVideoEstimate{
		GroupID: input.GroupID, Model: strings.TrimSpace(input.Model), Resolution: resolution,
		DurationSeconds: input.DurationSeconds, UnitPriceUSD: *price,
		EstimatedCostUSD: *price * float64(input.DurationSeconds), Currency: "USD",
	})
}

func (h *MobileVideoHandler) Create(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	input, err := parseMobileVideoJobRequest(c, true)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	group, err := h.authorizedVideoGroup(c.Request.Context(), userID, input.GroupID, input.Model)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	capabilities, supported := service.ResolveVideoModelCapabilities(group, group.Platform, input.Model)
	if !supported {
		h.writeVideoError(c, service.ErrMobileVideoModelUnavailable)
		return
	}
	if !capabilities.SupportsResolution(input.Resolution) || !capabilities.SupportsDuration(input.DurationSeconds) {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	if input.GenerateAudio && !capabilities.GenerateAudio {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	if input.Watermark && !capabilities.Watermark {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	if input.Ratio != "" && !containsVideoOperation(capabilities.SupportedRatios, input.Ratio) {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	if len(input.ReferenceAssetIDs) > capabilities.MaxReferenceImages+capabilities.MaxReferenceVideos+capabilities.MaxReferenceAudios {
		h.writeVideoError(c, service.ErrMobileVideoCapabilityUnavailable)
		return
	}
	taskInput, err := service.BuildMobileVideoTaskCreateInput(input)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	task, err := h.tasks.Create(c.Request.Context(), userID, taskInput)
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if h.jobs != nil {
		if err := h.jobs.Create(c.Request.Context(), userID, task.ID, input, group.Platform); err != nil {
			// Do not leave a publicly visible queued task with no durable private
			// request. The transition is optional for test doubles, but the real
			// MobileTaskService always implements it.
			if transitioner, ok := h.tasks.(interface {
				Transition(context.Context, int64, string, service.MobileTaskTransitionInput) (*service.MobileTask, error)
			}); ok {
				_, _ = transitioner.Transition(c.Request.Context(), userID, task.ID, service.MobileTaskTransitionInput{
					Status: service.MobileTaskStatusFailed,
					Error:  &service.MobileTaskError{Code: "VIDEO_REQUEST_PERSIST_FAILED", Message: "视频任务请求无法持久化", Retryable: true},
				})
			}
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频任务暂时无法排队", "VIDEO_REQUEST_PERSIST_FAILED", nil)
			return
		}
	}
	c.Header("Cache-Control", "private, no-store")
	response.Accepted(c, gin.H{"task": task, "execution": "queued"})
}

func (h *MobileVideoHandler) List(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	pageNumber, pageSize, err := parseMobileVideoPagination(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page, err := h.tasks.List(c.Request.Context(), userID, service.MobileTaskListFilter{Kind: service.MobileTaskKindVideo, Page: pageNumber, PageSize: pageSize})
	if writeMobileVideoTaskError(c, err) {
		return
	}
	response.Success(c, page)
}

func (h *MobileVideoHandler) Get(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	response.Success(c, task)
}

// Content returns the task projection and any server-owned artifacts. Binary
// bytes remain behind the authenticated asset/content endpoints; this handler
// never proxies an untrusted provider URL.
func (h *MobileVideoHandler) Content(c *gin.Context) {
	h.Get(c)
}

func (h *MobileVideoHandler) Cancel(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	current, err := h.tasks.Get(c.Request.Context(), userID, id)
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if current.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	task, err := h.tasks.Cancel(c.Request.Context(), userID, id)
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if h.jobs != nil {
		// A worker holding the lease will observe the cancelled public task and
		// release its private row; queued rows can be cancelled immediately.
		_ = h.jobs.MarkCancelled(c.Request.Context(), id, "")
	}
	response.Success(c, task)
}

func (h *MobileVideoHandler) Retry(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	task, err := h.tasks.Get(c.Request.Context(), userID, id)
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	var source *service.MobileVideoJob
	if h.jobs != nil {
		source, err = h.jobs.Get(c.Request.Context(), userID, id)
		if err != nil {
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频重试任务暂时无法读取", "VIDEO_REQUEST_PERSIST_FAILED", nil)
			return
		}
	}
	var request mobileVideoRetryRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.ClientRequestID) == "" {
		response.BadRequest(c, "重试请求标识不能为空")
		return
	}
	retry, err := h.tasks.Retry(c.Request.Context(), userID, id, strings.TrimSpace(request.ClientRequestID))
	if writeMobileVideoTaskError(c, err) {
		return
	}
	if h.jobs != nil {
		if createErr := h.jobs.Create(c.Request.Context(), userID, retry.ID, source.CreateInput(), source.Provider); createErr != nil {
			response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频重试任务暂时无法排队", "VIDEO_REQUEST_PERSIST_FAILED", nil)
			return
		}
	}
	response.Created(c, retry)
}

func (h *MobileVideoHandler) SaveAsAsset(c *gin.Context) {
	// A worker must first attach a completed artifact. Accepting a save request
	// before that would create an asset without bytes and lose accounting.
	response.ErrorWithDetails(c, http.StatusConflict, "视频任务尚未产生可保存的结果", "VIDEO_ARTIFACT_NOT_READY", nil)
}

func (h *MobileVideoHandler) available(c *gin.Context) bool {
	if h != nil && h.tasks != nil && h.groups != nil {
		return true
	}
	response.Error(c, http.StatusServiceUnavailable, "视频任务服务暂不可用")
	return false
}

func (h *MobileVideoHandler) authorizedVideoGroup(ctx context.Context, userID, groupID int64, model string) (service.Group, error) {
	if h == nil || h.groups == nil {
		return service.Group{}, service.ErrMobileVideoCapabilityUnavailable
	}
	groups, err := h.groups.GetNextChatSelectableGroups(ctx, userID)
	if err != nil {
		return service.Group{}, err
	}
	for _, group := range groups {
		if group.ID != groupID {
			continue
		}
		if !group.HasVideoGenerationCapability() {
			return service.Group{}, service.ErrMobileVideoCapabilityUnavailable
		}
		if _, ok := service.ResolveVideoModelCapabilities(group, group.Platform, model); !ok {
			return service.Group{}, service.ErrMobileVideoModelUnavailable
		}
		return group, nil
	}
	return service.Group{}, service.ErrMobileVideoCapabilityUnavailable
}

func (h *MobileVideoHandler) videoGroupSummary(group service.Group) mobileVideoGroup {
	models := append([]string(nil), group.ModelsListConfig.Models...)
	if strings.TrimSpace(group.DefaultMappedModel) != "" {
		models = append(models, strings.TrimSpace(group.DefaultMappedModel))
	}
	for model := range group.VideoModelPrices {
		models = append(models, model)
	}
	sort.Strings(models)
	unique := models[:0]
	seen := map[string]struct{}{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, exists := seen[strings.ToLower(model)]; exists {
			continue
		}
		seen[strings.ToLower(model)] = struct{}{}
		unique = append(unique, model)
	}
	result := mobileVideoGroup{ID: group.ID, Name: group.Name, Platform: group.Platform, Models: unique}
	if !group.HasVideoGenerationCapability() {
		result.VideoUnavailableCode = "VIDEO_GROUP_UNAVAILABLE"
		return result
	}
	for _, model := range unique {
		if capability, ok := service.ResolveVideoModelCapabilities(group, group.Platform, model); ok {
			result.VideoAvailable = true
			result.Capabilities = &service.MobileVideoCapabilities{
				TextToVideo: true, ImageToVideo: containsVideoOperation(capability.Operations, "image_to_video"),
				VideoReference: containsVideoOperation(capability.Operations, "video_reference"),
				Resolutions:    capability.SupportedResolutions, Ratios: capability.SupportedRatios,
				Durations: capability.SupportedDurations, GenerateAudio: capability.GenerateAudio, Watermark: capability.Watermark,
			}
			break
		}
	}
	if !result.VideoAvailable {
		result.VideoUnavailableCode = "VIDEO_MODEL_UNAVAILABLE"
	}
	return result
}

func containsVideoOperation(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func parseMobileVideoJobRequest(c *gin.Context, requireClientRequestID bool) (service.MobileVideoJobCreateInput, error) {
	var request mobileVideoJobRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return service.MobileVideoJobCreateInput{}, service.ErrMobileVideoPromptRequired
	}
	input := service.MobileVideoJobCreateInput{
		GroupID: request.GroupID, Model: strings.TrimSpace(request.Model), Prompt: strings.TrimSpace(request.Prompt),
		Resolution: strings.TrimSpace(request.Resolution), Ratio: strings.TrimSpace(request.Ratio), DurationSeconds: request.DurationSeconds,
		GenerateAudio: request.GenerateAudio, Watermark: request.Watermark, ReferenceAssetIDs: request.ReferenceAssetIDs,
		ClientRequestID: strings.TrimSpace(request.ClientRequestID),
	}
	validationInput := input
	if !requireClientRequestID && validationInput.ClientRequestID == "" {
		validationInput.ClientRequestID = "estimate"
	}
	return input, service.ValidateMobileVideoJobCreateInput(validationInput)
}

func parseMobileVideoPagination(c *gin.Context) (int, int, error) {
	page, pageSize := 1, 20
	if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			return 0, 0, errors.New("页码不正确")
		}
		page = value
	}
	if raw := strings.TrimSpace(c.Query("page_size")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			return 0, 0, errors.New("每页数量不正确")
		}
		pageSize = value
	}
	return page, pageSize, nil
}

func mobileVideoUserID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "请先登录")
		return 0, false
	}
	return subject.UserID, true
}

func writeMobileVideoTaskError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, service.ErrMobileTaskNotFound):
		response.NotFound(c, "视频任务不存在")
	case errors.Is(err, service.ErrMobileTaskClientRequestConflict):
		response.ErrorWithDetails(c, http.StatusConflict, "请求标识已被其他视频任务使用", "MOBILE_VIDEO_REQUEST_CONFLICT", nil)
	case errors.Is(err, service.ErrMobileTaskNotCancellable), errors.Is(err, service.ErrMobileTaskNotRetryable):
		response.ErrorWithDetails(c, http.StatusConflict, "当前视频任务状态不允许此操作", "MOBILE_VIDEO_TASK_STATE_CONFLICT", nil)
	case errors.Is(err, service.ErrMobileTaskStoreUnavailable):
		response.Error(c, http.StatusServiceUnavailable, "视频任务服务暂不可用")
	default:
		response.InternalError(c, "视频任务操作失败")
	}
	return true
}

func (h *MobileVideoHandler) writeVideoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrMobileVideoCapabilityUnavailable):
		response.ErrorWithDetails(c, http.StatusForbidden, "当前分组没有可用的视频能力", "VIDEO_CAPABILITY_UNAVAILABLE", nil)
	case errors.Is(err, service.ErrMobileVideoModelUnavailable):
		response.ErrorWithDetails(c, http.StatusForbidden, "当前分组没有可用的视频模型", "VIDEO_MODEL_UNAVAILABLE", nil)
	case errors.Is(err, service.ErrMobileVideoGroupRequired), errors.Is(err, service.ErrMobileVideoModelRequired), errors.Is(err, service.ErrMobileVideoPromptRequired), errors.Is(err, service.ErrMobileVideoResolutionInvalid), errors.Is(err, service.ErrMobileVideoDurationInvalid), errors.Is(err, service.ErrMobileVideoClientRequestIDRequired), errors.Is(err, service.ErrMobileVideoRatioInvalid), errors.Is(err, service.ErrMobileVideoReferenceInvalid):
		response.ErrorWithDetails(c, http.StatusBadRequest, "视频参数不正确", "VIDEO_REQUEST_INVALID", nil)
	default:
		response.InternalError(c, "视频请求处理失败")
	}
}

// mobileVideoFingerprint is kept here as a small helper for tests and future
// worker persistence. It never includes secrets or a provider URL.
func mobileVideoFingerprint(input service.MobileVideoJobCreateInput) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(strings.Join([]string{strings.TrimSpace(input.Model), strings.TrimSpace(input.Resolution), strings.TrimSpace(input.Ratio), strings.TrimSpace(input.Prompt)}, "\x00")))
	return hex.EncodeToString(hash.Sum(nil))
}
