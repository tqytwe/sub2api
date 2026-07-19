package routes

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const nextChatLaunchTokenKeyPrefix = "nextchat:launch:"

type nextChatSessionIssuer interface {
	IssueNextChatManagedSession(ctx context.Context, userID int64) (*service.NextChatManagedSession, error)
}

type nextChatFeatureGate interface {
	IsNextChatEnabled(ctx context.Context) bool
}

type nextChatLaunchTokenRecord struct {
	UserID     int64     `json:"user_id"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
}

type nextChatExchangeRequest struct {
	LaunchToken string `json:"launch_token"`
}

func RegisterNextChatRoutes(
	v1 *gin.RouterGroup,
	jwtAuth middleware.JWTAuthMiddleware,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	registerNextChatRoutes(v1, jwtAuth, apiKeyService, settingService, cfg, redisClient)
}

func registerNextChatRoutes(
	v1 *gin.RouterGroup,
	jwtAuth middleware.JWTAuthMiddleware,
	issuer nextChatSessionIssuer,
	gate nextChatFeatureGate,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	nextchat := v1.Group("/nextchat")
	{
		nextchat.POST("/session", func(c *gin.Context) {
			handleNextChatSessionExchange(c, issuer, gate, cfg, redisClient)
		})
	}

	authenticated := nextchat.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.POST("/launch", func(c *gin.Context) {
			handleNextChatLaunch(c, gate, cfg, redisClient)
		})
	}
}

func handleNextChatLaunch(
	c *gin.Context,
	gate nextChatFeatureGate,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	if redisClient == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat launch token store is unavailable")
		return
	}

	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	ttl := nextChatLaunchTokenTTL(cfg)
	now := time.Now().UTC()
	token, err := randomNextChatLaunchToken()
	if err != nil {
		response.InternalError(c, "Failed to create launch token")
		return
	}
	record := nextChatLaunchTokenRecord{
		UserID:     subject.UserID,
		IssuedAt:   now,
		ExpiresAt:  now.Add(ttl),
		RemoteAddr: c.ClientIP(),
	}
	raw, err := json.Marshal(record)
	if err != nil {
		response.InternalError(c, "Failed to create launch token")
		return
	}
	ok, err = redisClient.SetNX(c.Request.Context(), nextChatLaunchTokenKey(token), raw, ttl).Result()
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat launch token store is unavailable")
		return
	}
	if !ok {
		response.InternalError(c, "Failed to create launch token")
		return
	}

	response.Success(c, gin.H{
		"launch_url":  nextChatLaunchURL(cfg, token),
		"expires_at":  record.ExpiresAt,
		"ttl_seconds": int(ttl.Seconds()),
	})
}

func handleNextChatSessionExchange(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	gate nextChatFeatureGate,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	if issuer == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat session issuer is unavailable")
		return
	}
	if redisClient == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat launch token store is unavailable")
		return
	}
	if !validNextChatExchangeSecret(c.GetHeader("X-NextChat-Secret"), cfg) {
		response.Unauthorized(c, "Invalid NextChat exchange secret")
		return
	}

	var req nextChatExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	launchToken := strings.TrimSpace(req.LaunchToken)
	if launchToken == "" {
		response.BadRequest(c, "launch_token is required")
		return
	}

	raw, err := redisClient.GetDel(c.Request.Context(), nextChatLaunchTokenKey(launchToken)).Result()
	if err == redis.Nil {
		response.Unauthorized(c, "Invalid or consumed launch token")
		return
	}
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat launch token store is unavailable")
		return
	}
	var record nextChatLaunchTokenRecord
	if err := json.Unmarshal([]byte(raw), &record); err != nil || record.UserID <= 0 {
		response.Unauthorized(c, "Invalid launch token")
		return
	}
	if !record.ExpiresAt.After(time.Now().UTC()) {
		response.Unauthorized(c, "Launch token has expired")
		return
	}

	session, err := issuer.IssueNextChatManagedSession(c.Request.Context(), record.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	expiresAt := time.Now().UTC().Add(nextChatSessionTTL(cfg))
	response.Success(c, gin.H{
		"user_id":    session.UserID,
		"api_key":    session.APIKey,
		"api_key_id": session.KeyID,
		"expires_at": expiresAt,
	})
}

func randomNextChatLaunchToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func nextChatLaunchTokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return nextChatLaunchTokenKeyPrefix + hex.EncodeToString(sum[:])
}

func nextChatLaunchURL(cfg *config.Config, token string) string {
	base := "/ai"
	if cfg != nil && strings.TrimSpace(cfg.NextChat.PublicURL) != "" {
		base = strings.TrimSpace(cfg.NextChat.PublicURL)
	}
	parsed, err := url.Parse(base)
	if err != nil {
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		return base + separator + "launch_token=" + url.QueryEscape(token)
	}
	query := parsed.Query()
	query.Set("launch_token", token)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func validNextChatExchangeSecret(got string, cfg *config.Config) bool {
	want := ""
	if cfg != nil {
		want = strings.TrimSpace(cfg.NextChat.ExchangeSecret)
	}
	got = strings.TrimSpace(got)
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func nextChatLaunchTokenTTL(cfg *config.Config) time.Duration {
	seconds := 120
	if cfg != nil && cfg.NextChat.LaunchTokenTTLSeconds > 0 {
		seconds = cfg.NextChat.LaunchTokenTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}

func nextChatSessionTTL(cfg *config.Config) time.Duration {
	seconds := 7 * 24 * 60 * 60
	if cfg != nil && cfg.NextChat.SessionTTLSeconds > 0 {
		seconds = cfg.NextChat.SessionTTLSeconds
	}
	return time.Duration(seconds) * time.Second
}
