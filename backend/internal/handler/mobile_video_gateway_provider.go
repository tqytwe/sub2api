package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// MobileVideoGatewayProvider adapts the existing authenticated Agnes/Grok
// gateway handlers to the durable mobile worker. It deliberately creates an
// internal request context with the managed video key, so provider routing,
// account failover, moderation, and the existing billing hooks remain in one
// place instead of being reimplemented by the worker.
type MobileVideoGatewayProvider struct {
	apiKeys       mobileVideoAPIKeyResolver
	gateway       *OpenAIGatewayHandler
	subscriptions mobileVideoSubscriptionResolver
	httpClient    *http.Client
	assets        mobileVideoAssetReader
}

type mobileVideoAPIKeyResolver interface {
	IssueNextChatManagedSessionForPurposeAndGroup(context.Context, int64, string, int64) (*service.NextChatManagedSession, error)
	GetByID(context.Context, int64) (*service.APIKey, error)
}

type mobileVideoSubscriptionResolver interface {
	GetActiveSubscription(context.Context, int64, int64) (*service.UserSubscription, error)
}

type mobileVideoAssetReader interface {
	ReadAssetForVideo(context.Context, int64, string) ([]byte, string, error)
}

type mobileVideoAssetURLReader interface {
	ReferenceURLForVideo(context.Context, int64, string) (string, string, error)
}

func NewMobileVideoGatewayProvider(apiKeys *service.APIKeyService, gateway *OpenAIGatewayHandler, assets ...mobileVideoAssetReader) *MobileVideoGatewayProvider {
	var assetReader mobileVideoAssetReader
	if len(assets) > 0 {
		assetReader = assets[0]
	}
	return &MobileVideoGatewayProvider{apiKeys: apiKeys, gateway: gateway, httpClient: &http.Client{Timeout: 90 * time.Second}, assets: assetReader}
}

func (p *MobileVideoGatewayProvider) SetSubscriptionResolver(subscriptions mobileVideoSubscriptionResolver) *MobileVideoGatewayProvider {
	if p != nil {
		p.subscriptions = subscriptions
	}
	return p
}

func (p *MobileVideoGatewayProvider) Create(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	return p.call(ctx, job, "create")
}

func (p *MobileVideoGatewayProvider) Poll(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	return p.call(ctx, job, "poll")
}

func (p *MobileVideoGatewayProvider) Content(ctx context.Context, job *service.MobileVideoJob) (service.MobileVideoProviderResult, error) {
	return p.call(ctx, job, "content")
}

func (p *MobileVideoGatewayProvider) call(parent context.Context, job *service.MobileVideoJob, operation string) (service.MobileVideoProviderResult, error) {
	if p == nil || p.apiKeys == nil || p.gateway == nil || job == nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_GATEWAY_UNAVAILABLE", Message: "视频网关不可用", Retryable: true}
	}
	apiKey, err := p.executionAPIKey(parent, job)
	if err != nil || apiKey == nil || apiKey.Group == nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_SESSION_UNAVAILABLE", Message: "视频会话不可用", Retryable: true}
	}
	if apiKey.User == nil {
		apiKey.User = &service.User{ID: job.UserID, Concurrency: 1}
	}
	if apiKey.User.Concurrency <= 0 {
		apiKey.User.Concurrency = 1
	}
	subscription, err := p.activeSubscription(parent, job, apiKey)
	if err != nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_SUBSCRIPTION_UNAVAILABLE", Message: "当前视频套餐不可用", Retryable: true}
	}
	providerPlatform := apiKey.Group.Platform
	if providerPlatform == service.PlatformComposite {
		if detected, ok := service.DetectModelPlatform(job.Model); ok {
			providerPlatform = detected
		}
	}

	method := http.MethodGet
	path := "/v1/videos/" + strings.TrimSpace(job.ProviderRequestID)
	var body []byte
	if operation == "create" {
		method = http.MethodPost
		path = "/v1/videos"
		body, err = p.mobileVideoGatewayBody(parent, job)
		if err != nil {
			return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_REQUEST_INVALID", Message: "视频请求格式不正确", Retryable: false}
		}
	} else if operation == "content" {
		path += "/content"
	}
	seedance := service.IsSeedanceVideoModel(job.Model) && providerPlatform != service.PlatformGrok
	if providerPlatform != service.PlatformGrok && operation == "content" {
		// Agnes exposes the result URL from its status document; there is no
		// separate authenticated content route in its compatibility API.
		operation = "poll"
		if seedance {
			path = "/v1/seedance/videos/" + urlQueryEscape(job.ProviderRequestID)
		} else {
			path = "/v1/agnesapi?video_id=" + urlQueryEscape(job.ProviderRequestID)
		}
	}

	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = mobileVideoGatewayRequest(parent, method, path, body, job, operation)
	mobileVideoGatewayAttachIdentity(c, job, apiKey, subscription)
	// A composite group is only the public billing/authorization entry point.
	// Resolve the concrete provider from the selected model before dispatching;
	// otherwise a Grok model in a composite group would be sent through the
	// Agnes/OpenAI compatibility handler and fail with an opaque upstream error.
	ensureCompositeTargetPlatform(c, apiKey, job.Model)
	effectivePlatform := effectiveAPIKeyPlatform(c, apiKey)
	if effectivePlatform == service.PlatformGrok {
		c.Params = gin.Params{{Key: "request_id", Value: job.ProviderRequestID}}
	}
	if effectivePlatform == service.PlatformGrok {
		switch operation {
		case "create":
			p.gateway.GrokVideoGeneration(c)
		case "content":
			p.gateway.GrokVideoContent(c)
		default:
			p.gateway.GrokVideoStatus(c)
		}
	} else {
		if seedance && operation == "create" {
			p.gateway.SeedanceVideoCreate(c)
		} else if seedance {
			p.gateway.SeedanceVideoStatus(c)
		} else if operation == "create" {
			p.gateway.AgnesVideoCreate(c)
		} else {
			p.gateway.AgnesVideoStatus(c)
		}
	}
	if writer.Code >= http.StatusBadRequest {
		message := strings.TrimSpace(gjson.Get(writer.Body.String(), "error.message").String())
		if message == "" {
			message = "视频供应商请求失败"
		}
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_PROVIDER_HTTP_ERROR", Message: message, Retryable: writer.Code >= 500 || writer.Code == http.StatusTooManyRequests}
	}
	contentType := strings.TrimSpace(writer.Header().Get("Content-Type"))
	if strings.HasPrefix(contentType, "video/") || operation == "content" && !json.Valid(writer.Body.Bytes()) {
		return service.MobileVideoProviderResult{Status: "completed", Artifact: append([]byte(nil), writer.Body.Bytes()...), ContentType: contentType}, nil
	}
	result, resultErr := mobileVideoGatewayResult(writer.Body.Bytes(), operation)
	if resultErr != nil || result.ArtifactURL == "" || !mobileVideoProviderCompleted(result.Status) {
		return result, resultErr
	}
	// Agnes status responses commonly carry the completed URL instead of a
	// binary content endpoint. Copy it into the platform-owned mobile result
	// store in the worker, so the app can use the authenticated content route.
	if data, mediaType, fetchErr := p.fetchArtifact(parent, result.ArtifactURL); fetchErr == nil {
		result.Artifact = data
		if result.ContentType == "" {
			result.ContentType = mediaType
		}
	}
	return result, nil
}

func (p *MobileVideoGatewayProvider) executionAPIKey(ctx context.Context, job *service.MobileVideoJob) (*service.APIKey, error) {
	if p == nil || p.apiKeys == nil || job == nil || job.UserID <= 0 || job.GroupID <= 0 {
		return nil, service.ErrMobileVideoExecutionIdentity
	}
	keyID := job.ExecutionAPIKeyID
	if keyID <= 0 {
		// Compatibility for a job inserted by the previous binary during a
		// rolling upgrade. It still uses a group-scoped key and never mutates the
		// old shared video workspace key.
		session, err := p.apiKeys.IssueNextChatManagedSessionForPurposeAndGroup(ctx, job.UserID, service.NextChatSessionPurposeVideo, job.GroupID)
		if err != nil {
			return nil, err
		}
		keyID = session.KeyID
	}
	apiKey, err := p.apiKeys.GetByID(ctx, keyID)
	if err != nil || apiKey == nil || apiKey.UserID != job.UserID || apiKey.GroupID == nil || *apiKey.GroupID != job.GroupID || apiKey.Group == nil || !apiKey.IsActive() || apiKey.IsExpired() {
		return nil, service.ErrMobileVideoExecutionIdentity
	}
	return apiKey, nil
}

func (p *MobileVideoGatewayProvider) activeSubscription(ctx context.Context, job *service.MobileVideoJob, apiKey *service.APIKey) (*service.UserSubscription, error) {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.IsSubscriptionType() {
		return nil, nil
	}
	if p == nil || p.subscriptions == nil || job == nil {
		return nil, service.ErrMobileVideoExecutionIdentity
	}
	subscription, err := p.subscriptions.GetActiveSubscription(ctx, job.UserID, apiKey.Group.ID)
	if err != nil || subscription == nil {
		return nil, service.ErrMobileVideoExecutionIdentity
	}
	return subscription, nil
}

func mobileVideoGatewayAttachIdentity(c *gin.Context, job *service.MobileVideoJob, apiKey *service.APIKey, subscription *service.UserSubscription) {
	if c == nil || c.Request == nil || job == nil || apiKey == nil {
		return
	}
	executionCtx := context.WithValue(c.Request.Context(), ctxkey.UserID, job.UserID)
	if apiKey.Group != nil {
		executionCtx = context.WithValue(executionCtx, ctxkey.Group, apiKey.Group)
	}
	c.Request = c.Request.WithContext(executionCtx)
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	concurrency := 1
	role := ""
	if apiKey.User != nil {
		concurrency = apiKey.User.Concurrency
		role = apiKey.User.Role
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: job.UserID, Concurrency: concurrency})
	c.Set(string(middleware.ContextKeyUserRole), role)
	if subscription != nil {
		c.Set(string(middleware.ContextKeySubscription), subscription)
	}
}

func mobileVideoGatewayRequest(parent context.Context, method, path string, body []byte, job *service.MobileVideoJob, operation string) *http.Request {
	if parent == nil {
		parent = context.Background()
	}
	taskID := ""
	if job != nil {
		taskID = strings.TrimSpace(job.TaskID)
	}
	idempotencyKey := service.MobileVideoTaskIdempotencyKey(taskID)
	executionCtx := context.WithValue(parent, ctxkey.RequestID, taskID)
	executionCtx = context.WithValue(executionCtx, ctxkey.ClientRequestID, idempotencyKey)
	request := httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(executionCtx)
	request.Header.Set("Content-Type", "application/json")
	if taskID != "" {
		request.Header.Set("X-Request-ID", taskID)
	}
	request.Header.Set(middleware.ClientRequestIDHeader, idempotencyKey)
	if operation == "create" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return request
}

func mobileVideoProviderCompleted(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "complete", "succeeded", "success", "done":
		return true
	default:
		return false
	}
}

const mobileVideoMaxArtifactBytes int64 = 128 << 20

func (p *MobileVideoGatewayProvider) fetchArtifact(ctx context.Context, rawURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, "", errors.New("video artifact URL is not fetchable")
	}
	client := p.httpClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", "video/*,application/octet-stream")
	response, err := client.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("video artifact returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > mobileVideoMaxArtifactBytes {
		return nil, "", errors.New("video artifact is too large")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, mobileVideoMaxArtifactBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > mobileVideoMaxArtifactBytes {
		return nil, "", errors.New("video artifact is too large")
	}
	mediaType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	return data, mediaType, nil
}

func (p *MobileVideoGatewayProvider) mobileVideoGatewayBody(ctx context.Context, job *service.MobileVideoJob) ([]byte, error) {
	if service.IsSeedanceVideoModel(job.Model) {
		return p.seedanceVideoGatewayBody(ctx, job)
	}
	payload := map[string]any{
		"model":            strings.TrimSpace(job.Model),
		"prompt":           strings.TrimSpace(job.Prompt),
		"resolution":       strings.TrimSpace(job.Resolution),
		"ratio":            strings.TrimSpace(job.Ratio),
		"duration_seconds": job.DurationSeconds,
		"generate_audio":   job.GenerateAudio,
		"watermark":        job.Watermark,
	}
	if p != nil && p.assets != nil && len(job.ReferenceAssetIDs) > 0 {
		images := make([]string, 0, len(job.ReferenceAssetIDs))
		videos := make([]string, 0, len(job.ReferenceAssetIDs))
		audios := make([]string, 0, len(job.ReferenceAssetIDs))
		for _, assetID := range job.ReferenceAssetIDs {
			reference, contentType, err := p.videoReferenceURLOrData(ctx, job.UserID, assetID)
			if err != nil {
				return nil, &service.MobileVideoProviderError{Code: "VIDEO_REFERENCE_UNAVAILABLE", Message: "视频参考素材不可用", Retryable: false}
			}
			switch mobileAssetKindFromContentType(contentType, assetID) {
			case "image":
				images = append(images, reference)
			case "video":
				videos = append(videos, reference)
			case "audio":
				audios = append(audios, reference)
			default:
				return nil, &service.MobileVideoProviderError{Code: "VIDEO_REFERENCE_UNAVAILABLE", Message: "视频参考素材类型不支持", Retryable: false}
			}
		}
		if len(images) > 0 {
			// The existing Grok and Agnes adapters recognize these image aliases.
			payload["reference_images"] = images
			payload["images"] = images
			payload["image"] = images[0]
		}
		if len(videos) > 0 {
			payload["reference_videos"] = videos
			payload["videos"] = videos
			payload["video"] = videos[0]
		}
		if len(audios) > 0 {
			payload["reference_audios"] = audios
			payload["audios"] = audios
			payload["audio"] = audios[0]
		}
	}
	return json.Marshal(payload)
}

func (p *MobileVideoGatewayProvider) seedanceVideoGatewayBody(ctx context.Context, job *service.MobileVideoJob) ([]byte, error) {
	content := []map[string]any{{"type": "text", "text": strings.TrimSpace(job.Prompt)}}
	if p != nil && p.assets != nil {
		imageCount, videoCount, audioCount := 0, 0, 0
		for _, assetID := range job.ReferenceAssetIDs {
			reference, contentType, err := p.videoReferenceURLOrData(ctx, job.UserID, assetID)
			if err != nil {
				return nil, &service.MobileVideoProviderError{Code: "VIDEO_REFERENCE_UNAVAILABLE", Message: "视频参考素材不可用", Retryable: false}
			}
			switch mobileAssetKindFromContentType(contentType, assetID) {
			case "image":
				imageCount++
				content = append(content, map[string]any{"type": "image_url", "image_url": map[string]string{"url": reference}, "role": "reference_image"})
			case "video":
				videoCount++
				content = append(content, map[string]any{"type": "video_url", "video_url": map[string]string{"url": reference}, "role": "reference_video"})
			case "audio":
				audioCount++
				content = append(content, map[string]any{"type": "audio_url", "audio_url": map[string]string{"url": reference}, "role": "reference_audio"})
			default:
				return nil, &service.MobileVideoProviderError{Code: "VIDEO_REFERENCE_UNAVAILABLE", Message: "视频参考素材类型不支持", Retryable: false}
			}
		}
		if imageCount > 9 || videoCount > 3 || audioCount > 3 {
			return nil, &service.MobileVideoProviderError{Code: "VIDEO_REFERENCE_UNAVAILABLE", Message: "视频参考素材数量超出模型限制", Retryable: false}
		}
	}
	return json.Marshal(map[string]any{
		"model":          strings.TrimSpace(job.Model),
		"content":        content,
		"ratio":          strings.TrimSpace(job.Ratio),
		"resolution":     strings.TrimSpace(job.Resolution),
		"duration":       job.DurationSeconds,
		"generate_audio": job.GenerateAudio,
		"watermark":      job.Watermark,
	})
}

func (p *MobileVideoGatewayProvider) videoReferenceURLOrData(ctx context.Context, userID int64, assetID string) (string, string, error) {
	if p == nil || p.assets == nil {
		return "", "", errors.New("video reference storage is unavailable")
	}
	if urls, ok := p.assets.(mobileVideoAssetURLReader); ok {
		if referenceURL, contentType, err := urls.ReferenceURLForVideo(ctx, userID, assetID); err == nil && strings.TrimSpace(referenceURL) != "" {
			return referenceURL, contentType, nil
		}
	}
	data, contentType, err := p.assets.ReadAssetForVideo(ctx, userID, assetID)
	if err != nil {
		return "", "", err
	}
	return "data:" + strings.TrimSpace(contentType) + ";base64," + base64.StdEncoding.EncodeToString(data), contentType, nil
}

func mobileVideoGatewayResult(body []byte, operation string) (service.MobileVideoProviderResult, error) {
	if len(body) == 0 {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_PROVIDER_EMPTY_RESPONSE", Message: "视频供应商返回为空", Retryable: true}
	}
	requestID := ""
	for _, path := range []string{"id", "video_id", "task_id", "data.id", "data.video_id", "data.task_id"} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			requestID = value
			break
		}
	}
	status := strings.ToLower(strings.TrimSpace(firstMobileVideoJSON(body, "status", "state", "data.status")))
	if status == "" {
		if strings.TrimSpace(gjson.GetBytes(body, "video.url").String()) != "" || strings.TrimSpace(gjson.GetBytes(body, "data.video.url").String()) != "" {
			status = "completed"
		} else {
			status = "queued"
		}
	}
	result := service.MobileVideoProviderResult{RequestID: requestID, Status: status}
	for _, path := range []string{"progress", "progress_percent", "data.progress"} {
		if value := int(gjson.GetBytes(body, path).Int()); value > 0 {
			result.Progress = value
			break
		}
	}
	for _, path := range []string{
		"video.url", "video_url", "url", "result_url", "download_url", "content_url",
		"data.video.url", "data.video_url", "data.url", "data.result_url", "data.download_url", "data.content_url",
		"content.video_url", "data.content.video_url", "output.video_url", "data.output.video_url",
	} {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			result.ArtifactURL = value
			break
		}
	}
	if operation == "create" && requestID == "" {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_PROVIDER_REQUEST_ID_MISSING", Message: "视频供应商未返回任务标识", Retryable: false}
	}
	return result, nil
}

func firstMobileVideoJSON(body []byte, paths ...string) string {
	for _, path := range paths {
		if value := strings.TrimSpace(gjson.GetBytes(body, path).String()); value != "" {
			return value
		}
	}
	return ""
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(strings.TrimSpace(value))
}
