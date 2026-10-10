package handler

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

// A hook can be called again during relay teardown. Claim before any billing
// side effect; database dedup remains the final guard for persistence retries.
type openAIWSTurnSettlement struct {
	mu        sync.Mutex
	claimed   map[int]struct{}
	privateID string
	lastTurn  int
}

// A same-account retry calls Proxy again with the current payload and resets
// its local counter to 1. Bind every hook to one immutable logical offset so
// that retry retains its identity and cannot collide with a completed turn.
func (s *openAIWSTurnSettlement) bindAttempt(hooks *service.OpenAIWSIngressHooks) *service.OpenAIWSIngressHooks {
	s.mu.Lock()
	offset := max(s.lastTurn-1, 0)
	s.mu.Unlock()
	logicalTurn := func(turn int) int {
		turn += offset
		s.mu.Lock()
		s.lastTurn = max(s.lastTurn, turn)
		s.mu.Unlock()
		return turn
	}
	bound := *hooks
	if hooks.TurnStarted != nil {
		bound.TurnStarted = func(turn int, at time.Time) { hooks.TurnStarted(logicalTurn(turn), at) }
	}
	if hooks.BeforeRequest != nil {
		bound.BeforeRequest = func(turn int, payload []byte, model string) error {
			return hooks.BeforeRequest(logicalTurn(turn), payload, model)
		}
	}
	if hooks.BeforeTurn != nil {
		bound.BeforeTurn = func(turn int) error { return hooks.BeforeTurn(logicalTurn(turn)) }
	}
	if hooks.MapRequestModel != nil {
		bound.MapRequestModel = func(turn int, model string) (string, error) {
			return hooks.MapRequestModel(logicalTurn(turn), model)
		}
	}
	if hooks.AfterTurn != nil {
		bound.AfterTurn = func(turn int, result *service.OpenAIForwardResult, err error) {
			hooks.AfterTurn(logicalTurn(turn), result, err)
		}
	}
	return &bound
}

func (s *openAIWSTurnSettlement) claim(turn int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.claimed == nil {
		s.claimed = make(map[int]struct{})
	}
	if _, exists := s.claimed[turn]; exists {
		return false
	}
	s.claimed[turn] = struct{}{}
	return true
}

func (s *openAIWSTurnSettlement) context(parent context.Context, turn int) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.privateID == "" {
		s.privateID = "ws:" + uuid.NewString()
	}
	// An upstream response ID still takes precedence in WS RecordUsage. Only
	// the missing-ID fallback changes from connection-scoped to turn-scoped.
	return context.WithValue(parent, ctxkey.UsageBillingRequestID, s.privateID+":"+strconv.Itoa(turn))
}

func shouldReportOpenAIWSTurnForwardResult(account *service.Account, result *service.OpenAIForwardResult, turnErr error) bool {
	if account != nil && account.IsOpenAI() && service.IsOpenAITurnAdmissionError(turnErr) {
		return false
	}
	// Keep the pre-existing successful/image reporting path. Retaining partial
	// text for settlement must not turn its transport failure into account success.
	return result != nil && (turnErr == nil || result.ImageCount > 0)
}

func shouldRecordOpenAIWSTurnUsage(account *service.Account, result *service.OpenAIForwardResult, turnErr error, cyberPolicyRecorded bool) bool {
	if result == nil {
		return false
	}
	if turnErr == nil {
		return true
	}
	// Errored cyber-policy turns are settled by recordCyberPolicyIfMarked.
	if cyberPolicyRecorded {
		return false
	}
	if account != nil && !account.IsOpenAI() {
		return result.ImageCount > 0
	}
	// Only upstream-observed aggregate usage proves text work to settle.
	// Output deltas, timing and token-detail subsets must not become estimates.
	return result.HasObservedUsage()
}
