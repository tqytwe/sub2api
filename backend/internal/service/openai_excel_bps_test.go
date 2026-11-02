package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type excelBPSHTTPUpstreamStub struct {
	request *http.Request
	body    []byte
	resp    *http.Response
	err     error
}

func (s *excelBPSHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.request = req
	s.body, _ = io.ReadAll(req.Body)
	return s.resp, s.err
}

func (s *excelBPSHTTPUpstreamStub) DoWithTLS(req *http.Request, proxy string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxy, accountID, concurrency)
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

func TestForwardExcelBPSUsesIndependentGrantAndParsesTerminalUsage(t *testing.T) {
	account := testExcelAccount()
	account.Extra = map[string]any{"openai_excel_bps": true}
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"X-Request-Id": []string{"req-bps-1"}},
		Body:       io.NopCloser(strings.NewReader("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-bps-1\",\"model\":\"gpt-5\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":12,\"output_tokens\":4,\"input_tokens_details\":{\"cached_tokens\":3}}}}\n\n")),
	}}
	gateway := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-5","available":true}]}}`, true, upstream)
	c, recorder := newExcelBPSContext()
	input := []byte(`{"model":"gpt-5","input":"hello","stream":true,"reasoning":{"effort":"high"}}`)

	result, err := gateway.forwardExcelBPS(context.Background(), c, account, input, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "Bearer excel-only-access", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "acct-excel-42", upstream.request.Header.Get("Chatgpt-Account-Id"))
	require.Empty(t, upstream.request.Header.Get("Cookie"))
	require.Empty(t, upstream.request.Header.Get("X-Codex-Turn-Metadata"))
	require.Equal(t, "https://bps.openai.com/basispoints/api/responses", upstream.request.URL.String())
	require.Equal(t, "gpt-5", gjson.GetBytes(upstream.body, "model").String())
	require.Equal(t, "resp-bps-1", result.ResponseID)
	require.Equal(t, "response.completed", result.UpstreamTerminalEvent)
	require.Equal(t, "req-bps-1", result.RequestID)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 4, result.Usage.OutputTokens)
	require.Equal(t, 3, result.Usage.CacheReadInputTokens)
	require.Contains(t, recorder.Body.String(), "response.completed")
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
		input := []byte(`{"model":"gpt-5","input":[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/image.png"}]}]}`)

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
