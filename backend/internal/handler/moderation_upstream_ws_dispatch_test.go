package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestModerationUpstreamWebSocketUsesSelectedAccountBeforeForward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("selected=%t", selected), func(t *testing.T) {
			var audits, upstreamWrites atomic.Int32
			audit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				audits.Add(1)
				_, _ = w.Write([]byte(`{"results":[{"category_scores":{"sexual":0.99}}]}`))
			}))
			defer audit.Close()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := coderws.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer func() { _ = conn.CloseNow() }()
				ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
				defer cancel()
				if _, _, err := conn.Read(ctx); err != nil {
					return
				}
				upstreamWrites.Add(1)
				_ = conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_scope","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`))
				_, _, _ = conn.Read(ctx)
			}))
			defer upstream.Close()

			ids := "9952"
			if selected {
				ids = "9951"
			}
			policy := fmt.Sprintf(`{"enabled":true,"mode":"pre_block","all_groups":true,"sample_rate":100,"base_url":%q,"api_keys":["fixture"],"upstream_account_scope":"selected","upstream_account_ids":[%s],"upstream_account_bindings":{"9951":{"platform":"openai","type":"apikey","group_ids":[]}}}`, audit.URL, ids)
			h := newOpenAIWSPassthroughHandlerHarness(t, upstream.URL)
			settingRepo := &contentModerationHandlerSettingRepo{values: map[string]string{
				service.SettingKeyRiskControlEnabled:      "true",
				service.SettingKeyContentModerationConfig: policy,
			}}
			account := service.Account{ID: 9951, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, GroupIDs: []int64{}}
			h.handler.contentModerationService = service.NewContentModerationService(
				settingRepo, &contentModerationHandlerTestRepo{}, nil, nil, nil, nil, nil, nil,
				&openAIWSUsageHandlerAccountRepoStub{account: account},
			)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, h.clientConn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"synthetic scope input"}`)))
			_, event, readErr := h.clientConn.Read(ctx)
			if selected {
				if readErr == nil {
					require.Equal(t, "error", gjson.GetBytes(event, "type").String())
					require.Contains(t, string(event), "content_policy_violation")
				} else {
					require.Equal(t, coderws.StatusPolicyViolation, coderws.CloseStatus(readErr))
				}
				require.Equal(t, int32(1), audits.Load())
				require.Zero(t, upstreamWrites.Load())
			} else {
				require.NoError(t, readErr)
				require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
				require.Zero(t, audits.Load())
				require.Equal(t, int32(1), upstreamWrites.Load())
			}
			require.False(t, strings.Contains(string(event), "fixture"))
			_ = h.clientConn.CloseNow()
			select {
			case <-h.handlerDone:
			case <-time.After(3 * time.Second):
				t.Fatal("websocket handler did not finish")
			}
		})
	}
}
