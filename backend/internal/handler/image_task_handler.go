package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AsyncImageHandler struct {
	tasks         *service.ImageTaskService
	openAI        *OpenAIGatewayHandler
	assetReader   service.ImageAssetReader
	imageResults  *service.OpenAIImageResultService
	apiKeys       imageTaskAPIKeyLoader
	subscriptions imageTaskSubscriptionLoader
	runtime       *service.ImageTaskWorkerRuntime
	execute       func(platform string, c *gin.Context)
}

type imageTaskAPIKeyLoader interface {
	GetByID(ctx context.Context, id int64) (*service.APIKey, error)
}

type imageTaskSubscriptionLoader interface {
	GetActiveSubscription(ctx context.Context, userID, groupID int64) (*service.UserSubscription, error)
}

func NewAsyncImageHandler(tasks *service.ImageTaskService, openAI *OpenAIGatewayHandler, imageStorage service.ImageStorage) *AsyncImageHandler {
	h := &AsyncImageHandler{tasks: tasks, openAI: openAI}
	if reader, ok := imageStorage.(service.ImageAssetReader); ok {
		h.assetReader = reader
	}
	h.execute = h.executeWithGateway
	return h
}

func ProvideAsyncImageHandler(
	tasks *service.ImageTaskService,
	openAI *OpenAIGatewayHandler,
	imageStorage service.ImageStorage,
	imageResults *service.OpenAIImageResultService,
	apiKeys *service.APIKeyService,
	subscriptions *service.SubscriptionService,
	queue service.ImageTaskQueue,
	runtimeState *service.ImageTaskRuntimeState,
	cfg *config.Config,
) *AsyncImageHandler {
	h := NewAsyncImageHandler(tasks, openAI, imageStorage)
	h.imageResults = imageResults
	h.apiKeys = apiKeys
	h.subscriptions = subscriptions
	if openAI != nil && openAI.gatewayService != nil {
		openAI.gatewayService.SetOpenAIImageResultService(imageResults)
	}
	h.runtime = service.NewImageTaskWorkerRuntime(queue, tasks, h, runtimeState, cfg)
	h.runtime.Start()
	return h
}

// enabled reports whether the async image task feature is available. Result
// storage is the enablement gate: without it the endpoints are fully disabled
// so that large base64 results never land in Redis.
func (h *AsyncImageHandler) enabled() bool {
	return h != nil && h.tasks != nil && h.tasks.Enabled()
}

// pollable reports whether task lookups can be served. It is deliberately weaker
// than enabled(): results already written to Redis stay readable after the
// feature is switched off, so an in-flight task is never stranded.
func (h *AsyncImageHandler) pollable() bool {
	return h != nil && h.tasks != nil && h.tasks.Pollable()
}

// Submit accepts the same payload as the synchronous Images endpoint and
// returns before the upstream image generation begins.
func (h *AsyncImageHandler) Submit(c *gin.Context) {
	if !h.enabled() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	runtime := h.tasks.RuntimeSnapshot(c.Request.Context())
	if !runtime.APIEnabled {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	if !runtime.Ready {
		imageTaskError(c, service.ErrImageTaskNotReady)
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	platform := ""
	if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	if platform != service.PlatformOpenAI && platform != service.PlatformGrok {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Images API is not supported for this platform")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		imageTaskJSONError(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if h == nil || h.tasks == nil {
		imageTaskError(c, service.ErrImageTaskUnavailable)
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			imageTaskJSONError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	if asyncImageRequestStreams(c.GetHeader("Content-Type"), body) {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "streaming image requests cannot be submitted as asynchronous tasks")
		return
	}
	if err := h.validateRequest(c, platform, body); err != nil {
		code := openAIImagesValidationErrorCode(err)
		if code == "" {
			code = "invalid_request_error"
		}
		imageTaskJSONTypedError(c, openAIImagesValidationErrorStatus(err), "invalid_request_error", code, err.Error())
		return
	}
	if !h.checkSecurityAuditBeforeSubmit(c, apiKey, platform, body) {
		return
	}

	task, replayed, err := h.tasks.Submit(c.Request.Context(), service.ImageTaskSubmission{
		Owner:    service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID},
		Platform: platform,
		Envelope: service.ImageTaskRequestEnvelope{
			Method:      c.Request.Method,
			Path:        strings.TrimSuffix(c.Request.URL.Path, "/async"),
			ContentType: c.GetHeader("Content-Type"),
			Headers: map[string]string{
				"Accept":              c.GetHeader("Accept"),
				"User-Agent":          c.GetHeader("User-Agent"),
				"X-Request-ID":        c.GetHeader("X-Request-ID"),
				"X-Client-Request-ID": c.GetHeader("X-Client-Request-ID"),
			},
			Body: body,
		},
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		imageTaskError(c, err)
		return
	}

	pollURL := imageTaskPollURL(c.Request.URL.Path, task.ID)
	c.Header("Cache-Control", "no-store")
	c.Header("Location", pollURL)
	c.Header("Retry-After", "3")
	if replayed {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(http.StatusAccepted, gin.H{
		"id":         task.ID,
		"task_id":    task.TaskID,
		"object":     task.Object,
		"status":     task.Status,
		"created_at": task.CreatedAt,
		"expires_at": task.ExpiresAt,
		"poll_url":   pollURL,
	})

}

func (h *AsyncImageHandler) checkSecurityAuditBeforeSubmit(c *gin.Context, apiKey *service.APIKey, platform string, body []byte) bool {
	if h == nil || h.openAI == nil {
		return true
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		imageTaskJSONError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return false
	}
	model := ""
	moderationBody := body
	if platform == service.PlatformGrok {
		parsed := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	} else if h.openAI.gatewayService != nil {
		parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
		if err != nil {
			imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return false
		}
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	}
	if len(moderationBody) == 0 {
		c.Set(securityAuditCompletedContextKey, true)
		return true
	}
	reqLog := requestLogger(c, "handler.async_image.security_audit",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", model))
	decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, moderationBody)
	if decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return false
	}
	return true
}

func (h *AsyncImageHandler) Get(c *gin.Context) {
	// Polling deliberately does not require the feature to be enabled, only that
	// the task store is reachable. Turning the switch off in the admin UI must not
	// strand tasks that were already accepted — their results are still in Redis
	// and their submitters are still polling.
	if !h.pollable() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, c.Param("task_id"))
	if err != nil {
		imageTaskError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if task.Status == service.ImageTaskStatusQueued || task.Status == service.ImageTaskStatusProcessing {
		c.Header("Retry-After", "3")
	}
	c.JSON(http.StatusOK, task)
}

func (h *AsyncImageHandler) GetAsset(c *gin.Context) {
	if !h.enabled() || h == nil || h.assetReader == nil {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image task asset storage is not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	key := strings.TrimLeft(c.Param("filepath"), "/")
	taskID := imageTaskIDFromAssetKey(key)
	if taskID == "" {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "image task asset not found")
		return
	}
	if _, err := h.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, taskID); err != nil {
		imageTaskError(c, err)
		return
	}
	reader, contentType, err := h.assetReader.Open(c.Request.Context(), key)
	if err != nil {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "image task asset not found")
		return
	}
	defer func() { _ = reader.Close() }()
	c.Header("Cache-Control", "private, max-age=86400")
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

func (h *AsyncImageHandler) GetResult(c *gin.Context) {
	if h == nil || h.imageResults == nil || !h.imageResults.Enabled() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "image result not found")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	index, err := strconv.Atoi(strings.TrimSpace(c.Param("index")))
	if err != nil || index < 0 {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "image result not found")
		return
	}
	reader, contentType, err := h.imageResults.Open(
		c.Request.Context(),
		service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID},
		c.Param("result_id"),
		index,
	)
	if err != nil {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "image result not found")
		return
	}
	defer func() { _ = reader.Close() }()
	c.Header("Cache-Control", "private, no-store")
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

func (h *AsyncImageHandler) validateRequest(c *gin.Context, platform string, body []byte) error {
	if h.openAI == nil || h.openAI.gatewayService == nil {
		return nil
	}
	if platform == service.PlatformGrok {
		parsed, err := service.ParseGrokMediaRequestWithError(c.GetHeader("Content-Type"), body)
		if err != nil {
			return err
		}
		if strings.TrimSpace(parsed.Model) == "" {
			return errors.New("model is required")
		}
		return nil
	}
	parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		return err
	}
	if parsed.Stream {
		return errors.New("streaming image requests cannot be submitted as asynchronous tasks")
	}
	return nil
}

func (h *AsyncImageHandler) executeWithGateway(platform string, c *gin.Context) {
	if h.openAI == nil {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "image gateway is unavailable")
		return
	}
	if platform == service.PlatformGrok {
		h.openAI.GrokImages(c)
		return
	}
	h.openAI.Images(c)
}

func (h *AsyncImageHandler) ProcessImageTask(ctx context.Context, taskID string) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().Error("image_task.execution_panicked", zap.String("task_id", taskID), zap.Any("panic", recovered))
			err = h.failTask(ctx, taskID, http.StatusInternalServerError, imageTaskErrorPayload("api_error", "image generation task panicked"))
		}
	}()

	task, envelope, err := h.tasks.RequestEnvelope(ctx, taskID)
	if err != nil {
		logger.L().Error("image_task.request_recovery_failed", zap.String("task_id", taskID), zap.Error(err))
		return h.failTask(ctx, taskID, http.StatusInternalServerError, imageTaskErrorCodePayload("IMAGE_TASK_RECOVERY_UNAVAILABLE", "image task request could not be recovered"))
	}
	if task.Status == service.ImageTaskStatusCompleted || task.Status == service.ImageTaskStatusFailed {
		return nil
	}
	apiKey, err := h.reloadImageTaskAPIKey(ctx, task)
	if err != nil {
		return h.failTask(ctx, taskID, http.StatusForbidden, imageTaskErrorCodePayload("IMAGE_TASK_AUTH_INVALID", err.Error()))
	}
	requestBody := envelope.Body
	contentType := envelope.ContentType
	if task.Platform == service.PlatformOpenAI {
		requestBody, contentType, err = forceAsyncImageBase64(envelope.ContentType, envelope.Body)
		if err != nil {
			return h.failTask(ctx, taskID, http.StatusBadRequest, imageTaskErrorPayload("invalid_request_error", err.Error()))
		}
	}
	taskCtx, recorder, cancel, err := h.newWorkerImageContext(ctx, taskID, apiKey, envelope, contentType, requestBody)
	if err != nil {
		return h.failTask(ctx, taskID, http.StatusServiceUnavailable, imageTaskErrorCodePayload("IMAGE_TASK_SUBSCRIPTION_UNAVAILABLE", err.Error()))
	}
	defer cancel()

	h.execute(task.Platform, taskCtx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	responseBody := bytes.TrimSpace(recorder.Body.Bytes())
	if err := taskCtx.Request.Context().Err(); err != nil && len(responseBody) == 0 {
		return h.failTask(ctx, taskID, http.StatusGatewayTimeout, imageTaskErrorPayload("timeout_error", "image generation task timed out"))
	}
	statusCode := recorder.Code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		if len(responseBody) == 0 || !json.Valid(responseBody) {
			return h.failTask(ctx, taskID, http.StatusBadGateway, imageTaskErrorPayload("api_error", "upstream returned an invalid image response"))
		}
		if err := h.tasks.Complete(ctx, taskID, statusCode, json.RawMessage(responseBody)); err != nil {
			logger.L().Error("image_task.complete_store_failed", zap.String("task_id", taskID), zap.Error(err))
			return err
		}
		return nil
	}
	return h.failTask(ctx, taskID, statusCode, extractImageTaskError(responseBody))
}

func (h *AsyncImageHandler) failTask(ctx context.Context, taskID string, statusCode int, taskErr json.RawMessage) error {
	if err := h.tasks.Fail(ctx, taskID, statusCode, taskErr); err != nil {
		logger.L().Error("image_task.failure_store_failed", zap.String("task_id", taskID), zap.Error(err))
		return err
	}
	return nil
}

func (h *AsyncImageHandler) newWorkerImageContext(
	ctx context.Context,
	taskID string,
	apiKey *service.APIKey,
	envelope *service.ImageTaskRequestEnvelope,
	contentType string,
	body []byte,
) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc, error) {
	executionCtx, cancel := context.WithTimeout(ctx, h.tasks.ExecutionTimeout())
	executionCtx = context.WithValue(executionCtx, ctxkey.UserID, apiKey.UserID)
	executionCtx = context.WithValue(executionCtx, ctxkey.RequestID, taskID)
	executionCtx = context.WithValue(executionCtx, ctxkey.ClientRequestID, taskID)
	request := httptest.NewRequest(envelope.Method, envelope.Path, bytes.NewReader(body)).WithContext(executionCtx)
	request.Header.Set("Content-Type", contentType)
	for key, value := range envelope.Headers {
		if strings.TrimSpace(value) != "" {
			request.Header.Set(key, value)
		}
	}
	request.Header.Set("X-Request-ID", taskID)
	request.Header.Set("X-Client-Request-ID", taskID)
	recorder := httptest.NewRecorder()
	taskCtx, _ := gin.CreateTestContext(recorder)
	taskCtx.Request = request
	taskCtx.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	taskCtx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      apiKey.UserID,
		Concurrency: apiKey.User.Concurrency,
	})
	taskCtx.Set(string(middleware2.ContextKeyUserRole), apiKey.User.Role)
	if apiKey.Group != nil && apiKey.Group.IsSubscriptionType() {
		if h.subscriptions == nil {
			cancel()
			return nil, nil, func() {}, errors.New("subscription service is unavailable")
		}
		subscription, err := h.subscriptions.GetActiveSubscription(executionCtx, apiKey.UserID, apiKey.Group.ID)
		if err != nil || subscription == nil {
			cancel()
			return nil, nil, func() {}, errors.New("active subscription could not be restored")
		}
		taskCtx.Set(string(middleware2.ContextKeySubscription), subscription)
	}
	taskCtx.Set(securityAuditCompletedContextKey, true)
	return taskCtx, recorder, cancel, nil
}

func (h *AsyncImageHandler) reloadImageTaskAPIKey(ctx context.Context, task *service.ImageTaskRecord) (*service.APIKey, error) {
	if h == nil || h.apiKeys == nil || task == nil {
		return nil, errors.New("API key service is unavailable")
	}
	apiKey, err := h.apiKeys.GetByID(ctx, task.APIKeyID)
	if err != nil {
		return nil, errors.New("API key no longer exists")
	}
	if apiKey == nil || apiKey.User == nil || apiKey.Group == nil ||
		apiKey.UserID != task.UserID || !apiKey.IsActive() || apiKey.IsExpired() || !apiKey.User.IsActive() ||
		!service.GroupAllowsImageGeneration(apiKey.Group) {
		return nil, errors.New("API key is no longer eligible for image generation")
	}
	if apiKey.Group.Platform != task.Platform {
		return nil, errors.New("API key image platform changed after submission")
	}
	return apiKey, nil
}

func (h *AsyncImageHandler) Stop() {
	if h != nil && h.runtime != nil {
		h.runtime.Stop()
	}
}

func (h *AsyncImageHandler) Running() bool {
	return h != nil && h.runtime != nil && h.runtime.Running()
}

func forceAsyncImageBase64(contentType string, body []byte) ([]byte, string, error) {
	if !isMultipartImagesContentType(contentType) {
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			return nil, "", err
		}
		request["response_format"] = "b64_json"
		request["stream"] = false
		rewritten, err := json.Marshal(request)
		return rewritten, contentType, err
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, "", err
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, "", errors.New("multipart boundary is required")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var rewritten bytes.Buffer
	writer := multipart.NewWriter(&rewritten)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if part.FormName() == "response_format" || part.FormName() == "stream" {
			_ = part.Close()
			continue
		}
		target, err := writer.CreatePart(part.Header)
		if err != nil {
			_ = part.Close()
			return nil, "", err
		}
		if _, err := io.Copy(target, part); err != nil {
			_ = part.Close()
			return nil, "", err
		}
		_ = part.Close()
	}
	if err := writer.WriteField("response_format", "b64_json"); err != nil {
		return nil, "", err
	}
	if err := writer.WriteField("stream", "false"); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return rewritten.Bytes(), writer.FormDataContentType(), nil
}

func asyncImageRequestStreams(contentType string, body []byte) bool {
	if isMultipartImagesContentType(contentType) {
		return false
	}
	var envelope struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &envelope) == nil && envelope.Stream
}

func imageTaskPollURL(submitPath, taskID string) string {
	if strings.HasPrefix(submitPath, "/v1/") {
		return "/v1/images/tasks/" + taskID
	}
	return "/images/tasks/" + taskID
}

func imageTaskIDFromAssetKey(key string) string {
	base := path.Base(strings.ReplaceAll(key, "\\", "/"))
	if !strings.HasPrefix(base, "imgtask_") {
		return ""
	}
	idx := strings.LastIndex(base, "-")
	if idx <= len("imgtask_") {
		return ""
	}
	return base[:idx]
}

func extractImageTaskError(body []byte) json.RawMessage {
	if json.Valid(body) {
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(body, &envelope) == nil && len(envelope.Error) > 0 && json.Valid(envelope.Error) {
			return envelope.Error
		}
		return json.RawMessage(body)
	}
	return imageTaskErrorPayload("api_error", "image generation failed")
}

func imageTaskErrorPayload(errorType, message string) json.RawMessage {
	data, _ := json.Marshal(gin.H{"type": errorType, "message": message})
	return data
}

func imageTaskErrorCodePayload(code, message string) json.RawMessage {
	data, _ := json.Marshal(gin.H{"type": "api_error", "code": code, "message": message})
	return data
}

func imageTaskError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(code) == "" {
		code = "IMAGE_TASK_ERROR"
	}
	imageTaskJSONError(c, status, code, message)
}

func imageTaskJSONError(c *gin.Context, status int, code, message string) {
	imageTaskJSONTypedError(c, status, code, code, message)
}

func imageTaskJSONTypedError(c *gin.Context, status int, errorType, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"error": gin.H{"type": errorType, "code": code, "message": message}})
}
