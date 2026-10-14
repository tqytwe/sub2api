package service

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Account probes keep sending comment frames after events, unlike gateway
// compact keepalives which stop on the first semantic write. Keep their state
// separate so probes cannot change gateway retry or billing semantics.
func startAccountTestSSEKeepalive(c *gin.Context, interval time.Duration) func() {
	if c == nil || c.Writer == nil || c.Request == nil || interval <= 0 {
		return func() {}
	}
	originalWriter, originalContext := c.Writer, c.Request.Context()
	ctx, cancel := context.WithCancel(originalContext)
	c.Request = c.Request.WithContext(ctx)
	w := &accountTestKeepaliveWriter{ResponseWriter: originalWriter, ctx: ctx, cancel: cancel}
	// Finalize the existing writer stack before publishing our locking wrapper.
	// ServerTiming's first Write reads c.Writer.Header(); publishing w first
	// would reenter w.mu while beat holds it. No heartbeat goroutine exists yet.
	w.beat()
	c.Writer = w
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !w.beat() {
					return
				}
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
			c.Writer = originalWriter
			c.Request = c.Request.WithContext(originalContext)
		})
	}
}

type accountTestKeepaliveWriter struct {
	gin.ResponseWriter
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	paused  bool
	started bool
}

func (w *accountTestKeepaliveWriter) beat() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx.Err() != nil {
		return false
	}
	if w.paused {
		return true
	}
	if !w.started {
		h := w.ResponseWriter.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		h.Set("X-Accel-Buffering", "no")
		w.ResponseWriter.WriteHeader(http.StatusOK)
		w.started = true
	}
	if _, err := w.ResponseWriter.Write([]byte(": keepalive\n\n")); err != nil {
		w.cancel()
		return false
	}
	w.ResponseWriter.Flush()
	return true
}

// Header exposes a mutable map. Pause background writes until the request
// goroutine completes header construction with Write/Flush.
func (w *accountTestKeepaliveWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = true
	return w.ResponseWriter.Header()
}
func (w *accountTestKeepaliveWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	w.paused = false
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		w.cancel()
	}
	return n, err
}
func (w *accountTestKeepaliveWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }
func (w *accountTestKeepaliveWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ResponseWriter.WriteHeader(code)
}
func (w *accountTestKeepaliveWriter) WriteHeaderNow() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ResponseWriter.WriteHeaderNow()
}
func (w *accountTestKeepaliveWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx.Err() == nil {
		w.paused = false
		w.ResponseWriter.Flush()
	}
}
func (w *accountTestKeepaliveWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.Status()
}
func (w *accountTestKeepaliveWriter) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.Size()
}
func (w *accountTestKeepaliveWriter) Written() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.Written()
}
