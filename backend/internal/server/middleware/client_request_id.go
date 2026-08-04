package middleware

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// ClientRequestIDHeader is the stable client correlation header for mobile
	// and gateway requests. It is echoed only after bounded validation.
	ClientRequestIDHeader = "X-Client-Request-ID"
	clientRequestIDHeader = ClientRequestIDHeader
)

// ClientRequestID ensures every request has a unique client_request_id in request.Context().
//
// This is used by the Ops monitoring module for end-to-end request correlation.
func ClientRequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request == nil {
			c.Next()
			return
		}

		if v, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(v) != "" {
			var valid bool
			v, valid = normalizeCorrelationID(v)
			if !valid {
				v = uuid.New().String()
			}
			c.Header(clientRequestIDHeader, v)
			ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, v)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		if v, valid := normalizeCorrelationID(c.GetHeader(ClientRequestIDHeader)); valid {
			c.Header(ClientRequestIDHeader, v)
			ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, v)
			requestLogger := logger.FromContext(ctx).With(zap.String("client_request_id", strings.TrimSpace(v)))
			ctx = logger.IntoContext(ctx, requestLogger)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		id := uuid.New().String()
		c.Header(ClientRequestIDHeader, id)
		ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, id)
		requestLogger := logger.FromContext(ctx).With(zap.String("client_request_id", strings.TrimSpace(id)))
		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
