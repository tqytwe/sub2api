package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

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
	apiKeys *service.APIKeyService
	gateway *OpenAIGatewayHandler
}

func NewMobileVideoGatewayProvider(apiKeys *service.APIKeyService, gateway *OpenAIGatewayHandler) *MobileVideoGatewayProvider {
	return &MobileVideoGatewayProvider{apiKeys: apiKeys, gateway: gateway}
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
	session, err := p.apiKeys.IssueNextChatManagedSessionForPurpose(parent, job.UserID, service.NextChatSessionPurposeVideo)
	if err != nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_SESSION_UNAVAILABLE", Message: "视频会话不可用", Retryable: true}
	}
	if _, err := p.apiKeys.SetNextChatManagedKeyGroup(parent, job.UserID, session.KeyID, job.GroupID); err != nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_GROUP_BIND_FAILED", Message: "视频分组绑定失败", Retryable: false}
	}
	apiKey, err := p.apiKeys.GetByKey(parent, session.APIKey)
	if err != nil || apiKey == nil || apiKey.Group == nil {
		return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_SESSION_UNAVAILABLE", Message: "视频会话不可用", Retryable: true}
	}
	if apiKey.User == nil {
		apiKey.User = &service.User{ID: job.UserID, Concurrency: 1}
	}
	if apiKey.User.Concurrency <= 0 {
		apiKey.User.Concurrency = 1
	}

	method := http.MethodGet
	path := "/v1/videos/" + strings.TrimSpace(job.ProviderRequestID)
	var body []byte
	if operation == "create" {
		method = http.MethodPost
		path = "/v1/videos"
		body, err = mobileVideoGatewayBody(job)
		if err != nil {
			return service.MobileVideoProviderResult{}, &service.MobileVideoProviderError{Code: "VIDEO_REQUEST_INVALID", Message: "视频请求格式不正确", Retryable: false}
		}
	} else if operation == "content" {
		path += "/content"
	}
	if apiKey.Group.Platform != service.PlatformGrok && operation == "content" {
		// Agnes exposes the result URL from its status document; there is no
		// separate authenticated content route in its compatibility API.
		operation = "poll"
		path = "/v1/agnesapi?video_id=" + urlQueryEscape(job.ProviderRequestID)
	}

	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body)).WithContext(parent)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: job.UserID, Concurrency: apiKey.User.Concurrency})
	if apiKey.Group.Platform == service.PlatformGrok {
		c.Params = gin.Params{{Key: "request_id", Value: job.ProviderRequestID}}
	}
	if apiKey.Group.Platform == service.PlatformGrok {
		switch operation {
		case "create":
			p.gateway.GrokVideoGeneration(c)
		case "content":
			p.gateway.GrokVideoContent(c)
		default:
			p.gateway.GrokVideoStatus(c)
		}
	} else {
		if operation == "create" {
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
	return mobileVideoGatewayResult(writer.Body.Bytes(), operation)
}

func mobileVideoGatewayBody(job *service.MobileVideoJob) ([]byte, error) {
	return json.Marshal(map[string]any{
		"model":            strings.TrimSpace(job.Model),
		"prompt":           strings.TrimSpace(job.Prompt),
		"resolution":       strings.TrimSpace(job.Resolution),
		"ratio":            strings.TrimSpace(job.Ratio),
		"duration_seconds": job.DurationSeconds,
		"generate_audio":   job.GenerateAudio,
		"watermark":        job.Watermark,
	})
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
	for _, path := range []string{"video.url", "video_url", "url", "data.video.url", "data.video_url", "data.url"} {
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
