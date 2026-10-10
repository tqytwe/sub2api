package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestWSObservedUsageMeteredFirstErrorCannotReplay(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload: `{"type":"response.create","model":"gpt-5.1"}`, ingressMode: mode,
				upstreamEvent: func(int, string) string {
					return `{"type":"error","error":{"code":"rate_limit_exceeded","type":"rate_limit_error","message":"synthetic"},"response":{"id":"resp_metered_error","usage":{"input_tokens":9,"output_tokens":4}}}`
				},
			})
			require.Len(t, got.logs, 1)
			require.Equal(t, 9, got.logs[0].InputTokens)
			require.Equal(t, 4, got.logs[0].OutputTokens)
			require.Len(t, got.upstreamPayloads, 1)
		})
	}
}

type wsObservedUsageRetryUpstream struct {
	service.HTTPUpstream
	mu         sync.Mutex
	payloads   [][]byte
	accountIDs []int64
	noID       bool
}

func (u *wsObservedUsageRetryUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.payloads = append(u.payloads, payload)
	u.accountIDs = append(u.accountIDs, accountID)
	call := len(u.payloads)
	if call == 2 {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"rate_limit_error","message":"synthetic transient limit"}}`))}, nil
	}
	turn := call
	if call > 2 {
		turn--
	}
	id := fmt.Sprintf("resp_retry_%d", turn)
	if u.noID {
		id = ""
	}
	body := fmt.Sprintf("data: {\"type\":\"response.completed\",\"response\":{\"id\":%q,\"usage\":{\"input_tokens\":%d,\"output_tokens\":%d}}}\n\n", id, turn*9, turn*4)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestWSObservedUsageSameAccountBridgeRetryPreservesLogicalTurn(t *testing.T) {
	for _, noID := range []bool{false, true} {
		t.Run(fmt.Sprintf("no_id=%t", noID), func(t *testing.T) {
			upstream := &wsObservedUsageRetryUpstream{noID: noID}
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:  `{"type":"response.create","model":"gpt-5.1","input":[{"role":"user","content":"first"}]}`,
				secondPayload: `{"type":"response.create","model":"gpt-5.1","input":[{"role":"user","content":"second"}]}`,
				ingressMode:   service.OpenAIWSIngressModeHTTPBridge, accountType: service.AccountTypeOAuth, httpUpstream: upstream,
			})
			require.Len(t, got.logs, 2)
			for i, row := range got.logs {
				require.EqualValues(t, 1701, row.UserID)
				require.EqualValues(t, 1801, row.APIKeyID)
				require.EqualValues(t, 9901, row.AccountID)
				require.Equal(t, (i+1)*9, row.InputTokens)
				require.Equal(t, (i+1)*4, row.OutputTokens)
			}
			require.NotEqual(t, got.logs[0].RequestID, got.logs[1].RequestID)
			upstream.mu.Lock()
			defer upstream.mu.Unlock()
			require.Equal(t, []int64{9901, 9901, 9901}, upstream.accountIDs)
			require.Len(t, upstream.payloads, 3, "only the unmetered second turn may retry")
			require.Contains(t, gjson.GetBytes(upstream.payloads[2], "input").Raw, "second")
		})
	}
}

func TestWSObservedUsageHandlerPersistsEachTurn(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeCtxPool, service.OpenAIWSIngressModePassthrough} {
		for _, noID := range []bool{false, true} {
			for _, terminal := range []string{"response.completed", "response.in_progress", "response.incomplete", "response.failed", "error"} {
				t.Run(fmt.Sprintf("%s/no_id=%v/%s", mode, noID, terminal), func(t *testing.T) {
					got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
						firstPayload:  `{"type":"response.create","model":"gpt-5.1"}`,
						secondPayload: `{"type":"response.create","model":"gpt-5.1"}`,
						ingressMode:   mode,
						upstreamEvent: func(turn int, model string) string {
							id := fmt.Sprintf("resp_%d", turn)
							if noID {
								id = ""
							}
							event := "response.completed"
							if turn == 2 {
								event = terminal
							}
							return fmt.Sprintf(`{"type":%q,"response":{"id":%q,"model":%q,"usage":{"input_tokens":%d,"output_tokens":%d}}}`, event, id, model, turn*9, turn*4)
						},
					})
					require.Len(t, got.logs, 2)
					for i, row := range got.logs {
						require.EqualValues(t, 1701, row.UserID)
						require.EqualValues(t, 1801, row.APIKeyID)
						require.EqualValues(t, 9901, row.AccountID)
						require.Equal(t, (i+1)*9, row.InputTokens)
						require.Equal(t, (i+1)*4, row.OutputTokens)
						if noID {
							require.Contains(t, row.RequestID, "gateway:ws:")
						} else {
							require.Equal(t, fmt.Sprintf("resp_%d", i+1), row.RequestID)
						}
					}
					require.NotEqual(t, got.logs[0].RequestID, got.logs[1].RequestID)
					require.Len(t, got.upstreamPayloads, 2, "metered work must not be replayed")
				})
			}
		}
	}
}
