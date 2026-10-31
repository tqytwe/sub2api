//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestQualityProbePromptIsEncodedForProviderPayloads(t *testing.T) {
	const prompt = "Reply with the exact phrase quality-check-9f31"

	t.Run("Anthropic", func(t *testing.T) {
		payload, err := createTestPayload("claude-sonnet-4-5", prompt)
		require.NoError(t, err)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.Equal(t, prompt, gjson.GetBytes(encoded, "messages.0.content.0.text").String())
	})

	t.Run("OpenAI Responses", func(t *testing.T) {
		payload := createOpenAITestPayload("gpt-5.4", true)
		applyQualityProbePromptToOpenAIPayload(payload, prompt)
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		require.Equal(t, prompt, gjson.GetBytes(encoded, "input.0.content.0.text").String())
		require.Equal(t, false, gjson.GetBytes(encoded, "store").Bool())
	})

	t.Run("Grok Responses", func(t *testing.T) {
		encoded, err := buildQualityProbeGrokBody("grok-4.3", prompt)
		require.NoError(t, err)
		require.Equal(t, prompt, gjson.GetBytes(encoded, "input").String())
		require.Equal(t, "grok-4.3", gjson.GetBytes(encoded, "model").String())
		require.True(t, gjson.GetBytes(encoded, "stream").Bool())
	})
}

func TestQualityProbePayloadHelpersKeepDefaultConnectionPrompts(t *testing.T) {
	claudePayload, err := createTestPayload("claude-sonnet-4-5")
	require.NoError(t, err)
	claudeEncoded, err := json.Marshal(claudePayload)
	require.NoError(t, err)
	require.Equal(t, "hi", gjson.GetBytes(claudeEncoded, "messages.0.content.0.text").String())

	openAIPayload := createOpenAITestPayload("gpt-5.4", false)
	applyQualityProbePromptToOpenAIPayload(openAIPayload, "  ")
	openAIEncoded, err := json.Marshal(openAIPayload)
	require.NoError(t, err)
	require.Equal(t, "hi", gjson.GetBytes(openAIEncoded, "input.0.content.0.text").String())

	grokEncoded, err := buildQualityProbeGrokBody("grok-4.3", "  ")
	require.NoError(t, err)
	require.Equal(t, grokQuotaProbeInput, gjson.GetBytes(grokEncoded, "input").String())
}

func TestRunQualityProbeBackgroundSendsConfiguredPromptToProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const prompt = "Reply with the exact phrase quality-check-9f31"
	response := func(body string) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}

	tests := []struct {
		name    string
		account *Account
		setup   func(*Account, *httpUpstreamRecorder) *AccountTestService
		path    string
	}{
		{
			name: "Anthropic",
			account: &Account{ID: 701, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 1,
				Credentials: map[string]any{"access_token": "anthropic-token"}},
			setup: func(account *Account, upstream *httpUpstreamRecorder) *AccountTestService {
				return &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}, httpUpstream: upstream}
			},
			path: "messages.0.content.0.text",
		},
		{
			name: "OpenAI Responses",
			account: &Account{ID: 702, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1,
				Credentials: map[string]any{"access_token": "openai-token"}},
			setup: func(account *Account, upstream *httpUpstreamRecorder) *AccountTestService {
				repo := &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}}
				return &AccountTestService{accountRepo: repo, httpUpstream: upstream}
			},
			path: "input.0.content.0.text",
		},
		{
			name: "Grok Responses",
			account: &Account{ID: 703, Platform: PlatformGrok, Type: AccountTypeOAuth, Concurrency: 1,
				Credentials: map[string]any{"access_token": "grok-token", "refresh_token": "grok-refresh", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}},
			setup: func(account *Account, upstream *httpUpstreamRecorder) *AccountTestService {
				repo := &mockAccountRepoForGemini{accountsByID: map[int64]*Account{account.ID: account}}
				return &AccountTestService{accountRepo: repo, grokTokenProvider: NewGrokTokenProvider(repo, nil), httpUpstream: upstream}
			},
			path: "input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: response("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")}
			if tt.name == "Anthropic" {
				upstream.resp.Body = io.NopCloser(strings.NewReader("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
			}
			svc := tt.setup(tt.account, upstream)
			result, err := svc.RunQualityProbeBackground(context.Background(), tt.account.ID, "", prompt)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotEmpty(t, upstream.lastBody)
			require.Equal(t, prompt, gjson.GetBytes(upstream.lastBody, tt.path).String())
		})
	}
}
