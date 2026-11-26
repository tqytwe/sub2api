package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestOmitOpenAI400HistoricalCiphertextPreservesReadableContext(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.1","metadata":{"sequence":9007199254740993},"input":[
	{"type":"reasoning","id":"rs_fixture","status":"completed","encrypted_content":"REASONING_CIPHER","summary":[{"type":"summary_text","text":"visible summary survives"}],"content":[{"type":"reasoning_text","text":"visible reasoning survives"}]},
	{"type":"agent_message","author":"worker","recipient":"parent","encrypted_content":"AGENT_CIPHER","content":"readable finding"},
	{"type":"function_call","name":"lookup","call_id":"call_fixture","arguments":"{}"},
	{"type":"function_call_output","call_id":"call_fixture","output":[{"type":"input_text","text":"recorded result"},{"type":"encrypted_content","encrypted_content":"TOOL_CIPHER"}]},
	{"type":"message","role":"assistant","content":[{"type":"output_text","text":"prior answer"},{"type":"encrypted_content","encrypted_content":"PART_CIPHER"}]},
	{"type":"message","role":"user","content":[{"type":"input_text","text":"continue from visible findings"}]}]}`)

	clean, changed := omitOpenAIHistoricalCiphertextForRecovery(raw, true)
	require.True(t, changed)
	for _, secret := range []string{"REASONING_CIPHER", "AGENT_CIPHER", "TOOL_CIPHER", "PART_CIPHER"} {
		require.NotContains(t, string(clean), secret)
	}
	for _, text := range []string{"visible summary survives", "visible reasoning survives", "readable finding", "recorded result", "continue from visible findings", "9007199254740993"} {
		require.Contains(t, string(clean), text)
	}
	require.Equal(t, "message", gjson.GetBytes(clean, "input.0.type").String())
	require.Equal(t, "assistant", gjson.GetBytes(clean, "input.0.role").String())
	require.Equal(t, "message", gjson.GetBytes(clean, "input.1.type").String())
	require.Equal(t, "user", gjson.GetBytes(clean, "input.1.role").String())
	require.Equal(t, "call_fixture", gjson.GetBytes(clean, "input.2.call_id").String())
	require.JSONEq(t, gjson.GetBytes(raw, "input.5").Raw, gjson.GetBytes(clean, "input.5").Raw)
	require.Equal(t, "[Historical encrypted content omitted by the gateway. Its contents are unavailable; do not infer missing instructions or results.]", gjson.GetBytes(clean, "input.0.content.0.text").String())
}

func TestOmitOpenAI400HistoricalCiphertextRefusesUnsupportedInputs(t *testing.T) {
	for name, raw := range map[string]string{
		"current task unreadable":       `{"input":[{"type":"reasoning","encrypted_content":"OLD","summary":[]},{"type":"message","role":"user","content":[{"type":"encrypted_content","encrypted_content":"CURRENT"}]}]}`,
		"ciphertext after current task": `{"input":[{"type":"message","role":"user","content":"continue"},{"type":"reasoning","encrypted_content":"LATE","summary":[]}]}`,
		"unsupported reasoning field":   `{"input":[{"type":"reasoning","encrypted_content":"OLD","summary":[],"extra":"unknown"},{"type":"message","role":"user","content":"continue"}]}`,
		"previous response pointer":     `{"previous_response_id":"resp_fixture","input":[{"type":"reasoning","encrypted_content":"OLD","summary":[]},{"type":"message","role":"user","content":"continue"}]}`,
		"conversation pointer":          `{"conversation":{"id":"conv_fixture"},"input":[{"type":"reasoning","encrypted_content":"OLD","summary":[]},{"type":"message","role":"user","content":"continue"}]}`,
		"compaction":                    `{"input":[{"type":"compaction","encrypted_content":"STATE"},{"type":"message","role":"user","content":"continue"}]}`,
		"duplicate keys":                `{"input":[{"type":"reasoning","encrypted_content":"OLD","summary":[]},{"type":"message","role":"user","content":"continue"}],"input":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			clean, changed := omitOpenAIHistoricalCiphertextForRecovery([]byte(raw), true)
			require.False(t, changed)
			require.Equal(t, raw, string(clean), "refusal must not partially rewrite input")
		})
	}
}

func TestOpenAI400OmitEncryptedHistorySettingDefaultsOff(t *testing.T) {
	for _, value := range []string{"", "false", "true", "invalid"} {
		s := NewSettingService(&codexPolicyMigrationRepoStub{values: map[string]string{SettingKeyOpenAI400OmitEncryptedHistoryEnabled: value}}, nil)
		require.Equal(t, value == "true", s.IsOpenAI400OmitEncryptedHistoryEnabled(context.Background()))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.False(t, s.IsOpenAI400OmitEncryptedHistoryEnabled(ctx))
	}
	var missing *SettingService
	require.False(t, missing.IsOpenAI400OmitEncryptedHistoryEnabled(context.Background()))
}

func TestOpenAI400OmitEncryptedHistoryDiagnosticUsesFixedValues(t *testing.T) {
	for _, decision := range []string{"disabled", "retry_exhausted", "canceled_or_output_started", "unsupported_context", "delivery_unconfirmed", "history_integrity", "ownership_unavailable", "no_eligible_history", "same_account_retry_scheduled"} {
		require.NotNil(t, sanitizeOpenAI400HistoryDiagnostic(&OpenAI400HistoryDiagnostic{Decision: decision}), decision)
	}
	require.Nil(t, sanitizeOpenAI400HistoryDiagnostic(&OpenAI400HistoryDiagnostic{Decision: "credential=secret"}))
	require.Nil(t, sanitizeOpenAI400HistoryDiagnostic(nil))
}

func TestOpenAI400HistorySendGuards(t *testing.T) {
	const body = `{"model":"gpt-5.1","input":[{"type":"reasoning","encrypted_content":"fixture-cipher","summary":[]},{"type":"function_call","name":"lookup","call_id":"fixture","arguments":"{}"},{"type":"function_call_output","call_id":"fixture","output":"original result"},{"type":"message","role":"user","content":"continue"}]}`
	for _, mode := range []string{"unchanged", "task", "tool_result", "order", "account", "credential", "url", "proxy", "unreadable_body", "malformed_body", "budget", "canceled", "output_started", "delivery_pending", "plugin_response", "owner_lookup"} {
		t.Run(mode, func(t *testing.T) {
			svc := &OpenAIGatewayService{cfg: &config.Config{}, settingService: NewSettingService(&codexPolicyMigrationRepoStub{values: map[string]string{SettingKeyOpenAI400OmitEncryptedHistoryEnabled: "true"}}, nil)}
			c, _ := newTurnStateTestContext(t, 715, "fixture-session")
			c.Request.URL.Path = "/v1/responses"
			c.Set("api_key", &APIKey{ID: 715})
			ctx := svc.WithOpenAIAgentMessageRecovery(c.Request.Context(), c, []byte(body), "fixture-session", "gpt-5.1")
			a := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture"}}
			proxyID := int64(8)
			a.ProxyID, a.Proxy = &proxyID, &Proxy{ID: proxyID, Protocol: "http", Host: "proxy.example", Port: 8080}
			url, auth := "https://native.example/v1/responses", "Bearer fixture"
			request := func(payload []byte) *http.Request {
				req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
				require.NoError(t, err)
				req.Header.Set("Authorization", auth)
				return req
			}
			finish, err := svc.prepareOpenAIAgentRecoveryAttempt(ctx, a, request([]byte(body)))
			require.NoError(t, err)
			defer finish(false)
			r := openAIAgentRecoveryFromContext(ctx)
			require.NotNil(t, r.delivery)
			original := r.inputDigest
			attempts, refusal := 1, ""
			switch mode {
			case "budget":
				attempts, refusal = 3, "retry_exhausted"
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				refusal = "canceled_or_output_started"
			case "output_started":
				c.Writer.WriteHeaderNow()
				refusal = "canceled_or_output_started"
			case "delivery_pending":
				r.delivery.keys[[32]byte{1}] = true
				refusal = "delivery_unconfirmed"
			case "plugin_response":
				r.pluginAttempt = true
				refusal = "delivery_unconfirmed"
			case "owner_lookup":
				r.lookupErr = openAIAgentRecoveryError("store_unavailable")
				refusal = "unsupported_context"
			}
			next, clean, changed := svc.prepareOpenAI400HistoryRecovery(ctx, c, a, []byte(body), attempts)
			if refusal != "" {
				require.False(t, changed)
				require.Equal(t, body, string(clean))
				require.Equal(t, refusal, openAI400HistoryDiagnostic(c).Decision)
				return
			}
			require.True(t, changed, "%+v", openAI400HistoryDiagnostic(c))
			require.Equal(t, original, r.inputDigest)
			frozen := openAIAgentRecoveryFromContext(next).inputDigest
			switch mode {
			case "task":
				clean, err = sjson.SetBytes(clean, "input.3.content", "changed task")
			case "tool_result":
				clean, err = sjson.SetBytes(clean, "input.2.output", "changed result")
			case "order":
				clean, err = sjson.DeleteBytes(clean, "input.1")
			case "account":
				a.ID++
			case "credential":
				auth = "Bearer different-fixture"
			case "url":
				url = "https://other.example/v1/responses"
			case "proxy":
				a.Proxy.Host = "other-proxy.example"
			case "malformed_body":
				clean = append(clean, '{')
			}
			require.NoError(t, err)
			out := request(clean)
			if mode == "unreadable_body" {
				out.GetBody = nil
			}
			complete, err := svc.prepareOpenAIAgentRecoveryAttempt(next, a, out)
			defer complete(false)
			if mode == "unchanged" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, frozen, openAIAgentRecoveryFromContext(next).inputDigest)
		})
	}
}

type history400SettingsStub struct{ *codexPolicyMigrationRepoStub }

func (s history400SettingsStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string)
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func TestOpenAI400HistoryNativeRetrySequence(t *testing.T) {
	const body = `{"model":"gpt-5.1","input":[{"type":"reasoning","encrypted_content":"AAAA-fixture-1111","summary":[{"type":"summary_text","text":"keep first summary"}]},{"type":"reasoning","encrypted_content":"BBBB-fixture-2222","summary":[{"type":"summary_text","text":"keep second summary"}]},{"type":"message","role":"user","content":"continue the task"}]}`
	const rejection = `{"error":{"code":"invalid_encrypted_content","message":"The encrypted content could not be verified."}}`
	const located = `{"error":{"code":"invalid_encrypted_content","message":"The encrypted content AAAA...1111 could not be verified. Reason: Encrypted content could not be decrypted or parsed."}}`
	response := func(status int, contentType, payload string) *http.Response {
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(payload))}
	}
	for _, name := range []string{"precise then lossy", "HTTP 200 in-stream rejection"} {
		t.Run(name, func(t *testing.T) {
			payload := []byte(body)
			upstream := &httpUpstreamSequenceRecorder{}
			if name == "precise then lossy" {
				upstream.responses = []*http.Response{
					response(400, "application/json", located),
					response(400, "application/json", rejection),
					response(200, "application/json", `{"id":"resp_fixture","status":"completed","error":null,"model":"gpt-5.1","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`),
				}
			} else {
				var err error
				payload, err = sjson.SetBytes(payload, "stream", true)
				require.NoError(t, err)
				upstream.responses = []*http.Response{response(200, "text/event-stream", "data: {\"type\":\"error\",\"code\":\"invalid_encrypted_content\",\"message\":\"The encrypted content could not be verified.\"}\n\n")}
			}
			settings := history400SettingsStub{&codexPolicyMigrationRepoStub{values: map[string]string{SettingKeyOpenAI400OmitEncryptedHistoryEnabled: "true"}}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, settingService: NewSettingService(settings, nil)}
			c, rec := newTurnStateTestContext(t, 715, "fixture-session")
			c.Set("api_key", &APIKey{ID: 715, AllowLongContext: true})
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
			ctx := svc.WithOpenAIAgentMessageRecovery(c.Request.Context(), c, payload, "fixture-session", "gpt-5.1")
			c.Request = c.Request.WithContext(ctx)
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "fixture"}}
			_, err := svc.Forward(ctx, c, account, payload)
			require.NotContains(t, rec.Body.String(), "encrypted_history_400")
			if name == "HTTP 200 in-stream rejection" {
				require.Error(t, err)
				require.Contains(t, rec.Body.String(), "invalid_encrypted_content")
				require.Equal(t, 1, upstream.callCount)
				require.Nil(t, openAI400HistoryDiagnostic(c))
				return
			}
			require.NoError(t, err)
			require.Len(t, upstream.bodies, 3)
			require.NotContains(t, string(upstream.bodies[1]), "AAAA-fixture-1111")
			require.Contains(t, string(upstream.bodies[1]), "BBBB-fixture-2222")
			require.NotContains(t, string(upstream.bodies[2]), "BBBB-fixture-2222")
			for _, text := range []string{"keep first summary", "keep second summary", "continue the task"} {
				require.Contains(t, string(upstream.bodies[2]), text)
			}
			require.Equal(t, "same_account_retry_scheduled", openAI400HistoryDiagnostic(c).Decision)
		})
	}
}
