package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type nextChatRouteGateStub struct {
	enabled bool
}

func (s nextChatRouteGateStub) IsNextChatEnabled(context.Context) bool {
	return s.enabled
}

type nextChatRouteIssuerStub struct {
	calls  int
	userID int64
}

func (s *nextChatRouteIssuerStub) IssueNextChatManagedSession(_ context.Context, userID int64) (*service.NextChatManagedSession, error) {
	s.calls++
	s.userID = userID
	return &service.NextChatManagedSession{
		UserID: userID,
		APIKey: "sk-managed-nextchat",
		KeyID:  123,
	}, nil
}

type nextChatRouteEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type nextChatLaunchResponse struct {
	LaunchURL  string `json:"launch_url"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type nextChatSessionResponse struct {
	UserID int64  `json:"user_id"`
	APIKey string `json:"api_key"`
	KeyID  int64  `json:"api_key_id"`
}

func newNextChatRouteRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
	})
	return mr, rdb
}

func newNextChatRouteTestRouter(
	t *testing.T,
	gate nextChatRouteGateStub,
	issuer nextChatSessionIssuer,
	cfg *config.Config,
	rdb *redis.Client,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")
	auth := middleware.JWTAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "Bearer valid-user" {
			response.Unauthorized(c, "User not authenticated")
			c.Abort()
			return
		}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42, Concurrency: 1})
		c.Next()
	})
	registerNextChatRoutes(v1, auth, issuer, gate, cfg, rdb)
	return router
}

func decodeNextChatRouteResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()
	var envelope nextChatRouteEnvelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, 0, envelope.Code, "body=%s", recorder.Body.String())
	var data T
	require.NoError(t, json.Unmarshal(envelope.Data, &data))
	return data
}

func extractNextChatLaunchToken(t *testing.T, launchURL string) string {
	t.Helper()
	parsed, err := url.Parse(launchURL)
	require.NoError(t, err)
	token := parsed.Query().Get("launch_token")
	require.NotEmpty(t, token)
	return token
}

func postNextChatLaunch(router *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nextchat/launch", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func postNextChatSession(router *gin.Engine, secret, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	body := `{"launch_token":"` + token + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nextchat/session", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-NextChat-Secret", secret)
	}
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestNextChatLaunchRequiresJWT(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, &config.Config{}, rdb)

	recorder := postNextChatLaunch(router, "")

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "User not authenticated")
}

func TestNextChatLaunchDisabledReturnsNotFound(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: false}, &nextChatRouteIssuerStub{}, &config.Config{}, rdb)

	recorder := postNextChatLaunch(router, "Bearer valid-user")

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "NextChat is disabled")
}

func TestNextChatSessionRequiresExchangeSecret(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	launch := decodeNextChatRouteResponse[nextChatLaunchResponse](t, postNextChatLaunch(router, "Bearer valid-user"))
	token := extractNextChatLaunchToken(t, launch.LaunchURL)

	recorder := postNextChatSession(router, "wrong-secret", token)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Invalid NextChat exchange secret")
}

func TestNextChatLaunchTokenIsConsumedOnce(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	issuer := &nextChatRouteIssuerStub{}
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, issuer, &config.Config{
		NextChat: config.NextChatConfig{
			PublicURL:      "https://chat.example.com/ai",
			ExchangeSecret: "server-secret",
		},
	}, rdb)

	launch := decodeNextChatRouteResponse[nextChatLaunchResponse](t, postNextChatLaunch(router, "Bearer valid-user"))
	require.Equal(t, 120, launch.TTLSeconds)
	token := extractNextChatLaunchToken(t, launch.LaunchURL)

	first := postNextChatSession(router, "server-secret", token)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	session := decodeNextChatRouteResponse[nextChatSessionResponse](t, first)
	require.Equal(t, int64(42), session.UserID)
	require.Equal(t, "sk-managed-nextchat", session.APIKey)
	require.Equal(t, int64(123), session.KeyID)

	second := postNextChatSession(router, "server-secret", token)
	require.Equal(t, http.StatusUnauthorized, second.Code)
	require.Contains(t, second.Body.String(), "Invalid or consumed launch token")
	require.Equal(t, 1, issuer.calls)
	require.Equal(t, int64(42), issuer.userID)
}

func TestNextChatLaunchTokenExpires(t *testing.T) {
	mr, rdb := newNextChatRouteRedis(t)
	issuer := &nextChatRouteIssuerStub{}
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, issuer, &config.Config{
		NextChat: config.NextChatConfig{
			LaunchTokenTTLSeconds: 1,
			ExchangeSecret:        "server-secret",
		},
	}, rdb)

	launch := decodeNextChatRouteResponse[nextChatLaunchResponse](t, postNextChatLaunch(router, "Bearer valid-user"))
	require.Equal(t, 1, launch.TTLSeconds)
	token := extractNextChatLaunchToken(t, launch.LaunchURL)
	mr.FastForward(2 * time.Second)

	recorder := postNextChatSession(router, "server-secret", token)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Equal(t, 0, issuer.calls)
}
