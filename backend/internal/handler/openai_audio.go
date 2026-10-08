package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const openAIAudioNoAccountMessage = "No available OpenAI API key accounts support the audio API"

// openAIAudio forwards /v1/audio/{speech,transcriptions,translations} to
// OpenAI API key accounts with account failover and group audio billing.
func (h *OpenAIGatewayHandler) openAIAudio(c *gin.Context, op service.OpenAIAudioOperation) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.audio",
		zap.String("operation", string(op)),
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	reqModel := service.ExtractOpenAIAudioRequestModel(contentType, body)
	if reqModel == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	reqLog = reqLog.With(zap.String("model", reqModel))
	setOpsRequestContext(c, reqModel, false)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	speechInput := ""
	if op == service.OpenAIAudioOperationSpeech {
		speechInput = service.ExtractOpenAIAudioSpeechInput(body)
		// Moderation extractors understand chat messages; audit the spoken text.
		if speechInput != "" {
			if auditBody, mErr := json.Marshal(map[string]any{
				"messages": []map[string]any{{"role": "user", "content": speechInput}},
			}); mErr == nil {
				if decision := h.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIChat, reqModel, auditBody); decision != nil && !decision.AllowNextStage {
					h.openAISecurityAuditError(c, decision)
					return
				}
			}
		}
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	forwardModel := openAIChannelForwardModel(channelMapping, reqModel)
	channelMappedModel := ""
	if channelMapping.Mapped {
		channelMappedModel = channelMapping.MappedModel
	}

	streamStarted := false
	userRelease, acquired := h.acquireResponsesUserSlot(c, subject.UserID, subject.Concurrency, false, &streamStarted, reqLog)
	if !acquired {
		return
	}
	if userRelease != nil {
		defer userRelease()
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		h.writeOpenAIAudioBillingError(c, err)
		return
	}
	inflightDone, inflightErr := reserveInflightBalance(c, h.billingCacheService, h.gatewayService, apiKey, subscription, openAIAudioInflightEstimate(op, reqModel, body, speechInput))
	if inflightErr != nil {
		h.writeOpenAIAudioBillingError(c, inflightErr)
		return
	}
	defer inflightDone()

	// Audio is billed per unit (chars/hours) via group audio prices, so it is
	// outside the token profit gate like images/video/live/Grok voice.
	audioCtx := service.WithOpenAIProfitControlSuppressed(c.Request.Context())
	audioCtx, pricingAt := h.gatewayService.WithOpenAIRequestPricingContext(audioCtx, apiKey.GroupID)
	c.Request = c.Request.WithContext(audioCtx)

	maxSwitches := h.maxAccountSwitches
	if maxSwitches <= 0 {
		maxSwitches = 3
	}
	failed := make(map[int64]struct{})
	var lastFailoverErr *service.UpstreamFailoverError
	for switches := 0; ; {
		selection, _, selectErr := h.gatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(), apiKey.GroupID, "", "", forwardModel, failed,
			service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilityAudio,
			false, false, false, service.PlatformOpenAI,
		)
		if selectErr != nil || selection == nil || selection.Account == nil {
			if failoverClientGone(c) {
				return
			}
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
				return
			}
			reqLog.Warn("openai_audio.account_select_failed", zap.Error(selectErr))
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", openAIAudioNoAccountMessage)
			return
		}
		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)

		release, slot := h.acquireResponsesAccountSlot(c, apiKey.GroupID, "", selection, false, &streamStarted, reqLog)
		if slot == openAISlotAcquireProfitVetoed {
			failed[account.ID] = struct{}{}
			continue
		}
		if slot != openAISlotAcquireOK {
			return
		}

		writerSize := c.Writer.Size()
		result, fwdErr := func() (*service.OpenAIForwardResult, error) {
			if release != nil {
				defer release()
			}
			return h.gatewayService.ForwardOpenAIAudio(c.Request.Context(), c, account, op, body, contentType, channelMappedModel)
		}()
		if fwdErr == nil {
			h.gatewayService.ReportOpenAIAccountScheduleResult(account, openAIAccountScheduleModel(c, account, reqModel, false, result), true, nil)
			h.recordOpenAIAudioUsage(c, apiKey, account, subscription, channelMapping, reqModel, body, pricingAt, result)
			return
		}
		h.gatewayService.ReportOpenAIAccountScheduleResult(account, openAIAccountScheduleModel(c, account, reqModel, false, result), false, nil, fwdErr)
		var failoverErr *service.UpstreamFailoverError
		if !errors.As(fwdErr, &failoverErr) {
			if c.Writer.Size() == writerSize {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			reqLog.Warn("openai_audio.forward_failed", zap.Int64("account_id", account.ID), zap.Error(fwdErr))
			return
		}
		if c.Writer.Size() != writerSize {
			h.handleFailoverExhausted(c, failoverErr, true)
			return
		}
		if failoverClientGone(c) {
			return
		}
		h.gatewayService.RecordOpenAIAccountSwitch()
		failed[account.ID] = struct{}{}
		lastFailoverErr = failoverErr
		if switches >= maxSwitches {
			h.handleFailoverExhausted(c, failoverErr, false)
			return
		}
		switches++
		reqLog.Warn("openai_audio.upstream_failover_switching",
			zap.Int64("account_id", account.ID),
			zap.Int("upstream_status", failoverErr.StatusCode),
			zap.Int("switch_count", switches),
		)
	}
}

func (h *OpenAIGatewayHandler) writeOpenAIAudioBillingError(c *gin.Context, err error) {
	status, code, message, retryAfter := billingErrorDetails(err)
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	h.errorResponse(c, status, code, message)
}

// recordOpenAIAudioUsage bills OpenAI audio/realtime via group audio prices.
// Audio money events always use the mandatory usage queue.
func (h *OpenAIGatewayHandler) recordOpenAIAudioUsage(
	c *gin.Context,
	apiKey *service.APIKey,
	account *service.Account,
	subscription *service.UserSubscription,
	channelMapping service.ChannelMappingResult,
	reqModel string,
	body []byte,
	pricingAt time.Time,
	result *service.OpenAIForwardResult,
) {
	if h == nil || c == nil || apiKey == nil || account == nil || result == nil || result.AudioUsage == nil {
		return
	}
	if strings.TrimSpace(result.AudioUsage.Mode) == "realtime" {
		result.RequestID = service.StableOpenAIRealtimeBillingRequestID(result.RequestID)
	} else {
		result.RequestID = service.StableOpenAIAudioBillingRequestID(result.RequestID)
	}
	payloadHash := service.HashUsageRequestPayload(body)
	if payloadHash == "" {
		payloadHash = service.HashUsageRequestPayload([]byte(result.RequestID))
	}
	input := &service.OpenAIRecordUsageInput{
		Result:             result,
		APIKey:             apiKey,
		User:               apiKey.User,
		Account:            account,
		Subscription:       subscription,
		InboundEndpoint:    GetInboundEndpoint(c),
		UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
		UserAgent:          c.GetHeader("User-Agent"),
		IPAddress:          ip.GetClientIP(c),
		RequestPayloadHash: payloadHash,
		APIKeyService:      h.apiKeyService,
		QuotaPlatform:      service.QuotaPlatform(c.Request.Context(), apiKey),
		SessionID:          service.ExtractClientSessionID(c),
		ChannelUsageFields: clientRequestedUsageFields(c, channelMapping, reqModel, result.UpstreamModel),
		PricingAt:          pricingAt,
	}
	h.submitMandatoryUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
		if err := h.gatewayService.RecordUsage(ctx, input); err != nil {
			logger.L().With(
				zap.String("component", "handler.openai_gateway.audio"),
				zap.Int64("api_key_id", apiKey.ID),
				zap.Any("group_id", apiKey.GroupID),
				zap.String("model", reqModel),
				zap.Int64("account_id", account.ID),
			).Error("openai_audio.record_usage_failed", zap.Error(err))
		}
	})
}

// openAIAudioInflightEstimate mirrors the billing units used after the call.
func openAIAudioInflightEstimate(op service.OpenAIAudioOperation, model string, body []byte, speechInput string) service.InflightEstimateRequest {
	req := service.InflightEstimateRequest{Model: model, Kind: service.InflightEstimateAudio, AudioMode: op.AudioMode()}
	if op == service.OpenAIAudioOperationSpeech {
		req.AudioUnits = float64(len([]rune(speechInput))) / 1e6
	} else {
		// Use OpenAI bitrate constant (16KB/s compressed audio), not Grok's.
		const openAIAudioSTTBytesPerSecond = 16000.0
		req.AudioUnits = float64(len(body)) / openAIAudioSTTBytesPerSecond / 3600
	}
	return req
}
