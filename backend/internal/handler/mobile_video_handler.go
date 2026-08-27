package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const MobileVideoJobRequestBodyLimit int64 = 256 << 10

var (
	errMobileVideoExecutionIdentityUnavailable = errors.New("mobile video execution identity is unavailable")
	errMobileVideoFundingUnavailable           = errors.New("mobile video funding is unavailable")
)

type mobileVideoTaskStore interface {
	CreateVideoTask(context.Context, int64, service.MobileTaskCreateInput, service.MobileVideoJobCreateInput) (*service.MobileTask, error)
	List(context.Context, int64, service.MobileTaskListFilter) (*service.MobileTaskPage, error)
	Get(context.Context, int64, string) (*service.MobileTask, error)
	Cancel(context.Context, int64, string) (*service.MobileTask, error)
	RetryVideoTask(context.Context, int64, string, string, service.MobileVideoJobCreateInput) (*service.MobileTask, error)
}

type mobileVideoExecutionIssuer interface {
	IssueMobileVideoExecutionSession(context.Context, int64, int64) (*service.NextChatManagedSession, error)
	CaptureMobileVideoExecutionSnapshot(context.Context, int64, int64, int64) (*service.MobileVideoExecutionSnapshot, error)
}

// MobileVideoHandler owns the authenticated mobile video BFF. Public tasks are
// deliberately content-free; prompts, reference IDs, provider task IDs and
// pinned execution keys live only in the private video job row.
type MobileVideoHandler struct {
	tasks     mobileVideoTaskStore
	resolver  service.MobileVideoAvailabilityResolver
	execution mobileVideoExecutionIssuer
	jobs      service.MobileVideoJobStore
	storage   service.MobileAssetStorage
	billing   service.UsageBillingRepository
}

type mobileVideoJobRequest struct {
	Purpose           string   `json:"purpose"`
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

type mobileVideoEstimate struct {
	GroupID          int64   `json:"group_id"`
	Model            string  `json:"model"`
	Resolution       string  `json:"resolution"`
	DurationSeconds  int     `json:"duration_seconds"`
	UnitPriceUSD     float64 `json:"unit_price_usd"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
	Currency         string  `json:"currency"`
}

func NewMobileVideoHandler(
	tasks *service.MobileTaskService,
	resolver service.MobileVideoAvailabilityResolver,
	execution *service.APIKeyService,
	jobs service.MobileVideoJobStore,
	storage service.MobileAssetStorage,
	billing service.UsageBillingRepository,
) *MobileVideoHandler {
	return newMobileVideoHandlerWithDependencies(tasks, resolver, execution, jobs, storage, billing)
}

func newMobileVideoHandlerWithDependencies(
	tasks mobileVideoTaskStore,
	resolver service.MobileVideoAvailabilityResolver,
	execution mobileVideoExecutionIssuer,
	jobs service.MobileVideoJobStore,
	storage service.MobileAssetStorage,
	billing ...service.UsageBillingRepository,
) *MobileVideoHandler {
	var billingRepo service.UsageBillingRepository
	if len(billing) > 0 {
		billingRepo = billing[0]
	}
	return &MobileVideoHandler{tasks: tasks, resolver: resolver, execution: execution, jobs: jobs, storage: storage, billing: billingRepo}
}

func (h *MobileVideoHandler) Bootstrap(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	bootstrap, err := h.resolver.Bootstrap(c.Request.Context(), userID)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, bootstrap)
}

func (h *MobileVideoHandler) Models(c *gin.Context) {
	h.Bootstrap(c)
}

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
	resolved, err := h.resolver.Resolve(c.Request.Context(), userID, input.GroupID, input.Model)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	if err := validateMobileVideoResolvedInput(input, resolved); err != nil {
		h.writeVideoError(c, err)
		return
	}
	price := resolved.PriceForResolution(input.Resolution)
	if price == nil {
		h.writeVideoError(c, &service.MobileVideoAvailabilityError{Code: service.MobileVideoSuppressionPriceMissing})
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, mobileVideoEstimate{
		GroupID: input.GroupID, Model: input.Model, Resolution: input.Resolution,
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
	resolved, err := h.resolver.Resolve(c.Request.Context(), userID, input.GroupID, input.Model)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	if err := validateMobileVideoResolvedInput(input, resolved); err != nil {
		h.writeVideoError(c, err)
		return
	}
	if err := h.prepareFundedVideoInput(c.Request.Context(), userID, &input, resolved); err != nil {
		h.writeVideoFundingError(c, err)
		return
	}
	taskInput, err := service.BuildMobileVideoTaskCreateInput(input)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	task, err := h.tasks.CreateVideoTask(c.Request.Context(), userID, taskInput, input)
	if writeMobileTaskError(c, err) {
		return
	}
	if err := h.activateFundedVideoTask(c.Request.Context(), userID, task.ID); err != nil {
		h.writeVideoFundingError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Accepted(c, gin.H{"task": task, "execution": "queued"})
}

func (h *MobileVideoHandler) List(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	page, pageSize, err := parseMobileVideoPagination(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	items, err := h.tasks.List(c.Request.Context(), userID, service.MobileTaskListFilter{
		Kind: service.MobileTaskKindVideo, Page: page, PageSize: pageSize,
	})
	if writeMobileTaskError(c, err) {
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, items)
}

func (h *MobileVideoHandler) Get(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if writeMobileTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, task)
}

func (h *MobileVideoHandler) Cancel(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	task, err := h.tasks.Get(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	cancelled, err := h.tasks.Cancel(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	// A submitted upstream task still has to be polled and settled. Record the
	// public cancellation only; the worker will release before dispatch or
	// capture a completed provider task without publishing an artifact.
	if err := h.jobs.MarkClientCancelled(c.Request.Context(), id); err != nil && !errors.Is(err, service.ErrMobileVideoJobNotFound) {
		log.Printf("mobile video cancellation marker failed: task=%s err=%v", id, err)
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, cancelled)
}

func (h *MobileVideoHandler) Retry(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	task, err := h.tasks.Get(c.Request.Context(), userID, id)
	if writeMobileTaskError(c, err) {
		return
	}
	if task.Kind != service.MobileTaskKindVideo {
		response.NotFound(c, "视频任务不存在")
		return
	}
	var request mobileVideoRetryRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.ClientRequestID) == "" {
		response.BadRequest(c, "重试请求标识不能为空")
		return
	}
	source, err := h.jobs.Get(c.Request.Context(), userID, id)
	if errors.Is(err, service.ErrMobileVideoJobNotFound) {
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频重试任务暂时无法读取", "VIDEO_REQUEST_PERSIST_FAILED", nil)
		return
	}
	if err != nil || source == nil {
		response.InternalError(c, "读取视频重试任务失败")
		return
	}
	input := source.CreateInput()
	input.ClientRequestID = strings.TrimSpace(request.ClientRequestID)
	// A retry is a new authorization and pricing decision. It may reuse the
	// private prompt/reference payload, but never an old key snapshot or hold.
	input.ExecutionSnapshot = nil
	input.UnitPriceUSD = 0
	input.RateMultiplier = 0
	input.HoldAmount = 0
	resolved, err := h.resolver.Resolve(c.Request.Context(), userID, input.GroupID, input.Model)
	if err != nil {
		h.writeVideoError(c, err)
		return
	}
	if err := validateMobileVideoResolvedInput(input, resolved); err != nil {
		h.writeVideoError(c, err)
		return
	}
	if err := h.prepareFundedVideoInput(c.Request.Context(), userID, &input, resolved); err != nil {
		h.writeVideoFundingError(c, err)
		return
	}
	retry, err := h.tasks.RetryVideoTask(c.Request.Context(), userID, id, input.ClientRequestID, input)
	if writeMobileTaskError(c, err) {
		return
	}
	if err := h.activateFundedVideoTask(c.Request.Context(), userID, retry.ID); err != nil {
		h.writeVideoFundingError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	response.Created(c, retry)
}

func (h *MobileVideoHandler) Content(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	job, err := h.jobs.Get(c.Request.Context(), userID, strings.TrimSpace(c.Param("id")))
	if errors.Is(err, service.ErrMobileVideoJobNotFound) {
		response.NotFound(c, "视频任务不存在")
		return
	}
	if err != nil || job == nil {
		response.InternalError(c, "读取视频结果失败")
		return
	}
	if strings.TrimSpace(job.ArtifactStorageKey) == "" {
		response.Error(c, http.StatusConflict, "视频结果尚未落盘")
		return
	}
	body, contentType, err := h.storage.Open(c.Request.Context(), job.ArtifactStorageKey)
	if err != nil {
		response.NotFound(c, "视频文件不存在")
		return
	}
	defer func() { _ = body.Close() }()
	if strings.TrimSpace(job.ArtifactContentType) != "" {
		contentType = job.ArtifactContentType
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", `inline; filename="video.mp4"`)
	c.DataFromReader(http.StatusOK, job.ArtifactByteSize, contentType, body, nil)
}

func (h *MobileVideoHandler) AcknowledgeContent(c *gin.Context) {
	userID, ok := mobileVideoUserID(c)
	if !ok || !h.available(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	job, err := h.jobs.Get(c.Request.Context(), userID, id)
	if errors.Is(err, service.ErrMobileVideoJobNotFound) {
		response.NotFound(c, "视频任务不存在")
		return
	}
	if err != nil || job == nil {
		response.InternalError(c, "读取视频结果失败")
		return
	}
	if storageKey := strings.TrimSpace(job.ArtifactStorageKey); storageKey != "" {
		// Clear the canonical reference before touching object storage. A process
		// crash or transient database failure can otherwise leave a completed job
		// pointing at an object that has already been deleted.
		if err := h.jobs.ReleaseArtifact(c.Request.Context(), userID, id); err != nil {
			response.InternalError(c, "清理临时视频结果失败")
			return
		}
		// Storage deletes are idempotent for both supported backends. The durable
		// release above is authoritative, so a temporary cleanup error must not
		// make the client retry against a now-released result.
		if err := h.storage.Delete(c.Request.Context(), storageKey); err != nil {
			log.Printf("mobile video artifact cleanup failed after release: task=%s err=%v", id, err)
		}
	}
	c.Header("Cache-Control", "private, no-store")
	response.Success(c, gin.H{"released": true})
}

func (h *MobileVideoHandler) available(c *gin.Context) bool {
	if h != nil && h.tasks != nil && h.resolver != nil && h.execution != nil && h.jobs != nil && h.storage != nil && h.billing != nil {
		return true
	}
	response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频服务暂时不可用", "VIDEO_SERVICE_UNAVAILABLE", nil)
	return false
}

func mobileVideoUserID(c *gin.Context) (int64, bool) {
	return mobileTaskUserID(c)
}

func parseMobileVideoJobRequest(c *gin.Context, requireClientRequestID bool) (service.MobileVideoJobCreateInput, error) {
	var request mobileVideoJobRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return service.MobileVideoJobCreateInput{}, service.ErrMobileVideoRequestTooLarge
		}
		return service.MobileVideoJobCreateInput{}, service.ErrMobileVideoPurposeInvalid
	}
	if request.Purpose != service.NextChatSessionPurposeVideo {
		return service.MobileVideoJobCreateInput{}, service.ErrMobileVideoPurposeInvalid
	}
	clientRequestID := strings.TrimSpace(request.ClientRequestID)
	if !requireClientRequestID {
		clientRequestID = "video-estimate"
	}
	input := service.MobileVideoJobCreateInput{
		GroupID: request.GroupID, ExecutionAPIKeyID: 1, Adapter: "pending",
		Model: strings.TrimSpace(request.Model), Prompt: strings.TrimSpace(request.Prompt),
		Resolution: strings.TrimSpace(request.Resolution), Ratio: strings.TrimSpace(request.Ratio),
		DurationSeconds: request.DurationSeconds, GenerateAudio: request.GenerateAudio, Watermark: request.Watermark,
		ReferenceAssetIDs: append([]string(nil), request.ReferenceAssetIDs...), ClientRequestID: clientRequestID,
	}
	return input, service.ValidateMobileVideoJobCreateInput(input)
}

func validateMobileVideoResolvedInput(input service.MobileVideoJobCreateInput, resolved *service.MobileVideoResolvedModel) error {
	if resolved == nil || !resolved.Capabilities.SupportsOperation("generate") || !resolved.Capabilities.SupportsResolution(input.Resolution) || !resolved.Capabilities.SupportsDuration(input.DurationSeconds) || !mobileVideoAllowedString(resolved.Capabilities.SupportedRatios, input.Ratio) {
		return service.ErrMobileVideoCapabilityUnavailable
	}
	if input.GenerateAudio && !resolved.Capabilities.GenerateAudio {
		return service.ErrMobileVideoCapabilityUnavailable
	}
	if input.Watermark && !resolved.Capabilities.Watermark {
		return service.ErrMobileVideoCapabilityUnavailable
	}
	// The first capability contract does not advertise adapter-specific media
	// reference payloads. Reject them explicitly until an adapter declares and
	// implements a lossless, owner-scoped forwarding format.
	if len(input.ReferenceAssetIDs) > 0 {
		return service.ErrMobileVideoCapabilityUnavailable
	}
	if resolved.PriceForResolution(input.Resolution) == nil {
		return &service.MobileVideoAvailabilityError{Code: service.MobileVideoSuppressionPriceMissing}
	}
	return nil
}

func mobileVideoAllowedString(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}

func isStrictMobileVideoExecutionSession(session *service.NextChatManagedSession, userID int64) bool {
	return session != nil && session.UserID == userID && session.KeyID > 0 && strings.TrimSpace(session.APIKey) != "" && session.Purpose == service.NextChatSessionPurposeVideo
}

// prepareFundedVideoInput performs all mutable authorization work before a
// public task is created. The durable row receives only a redacted execution
// snapshot and the exact quoted hold; workers must not recompute either from
// live group membership or later price changes.
func (h *MobileVideoHandler) prepareFundedVideoInput(
	ctx context.Context,
	userID int64,
	input *service.MobileVideoJobCreateInput,
	resolved *service.MobileVideoResolvedModel,
) error {
	if h == nil || h.execution == nil || input == nil || resolved == nil {
		return errMobileVideoExecutionIdentityUnavailable
	}
	session, err := h.execution.IssueMobileVideoExecutionSession(ctx, userID, input.GroupID)
	if err != nil || !isStrictMobileVideoExecutionSession(session, userID) {
		return errMobileVideoExecutionIdentityUnavailable
	}
	snapshot, err := h.execution.CaptureMobileVideoExecutionSnapshot(ctx, userID, input.GroupID, session.KeyID)
	if err != nil || snapshot == nil {
		return errMobileVideoExecutionIdentityUnavailable
	}
	price := resolved.PriceForResolution(input.Resolution)
	if price == nil {
		return &service.MobileVideoAvailabilityError{Code: service.MobileVideoSuppressionPriceMissing}
	}
	holdAmount, err := service.ValidateMobileVideoHoldAmount(*price, input.DurationSeconds, snapshot.EffectiveVideoRateMultiplier)
	if err != nil {
		return err
	}
	input.ExecutionAPIKeyID = session.KeyID
	input.ExecutionSnapshot = snapshot
	input.UnitPriceUSD = *price
	input.RateMultiplier = snapshot.EffectiveVideoRateMultiplier
	input.HoldAmount = holdAmount
	input.Adapter = resolved.Adapter
	return nil
}

// activateFundedVideoTask turns a durable funding row into a worker-claimable
// job. The sequence intentionally uses the persisted row after Create: an
// idempotent client retry must retain the original snapshot and hold instead
// of charging at a changed price or using a new execution key.
func (h *MobileVideoHandler) activateFundedVideoTask(ctx context.Context, userID int64, taskID string) error {
	if h == nil || h.jobs == nil || h.billing == nil {
		return errMobileVideoFundingUnavailable
	}
	job, err := h.jobs.Get(ctx, userID, taskID)
	if err != nil || job == nil || job.UserID != userID {
		return errMobileVideoFundingUnavailable
	}
	if job.State == service.MobileVideoJobStateQueued && job.BillingState == service.MobileVideoBillingStateReserved {
		return nil
	}
	if job.State == service.MobileVideoJobStateFunding &&
		(job.BillingState == service.MobileVideoBillingStateReleasing || job.BillingState == service.MobileVideoBillingStateReleased) {
		// A prior queue/cancellation compensation may have released the balance
		// hold but lost its final database acknowledgement. Never reactivate that
		// row: finish the idempotent compensation and require a new request.
		if err := h.refundUnactivatedVideoTask(ctx, userID, job, job.ClientCancelledAt != nil); err != nil {
			return err
		}
		if job.ClientCancelledAt != nil {
			return service.ErrMobileVideoTaskCancelled
		}
		return service.ErrMobileVideoQueueLimit
	}
	if job.State != service.MobileVideoJobStateFunding {
		// A repeat client request can observe a terminal private task. Do not
		// create a second hold for a job that is already being reconciled.
		return nil
	}
	if job.ClientCancelledAt != nil && job.BillingState == service.MobileVideoBillingStateFunding {
		if err := h.jobs.MarkCancelled(ctx, job.TaskID, ""); err != nil {
			return err
		}
		return service.ErrMobileVideoTaskCancelled
	}
	if job.BillingState == service.MobileVideoBillingStateFunding {
		if err := service.ReserveMobileVideoBalance(ctx, h.billing, job); err != nil {
			if service.IsMobileVideoInsufficientBalance(err) {
				if discardErr := h.jobs.DiscardUnfunded(ctx, userID, job.TaskID); discardErr != nil {
					return errMobileVideoFundingUnavailable
				}
				return err
			}
			return service.MobileVideoFundingError("reserve", err)
		}
		if err := h.jobs.MarkFundingReserved(ctx, job.TaskID); err != nil {
			// The hold may have committed while the acknowledgement was lost. A
			// durable reread distinguishes that case from a failed state update;
			// never release blindly because a worker could later be activated.
			refreshed, refreshErr := h.jobs.Get(ctx, userID, job.TaskID)
			if refreshErr != nil || refreshed == nil || refreshed.State != service.MobileVideoJobStateFunding || refreshed.BillingState != service.MobileVideoBillingStateReserved {
				return errMobileVideoFundingUnavailable
			}
			job = refreshed
		} else {
			job.BillingState = service.MobileVideoBillingStateReserved
		}
	}
	if job.BillingState != service.MobileVideoBillingStateReserved {
		return errMobileVideoFundingUnavailable
	}
	if err := h.jobs.ActivateReserved(ctx, userID, job.TaskID, service.MobileVideoActiveJobsPerUser); err != nil {
		if errors.Is(err, service.ErrMobileVideoQueueLimit) {
			if releaseErr := h.refundUnactivatedVideoTask(ctx, userID, job, false); releaseErr != nil {
				return errMobileVideoFundingUnavailable
			}
			return err
		}
		if errors.Is(err, service.ErrMobileVideoTaskCancelled) {
			if releaseErr := h.refundUnactivatedVideoTask(ctx, userID, job, true); releaseErr != nil {
				return errMobileVideoFundingUnavailable
			}
			return err
		}
		// Activation is idempotent. It can have committed just before a transient
		// reply failure, so inspect once before declaring an unavailable service.
		refreshed, refreshErr := h.jobs.Get(ctx, userID, job.TaskID)
		if refreshErr == nil && refreshed != nil && refreshed.State == service.MobileVideoJobStateQueued && refreshed.BillingState == service.MobileVideoBillingStateReserved {
			return nil
		}
		return errMobileVideoFundingUnavailable
	}
	return nil
}

func (h *MobileVideoHandler) refundUnactivatedVideoTask(ctx context.Context, userID int64, job *service.MobileVideoJob, cancelled bool) error {
	if h == nil || h.jobs == nil || h.billing == nil || job == nil || job.UserID != userID || job.State != service.MobileVideoJobStateFunding {
		return errMobileVideoFundingUnavailable
	}
	if job.BillingState == service.MobileVideoBillingStateReserved {
		// Fence activation before invoking the external, idempotent release. If
		// the release commits but this request loses its acknowledgement, a retry
		// observes `releasing` and resumes compensation instead of dispatching an
		// unfunded task.
		if err := h.jobs.MarkFundingReleasePending(ctx, job.TaskID); err != nil {
			return err
		}
		job.BillingState = service.MobileVideoBillingStateReleasing
	}
	if job.BillingState != service.MobileVideoBillingStateReleasing && job.BillingState != service.MobileVideoBillingStateReleased {
		return errMobileVideoFundingUnavailable
	}
	if job.BillingState == service.MobileVideoBillingStateReleasing {
		if err := service.ReleaseMobileVideoBalance(ctx, h.billing, job); err != nil {
			return service.MobileVideoFundingError("release", err)
		}
		if err := h.jobs.MarkFundingReleased(ctx, job.TaskID); err != nil {
			return err
		}
		job.BillingState = service.MobileVideoBillingStateReleased
	}
	if cancelled {
		return h.jobs.MarkCancelled(ctx, job.TaskID, "")
	}
	return h.jobs.DiscardUnfunded(ctx, userID, job.TaskID)
}

func parseMobileVideoPagination(c *gin.Context) (int, int, error) {
	page, pageSize := 1, 20
	if value := strings.TrimSpace(c.Query("page")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			return 0, 0, errors.New("页码不正确")
		}
		page = parsed
	}
	if value := strings.TrimSpace(c.Query("page_size")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, errors.New("每页数量不正确")
		}
		pageSize = parsed
	}
	return page, pageSize, nil
}

func (h *MobileVideoHandler) writeVideoError(c *gin.Context, err error) {
	code := service.MobileVideoAvailabilityCode(err)
	if code != "" {
		response.ErrorWithDetails(c, http.StatusConflict, "当前视频模型暂不可用", strings.ToUpper(code), nil)
		return
	}
	switch {
	case errors.Is(err, service.ErrMobileVideoRequestTooLarge):
		response.ErrorWithDetails(c, http.StatusRequestEntityTooLarge, "视频请求过大", "MOBILE_VIDEO_REQUEST_TOO_LARGE", nil)
	case errors.Is(err, service.ErrMobileVideoPurposeInvalid):
		response.ErrorWithDetails(c, http.StatusBadRequest, "视频请求必须指定 purpose=video", "VIDEO_PURPOSE_INVALID", nil)
	case errors.Is(err, service.ErrMobileVideoModelUnavailable):
		response.ErrorWithDetails(c, http.StatusNotFound, "当前分组没有可用的视频模型", "VIDEO_MODEL_UNAVAILABLE", nil)
	case errors.Is(err, service.ErrMobileVideoCapabilityUnavailable):
		response.ErrorWithDetails(c, http.StatusConflict, "当前视频模型不支持该请求", "VIDEO_CAPABILITY_UNAVAILABLE", nil)
	case errors.Is(err, service.ErrMobileVideoGroupRequired), errors.Is(err, service.ErrMobileVideoModelRequired), errors.Is(err, service.ErrMobileVideoPromptRequired), errors.Is(err, service.ErrMobileVideoResolutionInvalid), errors.Is(err, service.ErrMobileVideoDurationInvalid), errors.Is(err, service.ErrMobileVideoClientRequestIDRequired), errors.Is(err, service.ErrMobileVideoRatioInvalid), errors.Is(err, service.ErrMobileVideoReferenceInvalid):
		response.BadRequest(c, "视频请求参数不正确")
	default:
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频服务暂时不可用", "VIDEO_SERVICE_UNAVAILABLE", nil)
	}
}

func (h *MobileVideoHandler) writeVideoFundingError(c *gin.Context, err error) {
	if code := service.MobileVideoAvailabilityCode(err); code != "" {
		h.writeVideoError(c, err)
		return
	}
	switch {
	case errors.Is(err, service.ErrBatchImageInsufficientBalance):
		response.ErrorWithDetails(c, http.StatusPaymentRequired, "余额不足，无法创建视频任务", "VIDEO_INSUFFICIENT_BALANCE", nil)
	case errors.Is(err, service.ErrMobileVideoQueueLimit):
		response.ErrorWithDetails(c, http.StatusConflict, "视频任务队列已满，请稍后重试", "VIDEO_QUEUE_LIMIT_REACHED", nil)
	case errors.Is(err, service.ErrMobileVideoTaskCancelled):
		response.ErrorWithDetails(c, http.StatusConflict, "视频任务已取消", "VIDEO_TASK_CANCELLED", nil)
	case errors.Is(err, errMobileVideoExecutionIdentityUnavailable):
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频执行会话暂不可用", "VIDEO_EXECUTION_IDENTITY_UNAVAILABLE", nil)
	case errors.Is(err, service.ErrMobileVideoFundingInvalid):
		response.ErrorWithDetails(c, http.StatusConflict, "视频计费参数暂不可用", "VIDEO_FUNDING_INVALID", nil)
	default:
		response.ErrorWithDetails(c, http.StatusServiceUnavailable, "视频服务暂时不可用", "VIDEO_FUNDING_UNAVAILABLE", nil)
	}
}
