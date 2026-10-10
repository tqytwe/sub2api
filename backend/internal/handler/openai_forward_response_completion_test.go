package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIForwardCompletionPreservesJSONErrors(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/responses/compact", "/v1/chat/completions"} {
		for _, status := range []int{400, 403, 429, 502} {
			for _, streamStarted := range []bool{false, true} {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, path, nil)
				before := c.Writer.Size()
				c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "protocol requirement"}})
				body := rec.Body.String()
				h := &OpenAIGatewayHandler{}
				require.True(t, openAIForwardErrorAlreadyCommunicated(c, before, errors.New("local protocol rejection")))
				require.False(t, h.ensureForwardErrorResponse(c, streamStarted))
				require.Equal(t, status, rec.Code)
				require.Equal(t, body, rec.Body.String())
				require.NotContains(t, body, "event:")
			}
		}
	}
}

func TestOpenAIForwardCompletionAdmissionKeepsExistingJSON(t *testing.T) {
	for _, contentType := range []string{"application/json; charset=utf-8", "Application/Problem+JSON; charset=utf-8"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Header("Content-Type", contentType)
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "requires Responses for tool calls"}})
		body := rec.Body.String()
		h := &OpenAIGatewayHandler{}
		require.True(t, h.handleOpenAITurnAdmissionError(c, &service.OpenAITurnAdmissionError{Reason: "responses_tools_protocol_mismatch"}, true))
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, body, rec.Body.String())
	}
}

func TestOpenAIForwardCompletionWritesOneSSETerminal(t *testing.T) {
	for _, path := range []string{"/v1/responses", "/v1/responses/compact", "/v1/chat/completions"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, path, nil)
			c.Header("Content-Type", "text/event-stream")
			_, err := c.Writer.WriteString(": ping\n\n")
			require.NoError(t, err)
			c.Writer.Flush()
			h := &OpenAIGatewayHandler{}
			require.True(t, h.ensureForwardErrorResponse(c, true))
			require.False(t, h.ensureForwardErrorResponse(c, true))
			terminal := "event: error\n"
			if strings.Contains(path, "/responses") {
				terminal = "event: response.failed\n"
			}
			require.Equal(t, 1, strings.Count(rec.Body.String(), terminal))
			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}
