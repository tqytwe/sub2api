package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	AgnesVideoDefaultModel = "agnes-video-v2.0"
)

type AgnesVideoEndpoint string

const (
	AgnesVideoEndpointCreate       AgnesVideoEndpoint = "create"
	AgnesVideoEndpointStatusVideo  AgnesVideoEndpoint = "status_video"
	AgnesVideoEndpointStatusLegacy AgnesVideoEndpoint = "status_legacy"
)

func AgnesVideoSessionHash(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	return "agnes-video:" + id
}

func ExtractAgnesVideoRequestModel(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "model").String())
}

func ExtractAgnesVideoResponseID(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{"video_id", "task_id", "id", "data.video_id", "data.task_id", "data.id"} {
		if id := strings.TrimSpace(gjson.GetBytes(body, path).String()); id != "" {
			return id
		}
	}
	return ""
}

func (e AgnesVideoEndpoint) httpMethod() string {
	if e == AgnesVideoEndpointCreate {
		return http.MethodPost
	}
	return http.MethodGet
}

func (s *OpenAIGatewayService) ForwardAgnesVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint AgnesVideoEndpoint,
	videoID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("openai-compatible account is required")
	}
	if account.Platform != PlatformOpenAI {
		return nil, fmt.Errorf("account platform %s is not supported for agnes video", account.Platform)
	}

	token, _, err := s.getRequestCredential(ctx, c, account)
	if err != nil {
		return nil, err
	}
	targetURL, err := s.buildAgnesVideoURL(account, endpoint, videoID)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if endpoint == AgnesVideoEndpointCreate {
		bodyReader = bytes.NewReader(body)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	authHeaders, err := s.buildOpenAIAuthenticationHeaders(ctx, account, token)
	if err != nil {
		return nil, fmt.Errorf("build agnes video authentication headers: %w", err)
	}
	for key, values := range authHeaders {
		for _, value := range values {
			upstreamReq.Header.Add(key, value)
		}
	}
	upstreamReq.Header.Set("Accept", "application/json")
	if endpoint == AgnesVideoEndpointCreate {
		contentType = strings.TrimSpace(contentType)
		if contentType == "" {
			contentType = "application/json"
		}
		upstreamReq.Header.Set("Content-Type", contentType)
	}
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, s.writeAgnesVideoUpstreamError(ctx, c, account, resp, respBody)
	}

	writeAgnesVideoResponse(c, resp, respBody, s.responseHeaderFilter)
	responseID := ExtractAgnesVideoResponseID(respBody)
	requestModel := ExtractAgnesVideoRequestModel(body)
	if requestModel == "" {
		requestModel = AgnesVideoDefaultModel
	}
	return &OpenAIForwardResult{
		RequestID:       firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("request-id")),
		ResponseID:      responseID,
		Model:           requestModel,
		BillingModel:    requestModel,
		UpstreamModel:   account.GetMappedModel(requestModel),
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}, nil
}

func (s *OpenAIGatewayService) buildAgnesVideoURL(account *Account, endpoint AgnesVideoEndpoint, videoID string) (string, error) {
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://apihub.agnes-ai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return "", err
	}
	switch endpoint {
	case AgnesVideoEndpointCreate:
		return buildOpenAIEndpointURL(validatedURL, "/v1/videos"), nil
	case AgnesVideoEndpointStatusVideo:
		return buildAgnesAPIURL(validatedURL, videoID)
	case AgnesVideoEndpointStatusLegacy:
		return buildOpenAIEndpointURL(validatedURL, "/v1/videos/"+url.PathEscape(strings.TrimSpace(videoID))), nil
	default:
		return "", fmt.Errorf("unsupported agnes video endpoint: %s", endpoint)
	}
}

func buildAgnesAPIURL(base string, videoID string) (string, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return "", fmt.Errorf("video_id is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	parsed.Path = "/agnesapi"
	parsed.RawPath = ""
	parsed.RawQuery = url.Values{"video_id": []string{videoID}}.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}

func (s *OpenAIGatewayService) writeAgnesVideoUpstreamError(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, body []byte) error {
	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if upstreamMsg == "" {
		upstreamMsg = fmt.Sprintf("Agnes video upstream returned status %d", resp.StatusCode)
	}
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)
	if account != nil {
		_ = s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, body, "")
	}
	if s.shouldFailoverUpstreamError(resp.StatusCode) {
		return &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			ResponseHeaders:        resp.Header.Clone(),
			RetryableOnSameAccount: account != nil && account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}
	MarkResponseCommitted(c)
	writeAgnesVideoErrorResponse(c, resp.StatusCode, agnesVideoErrorType(resp.StatusCode), upstreamMsg)
	return fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

func agnesVideoErrorType(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	default:
		return "upstream_error"
	}
}

func writeAgnesVideoErrorResponse(c *gin.Context, statusCode int, errType, message string) {
	if c == nil || c.Writer == nil || c.Writer.Written() {
		return
	}
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":    strings.TrimSpace(errType),
			"message": strings.TrimSpace(message),
		},
	})
}

func writeAgnesVideoResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
}
