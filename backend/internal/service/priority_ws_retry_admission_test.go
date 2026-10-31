package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// The first connection completes turn one, then fails the next physical write.
// Change the history only at that failed write so both logical BeforeTurn
// callbacks and the first send of turn two have already seen the fast account.
type priorityWSRetryHistoryConn struct {
	*openAIWSWriteFailAfterFirstTurnConn
	onWriteFailure func()
	writes         atomic.Int32
	failures       atomic.Int32
}

func (c *priorityWSRetryHistoryConn) WriteJSON(ctx context.Context, value any) error {
	c.writes.Add(1)
	err := c.openAIWSWriteFailAfterFirstTurnConn.WriteJSON(ctx, value)
	if err != nil && c.failures.Add(1) == 1 {
		c.onWriteFailure()
	}
	return err
}

func TestPriorityWSAdmissionPooledWriteRetryRechecksWithoutRepeatingBeforeTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cfg := passthroughLifecycleConfig()
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MinIdlePerAccount = 0
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.QueueLimitPerConn = 8
	account := passthroughLifecycleAccount()
	account.Extra["openai_apikey_responses_websockets_v2_mode"] = OpenAIWSIngressModeCtxPool
	first := &priorityWSRetryHistoryConn{
		openAIWSWriteFailAfterFirstTurnConn: &openAIWSWriteFailAfterFirstTurnConn{
			events: [][]byte{[]byte(`{"type":"response.completed","response":{"id":"resp_priority_retry_1","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`)},
		},
	}
	// If final admission is skipped on retry, this connection accepts the turn
	// and completes it, making the regression fail without waiting on a timeout.
	second := &openAIWSCaptureConn{events: [][]byte{[]byte(`{"type":"response.completed","response":{"id":"resp_priority_retry_2","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`)}}
	dialer := &openAIWSQueueDialer{conns: []openAIWSClientConn{first, second}}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(dialer)
	defer pool.Close()
	svc := &OpenAIGatewayService{
		cfg: cfg, httpUpstream: &httpUpstreamRecorder{}, cache: &stubGatewayCache{},
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(), openaiWSPool: pool,
	}
	priorityCfg := configurePriorityWSTest(svc, PriorityLatencyStop, "gpt-5.1")
	req := OpenAIAccountScheduleRequest{RequestedModel: "gpt-5.1"}
	seedPriorityWSTest(svc, priorityCfg, req, account.ID, 100)
	first.onWriteFailure = func() { seedPriorityWSTest(svc, priorityCfg, req, account.ID, 2500) }
	var hooksMu sync.Mutex
	beforeCalls := make(map[int]int)
	afterCalls := make(map[int]int)
	var deniedResult *OpenAIForwardResult
	var deniedErr error
	server, result := startPassthroughLifecycleServerWithHooks(t, ctx, svc, account, func(*gin.Context) *OpenAIWSIngressHooks {
		return &OpenAIWSIngressHooks{
			BeforeTurn: func(turn int) error {
				hooksMu.Lock()
				defer hooksMu.Unlock()
				beforeCalls[turn]++
				return nil
			},
			AfterTurn: func(turn int, turnResult *OpenAIForwardResult, err error) {
				hooksMu.Lock()
				defer hooksMu.Unlock()
				afterCalls[turn]++
				if turn == 2 {
					deniedResult, deniedErr = turnResult, err
				}
			},
		}
	})
	defer server.Close()
	client := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = client.CloseNow() }()
	firstResponse, err := readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	require.Equal(t, "resp_priority_retry_1", gjson.GetBytes(firstResponse, "response.id").String())
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"resp_priority_retry_1"}`)))
	select {
	case err = <-result:
		requirePriorityWSDenial(t, err)
	case <-ctx.Done():
		t.Fatal("same-turn physical write retry did not reject the newly slow account")
	}
	require.Equal(t, int32(2), first.writes.Load(), "first connection must attempt turn one and fail turn two")
	require.Equal(t, int32(1), first.failures.Load())
	require.Equal(t, 2, dialer.DialCount(), "retry must reach the replacement connection before final admission")
	second.mu.Lock()
	secondWrites := len(second.writes)
	second.mu.Unlock()
	require.Zero(t, secondWrites, "the rejected physical retry must not reach the replacement connection")
	hooksMu.Lock()
	defer hooksMu.Unlock()
	require.Equal(t, map[int]int{1: 1, 2: 1}, beforeCalls, "skipBeforeTurn must not skip physical-send admission")
	require.Equal(t, map[int]int{1: 1, 2: 1}, afterCalls)
	require.Nil(t, deniedResult, "a locally denied retry must not manufacture usage")
	requirePriorityWSDenial(t, deniedErr)
}
