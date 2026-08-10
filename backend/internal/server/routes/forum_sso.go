package routes

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RegisterForumSSORoutes exposes this platform as the OAuth2 provider, wallet and
// VIP authority for the NodeBB community forum.
//
// Paths are fixed by the deployed forum's environment
// (NODEBB_SSO_AUTHORIZE_URL / TOKEN_URL / USERINFO_URL and the plugin's
// hardcoded /api/v1/sso/forum/orders and /api/v1/sso/wallet/balance), so they
// are part of the integration contract and cannot be renamed unilaterally.
//
// AUTH SPLIT
//   - /oauth/authorize, /oauth/token are public by protocol: the first is a
//     browser navigation, the second authenticates the confidential client with
//     its own credentials.
//   - /oauth/userinfo, /forum/orders, /wallet/balance require the opaque SSO
//     bearer token, NOT a panel JWT.
//   - POST /oauth/authorize is the SPA's resume step and uses the panel JWT.
func RegisterForumSSORoutes(
	v1 *gin.RouterGroup,
	forumSSO *handler.ForumSSOHandler,
	ssoService *service.ForumSSOService,
	jwtAuth servermiddleware.JWTAuthMiddleware,
	redisClient *redis.Client,
) {
	if forumSSO == nil || ssoService == nil {
		return
	}

	rateLimiter := middleware.NewRateLimiter(redisClient)
	sso := v1.Group("/sso")

	// Public protocol endpoints. Both are unauthenticated entry points that mint
	// or exchange credentials, so they carry fail-close rate limits: a leaked
	// client_id must not become an unbounded code-guessing oracle.
	sso.GET("/oauth/authorize", rateLimiter.LimitWithOptions("forum-sso-authorize", 30, time.Minute, middleware.RateLimitOptions{
		FailureMode: middleware.RateLimitFailClose,
	}), forumSSO.Authorize)

	sso.POST("/oauth/token", rateLimiter.LimitWithOptions("forum-sso-token", 60, time.Minute, middleware.RateLimitOptions{
		FailureMode: middleware.RateLimitFailClose,
	}), forumSSO.Token)

	// SPA resume step: completes an authorize request for an already
	// authenticated panel session.
	sso.POST("/oauth/authorize", gin.HandlerFunc(jwtAuth), forumSSO.AuthorizeDecision)

	// Forum-authenticated endpoints, guarded by the opaque SSO token.
	authed := sso.Group("")
	authed.Use(handler.ForumSSOBearerAuth(ssoService))
	{
		authed.GET("/oauth/userinfo", forumSSO.UserInfo)
		authed.GET("/wallet/balance", forumSSO.WalletBalance)
		authed.POST("/forum/orders", rateLimiter.LimitWithOptions("forum-sso-orders", 30, time.Minute, middleware.RateLimitOptions{
			FailureMode: middleware.RateLimitFailClose,
		}), forumSSO.CreateOrder)
	}
}
