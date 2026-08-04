package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func executeUserIdempotentJSON(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	execute func(context.Context) (any, error),
) {
	executeUserIdempotentResponse(c, scope, payload, ttl, http.StatusOK, true, execute)
}

// executeUserIdempotentJSONOptionalKey keeps compatibility routes usable by
// older web clients while still deduplicating requests that do provide a key.
// The coordinator remains authoritative whenever a key is present; only the
// missing-key rejection is relaxed for the caller explicitly marked as
// observe-only by the protocol contract.
func executeUserIdempotentJSONOptionalKey(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	execute func(context.Context) (any, error),
) {
	executeUserIdempotentResponse(c, scope, payload, ttl, http.StatusOK, false, execute)
}

// mobileUserIdempotencyScope returns an account-scoped namespace for mobile
// writes. The idempotency table is intentionally shared by all handlers and
// is keyed by (scope, key hash), so the account must be part of the scope
// before a request reaches the coordinator. The mobile write handlers already
// require authentication; keep the base scope unchanged for an absent/invalid
// subject so the helper remains safe to call during error handling.
//
// Older records use the unscoped name. We deliberately do not replay those
// records here: they cannot be proven to belong to the current account. They
// remain subject to the existing TTL cleanup and new clients get an isolated
// namespace immediately without a schema migration.
func mobileUserIdempotencyScope(c *gin.Context, baseScope string) string {
	baseScope = strings.TrimSpace(baseScope)
	if c == nil || baseScope == "" {
		return baseScope
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		return baseScope
	}
	return baseScope + ".user." + strconv.FormatInt(subject.UserID, 10)
}

// executeUserIdempotentCreated preserves the normal 201 contract for create
// routes while allowing a retry with the same key to replay the exact resource
// instead of invoking storage or business side effects again.
func executeUserIdempotentCreated(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	execute func(context.Context) (any, error),
) {
	executeUserIdempotentResponse(c, scope, payload, ttl, http.StatusCreated, true, execute)
}

func executeUserIdempotentCreatedOptionalKey(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	execute func(context.Context) (any, error),
) {
	executeUserIdempotentResponse(c, scope, payload, ttl, http.StatusCreated, false, execute)
}

func executeUserIdempotentResponse(
	c *gin.Context,
	scope string,
	payload any,
	ttl time.Duration,
	status int,
	requireKey bool,
	execute func(context.Context) (any, error),
) {
	coordinator := service.DefaultIdempotencyCoordinator()
	if coordinator == nil {
		data, err := execute(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		writeUserIdempotentResponse(c, status, data)
		return
	}

	actorScope := "user:0"
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		actorScope = "user:" + strconv.FormatInt(subject.UserID, 10)
	}

	result, err := coordinator.Execute(c.Request.Context(), service.IdempotencyExecuteOptions{
		Scope:          scope,
		ActorScope:     actorScope,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
		Payload:        payload,
		RequireKey:     requireKey,
		TTL:            ttl,
	}, execute)
	if err != nil {
		if infraerrors.Code(err) == infraerrors.Code(service.ErrIdempotencyStoreUnavail) {
			service.RecordIdempotencyStoreUnavailable(c.FullPath(), scope, "handler_fail_close")
			logger.LegacyPrintf("handler.idempotency", "[Idempotency] store unavailable: method=%s route=%s scope=%s strategy=fail_close", c.Request.Method, c.FullPath(), scope)
		}
		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		response.ErrorFrom(c, err)
		return
	}
	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	writeUserIdempotentResponse(c, status, result.Data)
}

func writeUserIdempotentResponse(c *gin.Context, status int, data any) {
	if status == http.StatusCreated {
		response.Created(c, data)
		return
	}
	response.Success(c, data)
}
