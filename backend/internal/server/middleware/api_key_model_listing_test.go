package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMultiGroupModelListingOnlyUsesOrdinaryCollection(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/v1/models", true},
		{http.MethodGet, "/models", true},
		{http.MethodGet, "/models?unused=1", true},
		{http.MethodGet, "/v1/models?model=", false},
		{http.MethodGet, "/v1/models?model=gpt-5.5", false},
		{http.MethodGet, "/models?client_version=1.0", false},
		{http.MethodGet, "/v1/models/gpt-5.5", false},
		{http.MethodGet, "/antigravity/models", false},
		{http.MethodGet, "/v1beta/models", false},
		{http.MethodPost, "/models", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(tc.method, tc.path, nil)
			require.Equal(t, tc.want, isMultiGroupModelListing(c, &service.APIKey{GroupIDs: []int64{1, 2}}))
			require.False(t, isMultiGroupModelListing(c, &service.APIKey{GroupIDs: []int64{1}}))
			require.False(t, isMultiGroupModelListing(c, nil))
		})
	}
}
