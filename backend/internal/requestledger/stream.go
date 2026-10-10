package requestledger

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/tidwall/gjson"
)

var errStreamIncomplete = errors.New("upstream stream ended without terminal evidence")
var errTerminalFailure = errors.New("upstream reported failure")

// Only terminal metadata is inspected, never retained. The bounded line buffer
// is discarded after each SSE line. Oversized/unknown frames cannot prove success.
type streamEvidence struct {
	line      []byte
	oversized bool
	terminal  string
}

func (s *streamEvidence) observe(p []byte) {
	for len(p) > 0 {
		end := bytes.IndexByte(p, '\n')
		if end < 0 {
			end = len(p)
		}
		if !s.oversized {
			if len(s.line)+end > 64*1024 {
				s.oversized = true
				s.line = nil
			} else {
				s.line = append(s.line, p[:end]...)
			}
		}
		if end == len(p) {
			return
		}
		if !s.oversized {
			s.observeLine(strings.TrimSpace(string(s.line)))
		}
		s.line = nil
		s.oversized = false
		p = p[end+1:]
	}
}
func (s *streamEvidence) observeLine(line string) {
	if s.terminal == "failed" || s.terminal == "cancelled" {
		return
	}
	if strings.HasPrefix(line, "event:") {
		switch strings.TrimSpace(strings.TrimPrefix(line, "event:")) {
		case "message_stop":
			s.terminal = "succeeded"
		case "error":
			s.terminal = "failed"
		}
	}
	if !strings.HasPrefix(line, "data:") {
		return
	}
	data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if data == "[DONE]" {
		if s.terminal == "" {
			s.terminal = "succeeded"
		}
		return
	}
	switch gjson.Get(data, "type").String() {
	case "response.completed", "response.done", "message_stop":
		s.terminal = "succeeded"
		switch gjson.Get(data, "response.status").String() {
		case "failed", "incomplete":
			s.terminal = "failed"
		case "cancelled":
			s.terminal = "cancelled"
		}
	case "response.failed", "response.incomplete", "error":
		s.terminal = "failed"
	case "response.cancelled":
		s.terminal = "cancelled"
	}
	if gjson.Get(data, "choices.0.finish_reason").String() != "" || gjson.Get(data, "candidates.0.finishReason").String() != "" {
		if s.terminal == "" {
			s.terminal = "succeeded"
		}
	}
}
func ObserveOutput(ctx context.Context) {
	h := FromContext(CurrentContext(ctx))
	if h == nil {
		return
	}
	h.mu.Lock()
	attempt := h.currentAttempt
	h.mu.Unlock()
	if attempt != nil {
		attempt.ObserveOutput(ctx)
		return
	}
	h.mu.Lock()
	observed := h.outputObserved
	h.outputObserved = true
	h.mu.Unlock()
	if !observed && !observeOutput(ctx, h, 0) {
		h.mu.Lock()
		h.outputObserved = false
		h.mu.Unlock()
	}
}
func (a *Attempt) ObserveOutput(ctx context.Context) {
	if a != nil && !a.outputObserved.Swap(true) {
		if !observeOutput(ctx, a.handle, a.Number) {
			a.outputObserved.Store(false)
		}
	}
}
func observeOutput(ctx context.Context, h *Handle, attemptNo int) bool {
	writeCtx, cancel := detachedWrite(ctx)
	defer cancel()
	_, err := h.ledger.db.ExecContext(writeCtx, `WITH owner AS (SELECT id FROM gateway_requests WHERE id=$1 FOR UPDATE), observed AS (
 UPDATE gateway_request_attempts SET output_observed=TRUE FROM owner WHERE request_id=owner.id
 AND attempt_no=CASE WHEN $2>0 THEN $2 ELSE (SELECT MAX(attempt_no) FROM gateway_request_attempts WHERE request_id=$1) END
 AND NOT output_observed RETURNING request_id)
 UPDATE gateway_requests SET output_observed=TRUE WHERE id=(SELECT id FROM owner) AND NOT output_observed`, h.ID, attemptNo)
	if err != nil {
		slog.Error("request_ledger_output_observation_failed")
		return false
	}
	return true
}
