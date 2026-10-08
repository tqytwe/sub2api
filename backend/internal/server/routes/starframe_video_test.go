//go:build unit

package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayRoutesOpenAIStarframeContentReachesAuthenticatedHandler(t *testing.T) {
	router := newGatewayRoutesTestRouter(service.PlatformOpenAI)
	for _, path := range []string{"/v1/videos/sfv_00000000-0000-0000-0000-000000000001/content", "/videos/sfv_00000000-0000-0000-0000-000000000001/content"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.NotContains(t, w.Body.String(), "not supported for this platform", path)
	}
}
