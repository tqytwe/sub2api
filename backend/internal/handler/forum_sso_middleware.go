package handler

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// forumSSOUserIDContextKey carries the resolved platform user ID from the SSO
// bearer middleware to the handlers. Deliberately distinct from the panel JWT
// context key so an SSO token can never be mistaken for a full panel session.
const forumSSOUserIDContextKey = "forum_sso_user_id"

// ForumSSOBearerAuth resolves the opaque SSO access token issued to the forum.
//
// This is intentionally NOT the panel JWT middleware. The token the forum holds
// is an opaque Redis-backed string scoped to /api/v1/sso/*, so presenting it
// anywhere else fails, and a leaked forum token cannot drive the rest of the
// platform API.
func ForumSSOBearerAuth(sso *service.ForumSSOService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sso == nil || !sso.Enabled() {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error":             "temporarily_unavailable",
				"error_description": "forum sso is not enabled",
			})
			return
		}

		const bearerPrefix = "bearer "
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if len(header) <= len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
			// RFC 6750 §3: challenge the client on a missing credential.
			c.Header("WWW-Authenticate", `Bearer realm="forum-sso"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":             "invalid_token",
				"error_description": "authentication required",
			})
			return
		}

		token := strings.TrimSpace(header[len(bearerPrefix):])
		userID, err := sso.ResolveAccessToken(c.Request.Context(), token)
		if err != nil || userID <= 0 {
			c.Header("WWW-Authenticate", `Bearer realm="forum-sso", error="invalid_token"`)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":             "invalid_token",
				"error_description": "access token is invalid or expired",
			})
			return
		}

		c.Set(forumSSOUserIDContextKey, userID)
		c.Next()
	}
}
