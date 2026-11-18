package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
)

func (s *AccountTestService) openAIAccountTestUsesManagedConnection(ctx context.Context, account *Account) (bool, error) {
	if account == nil || account.Platform != PlatformOpenAI {
		return false, nil
	}
	if s.openaiGatewayService != nil && s.openaiGatewayService.excelOAuthReauth != nil {
		if repo, ok := s.openaiGatewayService.excelOAuthReauth.repo.(OpenAIAuthConnectionRepository); ok {
			return repo.AuthConnectionsManaged(ctx, account.ID)
		}
	}
	if account.HasManagedOpenAIConnections() {
		return true, nil
	}
	return false, nil
}

// Managed probes use the selected grant and the gateway's actual send fence.
// They never feed a connection's 401 into the legacy parent credential state.
func (s *AccountTestService) testManagedOpenAIConnection(c *gin.Context, parent *Account, model, prompt, mode string) error {
	gateway := s.openaiGatewayService
	if gateway == nil {
		return s.sendErrorAndEnd(c, "Authorization connection runtime is unavailable")
	}
	ctx := c.Request.Context()
	// Credential-bearing probes must never follow an upstream redirect to a
	// different origin. Keep this on the request context through request build
	// and the final transport boundary.
	ctx = WithHTTPUpstreamRedirectsDisabled(ctx)
	route := c.Copy()
	route.Request = c.Request.Clone(ctx)
	urlCopy := *route.Request.URL
	urlCopy.Path = "/v1/responses"
	route.Request.URL = &urlCopy
	account, err := gateway.prepareAuthConnection(ctx, route, parent, nil, model)
	if err != nil {
		return s.sendErrorAndEnd(c, "No authorized connection is available for this test")
	}
	if isOpenAIImageModel(model) {
		return s.sendErrorAndEnd(c, "Selected authorization connection does not support image tests")
	}
	if model == "" {
		model = openai.DefaultTestModel
	}
	model = normalizeOpenAIModelForUpstream(account, account.GetMappedModel(model))
	recordQualityProbeRoute(ctx, account, model, "openai:managed")
	isCodex := account.GetExtraString("openai_connection_kind") == "codex"
	payload := createOpenAITestPayload(model, isCodex)
	compactProbe := mode == AccountTestModeCompact
	if compactProbe {
		payload = createOpenAICompactProbePayload(model, isCodex)
	}
	if !compactProbe {
		if prompt == "" {
			prompt = qualityProbePrompt(ctx)
		}
		applyQualityProbePromptToOpenAIPayload(payload, prompt)
		if effort := qualityProbeEffort(ctx); effort != "" {
			payload["reasoning"] = map[string]string{"effort": effort}
		}
		if pelican, _ := ctx.Value(pelicanAccountTestContextKey{}).(bool); pelican {
			payload = createPelicanOpenAIPayload(model, isCodex, prompt, qualityProbeEffort(ctx))
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to prepare connection test")
	}
	token, _, err := gateway.GetAccessToken(ctx, account)
	if err != nil {
		return s.sendErrorAndEnd(c, "Selected authorization connection requires authorization")
	}
	var req *http.Request
	var bridge *basispoints.Bridge
	if account.IsExcelBPSEnabled() {
		if gateway.excelOAuthService == nil {
			return s.sendErrorAndEnd(c, "Excel connection runtime is unavailable")
		}
		credential := gateway.managedExcelRuntimeCredential(account, token)
		if err = gateway.excelOAuthService.EnsureModelAllowed(ctx, account.ID, model, credential); err != nil {
			return s.sendErrorAndEnd(c, "Excel model access could not be verified")
		}
		body, bridge, err = basispoints.PrepareText(body, fmt.Sprintf("test/account:%d/connection:%s", account.ID, account.AuthConnectionID), nil)
		if err == nil {
			req, err = http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodPost, basispoints.ResponsesURL, bytes.NewReader(body))
		}
		if req != nil {
			applyExcelBPSRequestHeaders(req, token, credential.ChatGPTAccountID)
		}
	} else {
		req, err = gateway.buildUpstreamRequest(ctx, route, account, body, token, true, "", false)
	}
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to prepare selected connection request")
	}
	proxy := ""
	if account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	s.sendEvent(c, TestEvent{Type: "test_start", Model: model})
	resp, err := gateway.doOpenAIUpstream(req, proxy, account)
	if err != nil {
		return s.sendErrorAndEnd(c, "Selected connection request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		if gateway.excelOAuthReauth != nil && (resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode >= 500) {
			_ = gateway.excelOAuthReauth.FailAuthConnection(ctx, account, resp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512<<10))
		return s.sendErrorAndEnd(c, fmt.Sprintf("Selected connection returned %d", resp.StatusCode))
	}
	if compactProbe {
		streamBody := io.ReadCloser(resp.Body)
		if bridge != nil {
			streamBody = bridge.Stream(resp.Body)
		}
		compactBody, readErr := io.ReadAll(io.LimitReader(streamBody, 2<<20))
		_ = streamBody.Close()
		if readErr != nil {
			return s.sendErrorAndEnd(c, "Selected connection compaction probe could not be read")
		}
		if !openAICompactProbeFoundCompactionItem(compactBody) {
			return s.sendErrorAndEnd(c, "Upstream returned 2xx without a compaction output item (native remote compaction v2 unsupported on this chain)")
		}
		return s.processOpenAIStream(c, io.NopCloser(bytes.NewReader(compactBody)))
	}
	if bridge != nil {
		converted := bridge.Stream(resp.Body)
		defer func() { _ = converted.Close() }()
		return s.processOpenAIStream(c, converted)
	}
	return s.processOpenAIStream(c, resp.Body)
}

func (s *OpenAIGatewayService) managedExcelRuntimeCredential(account *Account, token string) *OpenAIExcelOAuthRuntimeCredential {
	return &OpenAIExcelOAuthRuntimeCredential{AccessToken: token, ChatGPTAccountID: account.GetCredential("chatgpt_account_id"), Revision: account.AuthConnectionRevision, ConnectionID: account.AuthConnectionID, ValidateSend: func(ctx context.Context) error { return s.fenceAuthConnectionEgress(ctx, nil, account) }}
}

func applyExcelBPSRequestHeaders(req *http.Request, token, accountID string) {
	req.Header = http.Header{"Authorization": {"Bearer " + token}, "Chatgpt-Account-Id": {accountID}, "X-Openai-Account-Id": {accountID}, "X-Basispoints-Auth-Mode": {"chatgpt"}, "Content-Type": {"application/json"}, "Accept": {"text/event-stream"}, "Origin": {"https://bps.openai.com"}, "User-Agent": {"Mozilla/5.0"}, "X-Openai-Internal-Basispoints-Client-Product": {"basispoints-excel-plugin"}, "X-Openai-Internal-Basispoints-Client-Agent-Profile": {"excel"}}
}
