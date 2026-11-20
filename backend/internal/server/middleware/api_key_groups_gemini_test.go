package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupAllowsGooglePlatformBridgesNonGeminiModelsEndpoint(t *testing.T) {
	openai := &service.Group{Platform: service.PlatformOpenAI}
	gemini := &service.Group{Platform: service.PlatformGemini}
	typesafe := &service.Group{Platform: service.PlatformTypeSafe}

	require.True(t, groupAllowsGooglePlatform("/v1beta/models/gpt-4o:generateContent", openai))
	require.True(t, groupAllowsGooglePlatform("/v1beta/models", openai))
	require.False(t, groupAllowsGooglePlatform("/v1beta/cachedContents", openai))
	require.False(t, groupAllowsGooglePlatform("/antigravity/v1beta/models/gemini:generateContent", openai))
	require.True(t, groupAllowsGooglePlatform("/v1beta/cachedContents", gemini))
	require.False(t, groupAllowsGooglePlatform("/v1beta/models/jev:generateContent", typesafe))
}

func TestGeminiCachePinsGroupBeforeGoogleAuthenticationFinishes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	primary := int64(1)
	groups := map[int64]*service.Group{
		1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true},
		2: {ID: 2, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true},
	}
	key := &service.APIKey{ID: 7, UserID: 9, Key: "test-gemini-key", Status: service.StatusActive, GroupID: &primary, GroupIDs: []int64{1, 2}, Group: groups[1], User: &service.User{ID: 9, Status: service.StatusActive, Balance: 10}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	svc := service.NewAPIKeyService(multiGroupAuthKeyRepo{key: key}, nil, multiGroupAuthGroupRepo{groups: groups}, nil, nil, nil, cfg)
	lookup := func(_ context.Context, keyID int64, name string) (int64, error) {
		require.EqualValues(t, 7, keyID)
		switch name {
		case "owned", "cachedContents/owned":
			return 2, nil
		case "unbound":
			return 3, nil
		default:
			return 0, service.ErrGeminiCachedContentNotFound
		}
	}
	check := func(c *gin.Context) {
		selected, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.EqualValues(t, 2, *selected.GroupID)
		c.Status(http.StatusNoContent)
	}
	r := gin.New()
	v1beta := r.Group("/v1beta", APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg, lookup))
	v1beta.GET("/cachedContents/:cacheID", check)
	v1beta.POST("/models/*modelAction", check)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/v1beta/cachedContents/owned", "", http.StatusNoContent},
		{http.MethodPost, "/v1beta/models/gemini-flash:generateContent", `{"cachedContent":"cachedContents/owned"}`, http.StatusNoContent},
		{http.MethodGet, "/v1beta/cachedContents/unbound", "", http.StatusForbidden},
		{http.MethodGet, "/v1beta/cachedContents/foreign", "", http.StatusForbidden},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("x-goog-api-key", key.Key)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		require.Equal(t, tc.status, res.Code, res.Body.String())
	}
}
