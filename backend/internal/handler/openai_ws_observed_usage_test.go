package handler

import (
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
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
