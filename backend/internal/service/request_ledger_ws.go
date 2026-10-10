package service

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"

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
	inner    openaiwsv2.FrameConn
	account  *Account
	mu       sync.Mutex
	turn     int
	attempts map[int]*requestledger.Attempt
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
func (c *requestLedgerWSFrameConn) observeOutput(ctx context.Context) {
	c.mu.Lock()
	attempt := c.attempts[c.turn]
	c.mu.Unlock()
	attempt.ObserveOutput(ctx)
}
func (c *requestLedgerWSFrameConn) finishTurn(ctx context.Context, turn int, result *OpenAIForwardResult, cause error) {
	c.mu.Lock()
	attempt := c.attempts[turn]
	delete(c.attempts, turn)
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
	if finishErr := a.Finish(ctx, status, err); finishErr != nil {
		slog.Error("request_ledger_ws_handshake_finish_failed")
	}
	return conn, status, responseHeaders, err
}

func finishLedgerWSResult(ctx context.Context, attempt *requestledger.Attempt, result *OpenAIForwardResult, cause error) {
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
	finishLedgerWSAttempt(ctx, attempt, cause)
}
