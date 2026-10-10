package handler

import (
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Older Forward branches write complete JSON errors without a completion mark.
// A written error JSON is already the response; a request's stream flag alone
// must never turn it into an SSE stream. Heartbeat writers are stopped by callers.
func openAIJSONErrorResponseWritten(c *gin.Context) bool {
	if c == nil || c.Writer == nil || !c.Writer.Written() || c.Writer.Status() < http.StatusBadRequest {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(c.Writer.Header().Get("Content-Type"))
	return err == nil && (mediaType == "application/json" ||
		(strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json")))
}
