package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const accountRPMWarningRatio = 0.8

type openAIRPMMovableKey struct{}

func openAIRPMMovable(ctx context.Context) bool {
	movable, _ := ctx.Value(openAIRPMMovableKey{}).(bool)
	return movable
}

var (
	ErrOpenAIRPMExhausted   = errors.New("openai oauth rpm exhausted")
	ErrOpenAIRPMUnavailable = errors.New("openai oauth rpm unavailable")
)

type AccountRPMState struct {
	Current int
	Limit   int
	ResetAt time.Time
	Enabled bool
}

func (s AccountRPMState) Utilization() float64 {
	if !s.Enabled || s.Limit <= 0 {
		return 0
	}
	return float64(s.Current) / float64(s.Limit)
}

func accountRPMState(account *Account, count int) AccountRPMState {
	if account == nil || !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
		return AccountRPMState{}
	}
	return AccountRPMState{
		Current: count,
		Limit:   account.GetBaseRPM(),
		ResetAt: time.Now().Truncate(time.Minute).Add(time.Minute),
		Enabled: true,
	}
}

func accountRPMStateFromContext(ctx context.Context, account *Account) (AccountRPMState, bool) {
	if account == nil || !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
		return AccountRPMState{}, false
	}
	count, ok := rpmFromPrefetchContext(ctx, account.RPMAccountID())
	if !ok {
		return AccountRPMState{}, false
	}
	return accountRPMState(account, count), true
}

// OpenAIRPMSchedulable applies the hard ceiling and 80% headroom preference
// for movable requests. Sticky continuations can use headroom up to the cap.
func (s *OpenAIGatewayService) OpenAIRPMSchedulable(ctx context.Context, account *Account, movable bool) (bool, AccountRPMState, error) {
	if account == nil || !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
		return true, AccountRPMState{}, nil
	}
	state := accountRPMState(account, 0)
	if prefetched, ok := accountRPMStateFromContext(ctx, account); ok {
		state = prefetched
	} else {
		if s == nil || s.rpmCache == nil {
			return false, state, fmt.Errorf("%w: counter cache is not configured", ErrOpenAIRPMUnavailable)
		}
		count, err := s.rpmCache.GetRPM(ctx, account.RPMAccountID())
		if err != nil {
			return false, state, fmt.Errorf("%w: %v", ErrOpenAIRPMUnavailable, err)
		}
		state.Current = count
	}
	if state.Current >= state.Limit {
		return false, state, nil
	}
	if movable && state.Utilization() >= 0.8 {
		return false, state, nil
	}
	return true, state, nil
}

func withOpenAIRPMPrefetch(ctx context.Context, cache RPMCache, accounts []Account) (context.Context, error) {
	ids := make([]int64, 0, len(accounts))
	seen := make(map[int64]struct{}, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
			continue
		}
		id := account.RPMAccountID()
		if _, found := seen[id]; found {
			continue
		}
		ids = append(ids, id)
		seen[id] = struct{}{}
	}
	if len(ids) == 0 {
		return ctx, nil
	}
	if cache == nil {
		return ctx, fmt.Errorf("%w: counter cache is not configured", ErrOpenAIRPMUnavailable)
	}
	counts, err := cache.GetRPMBatch(ctx, ids)
	if err != nil {
		return ctx, fmt.Errorf("%w: %v", ErrOpenAIRPMUnavailable, err)
	}
	return context.WithValue(ctx, rpmPrefetchContextKey, counts), nil
}

type openAIRPMExhaustedError struct{ resetAt time.Time }

func (e *openAIRPMExhaustedError) Error() string { return ErrOpenAIRPMExhausted.Error() }
func (e *openAIRPMExhaustedError) Unwrap() error { return ErrOpenAIRPMExhausted }

func OpenAIRPMResetAt(err error) time.Time {
	var exhausted *openAIRPMExhaustedError
	if errors.As(err, &exhausted) {
		return exhausted.resetAt
	}
	return time.Time{}
}

func IsOpenAIRPMError(err error) bool {
	return errors.Is(err, ErrOpenAIRPMExhausted) || errors.Is(err, ErrOpenAIRPMUnavailable)
}

type openAIRPMReservationKey struct{}

type openAIRPMReservation struct {
	accountID int64
	resetAt   time.Time
	claimed   atomic.Bool
}

// WithOpenAIRPMReservation carries a pre-acquired handler slot to the first
// actual send. A retry or later WS turn must acquire a new slot.
func WithOpenAIRPMReservation(ctx context.Context, account *Account, state AccountRPMState) context.Context {
	if account == nil || !state.Enabled {
		return ctx
	}
	return context.WithValue(ctx, openAIRPMReservationKey{}, &openAIRPMReservation{
		accountID: account.RPMAccountID(),
		resetAt:   state.ResetAt,
	})
}

// TryAcquireOpenAIOAuthRPM reserves one request immediately before an actual
// upstream send. Other account types and accounts without a configured limit bypass.
func (s *OpenAIGatewayService) TryAcquireOpenAIOAuthRPM(ctx context.Context, account *Account) (bool, AccountRPMState, error) {
	if account == nil || !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
		return true, AccountRPMState{}, nil
	}
	state := AccountRPMState{Limit: account.GetBaseRPM(), Enabled: true}
	if s == nil || s.rpmCache == nil {
		return false, state, fmt.Errorf("%w: counter cache is not configured", ErrOpenAIRPMUnavailable)
	}
	strict, ok := s.rpmCache.(StrictRPMCache)
	if !ok {
		return false, state, fmt.Errorf("%w: atomic acquire is not supported", ErrOpenAIRPMUnavailable)
	}
	allowed, count, resetAt, err := strict.TryAcquireRPM(ctx, account.RPMAccountID(), state.Limit)
	state.Current = count
	state.ResetAt = resetAt
	if err != nil {
		return false, state, fmt.Errorf("%w: %v", ErrOpenAIRPMUnavailable, err)
	}
	if !allowed {
		return false, state, &openAIRPMExhaustedError{resetAt: resetAt}
	}
	return true, state, nil
}

func (s *OpenAIGatewayService) acquireOpenAIRPMForSend(ctx context.Context, account *Account) error {
	if account == nil || !account.IsOpenAIOAuth() || account.GetBaseRPM() <= 0 {
		return nil
	}
	if reservation, ok := ctx.Value(openAIRPMReservationKey{}).(*openAIRPMReservation); ok &&
		reservation.accountID == account.RPMAccountID() && time.Now().Before(reservation.resetAt) &&
		reservation.claimed.CompareAndSwap(false, true) {
		return nil
	}
	_, _, err := s.TryAcquireOpenAIOAuthRPM(ctx, account)
	return err
}
