package service

import (
	"context"
	"errors"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"github.com/tidwall/gjson"
)

type realtimeLedgerTurn struct {
	handle  *requestledger.Handle
	attempt *requestledger.Attempt
	eventID string
}

// Only bounded protocol identifiers live in memory. Neither audio nor text nor
// upstream IDs are persisted. Automatic turns are explicitly observed phases;
// their causal input stream has a precommitted session attempt.
type realtimeLedgerAudit struct {
	control *requestledger.Attempt
	mu      sync.Mutex
	ctx     context.Context
	account *Account
	input   *requestledger.Attempt
	pending []*realtimeLedgerTurn
	active  map[string]*realtimeLedgerTurn
}

func newRealtimeLedgerAudit(ctx context.Context, account *Account) *realtimeLedgerAudit {
	return &realtimeLedgerAudit{ctx: ctx, account: account, active: make(map[string]*realtimeLedgerTurn)}
}
func (a *realtimeLedgerAudit) enabled() bool {
	return a != nil && requestledger.FromContext(a.ctx) != nil
}
func (a *realtimeLedgerAudit) turn(observed bool) (*realtimeLedgerTurn, error) {
	if a.account == nil {
		return nil, requestledger.ErrUnavailable
	}
	atCapacity := len(a.active)+len(a.pending) >= 128
	var h *requestledger.Handle
	var err error
	if observed {
		h, err = requestledger.AcceptObservedTurn(a.ctx)
	} else {
		h, err = requestledger.AcceptTurn(a.ctx)
	}
	if err != nil || h == nil {
		return nil, requestledger.ErrUnavailable
	}
	ctx := requestledger.WithHandle(a.ctx, h)
	if atCapacity && !observed {
		_ = h.Finish(ctx, "failed", 0, "request_ledger_unavailable")
		return nil, requestledger.ErrUnavailable
	}
	var attempt *requestledger.Attempt
	if observed {
		attempt, err = requestledger.BeginObservedAttempt(ctx, a.account.ID, ledgerCredentialAccountID(a.account))
	} else {
		attempt, err = requestledger.BeginAttempt(ctx, a.account.ID, ledgerCredentialAccountID(a.account))
	}
	if err != nil {
		_ = h.Finish(ctx, "failed", 0, "request_ledger_unavailable")
		return nil, err
	}
	if atCapacity {
		// The automatic turn was already observed upstream. Keep its evidence
		// without adding another entry to the bounded in-memory registry.
		requestledger.ObserveOutput(ctx)
		_ = requestledger.FinishTurn(ctx, "failed", requestledger.ErrUnavailable)
		return nil, requestledger.ErrUnavailable
	}
	return &realtimeLedgerTurn{handle: h, attempt: attempt}, nil
}
func (a *realtimeLedgerAudit) BeforeWrite(payload []byte) error {
	if !a.enabled() {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.account == nil {
		return requestledger.ErrUnavailable
	}
	switch gjson.GetBytes(payload, "type").String() {
	case "response.create":
		turn, err := a.turn(false)
		if err != nil {
			return err
		}
		eventID := gjson.GetBytes(payload, "event_id").String()
		if len(eventID) <= 256 {
			turn.eventID = eventID
		}
		a.pending = append(a.pending, turn)
	case "input_audio_buffer.append":
		if a.input == nil {
			var err error
			a.input, err = requestledger.BeginSessionInputAttempt(a.ctx, a.account.ID, ledgerCredentialAccountID(a.account))
			return err
		}
	default:
		attempt, err := requestledger.BeginSessionControlAttempt(a.ctx, a.account.ID, ledgerCredentialAccountID(a.account))
		if err != nil {
			return err
		}
		a.control = attempt
	}
	return nil
}
func (a *realtimeLedgerAudit) AfterWrite(cause error) {
	if !a.enabled() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.control.Finish(a.ctx, 0, cause)
	a.control = nil
}

func (a *realtimeLedgerAudit) finish(turn *realtimeLedgerTurn, state string, cause error) error {
	ctx := requestledger.WithHandle(a.ctx, turn.handle)
	attemptErr := turn.attempt.Finish(ctx, 0, cause)
	turnErr := requestledger.FinishTurn(ctx, state, cause)
	return errors.Join(attemptErr, turnErr)
}
func (a *realtimeLedgerAudit) Observe(payload []byte) error {
	if !a.enabled() {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	kind := gjson.GetBytes(payload, "type").String()
	if kind == "error" {
		eventID := gjson.GetBytes(payload, "error.event_id").String()
		for i, turn := range a.pending {
			if eventID != "" && turn.eventID == eventID {
				a.pending = append(a.pending[:i], a.pending[i+1:]...)
				return a.finish(turn, "failed", errors.New("upstream rejected turn"))
			}
		}
		return nil
	}
	terminal := kind == "response.done" || kind == "response.completed" || kind == "response.failed" || kind == "response.incomplete" || kind == "response.cancelled"
	if kind != "response.created" && !terminal {
		return nil
	}
	id := gjson.GetBytes(payload, "response.id").String()
	if id == "" {
		id = gjson.GetBytes(payload, "response_id").String()
	}
	if len(id) == 0 || len(id) > 256 {
		return nil
	}
	turn := a.active[id]
	if turn == nil {
		if len(a.pending) > 0 {
			turn = a.pending[0]
			a.pending = a.pending[1:]
		} else {
			var err error
			turn, err = a.turn(true)
			if err != nil {
				return err
			}
		}
		a.active[id] = turn
	}
	ctx := requestledger.WithHandle(a.ctx, turn.handle)
	requestledger.ObserveOutput(ctx)
	if !terminal {
		return nil
	}
	var usageErr error
	input := gjson.GetBytes(payload, "response.usage.input_tokens")
	output := gjson.GetBytes(payload, "response.usage.output_tokens")
	if input.Type == gjson.Number && output.Type == gjson.Number && input.Float() >= 0 && output.Float() >= 0 {
		usageErr = turn.attempt.ObserveUsage(ctx)
	}
	state := "succeeded"
	var cause error
	switch gjson.GetBytes(payload, "response.status").String() {
	case "failed", "incomplete":
		state = "failed"
	case "cancelled":
		state = "cancelled"
	case "completed":
	default:
		if kind == "response.done" {
			state = "interrupted"
		}
	}
	switch kind {
	case "response.failed", "response.incomplete":
		state = "failed"
	case "response.cancelled":
		state = "cancelled"
	}
	switch state {
	case "cancelled":
		cause = context.Canceled
	case "failed", "interrupted":
		cause = errors.New("upstream terminal incomplete")
	}
	delete(a.active, id)
	return errors.Join(usageErr, a.finish(turn, state, cause))
}
func (a *realtimeLedgerAudit) Close(cause error) {
	if !a.enabled() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = a.input.Finish(a.ctx, 0, cause)
	if cause == nil {
		cause = errors.New("connection closed without terminal evidence")
	}
	for _, turn := range a.pending {
		_ = a.finish(turn, "interrupted", cause)
	}
	for _, turn := range a.active {
		_ = a.finish(turn, "interrupted", cause)
	}
	a.pending = nil
	a.active = make(map[string]*realtimeLedgerTurn)
}
