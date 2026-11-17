package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIHandlersTimeRejectedRequests(t *testing.T) {
	for _, endpoint := range []string{"responses", "messages"} {
		t.Run(endpoint, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			collector := requesttiming.New(time.Now(), 0)
			ctx := requesttiming.With(context.Background(), collector)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+endpoint, nil).WithContext(ctx)
			h := &OpenAIGatewayHandler{}
			if endpoint == "responses" {
				h.Responses(c)
			} else {
				h.Messages(c)
			}
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			var snapshot requesttiming.Snapshot
			collector.WhenFinished(func(s requesttiming.Snapshot) { snapshot = s })
			collector.Finish(recorder.Code, false)
			require.Contains(t, snapshot.Events, "handler_start")
			require.Len(t, snapshot.Spans, 1)
			require.Equal(t, "handler", snapshot.Spans[0].Name)
			require.Empty(t, snapshot.Attempts)
		})
	}
}
