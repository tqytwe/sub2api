package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ForumSSOHandler serves this platform as the OAuth2 provider for the NodeBB
// community forum.
//
// RESPONSE SHAPE
// --------------
// The OAuth2 and userinfo endpoints return RAW top-level JSON, not the
// platform's {code,message,data} envelope. This is not an inconsistency to be
// tidied up later: passport-oauth2 in the forum reads access_token off the top
// level per RFC 6749 §5.1, and the plugin's user-sync reads userInfo.id /
// .email directly. Wrapping either would break login outright.
type ForumSSOHandler struct {
	sso         *service.ForumSSOService
	authService *service.AuthService
}

func NewForumSSOHandler(sso *service.ForumSSOService, authService *service.AuthService) *ForumSSOHandler {
	return &ForumSSOHandler{sso: sso, authService: authService}
}

// oauthError emits an RFC 6749 §5.2 error body.
func oauthError(c *gin.Context, status int, code, description string) {
	c.JSON(status, gin.H{"error": code, "error_description": description})
}

// Authorize is the browser entry point.
// GET /api/v1/sso/oauth/authorize
//
// The platform SPA keeps its JWT in JS, not in a durable cookie, so a top-level
// navigation arriving here usually carries no credential. Rather than guess, the
// endpoint validates the client and redirect_uri first (so a bad client never
// reaches a login page), then either issues a code for an already-authenticated
// request or bounces the browser to the frontend login page with a return path
// back to this exact URL.
func (h *ForumSSOHandler) Authorize(c *gin.Context) {
	if h.sso == nil || !h.sso.Enabled() {
		oauthError(c, http.StatusNotFound, "temporarily_unavailable", "forum sso is not enabled")
		return
	}

	query := c.Request.URL.Query()
	responseType := query.Get("response_type")
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")
	scope := query.Get("scope")

	// Validate the client and redirect target BEFORE anything is echoed back or
	// any redirect happens. An unvalidated redirect_uri here is an open redirect
	// that leaks authorization codes.
	if err := h.sso.VerifyClientID(clientID); err != nil {
		oauthError(c, http.StatusUnauthorized, "invalid_client", "unknown client_id")
		return
	}
	if err := h.sso.VerifyRedirectURI(redirectURI); err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_request", "redirect_uri is not allowlisted")
		return
	}
	// From here on the redirect_uri is trusted, so protocol errors can be
	// reported to it per RFC 6749 §4.1.2.1.
	if responseType != "code" {
		redirectWithOAuthError(c, redirectURI, state, "unsupported_response_type", "only response_type=code is supported")
		return
	}

	userID, authenticated := h.resolveAuthenticatedUser(c)
	if !authenticated {
		h.redirectToLogin(c)
		return
	}

	code, err := h.sso.IssueAuthorizationCode(c.Request.Context(), userID, clientID, redirectURI, scope)
	if err != nil {
		redirectWithOAuthError(c, redirectURI, state, "server_error", "could not issue authorization code")
		return
	}

	c.Redirect(http.StatusFound, appendQuery(redirectURI, map[string]string{
		"code":  code,
		"state": state,
	}))
}

// AuthorizeDecision lets the already-authenticated SPA complete an authorize
// request with its Bearer token and receive the redirect target as JSON.
// POST /api/v1/sso/oauth/authorize
//
// This is the counterpart to the GET flow's login bounce: the frontend arrives
// at the login page, authenticates normally, then calls this with the original
// parameters and performs the navigation itself. Mounted behind the standard JWT
// middleware, and enveloped because our own frontend consumes it.
func (h *ForumSSOHandler) AuthorizeDecision(c *gin.Context) {
	if h.sso == nil || !h.sso.Enabled() {
		response.ErrorFrom(c, service.ErrForumSSODisabled)
		return
	}

	var req struct {
		ClientID    string `json:"client_id"`
		RedirectURI string `json:"redirect_uri"`
		State       string `json:"state"`
		Scope       string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	if err := h.sso.VerifyClientID(req.ClientID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.sso.VerifyRedirectURI(req.RedirectURI); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.ErrorFrom(c, infraerrors.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	code, err := h.sso.IssueAuthorizationCode(c.Request.Context(), subject.UserID, req.ClientID, req.RedirectURI, req.Scope)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"redirect_uri": appendQuery(req.RedirectURI, map[string]string{
			"code":  code,
			"state": req.State,
		}),
	})
}

// Token exchanges an authorization code for an SSO access token.
// POST /api/v1/sso/oauth/token
//
// Accepts client credentials either as form fields or via HTTP Basic, because
// passport-oauth2 defaults to Basic and RFC 6749 §2.3.1 requires supporting it.
func (h *ForumSSOHandler) Token(c *gin.Context) {
	if h.sso == nil || !h.sso.Enabled() {
		oauthError(c, http.StatusNotFound, "temporarily_unavailable", "forum sso is not enabled")
		return
	}

	grantType := c.PostForm("grant_type")
	code := c.PostForm("code")
	redirectURI := c.PostForm("redirect_uri")
	clientID := c.PostForm("client_id")
	clientSecret := c.PostForm("client_secret")

	// HTTP Basic takes precedence when present; passport-oauth2 sends it there.
	if basicID, basicSecret, ok := c.Request.BasicAuth(); ok {
		clientID, clientSecret = basicID, basicSecret
	}

	if err := h.sso.VerifyClientCredentials(clientID, clientSecret); err != nil {
		// 401 with WWW-Authenticate per RFC 6749 §5.2 for failed client auth.
		c.Header("WWW-Authenticate", `Basic realm="forum-sso"`)
		oauthError(c, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if grantType != "authorization_code" {
		oauthError(c, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}

	record, err := h.sso.RedeemAuthorizationCode(c.Request.Context(), code, clientID, redirectURI)
	if err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_grant", "authorization code is invalid, expired or already used")
		return
	}

	token, expiresIn, err := h.sso.IssueAccessToken(c.Request.Context(), record.UserID, record.Scope)
	if err != nil {
		oauthError(c, http.StatusInternalServerError, "server_error", "could not issue access token")
		return
	}

	// RFC 6749 §5.1 requires these at the top level and forbids caching.
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
		"scope":        record.Scope,
	})
}

// UserInfo returns the profile the forum syncs into its own user record.
// GET /api/v1/sso/oauth/userinfo
func (h *ForumSSOHandler) UserInfo(c *gin.Context) {
	userID, ok := forumSSOSubject(c)
	if !ok {
		return
	}

	info, err := h.sso.BuildUserInfo(c.Request.Context(), userID)
	if err != nil {
		writeForumSSOError(c, err)
		return
	}
	// Raw document: the plugin reads userInfo.id / .email / .vip_tier directly.
	c.JSON(http.StatusOK, info)
}

// CreateOrder charges the platform wallet for a forum purchase and grants the
// entitlement on the forum.
// POST /api/v1/sso/forum/orders
func (h *ForumSSOHandler) CreateOrder(c *gin.Context) {
	userID, ok := forumSSOSubject(c)
	if !ok {
		return
	}

	var req service.ForumOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request", "message": "invalid request body"})
		return
	}

	order, err := h.sso.CreateForumOrder(c.Request.Context(), userID, req)
	if err != nil {
		writeForumSSOError(c, err)
		return
	}

	// The wallet has already been debited, so the entitlement grant is sent
	// synchronously and its outcome reported. The forum is idempotent on
	// order_id, so a retry cannot double-grant.
	if err := h.sso.SendForumPaymentCallback(c.Request.Context(), userID, order, req.ItemID, req.ItemType); err != nil {
		// Persist a retry job so the callback is re-attempted in the background.
		// The user's wallet is already debited; this ensures the entitlement
		// eventually lands without requiring user action.
		h.sso.EnqueuePaymentCallbackRetry(order, userID, req.ItemID, req.ItemType)
		// Deliberately still 200: the charge succeeded and the forum reconciles
		// on retry. Reporting a failure here would invite the forum to re-POST
		// and confuse the user about whether they paid.
		c.JSON(http.StatusOK, gin.H{
			"order_id":       order.OrderID,
			"status":         order.Status,
			"amount":         order.Amount,
			"balance_after":  order.BalanceAfter,
			"transaction_id": order.TransactionID,
			"already_paid":   order.AlreadyPaid,
			"grant_pending":  true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":       order.OrderID,
		"status":         order.Status,
		"amount":         order.Amount,
		"balance_after":  order.BalanceAfter,
		"transaction_id": order.TransactionID,
		"already_paid":   order.AlreadyPaid,
		"grant_pending":  false,
	})
}

// WalletBalance returns the live wallet snapshot.
// GET /api/v1/sso/wallet/balance
func (h *ForumSSOHandler) WalletBalance(c *gin.Context) {
	userID, ok := forumSSOSubject(c)
	if !ok {
		return
	}

	balance, err := h.sso.GetWalletBalance(c.Request.Context(), userID)
	if err != nil {
		writeForumSSOError(c, err)
		return
	}
	c.JSON(http.StatusOK, balance)
}

// forumSSOSubject resolves the opaque SSO bearer token to a platform user,
// writing the error response itself when resolution fails.
func forumSSOSubject(c *gin.Context) (int64, bool) {
	value, exists := c.Get(forumSSOUserIDContextKey)
	if !exists {
		oauthError(c, http.StatusUnauthorized, "invalid_token", "authentication required")
		return 0, false
	}
	userID, ok := value.(int64)
	if !ok || userID <= 0 {
		oauthError(c, http.StatusUnauthorized, "invalid_token", "authentication required")
		return 0, false
	}
	return userID, true
}

// writeForumSSOError maps service errors onto raw JSON, keeping the forum's
// error handling simple while preserving the platform's status codes.
func writeForumSSOError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrForumSSOBadToken):
		oauthError(c, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
	case errors.Is(err, service.ErrForumSSODisabled):
		oauthError(c, http.StatusNotFound, "temporarily_unavailable", "forum sso is not enabled")
	default:
		status, status2 := infraerrors.ToHTTP(err)
		c.JSON(status, gin.H{
			"error":             status2.Reason,
			"error_description": status2.Message,
		})
	}
}

// appendQuery adds parameters while preserving any the redirect URI already has.
// Empty values are dropped so an absent state does not become "state=".
func appendQuery(base string, params map[string]string) string {
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	for key, value := range params {
		if value != "" {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func redirectWithOAuthError(c *gin.Context, redirectURI, state, code, description string) {
	c.Redirect(http.StatusFound, appendQuery(redirectURI, map[string]string{
		"error":             code,
		"error_description": description,
		"state":             state,
	}))
}

// resolveAuthenticatedUser looks for a platform session on a browser navigation.
// Falls back to the short-lived HttpOnly cookie the SPA can set via
// /api/v1/auth/oauth/bind-token, mirroring how the existing third-party OAuth
// bind flows authenticate a top-level GET.
func (h *ForumSSOHandler) resolveAuthenticatedUser(c *gin.Context) (int64, bool) {
	if subject, ok := servermiddleware.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return subject.UserID, true
	}
	if h.authService == nil {
		return 0, false
	}

	cookie, err := c.Request.Cookie(oauthBindAccessTokenCookieName)
	if err != nil {
		return 0, false
	}
	tokenString, err := url.QueryUnescape(strings.TrimSpace(cookie.Value))
	if err != nil || tokenString == "" {
		return 0, false
	}
	claims, err := h.authService.ValidateToken(tokenString)
	if err != nil || claims == nil || claims.UserID <= 0 {
		return 0, false
	}
	return claims.UserID, true
}

// redirectToLogin bounces an unauthenticated browser to the frontend login page,
// carrying the full original authorize URL so the SPA can resume the flow.
func (h *ForumSSOHandler) redirectToLogin(c *gin.Context) {
	loginPath := strings.TrimSpace(h.sso.LoginPagePath())
	if loginPath == "" {
		loginPath = "/login"
	}
	c.Redirect(http.StatusFound, appendQuery(loginPath, map[string]string{
		"forum_sso_resume": c.Request.URL.RequestURI(),
	}))
}
