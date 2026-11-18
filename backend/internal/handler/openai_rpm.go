package handler

import (
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAIRPMAdmission struct {
	exhausted   bool
	unavailable bool
	resetAt     time.Time
}

func (a *openAIRPMAdmission) observe(err error, accountID int64, excluded map[int64]struct{}) bool {
	if !service.IsOpenAIRPMError(err) {
		return false
	}
	if accountID > 0 {
		excluded[accountID] = struct{}{}
	}
	if errors.Is(err, service.ErrOpenAIRPMExhausted) {
		a.exhausted = true
		if resetAt := service.OpenAIRPMResetAt(err); !resetAt.IsZero() && (a.resetAt.IsZero() || resetAt.Before(a.resetAt)) {
			a.resetAt = resetAt
		}
	} else {
		a.unavailable = true
	}
	return true
}

func (a *openAIRPMAdmission) selectionError(err error) error {
	if errors.Is(err, service.ErrNoAvailableAccounts) || errors.Is(err, service.ErrNoAvailableCompactAccounts) {
		if a.unavailable {
			return service.ErrOpenAIRPMUnavailable
		}
		if a.exhausted {
			return service.ErrOpenAIRPMExhausted
		}
	}
	return err
}

func (a *openAIRPMAdmission) retryAfter(c *gin.Context, err error) {
	if !errors.Is(err, service.ErrOpenAIRPMExhausted) {
		return
	}
	resetAt := a.resetAt
	if resetAt.IsZero() {
		resetAt = service.OpenAIRPMResetAt(err)
	}
	if resetAt.IsZero() {
		resetAt = time.Now().Truncate(time.Minute).Add(time.Minute)
	}
	seconds := max(1, int(math.Ceil(time.Until(resetAt).Seconds())))
	c.Header("Retry-After", strconv.Itoa(seconds))
}

func (h *OpenAIGatewayHandler) handleOpenAIRPMError(c *gin.Context, err error, streamStarted, anthropic bool, admission *openAIRPMAdmission) {
	if admission != nil {
		admission.retryAfter(c, err)
	}
	if errors.Is(err, service.ErrOpenAIRPMExhausted) {
		message := "All eligible OpenAI OAuth accounts are at their per-minute request limit; retry after the current minute resets"
		if anthropic {
			h.anthropicStreamingAwareError(c, 429, "rate_limit_error", message, streamStarted)
		} else {
			h.handleStreamingAwareError(c, 429, "rate_limit_error", message, streamStarted)
		}
		return
	}
	message := "OpenAI OAuth RPM protection is temporarily unavailable. Please retry later."
	if anthropic {
		h.anthropicStreamingAwareError(c, 503, "api_error", message, streamStarted)
	} else {
		h.handleStreamingAwareError(c, 503, "api_error", message, streamStarted)
	}
}
