package service

import (
	"context"
	"errors"
	"time"
)

// MobileWebSearchBudget reserves one upstream search attempt for an account.
// The storage-backed implementation lives in the repository layer so HTTP
// handlers depend only on this small service contract.
type MobileWebSearchBudget interface {
	Reserve(context.Context, int64) (time.Duration, error)
}

// ErrMobileWebSearchBudgetUnavailable is returned when the shared budget
// store cannot make an authoritative reservation decision.
var ErrMobileWebSearchBudgetUnavailable = errors.New("mobile web search budget store unavailable")

// MobileWebSearchBudgetExceededError carries the earliest safe retry time when
// an account or global search budget has been exhausted.
type MobileWebSearchBudgetExceededError struct {
	RetryAfter time.Duration
}

func (e *MobileWebSearchBudgetExceededError) Error() string {
	return "mobile web search budget exceeded"
}

// NewMobileWebSearchBudgetExceededError creates a normalized quota error.
func NewMobileWebSearchBudgetExceededError(retryAfter time.Duration) error {
	if retryAfter <= 0 {
		retryAfter = time.Minute
	}
	return &MobileWebSearchBudgetExceededError{RetryAfter: retryAfter}
}
