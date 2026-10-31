package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

func ledgerCredentialAccountID(account *Account) int64 {
	if account.ParentAccountID != nil {
		return *account.ParentAccountID
	}
	return account.ID
}

func beginLedgerWSAttempt(ctx context.Context, account *Account) (*requestledger.Attempt, error) {
	if account == nil {
		return nil, requestledger.ErrUnavailable
	}
	return requestledger.BeginAttempt(requestledger.CurrentContext(ctx), account.ID, ledgerCredentialAccountID(account))
}

func finishLedgerWSAttempt(ctx context.Context, attempt *requestledger.Attempt, cause error) {
	if err := attempt.Finish(ctx, 0, cause); err != nil {
		slog.Error("request_ledger_ws_attempt_finish_failed")
	}
}

type requestLedgerWSFrameConn struct {
	inner            openaiwsv2.FrameConn
	account          *Account
	mu               sync.Mutex
	turn             int
	attempts         map[int]*requestledger.Attempt
	lastFinished     *requestledger.Attempt
	lastFinishedTurn int
}

func (c *requestLedgerWSFrameConn) WriteFrame(ctx context.Context, kind coderws.MessageType, payload []byte) error {
	turn := 0
	if gjson.GetBytes(payload, "type").String() == "response.create" {
		attempt, err := beginLedgerWSAttempt(ctx, c.account)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.turn++
		turn = c.turn
		if c.attempts == nil {
			c.attempts = make(map[int]*requestledger.Attempt)
		}
		c.attempts[turn] = attempt
		c.mu.Unlock()
	}
	err := c.inner.WriteFrame(ctx, kind, payload)
	if err != nil {
		if turn > 0 {
			c.finishTurn(ctx, turn, nil, err)
		} else {
			c.finish(ctx, err)
		}
	}
	return err
}
func (c *requestLedgerWSFrameConn) ReadFrame(ctx context.Context) (coderws.MessageType, []byte, error) {
	// Raw frames have not passed the relay's duplicate/turn attribution yet.
	return c.inner.ReadFrame(ctx)
}

// Only an admitted active turn may receive newly observed upstream evidence.
func (c *requestLedgerWSFrameConn) receiptForRead() requestledger.Receipt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.attempts[c.turn].Receipt()
}

// Relay completion precedes its terminal write. Keep only that one completed
// receipt, and only use it while its local turn still matches the writer.
func (c *requestLedgerWSFrameConn) receiptForWrite() requestledger.Receipt {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a := c.attempts[c.turn]; a != nil {
		return a.Receipt()
	}
	if c.lastFinishedTurn == c.turn {
		return c.lastFinished.Receipt()
	}
	return requestledger.Receipt{}
}

func (c *requestLedgerWSFrameConn) observePrioritySemantic(model, endpoint string, groupID *int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	attempt := c.attempts[c.turn]
	c.mu.Unlock()
	attempt.ObservePrioritySemantic(model, endpoint, groupID)
}

// observeOpenAIWSPrioritySemantic records the first parser-confirmed semantic
// output for native WS and HTTP-bridge turns. It shares the passthrough
// detector so metadata, terminal events, and empty deltas never become latency
// observations.
func observeOpenAIWSPrioritySemantic(ctx context.Context, attempt *requestledger.Attempt, payload []byte, fallbackModel, fallbackEndpoint string, groupID *int64) {
	if attempt == nil || !openAIWSPassthroughIsPrioritySemanticOutput(payload) {
		return
	}
	model := strings.TrimSpace(fallbackModel)
	endpoint := strings.TrimSpace(fallbackEndpoint)
	if ctx != nil {
		if value, ok := ctx.Value(ctxkey.RequestedPublicModel).(string); ok && strings.TrimSpace(value) != "" {
			model = strings.TrimSpace(value)
		}
		if value, ok := ctx.Value(ctxkey.InboundEndpoint).(string); ok && strings.TrimSpace(value) != "" {
			endpoint = strings.TrimSpace(value)
		}
		if groupID == nil {
			if value, ok := priorityObservationGroupFromContext(ctx); ok {
				groupID = &value
			}
		}
	}
	attempt.ObservePrioritySemantic(model, endpoint, groupID)
}

func (c *requestLedgerWSFrameConn) finishTurn(ctx context.Context, turn int, result *OpenAIForwardResult, cause error) {
	c.mu.Lock()
	attempt := c.attempts[turn]
	delete(c.attempts, turn)
	if attempt != nil {
		c.lastFinished, c.lastFinishedTurn = attempt, turn
	}
	c.mu.Unlock()
	if result != nil && (result.RequestID != "" || result.UpstreamTerminalEvent != "" || result.FirstTokenMs != nil || result.HasObservedUsage()) {
		attempt.ObserveOutput(ctx)
	}
	finishLedgerWSResult(ctx, attempt, result, cause)
}
func (c *requestLedgerWSFrameConn) finish(ctx context.Context, cause error) {
	c.mu.Lock()
	attempts := c.attempts
	c.attempts = nil
	c.mu.Unlock()
	for _, attempt := range attempts {
		finishLedgerWSAttempt(ctx, attempt, cause)
	}
}
func (c *requestLedgerWSFrameConn) Close() error { return c.inner.Close() }

// The handshake has its own non-metering attempt. Connection retries are thus
// durable before any bytes leave, separately from response.create attempts.
func dialWSWithLedger(ctx context.Context, dialer openAIWSClientDialer, account *Account, target string, headers http.Header, proxy string) (openAIWSClientConn, int, http.Header, error) {
	a, err := requestledger.BeginConnectionAttempt(ctx, account.ID, ledgerCredentialAccountID(account))
	if err != nil {
		return nil, 0, nil, err
	}
	conn, status, responseHeaders, err := dialer.Dial(ctx, target, headers, proxy)
	if conn == nil && err == nil {
		err = errors.New("openai ws dialer returned nil connection")
	}
	a.ObserveIdentifiers(ctx, requestledger.UpstreamRequestID(responseHeaders), "")
	if finishErr := a.Finish(ctx, status, err); finishErr != nil {
		slog.Error("request_ledger_ws_handshake_finish_failed")
	}
	return conn, status, responseHeaders, a.WrapError(err)
}

func finishLedgerWSResult(ctx context.Context, attempt *requestledger.Attempt, result *OpenAIForwardResult, cause error) {
	if result != nil {
		attempt.ObserveIdentifiers(ctx, requestledger.UpstreamRequestID(result.ResponseHeaders), "")
	}
	if result != nil && result.HasObservedUsage() {
		if err := attempt.ObserveUsage(ctx); err != nil {
			slog.Error("request_ledger_ws_usage_observation_failed")
		}
	}
	if cause == nil && result != nil {
		switch result.UpstreamTerminalEvent {
		case "response.failed", "response.incomplete":
			cause = errors.New("upstream reported failure")
		case "response.cancelled":
			cause = context.Canceled
		}
	}
	status := 0
	if cause == nil && result != nil && (result.UpstreamTerminalEvent == "response.completed" || result.UpstreamTerminalEvent == "response.done") {
		status = http.StatusOK
	}
	if err := attempt.Finish(ctx, status, cause); err != nil {
		slog.Error("request_ledger_ws_attempt_finish_failed")
	}
}
