package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requesttiming"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCompositeTimingFinishesBeforeDownstream(t *testing.T) {
	collector := requesttiming.New(time.Now(), 0)
	var snapshot requesttiming.Snapshot
	collector.WhenFinished(func(s requesttiming.Snapshot) { snapshot = s })
	router := gin.New()
	router.POST("/v1/responses", compositeTargetPlatformMiddleware(nil), func(c *gin.Context) { collector.Finish(http.StatusOK, false) })
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	request = request.WithContext(requesttiming.With(request.Context(), collector))
	router.ServeHTTP(httptest.NewRecorder(), request)
	require.Len(t, snapshot.Spans, 1)
	require.Equal(t, "composite_routing", snapshot.Spans[0].Name)
}
