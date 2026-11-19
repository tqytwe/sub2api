package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestManagedConnectionAccountProbeUsesSelectedGrant(t *testing.T) {
	for _, kind := range []string{"codex", "siwc", "api_key"} {
		t.Run(kind, func(t *testing.T) {
			svc, repo := connectionTypeService(AccountTypeOAuth)
			repo.managed = true
			row := &OpenAIAuthConnection{ID: "91afc56d-430a-4257-b4f4-b07ceaf76bd2", AccountID: 42, Kind: kind, Status: "active", Revision: 1, Preferred: true, RefreshState: "idle", Capabilities: []string{"responses"}}
			client := openai.ClientID
			if kind == "siwc" {
				client = "issued-test-siwc"
			}
			require.NoError(t, svc.encryptConnectionCredentials(row, map[string]any{"access_token": "selected-token", "api_key": "selected-token", "client_id": client, "expires_at": strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)}))
			repo.rows = []*OpenAIAuthConnection{row}
			account, err := svc.accounts.GetAccount(context.Background(), 42)
			require.NoError(t, err)
			upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"probe\",\"output\":[]}}\n\n"))}}
			gateway := &OpenAIGatewayService{excelOAuthReauth: svc, httpUpstream: upstream}
			testService := &AccountTestService{openaiGatewayService: gateway}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/test", nil)
			err = testService.testManagedOpenAIConnection(c, account, "gpt-test", "", "")
			require.NoError(t, err)
			require.Equal(t, "Bearer selected-token", upstream.request.Header.Get("Authorization"))
			require.Empty(t, upstream.request.Header.Get("Cookie"))
			require.True(t, HTTPUpstreamRedirectsDisabled(upstream.request.Context()), "every managed send must re-enter the connection fence instead of following redirects inside HTTP transport")
			if kind == "siwc" {
				require.Equal(t, "api.openai.com", upstream.request.URL.Host)
			}
		})
	}
}

func TestManagedCompactAccountProbeUsesNativeResponsesCompaction(t *testing.T) {
	for _, output := range []string{"[]", `[{"type":"compaction","encrypted_content":"synthetic"}]`} {
		t.Run(output, func(t *testing.T) {
			svc, repo := connectionTypeService(AccountTypeOAuth)
			repo.managed = true
			row := &OpenAIAuthConnection{ID: "91afc56d-430a-4257-b4f4-b07ceaf76bd2", AccountID: 42, Kind: "codex", Status: "active", Revision: 1, Preferred: true, RefreshState: "idle", Capabilities: []string{"responses"}}
			require.NoError(t, svc.encryptConnectionCredentials(row, map[string]any{"access_token": "selected-token", "client_id": openai.ClientID, "expires_at": strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)}))
			repo.rows = []*OpenAIAuthConnection{row}
			account, err := svc.accounts.GetAccount(context.Background(), 42)
			require.NoError(t, err)
			upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"probe\",\"output\":" + output + "}}\n\n"))}}
			gateway := &OpenAIGatewayService{excelOAuthReauth: svc, httpUpstream: upstream}
			testService := &AccountTestService{openaiGatewayService: gateway}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/test", nil)
			err = testService.testManagedOpenAIConnection(c, account, "gpt-test", "", AccountTestModeCompact)
			if output == "[]" {
				require.ErrorContains(t, err, "without a compaction output item")
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, "/backend-api/codex/responses", upstream.request.URL.Path)
			require.Contains(t, string(upstream.body), `"type":"compaction_trigger"`)
			require.NotContains(t, upstream.request.URL.Path, "/compact")
		})
	}
}
