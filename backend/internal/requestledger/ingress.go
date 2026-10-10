package requestledger

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ManagedPath(path string) bool {
	for _, prefix := range []string{
		"/v1", "/v1beta", "/antigravity", "/backend-api/codex", "/responses", "/models",
		"/images", "/videos", "/custom-voices", "/contents/generations/tasks",
		"/api/v3/contents/generations/tasks", "/v3/contents/generations/tasks",
		"/api/v1/image-studio", "/api/v1/nextchat/image-studio", "/api/v1/mobile/video",
		"/api/v1/mobile/tasks", "/api/v1/mobile/image-history",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	switch path {
	case "/chat/completions", "/embeddings", "/messages/count_tokens", "/alpha/search", "/tts", "/stt", "/realtime", "/agnesapi", "/api/v1/mobile/web-search":
		return true
	}
	return false
}

func Reject(c *gin.Context) {
	c.Header("Retry-After", "1")
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{
		"type": "service_unavailable", "code": "request_ledger_unavailable", "message": "Request audit storage is temporarily unavailable; retry later.",
	}})
}

// Middleware must precede auth, quota, body-limit, frontend fallback and CORS
// middleware so even early rejections have a committed private identity.
func Middleware(l *Ledger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ManagedPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		route := c.FullPath()
		if route == "" {
			route = "unmatched_gateway"
		}
		kind := "http"
		if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
			kind = "ws_session"
		}
		h, err := l.Begin(c.Request.Context(), route, c.Request.Method, kind)
		if err != nil {
			Reject(c)
			return
		}
		c.Request = c.Request.WithContext(WithHandle(c.Request.Context(), h))
		c.Writer = &ledgerResponseWriter{ResponseWriter: c.Writer, ctx: c.Request.Context(), handle: h}
		// Deliberately not a response header: private IDs are visible only through
		// the authenticated ledger API, never forwarded upstream or client-chosen.
		defer func() {
			if err := h.closeOpenTurns(c.Request.Context()); err != nil {
				slog.Error("request_ledger_turn_finish_failed", "private_id", h.ID)
			}
			if v := recover(); v != nil {
				status := http.StatusInternalServerError
				if c.Writer.Written() {
					status = c.Writer.Status()
				}
				if err := h.Finish(c.Request.Context(), "failed", status, "internal_error"); err != nil {
					slog.Error("request_ledger_finish_failed", "private_id", h.ID)
				}
				panic(v)
			}
			status := c.Writer.Status()
			state := Outcome(status, c.Request.Context().Err())
			code := ErrorCode(c.Request.Context().Err())
			if code == "" {
				code = HTTPErrorCode(status)
			}
			if admissionFailed(c.Request.Context()) {
				state = "failed"
				code = "request_ledger_unavailable"
			}
			if err := h.Finish(c.Request.Context(), state, status, code); err != nil {
				slog.Error("request_ledger_finish_failed", "private_id", h.ID)
			}
		}()
		c.Next()
	}
}

func HTTPErrorCode(status int) string {
	switch {
	case status == 401:
		return "unauthorized"
	case status == 403:
		return "forbidden"
	case status == 429:
		return "rate_limited"
	case status == 408 || status == 504:
		return "timeout"
	case status >= 500:
		return "internal_error"
	case status >= 400:
		return "invalid_request"
	default:
		return ""
	}
}

func admissionFailed(ctx context.Context) bool {
	h := FromContext(CurrentContext(ctx))
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.admissionFailed
}

// Preserve the per-request refusal even when an older provider maps transport
// errors to 502. A response already streaming cannot change its HTTP status.
// Embedding Gin's writer preserves Hijacker, Flusher and CloseNotifier.
type ledgerResponseWriter struct {
	gin.ResponseWriter
	ctx     context.Context
	handle  *Handle
	refused bool
}

func (w *ledgerResponseWriter) refuse() bool {
	if w.refused {
		return true
	}
	if !admissionFailed(w.ctx) || w.Written() {
		return false
	}
	w.refused = true
	w.Header().Set("Retry-After", "1")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.ResponseWriter.Write([]byte(`{"error":{"type":"service_unavailable","code":"request_ledger_unavailable","message":"Request audit storage is temporarily unavailable; retry later."}}`))
	return true
}
func (w *ledgerResponseWriter) WriteHeader(code int) {
	if !w.refuse() {
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *ledgerResponseWriter) WriteHeaderNow() {
	if !w.refuse() {
		w.ResponseWriter.WriteHeaderNow()
	}
}
func (w *ledgerResponseWriter) Write(p []byte) (int, error) {
	if w.refuse() {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}
func (w *ledgerResponseWriter) WriteString(p string) (int, error) {
	if w.refuse() {
		return len(p), nil
	}
	return w.ResponseWriter.WriteString(p)
}
func (w *ledgerResponseWriter) Flush() {
	if !w.refuse() {
		w.ResponseWriter.Flush()
	}
}
