package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// openAIRealtime relays /v1/realtime WebSocket sessions to OpenAI API key
// accounts. The upstream handshake completes before the client upgrade is
// accepted, so auth/endpoint failures stay HTTP errors and can fail over.
func (h *OpenAIGatewayHandler) openAIRealtime(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if !h.ensureResponsesDependencies(c, nil) {
		return
	}
	reqLog := requestLogger(c, "handler.openai_gateway.openai_realtime")
	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription, service.QuotaPlatform(c.Request.Context(), apiKey)); err != nil {
		h.writeOpenAIAudioBillingError(c, err)
		return
	}
	model := strings.TrimSpace(c.Query("model"))
	if model == "" {
		model = service.DefaultOpenAIRealtimeModel
	}
	setOpsRequestContext(c, model, false)
	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, model)
	forwardModel := openAIChannelForwardModel(channelMapping, model)

	inflightDone, inflightErr := reserveInflightBalance(c, h.billingCacheService, h.gatewayService, apiKey, subscription,
		service.InflightEstimateRequest{Model: model, Kind: service.InflightEstimateAudio, AudioMode: "realtime", AudioUnits: 1})
	if inflightErr != nil {
		h.writeOpenAIAudioBillingError(c, inflightErr)
		return
	}
	defer inflightDone()

	realtimeCtx := service.WithOpenAIProfitControlSuppressed(c.Request.Context())
	realtimeCtx, pricingAt := h.gatewayService.WithOpenAIRequestPricingContext(realtimeCtx, apiKey.GroupID)
	c.Request = c.Request.WithContext(realtimeCtx)

	failed := map[int64]struct{}{}
	var selection *service.AccountSelectionResult
	var release func()
	var upstream *service.GrokRealtimeUpstream
	candidateSeen := false
	for attempts := 0; attempts < 4; attempts++ {
		candidate, _, selectErr := h.gatewayService.SelectAccountWithSchedulerForCapability(
			c.Request.Context(), apiKey.GroupID, "", "", forwardModel, failed,
			service.OpenAIUpstreamTransportHTTPSSE, service.OpenAIEndpointCapabilityAudio,
			false, false, false, service.PlatformOpenAI,
		)
		if selectErr != nil || candidate == nil || candidate.Account == nil {
			break
		}
		candidateSeen = true
		account := candidate.Account
		streamStarted := false
		slotRelease, slot := h.acquireResponsesAccountSlot(c, apiKey.GroupID, "", candidate, false, &streamStarted, reqLog)
		if slot != openAISlotAcquireOK {
			if slot == openAISlotAcquireFailed {
				return
			}
			failed[account.ID] = struct{}{}
			continue
		}
		dialCtx, cancelDial := context.WithTimeout(c.Request.Context(), service.DefaultGrokRealtimeDialTimeout)
		candidateUpstream, openErr := h.gatewayService.OpenOpenAIRealtime(dialCtx, account, forwardModel, c.Request.Header)
		cancelDial()
		if openErr != nil {
			reqLog.Warn("openai_realtime.pre_accept_failed", zap.Int64("account_id", account.ID), zap.Error(openErr))
			var dialErr *service.GrokRealtimeDialError
			if errors.As(openErr, &dialErr) {
				h.gatewayService.HandleOpenAIRealtimeUpstreamError(c.Request.Context(), account, dialErr.StatusCode, []byte(openErr.Error()))
			}
			if slotRelease != nil {
				slotRelease()
			}
			failed[account.ID] = struct{}{}
			continue
		}
		selection, upstream, release = candidate, candidateUpstream, slotRelease
		break
	}
	if selection == nil || upstream == nil {
		if !candidateSeen {
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", openAIAudioNoAccountMessage)
		} else {
			h.errorResponse(c, http.StatusBadGateway, "upstream_error", "OpenAI realtime upstream unavailable")
		}
		return
	}
	if release != nil {
		defer release()
	}
	defer func() { _ = upstream.Close() }()
	setOpsSelectedAccount(c, selection.Account.ID, selection.Account.Platform)

	conn, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
	if err != nil {
		return
	}
	defer func() { _ = conn.CloseNow() }()

	started := time.Now()
	audioObserved, proxyErr := h.gatewayService.ProxyGrokRealtimeConn(c.Request.Context(), c, conn, upstream)
	elapsed := time.Since(started)
	if proxyErr != nil {
		reqLog.Info("openai_realtime.proxy_closed", zap.Error(proxyErr))
		if !isExpectedGrokRealtimeClose(proxyErr) {
			_ = conn.Close(coderws.StatusInternalError, "upstream realtime websocket failed")
		}
	}
	if result := openAIRealtimeBillingResult(model, elapsed, audioObserved); result != nil {
		h.recordOpenAIAudioUsage(c, apiKey, selection.Account, subscription, channelMapping, model, nil, pricingAt, result)
	}
}

// openAIRealtimeBillingResult bills a realtime session by minutes, only when
// audio frames were actually exchanged.
func openAIRealtimeBillingResult(model string, elapsed time.Duration, audioObserved bool) *service.OpenAIForwardResult {
	if !audioObserved || elapsed <= 0 {
		return nil
	}
	return &service.OpenAIForwardResult{
		RequestID:  service.StableOpenAIRealtimeBillingRequestID(""),
		Model:      model,
		Duration:   elapsed,
		AudioUsage: &service.AudioUsage{Mode: "realtime", DurationOrUnits: elapsed.Minutes()},
	}
}
