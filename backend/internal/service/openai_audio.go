package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// OpenAIEndpointCapabilityAudio marks OpenAI API key upstreams that may serve
// the standard /v1/audio/* and /v1/realtime APIs. ChatGPT OAuth upstreams do
// not expose these endpoints.
const OpenAIEndpointCapabilityAudio OpenAIEndpointCapability = "audio"

// DefaultOpenAIRealtimeModel is used when /v1/realtime is opened without ?model=.
const DefaultOpenAIRealtimeModel = "gpt-realtime"

// openAIAudioSTTBytesPerSecond is a conservative compressed-speech bitrate
// (~128kbps) used as a billing floor when the upstream omits a duration.
const openAIAudioSTTBytesPerSecond = 16000.0

// OpenAIAudioOperation identifies one standard OpenAI audio HTTP endpoint.
type OpenAIAudioOperation string

const (
	OpenAIAudioOperationSpeech         OpenAIAudioOperation = "speech"
	OpenAIAudioOperationTranscriptions OpenAIAudioOperation = "transcriptions"
	OpenAIAudioOperationTranslations   OpenAIAudioOperation = "translations"
)

// Endpoint returns the upstream path for the operation.
func (op OpenAIAudioOperation) Endpoint() string {
	return "/v1/audio/" + string(op)
}

// AudioMode maps the operation onto the group audio pricing mode.
func (op OpenAIAudioOperation) AudioMode() string {
	if op == OpenAIAudioOperationSpeech {
		return "tts"
	}
	return "stt"
}

func (op OpenAIAudioOperation) valid() bool {
	switch op {
	case OpenAIAudioOperationSpeech, OpenAIAudioOperationTranscriptions, OpenAIAudioOperationTranslations:
		return true
	default:
		return false
	}
}

// ExtractOpenAIAudioRequestModel returns the client-requested model from a
// JSON (speech) or multipart (transcriptions/translations) audio body.
func ExtractOpenAIAudioRequestModel(contentType string, body []byte) string {
	return strings.TrimSpace(requestmodel.FromBody(contentType, body))
}

// ExtractOpenAIAudioSpeechInput returns the text that a TTS request will speak.
func ExtractOpenAIAudioSpeechInput(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "input").String())
}

// ForwardOpenAIAudio forwards a standard OpenAI audio request to an API key
// account. Responses are passed through byte-for-byte (TTS returns audio,
// STT returns JSON/text/SRT/VTT). Retryable upstream failures are returned as
// *UpstreamFailoverError without writing to the client.
func (s *OpenAIGatewayService) ForwardOpenAIAudio(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	op OpenAIAudioOperation,
	body []byte,
	contentType string,
	channelMappedModel string,
) (*OpenAIForwardResult, error) {
	if s == nil || account == nil {
		return nil, fmt.Errorf("openai audio service/account is required")
	}
	if !op.valid() {
		return nil, fmt.Errorf("unsupported openai audio operation: %s", op)
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("account %d does not support openai audio", account.ID)
	}
	startTime := time.Now()
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}

	originalModel := ExtractOpenAIAudioRequestModel(contentType, body)
	if originalModel == "" {
		writeOpenAIEmbeddingsError(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return nil, fmt.Errorf("missing model in audio request")
	}
	requestedModel := originalModel
	if mapped := strings.TrimSpace(channelMappedModel); mapped != "" {
		requestedModel = mapped
	}
	billingModel := resolveOpenAIForwardModel(account, requestedModel, "")
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	SetOpsUpstreamModel(c, upstreamModel)

	upstreamBody, upstreamContentType := body, contentType
	if upstreamModel != originalModel {
		rewritten, rewrittenType, err := rewriteOpenAIImagesModel(body, contentType, upstreamModel)
		if err != nil {
			writeOpenAIEmbeddingsError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
			return nil, fmt.Errorf("rewrite audio model: %w", err)
		}
		upstreamBody, upstreamContentType = rewritten, rewrittenType
	}

	apiKey := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenAIFormatBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	targetURL := buildOpenAIEndpointURL(validatedURL, op.Endpoint())

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(upstreamBody))
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	upstreamReq = upstreamReq.WithContext(WithHTTPUpstreamProfile(upstreamReq.Context(), HTTPUpstreamProfileOpenAI))
	upstreamReq.Header.Set("Content-Type", upstreamContentType)
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			if openaiCCRawAllowedHeaders[strings.ToLower(key)] {
				for _, v := range values {
					upstreamReq.Header.Add(key, v)
				}
			}
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		upstreamReq.Header.Set("user-agent", customUA)
	}
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.doOpenAIUpstream(upstreamReq, proxyURL, account)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			ProxyID:     opsUpstreamProxyID(account),
			ProxyName:   opsUpstreamProxyName(account),
			Platform:    account.Platform,
			AccountID:   account.ID,
			AccountName: account.Name,
			Kind:        "request_error",
			Message:     safeErr,
		})
		writeOpenAIEmbeddingsError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		return nil, fmt.Errorf("upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, s.handleOpenAIAudioErrorResponse(ctx, c, account, resp, upstreamModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		if !errors.Is(err, ErrUpstreamResponseBodyTooLarge) {
			writeOpenAIEmbeddingsError(c, http.StatusBadGateway, "api_error", "Failed to read upstream response")
		}
		return nil, fmt.Errorf("read upstream body: %w", err)
	}
	writeOpenAIEmbeddingsUpstreamResponse(c, resp, respBody, s.responseHeaderFilter)

	upstreamID := firstNonEmptyString(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"))
	return &OpenAIForwardResult{
		// Forced durable money-event id: usage_billing_dedup must not collapse
		// separate audio requests that reuse a client request id.
		RequestID:       StableOpenAIAudioBillingRequestID(upstreamID),
		UpstreamHeaders: resp.Header,
		Model:           originalModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
		Duration:        time.Since(startTime),
		AudioUsage:      estimateOpenAIAudioUsage(op, body, respBody),
	}, nil
}

// handleOpenAIAudioErrorResponse applies the shared OpenAI account policy to an
// upstream 4xx/5xx. Failover-eligible responses are returned without touching
// the client writer; everything else is passed through to the client.
func (s *OpenAIGatewayService) handleOpenAIAudioErrorResponse(ctx context.Context, c *gin.Context, account *Account, resp *http.Response, upstreamModel string) error {
	respBody := s.readUpstreamErrorBody(resp)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(respBody))

	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
	if !s.shouldFailoverOpenAIUpstreamResponse(account, resp.StatusCode, upstreamMsg, respBody) {
		writeOpenAIEmbeddingsUpstreamResponse(c, resp, respBody, s.responseHeaderFilter)
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(respBody), maxBytes)
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  resp.Header.Get("x-request-id"),
		Kind:               "failover",
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	shouldDisable := s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
	retryableOnSameAccount := !shouldDisable && account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode)
	if isOpenAIHTTPUpstreamAccessStateError(resp.StatusCode, upstreamMsg, respBody) {
		return newOpenAIUpstreamFailoverError(resp.StatusCode, resp.Header, respBody, upstreamMsg, retryableOnSameAccount)
	}
	return &UpstreamFailoverError{StatusCode: resp.StatusCode, ResponseBody: respBody, RetryableOnSameAccount: retryableOnSameAccount}
}

// estimateOpenAIAudioUsage derives group-audio billing units.
// speech: million characters of `input`.
// transcriptions/translations: hours of audio from the upstream-reported
// duration (verbose_json `duration` or whisper `usage.seconds`), floored by a
// body-size heuristic so a response without duration is never billed at zero.
func estimateOpenAIAudioUsage(op OpenAIAudioOperation, reqBody, respBody []byte) *AudioUsage {
	if op == OpenAIAudioOperationSpeech {
		chars := len([]rune(ExtractOpenAIAudioSpeechInput(reqBody)))
		if chars <= 0 {
			return nil
		}
		return &AudioUsage{Mode: "tts", DurationOrUnits: float64(chars) / 1_000_000.0}
	}
	secs := 0.0
	if gjson.ValidBytes(respBody) {
		for _, path := range []string{"usage.seconds", "duration"} {
			if v := gjson.GetBytes(respBody, path); v.Type == gjson.Number && v.Float() > 0 {
				secs = v.Float()
				break
			}
		}
	}
	if secs <= 0 {
		secs = float64(len(reqBody)) / openAIAudioSTTBytesPerSecond
	}
	if secs <= 0 {
		return nil
	}
	return &AudioUsage{Mode: "stt", DurationOrUnits: secs / 3600.0}
}

// StableOpenAIAudioBillingRequestID forces a unique, durable billing id for
// OpenAI audio HTTP requests.
func StableOpenAIAudioBillingRequestID(upstreamRequestID string) string {
	return stablePrefixedBillingRequestID("openai_audio:", upstreamRequestID)
}

// StableOpenAIRealtimeBillingRequestID forces a unique billing id per
// OpenAI realtime WebSocket session.
func StableOpenAIRealtimeBillingRequestID(sessionID string) string {
	return stablePrefixedBillingRequestID("openai_realtime:", sessionID)
}

func stablePrefixedBillingRequestID(prefix, id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, prefix) {
		return id
	}
	if id == "" {
		id = generateRequestID()
	}
	return prefix + id
}

// openAIRealtimeForwardedHeaders are client headers relayed to the upstream
// realtime handshake (protocol negotiation only; never client credentials).
var openAIRealtimeForwardedHeaders = []string{"OpenAI-Beta", "OpenAI-Organization", "OpenAI-Project"}

// OpenOpenAIRealtime dials the upstream OpenAI Realtime WebSocket
// ({base_url}/v1/realtime?model=...) for an API key account. Handlers call it
// before accepting the client upgrade so handshake failures remain HTTP errors
// and can fail over to another account.
func (s *OpenAIGatewayService) OpenOpenAIRealtime(ctx context.Context, account *Account, model string, clientHeaders http.Header) (*GrokRealtimeUpstream, error) {
	if s == nil || account == nil {
		return nil, fmt.Errorf("openai realtime service/account is required")
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("account %d does not support openai realtime", account.ID)
	}
	apiKey := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if apiKey == "" {
		return nil, fmt.Errorf("account %d missing api_key", account.ID)
	}
	baseURL := account.GetOpenAIFormatBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	validatedURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}
	u, err := url.Parse(buildOpenAIEndpointURL(validatedURL, "/v1/realtime"))
	if err != nil {
		return nil, fmt.Errorf("parse realtime url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	default:
		u.Scheme = "wss"
	}
	requested := strings.TrimSpace(model)
	if requested == "" {
		requested = DefaultOpenAIRealtimeModel
	}
	upstreamModel := normalizeOpenAIModelForUpstream(account, resolveOpenAIForwardModel(account, requested, ""))
	q := u.Query()
	q.Set("model", upstreamModel)
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	for _, name := range openAIRealtimeForwardedHeaders {
		if v := strings.TrimSpace(clientHeaders.Get(name)); v != "" {
			headers.Set(name, v)
		}
	}
	if customUA := account.GetOpenAIUserAgent(); customUA != "" {
		headers.Set("User-Agent", customUA)
	}
	account.ApplyHeaderOverrides(headers)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	conn, status, _, err := dialWSWithLedger(ctx, s.getOpenAIWSPassthroughDialer(), account, u.String(), headers, proxyURL)
	if err != nil {
		return nil, &GrokRealtimeDialError{StatusCode: status, Err: err}
	}
	return &GrokRealtimeUpstream{conn: conn, account: account}, nil
}

// HandleOpenAIRealtimeUpstreamError applies the shared OpenAI account policy to
// a failed pre-accept realtime handshake. Network errors without an HTTP status
// do not penalize the account.
func (s *OpenAIGatewayService) HandleOpenAIRealtimeUpstreamError(ctx context.Context, account *Account, statusCode int, body []byte) {
	if s == nil || account == nil || statusCode <= 0 {
		return
	}
	s.handleOpenAIAccountUpstreamError(ctx, account, statusCode, nil, body)
}
