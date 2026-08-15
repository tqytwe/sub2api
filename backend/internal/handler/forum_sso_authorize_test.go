//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Fix #8: GET /oauth/authorize carries no JWT middleware (only a rate limiter),
// so a top-level browser navigation authenticates via the bind-token cookie.
// A non-browser client (native app, server agent) cannot ride that cookie
// handoff and would be bounced to an HTML login page it cannot render. These
// tests pin that an Authorization: Bearer panel JWT is now accepted at parity
// with the cookie, and that the precedence and negative paths behave.

func forumSSOAuthTestService(t *testing.T) *service.AuthService {
	t.Helper()
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "forum-sso-authorize-secret", ExpireHour: 1}}
	// Pure JWT mint/verify: no repo or DB is touched by GenerateToken/ValidateToken.
	return service.NewAuthService(nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
}

func newForumSSOAuthContext(t *testing.T, method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, target, nil)
	return c, rec
}

func TestResolveAuthenticatedUserAcceptsBearer(t *testing.T) {
	authService := forumSSOAuthTestService(t)
	token, err := authService.GenerateToken(context.Background(), &service.User{ID: 77, Email: "a@example.com", Role: service.RoleUser})
	require.NoError(t, err)

	h := &ForumSSOHandler{authService: authService}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	c.Request.Header.Set("Authorization", "Bearer "+token)

	userID, ok := h.resolveAuthenticatedUser(c)
	require.True(t, ok, "a valid Bearer JWT must authenticate the authorize GET")
	require.Equal(t, int64(77), userID)
}

func TestResolveAuthenticatedUserBearerBeatsCookie(t *testing.T) {
	authService := forumSSOAuthTestService(t)
	bearerToken, err := authService.GenerateToken(context.Background(), &service.User{ID: 11, Role: service.RoleUser})
	require.NoError(t, err)
	cookieToken, err := authService.GenerateToken(context.Background(), &service.User{ID: 22, Role: service.RoleUser})
	require.NoError(t, err)

	h := &ForumSSOHandler{authService: authService}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	c.Request.Header.Set("Authorization", "Bearer "+bearerToken)
	c.Request.AddCookie(&http.Cookie{Name: oauthBindAccessTokenCookieName, Value: url.QueryEscape(cookieToken)})

	// An explicit credential wins: the client that attached a Bearer is asking
	// to authenticate as that token's subject, not the ambient cookie's.
	userID, ok := h.resolveAuthenticatedUser(c)
	require.True(t, ok)
	require.Equal(t, int64(11), userID, "Bearer must take precedence over the cookie")
}

func TestResolveAuthenticatedUserFallsBackToCookieWhenNoBearer(t *testing.T) {
	authService := forumSSOAuthTestService(t)
	cookieToken, err := authService.GenerateToken(context.Background(), &service.User{ID: 33, Role: service.RoleUser})
	require.NoError(t, err)

	h := &ForumSSOHandler{authService: authService}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	c.Request.AddCookie(&http.Cookie{Name: oauthBindAccessTokenCookieName, Value: url.QueryEscape(cookieToken)})

	userID, ok := h.resolveAuthenticatedUser(c)
	require.True(t, ok, "cookie path must still work when no Bearer is present")
	require.Equal(t, int64(33), userID)
}

func TestResolveAuthenticatedUserContextSubjectWins(t *testing.T) {
	h := &ForumSSOHandler{authService: forumSSOAuthTestService(t)}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	// Upstream middleware already resolved the subject: no token work needed.
	c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 99})

	userID, ok := h.resolveAuthenticatedUser(c)
	require.True(t, ok)
	require.Equal(t, int64(99), userID)
}

func TestResolveAuthenticatedUserRejectsGarbageBearer(t *testing.T) {
	h := &ForumSSOHandler{authService: forumSSOAuthTestService(t)}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	c.Request.Header.Set("Authorization", "Bearer not-a-real-jwt")

	// A malformed Bearer with no cookie is an anonymous request, not a 500.
	_, ok := h.resolveAuthenticatedUser(c)
	require.False(t, ok, "an invalid Bearer must not authenticate")
}

func TestResolveAuthenticatedUserWrongSecretRejected(t *testing.T) {
	// A JWT minted by a different secret must not validate against ours.
	otherCfg := &config.Config{JWT: config.JWTConfig{Secret: "some-other-secret", ExpireHour: 1}}
	otherAuth := service.NewAuthService(nil, nil, nil, nil, otherCfg, nil, nil, nil, nil, nil, nil, nil, nil)
	foreignToken, err := otherAuth.GenerateToken(context.Background(), &service.User{ID: 5, Role: service.RoleUser})
	require.NoError(t, err)

	h := &ForumSSOHandler{authService: forumSSOAuthTestService(t)}
	c, _ := newForumSSOAuthContext(t, http.MethodGet, "/api/v1/sso/oauth/authorize")
	c.Request.Header.Set("Authorization", "Bearer "+foreignToken)

	_, ok := h.resolveAuthenticatedUser(c)
	require.False(t, ok, "a token signed by a foreign secret must be rejected")
}

func TestUserFromBearerMalformedHeaderIsSoftMiss(t *testing.T) {
	h := &ForumSSOHandler{authService: forumSSOAuthTestService(t)}

	cases := map[string]string{
		"empty":        "",
		"no scheme":    "abcdef",
		"wrong scheme": "Basic Zm9vOmJhcg==",
		"bearer only":  "Bearer",
		"empty token":  "Bearer ",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			c, _ := newForumSSOAuthContext(t, http.MethodGet, "/x")
			if header != "" {
				c.Request.Header.Set("Authorization", header)
			}
			_, ok := h.userFromBearer(c)
			require.False(t, ok)
		})
	}
}
