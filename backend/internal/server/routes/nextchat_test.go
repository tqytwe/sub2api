package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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
	enabled     bool
	settings    *service.PublicSettings
	frontendURL string
}

func (s nextChatRouteGateStub) IsNextChatEnabled(context.Context) bool {
	return s.enabled
}

func (s nextChatRouteGateStub) GetPublicSettings(context.Context) (*service.PublicSettings, error) {
	if s.settings != nil {
		return s.settings, nil
	}
	return &service.PublicSettings{SiteName: "极速蹬", NextChatEnabled: s.enabled}, nil
}

func (s nextChatRouteGateStub) GetFrontendURL(context.Context) string {
	return s.frontendURL
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

func (s *nextChatRouteIssuerStub) GetNextChatWorkspaceIdentity(_ context.Context, userID, apiKeyID int64) (*service.NextChatWorkspaceIdentity, error) {
	return &service.NextChatWorkspaceIdentity{
		User: service.NextChatWorkspaceUser{
			ID:       userID,
			Username: "tester",
			Email:    "tester@example.com",
			Balance:  12.5,
		},
		APIKey: service.NextChatWorkspaceAPIKey{
			ID:            apiKeyID,
			Name:          service.NextChatManagedAPIKeyName,
			GroupName:     "OpenAI main",
			GroupPlatform: service.PlatformOpenAI,
		},
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

type nextChatBootstrapResponse struct {
	User          service.NextChatWorkspaceUser   `json:"user"`
	ManagedAPIKey service.NextChatWorkspaceAPIKey `json:"managed_api_key"`
	Brand         struct {
		SiteName      string `json:"site_name"`
		WorkspaceName string `json:"workspace_name"`
	} `json:"brand"`
	Features struct {
		Chat          bool `json:"chat"`
		ImageStudio   bool `json:"image_studio"`
		CloudSync     bool `json:"cloud_sync"`
		HistoryExport bool `json:"history_export"`
	} `json:"features"`
	Models struct {
		Source string `json:"source"`
	} `json:"models"`
	URLs struct {
		ReturnURL   string `json:"return_url"`
		RechargeURL string `json:"recharge_url"`
		ProfileURL  string `json:"profile_url"`
	} `json:"urls"`
	Retention struct {
		TextSessionDays int  `json:"text_session_days"`
		ImageAssetHours int  `json:"image_asset_hours"`
		ServerChatLog   bool `json:"server_chat_log"`
	} `json:"retention"`
}

type nextChatPromptsResponse struct {
	ChatPrompts    []service.NextChatPrompt   `json:"chat_prompts"`
	ImageTemplates service.ImageStudioCatalog `json:"image_templates"`
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

func getNextChatBFF(router *gin.Engine, path, secret string, userID, apiKeyID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if secret != "" {
		req.Header.Set("X-NextChat-Secret", secret)
	}
	if userID > 0 {
		req.Header.Set("X-NextChat-User-ID", strconv.FormatInt(userID, 10))
	}
	if apiKeyID > 0 {
		req.Header.Set("X-NextChat-API-Key-ID", strconv.FormatInt(apiKeyID, 10))
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

func TestNextChatBootstrapRequiresBFFSecret(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/bootstrap", "", 42, 123)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "Invalid NextChat exchange secret")
}

func TestNextChatBootstrapReturnsWorkspaceStateWithoutAPIKeySecret(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{
		enabled:     true,
		frontendURL: "https://www.jisudeng.com",
		settings: &service.PublicSettings{
			SiteName:                    "极速蹬",
			SiteLogo:                    "/logo.png",
			NextChatEnabled:             true,
			ImageStudioEnabled:          true,
			BalanceLowNotifyRechargeURL: "https://www.jisudeng.com/payment",
		},
	}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/bootstrap", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[nextChatBootstrapResponse](t, recorder)
	require.Equal(t, int64(42), got.User.ID)
	require.Equal(t, "tester", got.User.Username)
	require.Equal(t, int64(123), got.ManagedAPIKey.ID)
	require.Equal(t, service.NextChatManagedAPIKeyName, got.ManagedAPIKey.Name)
	require.Equal(t, "极速蹬", got.Brand.SiteName)
	require.Equal(t, "极速蹬 AI 工作台", got.Brand.WorkspaceName)
	require.True(t, got.Features.Chat)
	require.True(t, got.Features.ImageStudio)
	require.False(t, got.Features.CloudSync)
	require.Equal(t, "/v1/models", got.Models.Source)
	require.Equal(t, "https://www.jisudeng.com", got.URLs.ReturnURL)
	require.Equal(t, "https://www.jisudeng.com/payment", got.URLs.RechargeURL)
	require.Equal(t, 7, got.Retention.TextSessionDays)
	require.Equal(t, 24, got.Retention.ImageAssetHours)
	require.False(t, got.Retention.ServerChatLog)
	require.NotContains(t, recorder.Body.String(), "sk-managed-nextchat")
	require.NotContains(t, recorder.Body.String(), `"api_key"`)
}

func TestNextChatPromptsHideInternalImagePromptTemplate(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/prompts", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[nextChatPromptsResponse](t, recorder)
	require.NotEmpty(t, got.ChatPrompts)
	require.NotEmpty(t, got.ImageTemplates.Intents)
	require.Contains(t, recorder.Body.String(), "ecom-white-bg")
	require.NotContains(t, recorder.Body.String(), "Professional product photo")
	require.NotContains(t, recorder.Body.String(), "prompt_template")
}
