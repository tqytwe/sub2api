package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// mobileVideoGatewayExecutor is intentionally narrower than the full OpenAI
// handler. The durable worker may dispatch only the adapter declared by the
// catalog, not an endpoint inferred from a model name.
type mobileVideoGatewayExecutor interface {
	GrokVideoGeneration(*gin.Context)
	GrokVideoStatus(*gin.Context)
	GrokVideoContent(*gin.Context)
	AgnesVideoCreate(*gin.Context)
	AgnesVideoStatus(*gin.Context)
}

type mobileVideoExecutionKeyResolver interface {
	GetMobileVideoExecutionKey(context.Context, int64, int64, int64) (*service.APIKey, error)
}

type mobileVideoExecutionSnapshotResolver interface {
	RestoreMobileVideoExecutionSnapshot(*service.MobileVideoExecutionSnapshot) (*service.APIKey, error)
	ValidateMobileVideoExecutionKeyForExecution(context.Context, int64, int64, int64) error
}

type mobileVideoGatewaySubscriptionResolver interface {
	GetActiveSubscription(context.Context, int64, int64) (*service.UserSubscription, error)
}

// MobileVideoGatewayProvider adapts the existing provider gateways to the
// durable mobile worker. It never issues a new session or falls back to a
// chat/image key: the task's pinned Video Execution key is verified again
// immediately before every upstream call.
type MobileVideoGatewayProvider struct {
	apiKeys       mobileVideoExecutionKeyResolver
	gateway       mobileVideoGatewayExecutor
	subscriptions mobileVideoGatewaySubscriptionResolver
}

func NewMobileVideoGatewayProvider(
	apiKeys *service.APIKeyService,
	gateway *OpenAIGatewayHandler,
	subscriptions *service.SubscriptionService,
) *MobileVideoGatewayProvider {
	return newMobileVideoGatewayProviderWithDependencies(apiKeys, gateway, subscriptions)
}

func newMobileVideoGatewayProviderWithDependencies(
	apiKeys mobileVideoExecutionKeyResolver,
	gateway mobileVideoGatewayExecutor,
	subscriptions mobileVideoGatewaySubscriptionResolver,
) *MobileVideoGatewayProvider {
	return &MobileVideoGatewayProvider{apiKeys: apiKeys, gateway: gateway, subscriptions: subscriptions}
}

func (p *MobileVideoGatewayProvider) Create(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	return p.call(ctx, job, mobileVideoGatewayOperationCreate)
}

func (p *MobileVideoGatewayProvider) Poll(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	return p.call(ctx, job, mobileVideoGatewayOperationPoll)
}

func (p *MobileVideoGatewayProvider) Content(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	// Agnes returns its completed artifact URL in the status document. It has
	// no authenticated content endpoint compatible with the worker contract.
	if job != nil && strings.TrimSpace(job.Adapter) == service.MobileVideoAdapterAgnes {
		return p.call(ctx, job, mobileVideoGatewayOperationPoll)
	}
	return p.call(ctx, job, mobileVideoGatewayOperationContent)
}

type mobileVideoGatewayOperation string

const (
	mobileVideoGatewayOperationCreate  mobileVideoGatewayOperation = "create"
	mobileVideoGatewayOperationPoll    mobileVideoGatewayOperation = "poll"
	mobileVideoGatewayOperationContent mobileVideoGatewayOperation = "content"
)

func (p *MobileVideoGatewayProvider) call(parent context.Context, job *service.MobileVideoJob, operation mobileVideoGatewayOperation) (service.MobileVideoProviderResult, error) {
	if p == nil || p.apiKeys == nil || p.gateway == nil || job == nil {
		return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_GATEWAY_UNAVAILABLE", "视频网关暂不可用", true)
	}
	apiKey, platform, err := p.executionKey(parent, job)
	if err != nil {
		return service.MobileVideoProviderResult{}, err
	}
	subscription, err := p.subscription(parent, job, apiKey)
	if err != nil {
		return service.MobileVideoProviderResult{}, err
	}

	method, path, body, err := mobileVideoGatewayRequestFor(job, operation)
	if err != nil {
		return service.MobileVideoProviderResult{}, err
	}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = mobileVideoGatewayRequest(parent, method, path, body, job, operation, platform)
	mobileVideoGatewayAttachIdentity(c, job, apiKey, subscription)

	switch strings.TrimSpace(job.Adapter) {
	case service.MobileVideoAdapterGrok:
		switch operation {
		case mobileVideoGatewayOperationCreate:
			p.gateway.GrokVideoGeneration(c)
		case mobileVideoGatewayOperationPoll:
			c.Params = gin.Params{{Key: "request_id", Value: strings.TrimSpace(job.ProviderRequestID)}}
			p.gateway.GrokVideoStatus(c)
		case mobileVideoGatewayOperationContent:
			c.Params = gin.Params{{Key: "request_id", Value: strings.TrimSpace(job.ProviderRequestID)}}
			p.gateway.GrokVideoContent(c)
		default:
			return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不支持该操作", false)
		}
	case service.MobileVideoAdapterAgnes:
		switch operation {
		case mobileVideoGatewayOperationCreate:
			p.gateway.AgnesVideoCreate(c)
		case mobileVideoGatewayOperationPoll:
			p.gateway.AgnesVideoStatus(c)
		default:
			return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不支持该操作", false)
		}
	default:
		return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不受支持", false)
	}

	if writer.Code >= http.StatusBadRequest {
		return service.MobileVideoProviderResult{}, mobileVideoProviderHTTPError(writer.Code, writer.Body.Bytes())
	}
	contentType := strings.TrimSpace(strings.Split(writer.Header().Get("Content-Type"), ";")[0])
	if strings.HasPrefix(strings.ToLower(contentType), "video/") || (operation == mobileVideoGatewayOperationContent && !json.Valid(writer.Body.Bytes())) {
		return service.MobileVideoProviderResult{
			Status: "completed", Artifact: append([]byte(nil), writer.Body.Bytes()...), ContentType: contentType,
		}, nil
	}
	return parseMobileVideoGatewayResult(writer.Body.Bytes(), operation == mobileVideoGatewayOperationCreate, job.Adapter)
}

func (p *MobileVideoGatewayProvider) executionKey(ctx context.Context, job *service.MobileVideoJob) (*service.APIKey, string, error) {
	if job == nil || job.UserID <= 0 || job.GroupID <= 0 || job.ExecutionAPIKeyID <= 0 {
		return nil, "", mobileVideoProviderError("VIDEO_EXECUTION_IDENTITY_UNAVAILABLE", "视频执行会话不可用", true)
	}
	platform, supported := mobileVideoAdapterPlatform(job.Adapter)
	if !supported {
		return nil, "", mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不受支持", false)
	}
	var apiKey *service.APIKey
	var err error
	if job.ExecutionSnapshot != nil {
		resolver, ok := p.apiKeys.(mobileVideoExecutionSnapshotResolver)
		if !ok || !service.ValidMobileVideoExecutionSnapshot(job.ExecutionSnapshot, job.UserID, job.GroupID, job.ExecutionAPIKeyID) {
			return nil, "", mobileVideoProviderError("VIDEO_EXECUTION_SNAPSHOT_INVALID", "视频执行快照无效", false)
		}
		// Do not re-run entitlement/balance checks here. An accepted task must
		// keep its execution identity after a user is removed from a group; a
		// disabled/deleted dedicated key remains a real operational stop.
		if err := resolver.ValidateMobileVideoExecutionKeyForExecution(ctx, job.UserID, job.GroupID, job.ExecutionAPIKeyID); err != nil {
			return nil, "", mobileVideoProviderError("VIDEO_EXECUTION_KEY_UNAVAILABLE", "视频执行密钥已不可用", false)
		}
		apiKey, err = resolver.RestoreMobileVideoExecutionSnapshot(job.ExecutionSnapshot)
	} else {
		// Compatibility only for rows created before the funded snapshot
		// migration. New jobs must always use the branch above.
		apiKey, err = p.apiKeys.GetMobileVideoExecutionKey(ctx, job.UserID, job.GroupID, job.ExecutionAPIKeyID)
		if errors.Is(err, service.ErrGroupNotAllowed) {
			return nil, "", mobileVideoProviderError("VIDEO_GROUP_UNAVAILABLE", "当前视频分组已不可用", false)
		}
	}
	if err != nil || apiKey == nil || apiKey.Group == nil || apiKey.User == nil {
		return nil, "", mobileVideoProviderError("VIDEO_EXECUTION_IDENTITY_UNAVAILABLE", "视频执行会话不可用", true)
	}
	if apiKey.UserID != job.UserID || apiKey.GroupID == nil || *apiKey.GroupID != job.GroupID || !apiKey.IsActive() || apiKey.IsExpired() {
		return nil, "", mobileVideoProviderError("VIDEO_EXECUTION_IDENTITY_UNAVAILABLE", "视频执行会话不可用", true)
	}
	groupPlatform := strings.TrimSpace(apiKey.Group.Platform)
	if groupPlatform != platform && groupPlatform != service.PlatformComposite {
		return nil, "", mobileVideoProviderError("VIDEO_ADAPTER_GROUP_MISMATCH", "视频模型与当前分组不兼容", false)
	}
	return apiKey, platform, nil
}

func (p *MobileVideoGatewayProvider) subscription(ctx context.Context, job *service.MobileVideoJob, apiKey *service.APIKey) (*service.UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if job != nil && job.ExecutionSnapshot != nil {
		// A funded job has already passed the subscription/entitlement check and
		// persisted its immutable execution snapshot before its balance hold was
		// accepted. Rechecking a revocable subscription here would strand an
		// already-priced task after the user loses current group access. The
		// gateway still receives the verified key/group identity, and managed
		// video billing deliberately bypasses a second subscription charge.
		return nil, nil
	}
	if p == nil || p.subscriptions == nil || job == nil {
		return nil, mobileVideoProviderError("VIDEO_SUBSCRIPTION_UNAVAILABLE", "当前视频套餐不可用", true)
	}
	subscription, err := p.subscriptions.GetActiveSubscription(ctx, job.UserID, apiKey.Group.ID)
	if err != nil || subscription == nil {
		return nil, mobileVideoProviderError("VIDEO_SUBSCRIPTION_UNAVAILABLE", "当前视频套餐不可用", false)
	}
	return subscription, nil
}

func mobileVideoAdapterPlatform(adapter string) (string, bool) {
	switch strings.TrimSpace(adapter) {
	case service.MobileVideoAdapterGrok:
		return service.PlatformGrok, true
	case service.MobileVideoAdapterAgnes:
		return service.PlatformOpenAI, true
	default:
		return "", false
	}
}

func mobileVideoGatewayRequestFor(job *service.MobileVideoJob, operation mobileVideoGatewayOperation) (string, string, []byte, error) {
	if job == nil {
		return "", "", nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "视频任务参数不正确", false)
	}
	providerRequestID := strings.TrimSpace(job.ProviderRequestID)
	if operation != mobileVideoGatewayOperationCreate && providerRequestID == "" {
		return "", "", nil, mobileVideoProviderError("VIDEO_PROVIDER_REQUEST_ID_MISSING", "视频上游任务标识缺失", false)
	}

	switch strings.TrimSpace(job.Adapter) {
	case service.MobileVideoAdapterGrok:
		switch operation {
		case mobileVideoGatewayOperationCreate:
			body, err := mobileVideoGatewayCreateBody(job)
			return http.MethodPost, "/v1/videos/generations", body, err
		case mobileVideoGatewayOperationPoll:
			return http.MethodGet, "/v1/videos/generations/" + url.PathEscape(providerRequestID), nil, nil
		case mobileVideoGatewayOperationContent:
			return http.MethodGet, "/v1/videos/generations/" + url.PathEscape(providerRequestID) + "/content", nil, nil
		}
	case service.MobileVideoAdapterAgnes:
		switch operation {
		case mobileVideoGatewayOperationCreate:
			body, err := mobileVideoGatewayCreateBody(job)
			return http.MethodPost, "/v1/videos", body, err
		case mobileVideoGatewayOperationPoll:
			query := url.Values{}
			query.Set("video_id", providerRequestID)
			return http.MethodGet, "/agnesapi?" + query.Encode(), nil, nil
		}
	}
	return "", "", nil, mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不支持该操作", false)
}

func mobileVideoGatewayCreateBody(job *service.MobileVideoJob) ([]byte, error) {
	if job == nil || len(job.ReferenceAssetIDs) != 0 {
		return nil, mobileVideoProviderError("VIDEO_REFERENCE_UNSUPPORTED", "当前视频模型不支持参考素材", false)
	}
	switch strings.TrimSpace(job.Adapter) {
	case service.MobileVideoAdapterGrok:
		return mobileVideoGatewayGrokCreateBody(job)
	case service.MobileVideoAdapterAgnes:
		return mobileVideoGatewayAgnesCreateBody(job)
	default:
		return nil, mobileVideoProviderError("VIDEO_ADAPTER_UNSUPPORTED", "视频执行适配器不受支持", false)
	}
}

func mobileVideoGatewayGrokCreateBody(job *service.MobileVideoJob) ([]byte, error) {
	payload := map[string]any{
		"model":          strings.TrimSpace(job.Model),
		"prompt":         strings.TrimSpace(job.Prompt),
		"resolution":     strings.TrimSpace(job.Resolution),
		"duration":       job.DurationSeconds,
		"generate_audio": job.GenerateAudio,
		"watermark":      job.Watermark,
	}
	if payload["model"] == "" || payload["prompt"] == "" || payload["resolution"] == "" || job.DurationSeconds <= 0 {
		return nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "视频任务参数不正确", false)
	}
	if ratio := strings.TrimSpace(job.Ratio); ratio != "" {
		// Grok's existing gateway parser and documented video body use
		// aspect_ratio. Do not forward the mobile UI field name directly.
		payload["aspect_ratio"] = ratio
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "视频任务参数不正确", false)
	}
	return body, nil
}

// mobileVideoGatewayAgnesCreateBody intentionally does not reuse the Grok
// request schema. Agnes Video V2 accepts dimensions and frame controls only;
// forwarding generic OpenAI video fields makes an otherwise schedulable model
// fail upstream. The capability resolver publishes only this verified matrix,
// but the worker validates it again before dispatching a persisted job.
func mobileVideoGatewayAgnesCreateBody(job *service.MobileVideoJob) ([]byte, error) {
	if job == nil || strings.TrimSpace(job.Model) == "" || strings.TrimSpace(job.Prompt) == "" {
		return nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "视频任务参数不正确", false)
	}
	request, ok := service.ResolveAgnesVideoMobileRequest(strings.TrimSpace(job.Resolution), strings.TrimSpace(job.Ratio), job.DurationSeconds)
	if !ok {
		return nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "当前视频模型不支持所选尺寸、比例或时长", false)
	}
	payload := map[string]any{
		"model":      strings.TrimSpace(job.Model),
		"prompt":     strings.TrimSpace(job.Prompt),
		"width":      request.Width,
		"height":     request.Height,
		"num_frames": request.NumFrames,
		"frame_rate": request.FrameRate,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, mobileVideoProviderError("VIDEO_REQUEST_INVALID", "视频任务参数不正确", false)
	}
	return body, nil
}

func mobileVideoGatewayRequest(parent context.Context, method, path string, body []byte, job *service.MobileVideoJob, operation mobileVideoGatewayOperation, platform string) *http.Request {
	if parent == nil {
		parent = context.Background()
	}
	requestContext := context.WithValue(parent, ctxkey.UserID, job.UserID)
	requestContext = service.WithResolvedTargetPlatform(requestContext, platform)
	if job != nil && job.BillingState != "" && job.BillingState != service.MobileVideoBillingStateNotRequired {
		requestContext = service.WithMobileVideoManagedExecution(requestContext)
		requestContext = service.WithImageStudioBillingActualCostCap(requestContext, job.HoldAmount)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(requestContext)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Request-ID", strings.TrimSpace(job.TaskID))
	if operation == mobileVideoGatewayOperationCreate {
		request.Header.Set("Idempotency-Key", "mobile-video:"+strings.TrimSpace(job.TaskID))
	}
	return request
}

func mobileVideoGatewayAttachIdentity(c *gin.Context, job *service.MobileVideoJob, apiKey *service.APIKey, subscription *service.UserSubscription) {
	if c == nil || c.Request == nil || job == nil || apiKey == nil || apiKey.Group == nil || apiKey.User == nil {
		return
	}
	requestContext := context.WithValue(c.Request.Context(), ctxkey.UserID, job.UserID)
	requestContext = context.WithValue(requestContext, ctxkey.Group, apiKey.Group)
	c.Request = c.Request.WithContext(requestContext)
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	concurrency := apiKey.User.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: job.UserID, Concurrency: concurrency})
	c.Set(string(middleware.ContextKeyUserRole), apiKey.User.Role)
	if subscription != nil {
		c.Set(string(middleware.ContextKeySubscription), subscription)
	}
}

func parseMobileVideoGatewayResult(body []byte, requireRequestID bool, adapter string) (service.MobileVideoProviderResult, error) {
	if len(body) == 0 || !json.Valid(body) {
		return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_PROVIDER_EMPTY_RESPONSE", "视频供应商返回为空", true)
	}
	result := service.MobileVideoProviderResult{}
	requestIDPaths := []string{"id", "video_id", "task_id", "data.id", "data.video_id", "data.task_id"}
	if strings.TrimSpace(adapter) == service.MobileVideoAdapterAgnes {
		// Agnes documents video_id as the stable polling key. Create responses
		// may also contain an opaque id/task_id, neither of which is valid for
		// the /agnesapi status route.
		requestIDPaths = []string{"video_id", "data.video_id", "task_id", "data.task_id", "id", "data.id"}
	}
	for _, path := range requestIDPaths {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			result.RequestID = value
			break
		}
	}
	for _, path := range []string{"status", "state", "data.status", "data.state"} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			result.Status = strings.ToLower(value)
			break
		}
	}
	for _, path := range []string{"progress", "progress_percent", "data.progress", "data.progress_percent"} {
		if value := int(gjson.GetBytes(body, path).Int()); value > 0 {
			result.Progress = value
			break
		}
	}
	for _, path := range []string{
		"video.url", "video_url", "url", "result_url", "download_url", "content_url", "metadata.url",
		"data.video.url", "data.video_url", "data.url", "data.result_url", "data.download_url", "data.content_url", "data.metadata.url",
	} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			result.ArtifactURL = value
			break
		}
	}
	if result.Status == "" {
		if result.ArtifactURL != "" {
			result.Status = "completed"
		} else {
			result.Status = "queued"
		}
	}
	if requireRequestID && result.RequestID == "" {
		return service.MobileVideoProviderResult{}, mobileVideoProviderError("VIDEO_PROVIDER_REQUEST_ID_MISSING", "视频供应商未返回任务标识", false)
	}
	return result, nil
}

func mobileVideoProviderHTTPError(status int, body []byte) error {
	message := strings.TrimSpace(service.ExtractUpstreamErrorMessage(body))
	message = service.SanitizeUpstreamErrorMessage(message)
	if message == "" {
		message = "视频供应商请求失败"
	}
	if len(message) > 240 {
		message = strings.TrimSpace(message[:240])
	}
	return mobileVideoProviderError("VIDEO_PROVIDER_HTTP_ERROR", message, status == http.StatusTooManyRequests || status >= http.StatusInternalServerError)
}

func mobileVideoProviderError(code, message string, retryable bool) error {
	return &service.MobileVideoProviderError{Code: code, Message: message, Retryable: retryable}
}

var _ service.MobileVideoProvider = (*MobileVideoGatewayProvider)(nil)
