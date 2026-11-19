package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var excelBPSReplay basispoints.ReplayCache
var excelBPSCatalog basispoints.CatalogCache

const excelBPSResponsesPath = "/basispoints/api/responses"

var excelBPSAttachments basispoints.AttachmentCache

func (s *OpenAIGatewayService) excelBPSRuntimeCredential(ctx context.Context, account *Account) (*OpenAIExcelOAuthRuntimeCredential, error) {
	if s.excelOAuthService == nil {
		return nil, infraerrors.ServiceUnavailable("basispoints_auth_unavailable", "Excel OAuth runtime is unavailable")
	}
	if account.AuthConnectionID == "" {
		return s.excelOAuthService.RuntimeCredential(ctx, account.ID)
	}
	if account.GetExtraString("openai_connection_kind") != "bps" {
		return nil, connectionChanged()
	}
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	return s.managedExcelRuntimeCredential(account, token), nil
}

// BPS uses the selected Excel grant and does not send Codex tickets or cookies.
func (s *OpenAIGatewayService) forwardExcelBPS(ctx context.Context, c *gin.Context, account *Account, body []byte, start time.Time) (result *OpenAIForwardResult, forwardErr error) {
	fail := func(status int, code, message string) (*OpenAIForwardResult, error) {
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "code": code, "message": message}})
		return nil, fmt.Errorf("excel bps: %s", code)
	}
	requestBodyBytes := len(body)
	originalModel := gjson.GetBytes(body, "model").String()
	model := account.GetMappedModel(originalModel)
	stream := gjson.GetBytes(body, "stream").Bool()
	var compactUsage OpenAIUsage
	var compactWindow []any
	var compactWireUsage map[string]any
	var compactID string
	cacheCreationAsInput := account.IsExcelBPSCacheCreationAsInputEnabledForModel(originalModel)
	defer func() {
		if compactUsage.InputTokens == 0 && compactUsage.OutputTokens == 0 {
			return
		}
		if result == nil {
			result = &OpenAIForwardResult{Model: originalModel, UpstreamModel: model, UpstreamEndpoint: excelBPSResponsesPath,
				Stream: stream, RequestID: compactID, Duration: time.Since(start), bpsCacheCreationAsInput: &cacheCreationAsInput}
		}
		addImagePolicyUsage(&result.Usage, compactUsage)
	}()
	var err error
	body, err = sjson.SetBytes(body, "model", model)
	if err != nil {
		return fail(400, "basispoints_request_invalid", "Invalid model request")
	}
	body, err = s.applyOpenAIFastPolicyToBody(ctx, account, model, body)
	if err != nil {
		var blocked *OpenAIFastBlockedError
		if errors.As(err, &blocked) {
			MarkResponseCommitted(c)
			writeOpenAIFastPolicyBlockedResponse(c, blocked)
		}
		return nil, err
	}
	identity := explicitOpenAIRequestSessionID(c, body)
	if thread := gjson.GetBytes(body, "client_metadata.thread_id").String(); thread != "" {
		identity = thread
	}
	if identity != "" {
		body, err = sjson.SetBytes(body, "prompt_cache_key", identity)
		if err != nil {
			return nil, err
		}
	}
	if isOpenAIResponsesCompactPath(c) {
		var request map[string]any
		if err = json.Unmarshal(body, &request); err != nil {
			return fail(400, "basispoints_request_invalid", "Invalid compact request")
		}
		var input []any
		switch v := request["input"].(type) {
		case []any:
			input = v
		case string:
			input = []any{map[string]any{"role": "user", "content": v}}
		default:
			return fail(400, "basispoints_request_invalid", "Compact requires input")
		}
		request["input"] = append(input, map[string]any{"type": "compaction_trigger"})
		request["tool_choice"] = "none"
		body, err = json.Marshal(request)
		if err != nil {
			return nil, err
		}
	}
	scope := fmt.Sprintf("account:%d/key:%d/connection:%s/revision:%d/thread:%s", account.ID, getAPIKeyIDFromContext(c), account.AuthConnectionID, account.AuthConnectionRevision, identity)
	imageSettings, err := s.settingService.GetExcelBPSImageRelaySettings(ctx)
	if err != nil {
		return fail(http.StatusServiceUnavailable, "basispoints_image_settings_unavailable", "Excel BPS image settings are unavailable")
	}
	releaseImageAdmission, err := s.admitExcelBPSImageRequest(c, imageSettings, requestBodyBytes)
	if err != nil {
		return nil, err
	}
	defer releaseImageAdmission()
	imagePolicy, err := s.prepareExcelImagePolicy(ctx, c, body, imageSettings, scope+"/model:"+model, identity != "")
	if err != nil {
		var policyErr *excelImagePolicyError
		if errors.As(err, &policyErr) {
			return fail(policyErr.Status, policyErr.Code, policyErr.Message)
		}
		return fail(http.StatusBadRequest, "basispoints_request_invalid", err.Error())
	}
	if len(imagePolicy.reconciledBody) > 0 {
		body = imagePolicy.reconciledBody
	}
	if imageSettings.Enabled && imageSettings.Mode == ExcelBPSImageModeRelay {
		relay, relayErr := s.getExcelBPSImageRelay(imageSettings)
		if relayErr != nil {
			return fail(http.StatusServiceUnavailable, "basispoints_image_relay_unavailable", "Excel BPS image relay is unavailable")
		}
		body, err = relay.RewriteWithImageLimit(body, scope, imagePolicy.maxImages)
		if errors.Is(err, basispoints.ErrImageRelayFull) || errors.Is(err, basispoints.ErrImageRelayStorage) {
			return fail(http.StatusServiceUnavailable, "basispoints_image_relay_unavailable", "Excel BPS image relay is unavailable")
		}
		if err != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", err.Error())
		}
	}
	images, err := basispoints.PrepareNativeImagesWithLimit(body, imagePolicy.maxImages)
	if err != nil {
		return fail(400, "basispoints_request_invalid", err.Error())
	}
	if !imageSettings.Enabled && images.HasImages() {
		return fail(http.StatusBadRequest, "basispoints_image_disabled", "Excel BPS image input is disabled")
	}
	var replay *basispoints.ReplayCache
	var catalog *basispoints.CatalogCache
	if identity != "" {
		replay, catalog = &excelBPSReplay, &excelBPSCatalog
	}
	upstreamBody, bridge, err := images.PrepareWithCatalog(scope, replay, catalog)
	if err != nil {
		return fail(400, "basispoints_request_invalid", err.Error())
	}
	if err := s.checkAPIKeyLongContextBeforeSend(ctx, c, upstreamBody, model); err != nil {
		return nil, err
	}
	runtimeCredential, err := s.excelBPSRuntimeCredential(ctx, account)
	if err != nil {
		if infraErr, ok := err.(*infraerrors.Error); ok {
			return fail(int(infraErr.Code), infraErr.Reason, infraErr.Message)
		}
		return fail(502, "basispoints_auth_unavailable", "Account OAuth credential is unavailable")
	}
	if err = s.excelOAuthService.EnsureModelAllowed(ctx, account.ID, model, runtimeCredential); err != nil {
		if infraErr, ok := err.(*infraerrors.Error); ok {
			return fail(int(infraErr.Code), infraErr.Reason, infraErr.Message)
		}
		return fail(502, "basispoints_model_access_unavailable", "Excel BPS model access could not be verified")
	}
	token := runtimeCredential.AccessToken
	accountID := runtimeCredential.ChatGPTAccountID
	requestCtx := WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileLongStream))
	if images.HasImages() {
		credentialDigest := sha256.Sum256([]byte(token))
		credentialScope := hex.EncodeToString(credentialDigest[:])
		attachmentScope := fmt.Sprintf("transient:account:%d/key:%d/connection:%s/revision:%d/credential:%s/request:%d", account.ID, getAPIKeyIDFromContext(c), account.AuthConnectionID, runtimeCredential.Revision, credentialScope, time.Now().UnixNano())
		if identity != "" {
			attachmentScope = fmt.Sprintf("account:%d/key:%d/connection:%s/revision:%d/credential:%s/thread:%s/account-id:%s", account.ID, getAPIKeyIDFromContext(c), account.AuthConnectionID, runtimeCredential.Revision, credentialScope, identity, accountID)
		}
		body, err = images.Upload(requestCtx, &excelBPSAttachments, attachmentScope, func(uploadCtx context.Context, img basispoints.InlineAttachment) (string, error) {
			return s.uploadExcelBPSAttachment(uploadCtx, account, token, accountID, img)
		})
		if err != nil {
			var rejection *excelBPSAttachmentRejection
			if errors.As(err, &rejection) && rejection.status == http.StatusTooManyRequests {
				s.applyExcelBPSCooldown(ctx, account, rejection.headers)
				return nil, newExcelBPSRateLimitedFailoverError(rejection.headers.Get("Retry-After"))
			}
			return fail(502, "basispoints_attachment_error", "Excel BPS attachment upload failed; generation was not sent")
		}
		upstreamBody, bridge, err = bridge.Reprepare(body)
		if err != nil {
			return fail(400, "basispoints_request_invalid", err.Error())
		}
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	if imagePolicy.split > 0 {
		uploaded, inspectErr := basispoints.InspectImageHistory(body)
		if inspectErr != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", inspectErr.Error())
		}
		compactBody, buildErr := uploaded.WithInput(uploaded.Input[:imagePolicy.split], true)
		if buildErr != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", "Could not prepare image compaction")
		}
		compactWire, _, buildErr := bridge.Reprepare(compactBody)
		if buildErr != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", buildErr.Error())
		}
		compactReq, buildErr := newExcelBPSRequest(requestCtx, compactWire, token, accountID)
		if buildErr != nil {
			return nil, buildErr
		}
		if err := s.fenceAuthConnectionEgress(compactReq.Context(), compactReq, account); err != nil {
			return nil, err
		}
		SetActualOpenAIUpstreamEndpoint(c, excelBPSResponsesPath)
		SetOpsUpstreamModel(c, model)
		compactResp, sendErr := s.httpUpstream.Do(compactReq, proxyURL, account.ID, account.Concurrency)
		if sendErr != nil {
			return fail(http.StatusBadGateway, "basispoints_image_compaction_failed", "Image history compaction connection failed; generation was not started")
		}
		compactID = compactResp.Header.Get("x-request-id")
		if compactResp.StatusCode < 200 || compactResp.StatusCode >= 300 {
			_ = compactResp.Body.Close()
			if compactResp.StatusCode == http.StatusTooManyRequests {
				s.applyExcelBPSCooldown(ctx, account, compactResp.Header)
			}
			return fail(compactResp.StatusCode, "basispoints_image_compaction_failed", "Image history compaction was rejected; generation was not started")
		}
		s.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, compactResp.Header)
		compactStream := s.guardExcelBPSProgress(requestCtx, compactResp.Body)
		compactResponse, compactErr := basispoints.ReadImageCompaction(compactStream, func(payload []byte) { s.parseSSEUsageBytes(payload, &compactUsage) })
		_ = compactStream.Close()
		if compactResponse != nil {
			compactWireUsage, _ = compactResponse["usage"].(map[string]any)
			encoded, _ := json.Marshal(map[string]any{"type": "response.completed", "response": compactResponse})
			s.parseSSEUsageBytes(encoded, &compactUsage)
		}
		if compactErr != nil {
			return fail(http.StatusBadGateway, "basispoints_image_compaction_failed", "Image history compaction did not complete; generation was not started")
		}
		compactWindow, _, compactErr = basispoints.CompactWindow(compactResponse)
		if compactErr != nil {
			return fail(http.StatusBadGateway, "basispoints_image_compaction_failed", compactErr.Error())
		}
		next := append(append([]any{}, compactWindow...), uploaded.Input[imagePolicy.split:]...)
		if basispoints.CountInlineImages(compactWindow)+basispoints.CountInlineImages(imagePolicy.history.Input[imagePolicy.split:]) > imageSettings.Limits.MaxImages {
			return fail(http.StatusBadRequest, "basispoints_image_compaction_insufficient", "Compacted history still exceeds the image limit; reduce new images or compact manually")
		}
		body, compactErr = uploaded.WithInput(next, false)
		if compactErr != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", "Could not prepare compacted continuation")
		}
		upstreamBody, bridge, compactErr = bridge.Reprepare(body)
		if compactErr != nil {
			return fail(http.StatusBadRequest, "basispoints_request_invalid", compactErr.Error())
		}
		if compactErr = imagePolicy.checkpoint(ctx, compactWindow); compactErr != nil {
			return fail(http.StatusServiceUnavailable, "basispoints_image_checkpoint_unavailable", "Could not save compacted history safely; retry or run compact manually")
		}
	}
	req, err := newExcelBPSRequest(requestCtx, upstreamBody, token, accountID)
	if err != nil {
		return nil, err
	}
	SetActualOpenAIUpstreamEndpoint(c, excelBPSResponsesPath)
	SetOpsUpstreamModel(c, model)
	sent := time.Now()
	if err = s.fenceAuthConnectionEgress(req.Context(), req, account); err != nil {
		return nil, err
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	s.rateLimitService.observeQualityResponse(req.Context(), account, resp, err)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(sent).Milliseconds())
	if err != nil {
		return fail(502, "basispoints_transport_error", "Excel BPS connection failed; request was not replayed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
		if resp.StatusCode == http.StatusTooManyRequests {
			s.applyExcelBPSCooldown(ctx, account, resp.Header)
			if len(compactWindow) > 0 {
				return fail(resp.StatusCode, "basispoints_rate_limited", "Excel BPS rate limited the compacted continuation")
			}
			return nil, newExcelBPSRateLimitedFailoverError(resp.Header.Get("Retry-After"))
		}
		code := gjson.GetBytes(raw, "error.code").String()
		message := s.handleExcelBPSAccessRejection(ctx, account, resp.StatusCode, code, model)
		if !isExcelBPSModelPermissionError(resp.StatusCode, code) {
			code = "basispoints_upstream_error"
		}
		return fail(resp.StatusCode, code, message)
	}
	s.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, resp.Header)
	converted := bridge.StreamWithRepairs(requestCtx, s.guardExcelBPSProgress(requestCtx, resp.Body), func(repairCtx context.Context, failed map[string]any, validation error) (map[string]any, error) {
		correctedBody, repairErr := basispoints.BuildToolRepairRequest(upstreamBody, failed, validation)
		if repairErr != nil {
			return nil, repairErr
		}
		return s.repairExcelBPSResponse(repairCtx, account, token, accountID, proxyURL, correctedBody)
	}, func(repairCtx context.Context) (io.ReadCloser, error) {
		repairBody, repairErr := basispoints.RepairRequest(upstreamBody)
		if repairErr != nil {
			return nil, repairErr
		}
		request, requestErr := newExcelBPSRequest(repairCtx, repairBody, token, accountID)
		if requestErr != nil {
			return nil, requestErr
		}
		if sendErr := s.fenceAuthConnectionEgress(repairCtx, request, account); sendErr != nil {
			return nil, sendErr
		}
		repaired, sendErr := s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
		if sendErr != nil {
			return nil, fmt.Errorf("excel BPS tool correction transport failed")
		}
		if repaired.StatusCode < 200 || repaired.StatusCode >= 300 {
			_ = repaired.Body.Close()
			if repaired.StatusCode == http.StatusTooManyRequests {
				s.applyExcelBPSCooldown(repairCtx, account, repaired.Header)
			}
			return nil, fmt.Errorf("excel BPS tool correction returned HTTP %d", repaired.StatusCode)
		}
		s.UpdateCodexUsageSnapshotFromHeaders(repairCtx, account.ID, repaired.Header)
		return s.guardExcelBPSProgress(repairCtx, repaired.Body), nil
	})
	if len(compactWindow) > 0 {
		converted = basispoints.WithCompactedWindow(requestCtx, converted, compactWindow, compactWireUsage)
	}
	defer func() { _ = converted.Close() }()
	result = &OpenAIForwardResult{
		Model: originalModel, UpstreamModel: model, UpstreamEndpoint: excelBPSResponsesPath,
		Stream: stream, ReasoningEffort: &bridge.Effort,
		RequestedReasoningEffort: coalesceRequestedReasoningEffort(RequestedReasoningEffortFromContext(ctx), &bridge.RequestedEffort),
		ServiceTier:              extractOpenAIServiceTierFromBody(upstreamBody),
		RequestID:                resp.Header.Get("x-request-id"),
		UpstreamHeaders:          resp.Header.Clone(), ResponseHeaders: resp.Header.Clone(),
	}
	if stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")
	}
	scanner := bufio.NewScanner(converted)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	result.bpsCacheCreationAsInput = &cacheCreationAsInput
	var completed []byte
	var upstreamFailure *basispoints.UpstreamFailure
	terminal := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			payload := []byte(strings.TrimPrefix(line, "data: "))
			kind := gjson.GetBytes(payload, "type").String()
			s.parseSSEUsageBytes(payload, &result.Usage)
			if len(compactWindow) > 0 {
				switch kind {
				case "response.completed", "response.failed", "response.cancelled", "response.incomplete":
					subtractImagePolicyUsage(&result.Usage, compactUsage)
				}
			}
			if cacheCreationAsInput {
				payload, err = excelBPSDownstreamUsage(payload)
				if err != nil {
					return result, fmt.Errorf("format Excel BPS response usage: %w", err)
				}
				line = "data: " + string(payload)
			}
			if result.FirstTokenMs == nil && (kind == "response.output_text.delta" || kind == "response.output_item.added") {
				ms := int(time.Since(start).Milliseconds())
				result.FirstTokenMs = &ms
			}
			switch kind {
			case "response.completed", "response.failed", "response.incomplete", "response.cancelled", "error":
				terminal = kind
				upstreamFailure = basispoints.ParseUpstreamFailure(payload)
				if upstreamFailure != nil {
					SetOpsUpstreamError(c, resp.StatusCode, upstreamFailure.Message, "")
					MarkOpsStreamErrorValue(c, OpsStreamError{
						ErrType: upstreamFailure.Type, Code: upstreamFailure.Code, Message: upstreamFailure.Message,
						IntendedStatus: upstreamFailure.Status, CountTowardsSLA: true, NonStream: !stream,
					})
					if upstreamFailure.Status == http.StatusTooManyRequests {
						s.applyExcelBPSCooldown(ctx, account, resp.Header)
					}
				}
				completed = []byte(gjson.GetBytes(payload, "response").Raw)
				result.ResponseID = gjson.GetBytes(payload, "response.id").String()
				result.UpstreamResponseModel = gjson.GetBytes(payload, "response.model").String()
				result.UpstreamResponseServiceTier = normalizeObservedOpenAIServiceTier(gjson.GetBytes(payload, "response.service_tier").String())
			}
		}
		if stream {
			if _, err = c.Writer.WriteString(line + "\n"); err != nil {
				result.ClientDisconnect = true
				result.Duration = time.Since(start)
				return result, err
			}
			if line == "" {
				c.Writer.Flush()
			}
		}
	}
	result.Duration = time.Since(start)
	result.UpstreamTerminalEvent = terminal
	if err = scanner.Err(); err != nil || terminal == "" {
		if stream {
			_, _ = c.Writer.WriteString("event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"basispoints_stream_incomplete\",\"message\":\"Upstream stream ended before completion\"}}}\n\n")
			c.Writer.Flush()
		} else {
			c.JSON(502, gin.H{"error": gin.H{"code": "basispoints_stream_incomplete", "message": "Excel BPS stream ended before completion"}})
		}
		return result, fmt.Errorf("excel bps stream incomplete")
	}
	if !stream {
		if upstreamFailure != nil {
			c.JSON(upstreamFailure.Status, gin.H{"error": upstreamFailure.Details()})
		} else if terminal != "response.completed" {
			c.JSON(502, gin.H{"error": gin.H{"code": "basispoints_protocol_error", "message": "Excel BPS did not complete the response"}})
		} else {
			c.Data(200, "application/json", completed)
		}
	}
	if terminal != "response.completed" {
		MarkResponseCommitted(c)
		return result, fmt.Errorf("excel bps terminal: %s", terminal)
	}
	imagePolicy.finish(ctx, imagePolicy.compact || len(compactWindow) > 0 || (imagePolicy.history != nil && imagePolicy.history.Count < imageSettings.Limits.MaxImages-imageSettings.WarningRemaining))
	s.bindHTTPResponseAccount(ctx, c, account, result.ResponseID)
	return result, nil
}

func (s *OpenAIGatewayService) guardExcelBPSProgress(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	idle := 120 * time.Second
	if s.cfg != nil {
		idle = time.Duration(s.cfg.Gateway.ExcelBPSStreamDataIntervalTimeout) * time.Second
	}
	return basispoints.WithProgressTimeout(ctx, body, idle)
}
