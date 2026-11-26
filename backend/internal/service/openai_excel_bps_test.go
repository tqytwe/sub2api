package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type excelBPSHTTPUpstreamStub struct {
	request   *http.Request
	body      []byte
	resp      *http.Response
	requests  []*http.Request
	bodies    [][]byte
	responses []*http.Response
	answer    func([]byte) *http.Response
	err       error
}

func (s *excelBPSHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	s.body, _ = io.ReadAll(req.Body)
	s.requests = append(s.requests, req.Clone(req.Context()))
	s.bodies = append(s.bodies, append([]byte(nil), s.body...))
	if len(s.responses) > 0 {
		response := s.responses[0]
		s.responses = s.responses[1:]
		return response, s.err
	}
	if s.answer != nil {
		return s.answer(s.body), s.err
	}
	return s.resp, s.err
}

func (s *excelBPSHTTPUpstreamStub) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxy, accountID, concurrency)
}

func TestForwardExcelBPSUploadsInlineImageAsNativeAttachment(t *testing.T) {
	account := testExcelAccount()
	account.Extra = map[string]any{"openai_excel_bps": true}
	upstream := &excelBPSHTTPUpstreamStub{responses: []*http.Response{
		{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"openai_file_id":"file-image-1"}`))},
		{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-image\",\"model\":\"gpt-5\",\"status\":\"completed\",\"output\":[]}}\n\n"))},
	}}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	c, _ := newExcelBPSContext()
	body := []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jv5sAAAAASUVORK5CYII="}]}]}`)
	_, err := gateway.forwardExcelBPS(context.Background(), c, account, body, time.Now())
	require.NoError(t, err)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "https://bps.openai.com/basispoints/api/attachments", upstream.requests[0].URL.String())
	require.Contains(t, upstream.requests[0].Header.Get("Content-Type"), "multipart/form-data")
	require.Equal(t, "https://bps.openai.com/basispoints/api/responses", upstream.requests[1].URL.String())
	imagePart := gjson.GetBytes(upstream.bodies[1], `input.#(content.#(type=="input_image").type=="input_image").content.#(type=="input_image")`)
	require.Equal(t, "file-image-1", imagePart.Get("file_id").String())
	require.False(t, imagePart.Get("image_url").Exists())
}

func newExcelBPSRuntime(t *testing.T, account *Account, catalog string, credential bool, upstream HTTPUpstream) *OpenAIGatewayService {
	t.Helper()

	var record *OpenAIExcelOAuthCredentialRecord
	if credential {
		payload, err := json.Marshal(openAIExcelOAuthCredentialPayload{
			AccessToken: "excel-only-access", ChatGPTAccountID: "acct-excel-42",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		})
		require.NoError(t, err)
		ciphertext, err := (excelOAuthEncryptorStub{}).Encrypt(string(payload))
		require.NoError(t, err)
		record = &OpenAIExcelOAuthCredentialRecord{AccountID: account.ID, CredentialsEncrypted: ciphertext, Revision: 1}
	}
	repo := &excelOAuthCredentialRepoStub{record: record}
	oauth := NewOpenAIOAuthService(nil, &excelOAuthClientStub{})
	t.Cleanup(oauth.Stop)
	excel := NewOpenAIExcelOAuthService(
		repo,
		&excelOAuthAccountReaderStub{account: account},
		oauth,
		excelOAuthEncryptorStub{},
		&excelOAuthAccessStub{body: []byte(catalog)},
	)
	gateway := &OpenAIGatewayService{httpUpstream: upstream}
	gateway.SetOpenAIExcelOAuthService(excel)
	return gateway
}

func newExcelBPSContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, recorder
}

func excelAccount() *Account {
	account := testExcelAccount()
	account.Extra = map[string]any{"openai_excel_bps": true, "openai_passthrough": true}
	return account
}

func TestForwardExcelBPSUsesIndependentGrantAndParsesTerminalUsage(t *testing.T) {
	account := testExcelAccount()
	account.Extra = map[string]any{"openai_excel_bps": true}
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Request-Id": []string{"req-bps-1"}},
		Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-bps-1\",\"model\":\"gpt-5\",\"service_tier\":\"default\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":12,\"output_tokens\":4,\"input_tokens_details\":{\"cached_tokens\":3}}}}\n\n")),
	}}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	c, recorder := newExcelBPSContext()
	input := []byte(`{"model":"gpt-5","input":"hello","stream":true,"service_tier":"fast","reasoning":{"effort":"high"}}`)

	result, err := gateway.forwardExcelBPS(WithRequestedReasoningEffort(context.Background(), "xhigh"), c, account, input, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Bearer excel-only-access", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "acct-excel-42", upstream.request.Header.Get("Chatgpt-Account-Id"))
	require.Empty(t, upstream.request.Header.Get("Cookie"))
	require.Empty(t, upstream.request.Header.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, "https://bps.openai.com/basispoints/api/responses", upstream.request.URL.String())
	require.Equal(t, "gpt-5", gjson.GetBytes(upstream.body, "model").String())
	require.False(t, gjson.GetBytes(upstream.body, "service_tier").Exists())
	require.Nil(t, result.ServiceTier)
	require.Equal(t, "default", result.UpstreamResponseServiceTier)
	require.Equal(t, "xhigh", optionalStringValue(result.RequestedReasoningEffort))
	require.Equal(t, "resp-bps-1", result.ResponseID)
	require.Equal(t, "response.completed", result.UpstreamTerminalEvent)
	require.Equal(t, "req-bps-1", result.RequestID)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.Contains(t, recorder.Body.String(), "response.completed")
}

func TestForwardExcelBPSRejectsUnsupportedMediaBeforeTransport(t *testing.T) {
	for _, content := range []string{
		`[{"type":"input_audio"}]`,
		`[{"type":"message","role":"user","content":[{"type":"input_file","file_id":"file-test"}]}]`,
	} {
		account := excelAccount()
		upstream := &excelBPSHTTPUpstreamStub{}
		gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
		c, recorder := newExcelBPSContext()
		_, err := gateway.forwardExcelBPS(context.Background(), c, account, []byte(`{"model":"gpt-5","input":`+content+`}`), time.Now())
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Empty(t, upstream.requests)
	}
}

func TestForwardExcelBPSCacheCreationUsageMatchesBilling(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			account := excelAccount()
			account.Extra["openai_excel_bps_cache_creation_as_input"] = enabled
			const usage = `{"input_tokens":12,"output_tokens":4,"input_tokens_details":{"cached_tokens":3,"cache_write_tokens":5}}`
			const response = `{"type":"response.completed","response":{"id":"resp-bps-usage","model":"gpt-5","status":"completed","output":[],"usage":` + usage + `}}`
			upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("data: " + response + "\n\n"))}}
			gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
			c, recorder := newExcelBPSContext()
			body, err := json.Marshal(map[string]any{"model": "gpt-5", "input": "hello", "stream": stream})
			require.NoError(t, err)
			result, err := gateway.forwardExcelBPS(context.Background(), c, account, body, time.Now())
			require.NoError(t, err)
			require.Equal(t, 5, result.Usage.CacheCreationInputTokens)
			require.Equal(t, 3, result.Usage.CacheReadInputTokens)
			require.Equal(t, 12, result.Usage.InputTokens)
			want := `"cache_write_tokens":5`
			if enabled {
				want = `"cache_write_tokens":0`
			}
			require.Contains(t, recorder.Body.String(), want, "stream=%t enabled=%t", stream, enabled)
			require.Contains(t, recorder.Body.String(), `"input_tokens":12`)
			require.Contains(t, recorder.Body.String(), `"cached_tokens":3`)
		}
	}
}

func TestForwardExcelBPSEnforcesFastPolicyBeforeTransport(t *testing.T) {
	account := excelAccount()
	upstream := &excelBPSHTTPUpstreamStub{}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	settings := &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
		ServiceTier: OpenAIFastTierPriority, Action: BetaPolicyActionBlock, Scope: BetaPolicyScopeAll,
	}}}
	gateway.settingService = newOpenAIGatewayServiceWithSettings(t, settings).settingService
	c, recorder := newExcelBPSContext()

	_, err := gateway.forwardExcelBPS(context.Background(), c, account, []byte(`{"model":"gpt-5","input":"hello","service_tier":"priority"}`), time.Now())
	var blocked *OpenAIFastBlockedError
	require.ErrorAs(t, err, &blocked)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Empty(t, upstream.requests)
}

func TestExcelBPSRequestHeadersUseSelectedBearerUser(t *testing.T) {
	token := reauthTestJWT(map[string]any{"https://api.openai.com/auth": map[string]any{
		"chatgpt_user_id": "excel-user", "user_id": "fallback-user",
	}})
	req, err := newExcelBPSRequest(context.Background(), nil, token, "excel-account")
	require.NoError(t, err)
	require.Equal(t, "excel-user", req.Header.Get("X-Openai-Account-User-Id"))
	require.Equal(t, "excel-account", req.Header.Get("Chatgpt-Account-Id"))
}

func TestForwardExcelBPSRejectsModelOutsideCatalogBeforeTransport(t *testing.T) {
	account := testExcelAccount()
	account.Extra = map[string]any{"openai_excel_bps": true}
	upstream := &excelBPSHTTPUpstreamStub{}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	c, recorder := newExcelBPSContext()

	_, err := gateway.forwardExcelBPS(context.Background(), c, account, []byte(`{"model":"gpt-5-mini","input":"hello"}`), time.Now())
	require.Error(t, err)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Nil(t, upstream.request)
}

func TestForwardExcelBPSRejectsUnboundGrantAndImagesBeforeTransport(t *testing.T) {
	t.Run("unbound grant", func(t *testing.T) {
		account := testExcelAccount()
		account.Extra = map[string]any{"openai_excel_bps": true}
		upstream := &excelBPSHTTPUpstreamStub{}
		gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, false, upstream)
		c, recorder := newExcelBPSContext()

		_, err := gateway.forwardExcelBPS(context.Background(), c, account, []byte(`{"model":"gpt-5","input":"hello"}`), time.Now())
		require.Error(t, err)
		require.Equal(t, http.StatusConflict, recorder.Code)
		require.Nil(t, upstream.request)
	})

	t.Run("image input", func(t *testing.T) {
		account := testExcelAccount()
		account.Extra = map[string]any{"openai_excel_bps": true}
		upstream := &excelBPSHTTPUpstreamStub{}
		gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
		c, _ := newExcelBPSContext()
		input := []byte(`{"model":"gpt-5","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,not-valid"}]}]}`)

		_, err := gateway.forwardExcelBPS(context.Background(), c, account, input, time.Now())
		require.Error(t, err)
		require.Nil(t, upstream.request)

		c, _ = newExcelBPSContext()
		input = []byte(`{"model":"gpt-5","input":[{"type":"message","role":"user","content":{"type":"input_image","image_url":"https://example.test/image.png"}}]}`)
		_, err = gateway.forwardExcelBPS(context.Background(), c, account, input, time.Now())
		require.Error(t, err)
		require.Nil(t, upstream.request)
	})
}

// BPS adds a protocol prologue and can expand tool history. A small ingress body
// cannot bypass a key ceiling by using this Fork-specific transport.
func TestForwardExcelBPSRechecksActualExpandedContext(t *testing.T) {
	account := testExcelAccount()
	upstream := &excelBPSHTTPUpstreamStub{}
	svc := newExcelBPSRuntime(t, account, `{"models":[]}`, false, upstream)
	c, _ := newExcelBPSContext()
	key := &APIKey{AllowLongContext: true, ContextTokenLimit: 10}
	c.Set("api_key", key)
	body := []byte(`{"model":"gpt-5","input":"hi"}`)
	require.NoError(t, svc.CheckAPIKeyLongContext(context.Background(), key, body, "gpt-5"))
	_, err := svc.forwardExcelBPS(context.Background(), c, account, body, time.Now())
	var policyErr *APIKeyLongContextError
	require.ErrorAs(t, err, &policyErr)
	require.Equal(t, "context_length_exceeded", policyErr.Code)
	require.True(t, IsOpenAITurnAdmissionError(err))
	require.Nil(t, upstream.request, "no credential/network work may bypass the expanded-body policy")
}

func TestForwardExcelBPSRateLimitLeavesNativeRouteAvailable(t *testing.T) {
	account := testExcelAccount()
	account.Status = StatusActive
	account.Schedulable = true
	account.Extra = map[string]any{"openai_excel_bps": true, "openai_excel_bps_models": []string{"gpt-5"}}
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": {"60"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"private upstream detail"}}`)),
	}}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	c, _ := newExcelBPSContext()
	_, err := gateway.forwardExcelBPS(c.Request.Context(), c, account, []byte(`{"model":"gpt-5","input":"test"}`), time.Now())
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, ExcelBPSRateLimitedReason, failover.Reason)
	require.Equal(t, "60", failover.ResponseHeaders.Get("Retry-After"))
	require.Empty(t, failover.ResponseBody)
	require.False(t, c.Writer.Written())
	require.False(t, failover.ShouldReportAccountScheduleFailure())
	require.False(t, gateway.isOpenAIAccountSchedulableForRequest(c.Request.Context(), account, "gpt-5", false))
	require.True(t, gateway.isOpenAIAccountSchedulableForRequest(c.Request.Context(), account, "gpt-5-mini", false))
}

func TestForwardExcelBPSStreamRateLimitDoesNotReplay(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		name := "json"
		if streaming {
			name = "sse"
		}
		t.Run(name, func(t *testing.T) {
			account := excelAccount()
			upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Retry-After": []string{"60"}},
				Body: io.NopCloser(strings.NewReader("event: response.failed\n" +
					`data: {"type":"response.failed","response":{"id":"resp-limited","status":"failed","error":{"code":"rate_limit_exceeded","message":"Rate limit reached"}}}` + "\n\n")),
			}}
			gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
			c, rec := newExcelBPSContext()
			body, err := json.Marshal(map[string]any{"model": "gpt-5", "input": "hi", "stream": streaming})
			require.NoError(t, err)
			result, err := gateway.forwardExcelBPS(c.Request.Context(), c, account, body, time.Now())
			require.Error(t, err)
			require.NotNil(t, result)
			require.Equal(t, "response.failed", result.UpstreamTerminalEvent)
			require.Len(t, upstream.requests, 1)
			require.Contains(t, rec.Body.String(), "rate_limit_exceeded")
			if streaming {
				require.Equal(t, http.StatusOK, rec.Code)
				require.Contains(t, rec.Body.String(), "event: response.failed")
			} else {
				require.Equal(t, http.StatusTooManyRequests, rec.Code)
				require.Equal(t, "rate_limit_exceeded", gjson.Get(rec.Body.String(), "error.code").String())
			}
			require.False(t, gateway.isOpenAIAccountSchedulableForRequest(c.Request.Context(), account, "gpt-5", false))
		})
	}
}

func TestForwardExcelBPSModelPermissionDoesNotBlockOtherModels(t *testing.T) {
	for _, code := range []string{"basispoints_model_access_changed", "model_not_allowed"} {
		t.Run(code, func(t *testing.T) {
			account := excelAccount()
			account.Credentials = map[string]any{"model_mapping": map[string]any{"alias": "gpt-5", "second-alias": "gpt-5", "gpt-5-mini": "gpt-5-mini"}}
			account.Extra["openai_excel_bps_models"] = []any{"gpt-5", "gpt-5-mini"}
			upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"` + code + `"}}`)),
			}}
			gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
			c, recorder := newExcelBPSContext()
			_, err := gateway.forwardExcelBPS(c.Request.Context(), c, account, []byte(`{"model":"alias","input":"test"}`), time.Now())
			require.Error(t, err)
			require.Equal(t, http.StatusForbidden, recorder.Code)
			require.Equal(t, code, gjson.Get(recorder.Body.String(), "error.code").String())
			require.True(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "second-alias"))
			require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "gpt-5-mini"))
			account.Extra["openai_excel_bps"] = false
			require.False(t, gateway.isOpenAIAccountRequestRuntimeBlocked(account, "second-alias"))
		})
	}
}

func TestForwardExcelBPSOmitsRejectedServiceTier(t *testing.T) {
	for _, tier := range []string{"auto", "default", "priority", "fast", "group-force", "global-force"} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%t", tier, stream), func(t *testing.T) {
				account := excelAccount()
				upstream := &excelBPSHTTPUpstreamStub{answer: func(body []byte) *http.Response {
					if gjson.GetBytes(body, "service_tier").Exists() {
						return &http.Response{StatusCode: http.StatusUnprocessableEntity, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"Invalid request body"}}`))}
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`data: {"type":"response.completed","response":{"id":"resp-bps","status":"completed","model":"gpt-5","service_tier":"priority","output":[],"usage":{"input_tokens":2,"output_tokens":1}}}` + "\n\n"))}
				}}
				gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
				c, rec := newExcelBPSContext()
				ctx := t.Context()
				request := map[string]any{"model": "gpt-5", "input": "hello", "stream": stream}
				switch tier {
				case "group-force":
					ctx = context.WithValue(ctx, ctxkey.Group, &Group{ID: 7, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, ForceOpenAIFast: true})
				case "global-force":
					gateway.settingService = newOpenAIGatewayServiceWithSettings(t, &OpenAIFastPolicySettings{Rules: []OpenAIFastPolicyRule{{
						ServiceTier: OpenAIFastTierMissing, Action: OpenAIFastPolicyActionForcePriority, Scope: BetaPolicyScopeAll,
					}}}).settingService
				default:
					request["service_tier"] = tier
				}
				body, encodeErr := json.Marshal(request)
				require.NoError(t, encodeErr)
				result, err := gateway.forwardExcelBPS(ctx, c, account, body, time.Now())
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, rec.Code)
				require.Len(t, upstream.bodies, 1, "the first send must succeed without a failover or replay")
				require.False(t, gjson.GetBytes(upstream.bodies[0], "service_tier").Exists())
				require.Nil(t, result.ServiceTier, "provider metadata must not turn an omitted tier into billable Fast")
				require.Equal(t, "priority", result.UpstreamResponseServiceTier)
				require.Equal(t, 2, result.Usage.InputTokens)
			})
		}
	}
}
