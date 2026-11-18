package service

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/Wei-Shaw/sub2api/internal/util/transportdiag"
)

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (response *http.Response, err error) {
	if s.excelOAuthReauth != nil {
		if err := s.excelOAuthReauth.validateAuthConnectionPolicy(request.Context(), account); err != nil {
			return nil, err
		}
	}
	proxyID := int64(0)
	if account.ProxyID != nil {
		proxyID = *account.ProxyID
	}
	request, trace := requesttiming.StartAttempt(request, account.ID, proxyID)
	defer func() { trace.Response(response, err) }()
	defer func() { s.rateLimitService.observeQualityResponse(request.Context(), account, response, err) }()
	request = request.WithContext(requestledger.WithAccount(request.Context(), account.ID, ledgerCredentialAccountID(account)))
	if account.IsSIWCEnabled() {
		if request.URL == nil || request.URL.Scheme != "https" || request.URL.Host != "api.openai.com" || request.URL.Path != "/v1/responses" {
			return nil, fmt.Errorf("SIWC request must use the official Responses endpoint")
		}
		applySIWCRequestHeaders(request)
		request.Host = ""
	}
	if account.AuthConnectionID == "" && s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			// The host can observe the plugin boundary, but cannot measure the
			// network calls performed inside the plugin process.
			requesttiming.SetRoute(request.Context(), transportdiag.Route{Downstream: "http", Upstream: "plugin", Mode: "plugin", Endpoint: transportdiag.EndpointCategory(request.URL.Path)})
			trace.Headers(response)
			return response, err
		}
	}
	if err := s.fenceAuthConnectionEgress(request.Context(), request, account); err != nil {
		return nil, err
	}
	return s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
}

// doOpenAIAccountTestUpstream 让 OpenAI OAuth 账号测试与真实转发使用同一插件路径。
// API Key 和未命中插件的账号保持各自原有的 HTTPUpstream 行为。
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (*http.Response, error) {
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			if evidence, _ := request.Context().Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence); evidence != nil {
				evidence.NativeOAuth = false
				if evidence.Batch != nil {
					evidence.Batch.Route = "openai:plugin"
				}
			}
			return response, err
		}
	}
	if evidence, _ := request.Context().Value(qualityProbeEvidenceKey{}).(*qualityProbeEvidence); evidence != nil {
		evidence.NativeOAuth = account.IsOAuth() && !account.IsCredentialShadow() && !account.IsOpenAIAgentIdentity() && !account.IsExcelBPSEnabled()
	}
	if useTLSFallback {
		return s.httpUpstream.DoWithTLS(
			request,
			proxyURL,
			account.ID,
			account.Concurrency,
			s.tlsFPProfileService.ResolveTLSProfile(account),
		)
	}
	return s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
}
