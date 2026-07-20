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
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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
	calls          int
	userID         int64
	selectedGroup  int64
	switchRequests []int64
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
	groupID := s.selectedGroup
	if groupID == 0 {
		groupID = 7
	}
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
			GroupID:       &groupID,
			GroupName:     "OpenAI main",
			GroupPlatform: service.PlatformOpenAI,
		},
	}, nil
}

func (s *nextChatRouteIssuerStub) SetNextChatManagedKeyGroup(_ context.Context, userID, apiKeyID, groupID int64) (*service.NextChatWorkspaceIdentity, error) {
	s.switchRequests = append(s.switchRequests, groupID)
	s.selectedGroup = groupID
	return s.GetNextChatWorkspaceIdentity(context.Background(), userID, apiKeyID)
}

type nextChatRouteModelProviderStub struct{}

func (s nextChatRouteModelProviderStub) GetNextChatWorkspaceModels(_ context.Context, _ int64, _ int64) (*service.NextChatWorkspaceModels, error) {
	g1 := int64(7)
	g2 := int64(8)
	return &service.NextChatWorkspaceModels{
		Source:          "/v1/models",
		DefaultModel:    "gpt-4o-mini",
		SelectedGroupID: &g1,
		Groups: []service.NextChatWorkspaceGroup{
			{
				ID:        g1,
				Name:      "OpenAI main",
				Platform:  service.PlatformOpenAI,
				IsCurrent: true,
				Models: []service.NextChatWorkspaceModel{
					{ID: "gpt-4o-mini", Name: "gpt-4o-mini", DisplayName: "gpt-4o-mini"},
				},
			},
			{
				ID:       g2,
				Name:     "Grok backup",
				Platform: service.PlatformGrok,
				Models: []service.NextChatWorkspaceModel{
					{ID: "grok-4-fast", Name: "grok-4-fast", DisplayName: "grok-4-fast"},
				},
			},
		},
	}, nil
}

type nextChatRoutePromptProviderStub struct {
	list       []service.PublicPrompt
	detailByID map[int64]service.PublicPrompt
}

func (s *nextChatRoutePromptProviderStub) ListPublic(_ context.Context, filter service.PromptListFilter, _ *int64) ([]service.PublicPrompt, *pagination.PaginationResult, error) {
	pageSize := filter.Pagination.PageSize
	if pageSize <= 0 || pageSize > len(s.list) {
		pageSize = len(s.list)
	}
	return append([]service.PublicPrompt(nil), s.list[:pageSize]...), &pagination.PaginationResult{
		Total:    int64(len(s.list)),
		Page:     filter.Pagination.Page,
		PageSize: filter.Pagination.PageSize,
		Pages:    1,
	}, nil
}

func (s *nextChatRoutePromptProviderStub) GetPublic(_ context.Context, id int64, _ *int64) (*service.PublicPrompt, error) {
	prompt, ok := s.detailByID[id]
	if !ok {
		return nil, nil
	}
	return &prompt, nil
}

type nextChatRouteImageStudioStub struct {
	modelsUserID  int64
	modelsAPIKey  int64
	generateUser  int64
	generateInput service.ImageStudioGenerateRequest
	referenceTTL  time.Duration
}

func (s *nextChatRouteImageStudioStub) Models(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	apiKeyID, _ := strconv.ParseInt(c.Query("api_key_id"), 10, 64)
	s.modelsUserID = subject.UserID
	s.modelsAPIKey = apiKeyID
	response.Success(c, gin.H{
		"models": []service.ImageStudioModelOption{
			{ID: "gpt-image-1.5", DisplayName: "GPT Image 1.5"},
		},
	})
}

func (s *nextChatRouteImageStudioStub) Generate(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	s.generateUser = subject.UserID
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&s.generateInput); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	response.Success(c, gin.H{
		"job": gin.H{
			"id":          "job-nextchat",
			"api_key_id":  s.generateInput.APIKeyID,
			"retain_days": s.generateInput.RetainDays,
		},
		"async": true,
		"poll":  "/api/v1/image-studio/jobs/job-nextchat",
	})
}

func (s *nextChatRouteImageStudioStub) Estimate(c *gin.Context) { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) UploadReference(c *gin.Context) {
	s.referenceTTL = service.ImageStudioReferenceTTL(c.Request.Context())
	response.Success(c, gin.H{})
}
func (s *nextChatRouteImageStudioStub) DeleteReference(c *gin.Context) { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) ActiveJob(c *gin.Context)       { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) ListJobs(c *gin.Context) {
	response.Success(c, gin.H{
		"jobs": []gin.H{
			{
				"id": "job-nextchat",
				"assets": []gin.H{
					{
						"id":            "asset-nextchat",
						"url":           "/api/v1/image-studio/assets/asset-nextchat/content",
						"preview_url":   "/api/v1/image-studio/assets/asset-nextchat/content",
						"thumbnail_url": "/api/v1/image-studio/assets/asset-nextchat/thumbnail",
						"download_url":  "/api/v1/image-studio/assets/asset-nextchat/download",
					},
				},
			},
		},
	})
}
func (s *nextChatRouteImageStudioStub) GetJob(c *gin.Context)         { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) JobDownload(c *gin.Context)    { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) CancelJob(c *gin.Context)      { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) DeleteJob(c *gin.Context)      { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) AssetThumbnail(c *gin.Context) { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) AssetContent(c *gin.Context)   { response.Success(c, gin.H{}) }
func (s *nextChatRouteImageStudioStub) AssetDownload(c *gin.Context)  { response.Success(c, gin.H{}) }

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
	Models service.NextChatWorkspaceModels `json:"models"`
	URLs   struct {
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
	return newNextChatRouteTestRouterWithImageStudio(t, gate, issuer, nil, cfg, rdb)
}

func newNextChatRouteTestRouterWithImageStudio(
	t *testing.T,
	gate nextChatRouteGateStub,
	issuer nextChatSessionIssuer,
	imageStudio nextChatImageStudioBFFHandler,
	cfg *config.Config,
	rdb *redis.Client,
) *gin.Engine {
	return newNextChatRouteTestRouterWithPromptProvider(t, gate, issuer, nil, imageStudio, cfg, rdb)
}

func newNextChatRouteTestRouterWithPromptProvider(
	t *testing.T,
	gate nextChatRouteGateStub,
	issuer nextChatSessionIssuer,
	promptProvider nextChatPromptProvider,
	imageStudio nextChatImageStudioBFFHandler,
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
	registerNextChatRoutes(v1, auth, issuer, nextChatRouteModelProviderStub{}, promptProvider, imageStudio, gate, cfg, rdb)
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

func postNextChatBFF(router *gin.Engine, path, secret string, userID, apiKeyID int64, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	require.Len(t, got.Models.Groups, 2)
	require.Equal(t, int64(7), got.Models.Groups[0].ID)
	require.Equal(t, "gpt-4o-mini", got.Models.Groups[0].Models[0].Name)
	require.Equal(t, int64(8), got.Models.Groups[1].ID)
	require.Equal(t, "grok-4-fast", got.Models.Groups[1].Models[0].Name)
	require.Equal(t, "https://www.jisudeng.com/dashboard", got.URLs.ReturnURL)
	require.Equal(t, "https://www.jisudeng.com/purchase", got.URLs.RechargeURL)
	require.Equal(t, 7, got.Retention.TextSessionDays)
	require.Equal(t, 24, got.Retention.ImageAssetHours)
	require.False(t, got.Retention.ServerChatLog)
	require.NotContains(t, recorder.Body.String(), "sk-managed-nextchat")
	require.NotContains(t, recorder.Body.String(), `"api_key"`)
}

func TestNextChatBootstrapDefaultsReturnAndRechargeToConsolePages(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{
		enabled:     true,
		frontendURL: "https://www.jisudeng.com/",
		settings: &service.PublicSettings{
			SiteName:        "极速蹬",
			NextChatEnabled: true,
		},
	}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/bootstrap", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[nextChatBootstrapResponse](t, recorder)
	require.Equal(t, "https://www.jisudeng.com/dashboard", got.URLs.ReturnURL)
	require.Equal(t, "https://www.jisudeng.com/purchase", got.URLs.RechargeURL)
	require.Equal(t, "https://www.jisudeng.com/profile", got.URLs.ProfileURL)
}

func TestNextChatBootstrapIgnoresLegacyRechargeSetting(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{
		enabled:     true,
		frontendURL: "https://www.jisudeng.com",
		settings: &service.PublicSettings{
			SiteName:                    "极速蹬",
			NextChatEnabled:             true,
			BalanceLowNotifyRechargeURL: "https://jisuodeng.zeabur.app",
			PurchaseSubscriptionURL:     "https://www.jisudeng.com/payment",
		},
	}, &nextChatRouteIssuerStub{}, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/bootstrap", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[nextChatBootstrapResponse](t, recorder)
	require.Equal(t, "https://www.jisudeng.com/dashboard", got.URLs.ReturnURL)
	require.Equal(t, "https://www.jisudeng.com/purchase", got.URLs.RechargeURL)
}

func TestNextChatGroupSwitchUpdatesManagedKeyGroup(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	issuer := &nextChatRouteIssuerStub{}
	router := newNextChatRouteTestRouter(t, nextChatRouteGateStub{enabled: true}, issuer, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := postNextChatBFF(router, "/api/v1/nextchat/group", "server-secret", 42, 123, `{"group_id":8}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[struct {
		ManagedAPIKey service.NextChatWorkspaceAPIKey `json:"managed_api_key"`
		Models        service.NextChatWorkspaceModels `json:"models"`
	}](t, recorder)
	require.Equal(t, []int64{8}, issuer.switchRequests)
	require.NotNil(t, got.ManagedAPIKey.GroupID)
	require.Equal(t, int64(8), *got.ManagedAPIKey.GroupID)
	require.Equal(t, "/v1/models", got.Models.Source)
	require.NotContains(t, recorder.Body.String(), "sk-managed-nextchat")
}

func TestNextChatImageStudioModelsUsesBFFIdentity(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	imageStudio := &nextChatRouteImageStudioStub{}
	router := newNextChatRouteTestRouterWithImageStudio(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, imageStudio, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/image-studio/models", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), imageStudio.modelsUserID)
	require.Equal(t, int64(123), imageStudio.modelsAPIKey)
	require.Contains(t, recorder.Body.String(), "gpt-image-1.5")
}

func TestNextChatImageStudioGenerateForcesManagedKeyAndOneDayRetention(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	imageStudio := &nextChatRouteImageStudioStub{}
	router := newNextChatRouteTestRouterWithImageStudio(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, imageStudio, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := postNextChatBFF(router, "/api/v1/nextchat/image-studio/generate", "server-secret", 42, 123, `{
		"template_id":"free-create",
		"user_prompt":"draw a clean product photo",
		"api_key_id":999,
		"retain_days":7,
		"count":1
	}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, int64(42), imageStudio.generateUser)
	require.Equal(t, int64(123), imageStudio.generateInput.APIKeyID)
	require.NotNil(t, imageStudio.generateInput.RetainDays)
	require.Equal(t, 1, *imageStudio.generateInput.RetainDays)
	require.NotContains(t, recorder.Body.String(), "999")
	require.Contains(t, recorder.Body.String(), "/api/v1/nextchat/image-studio/jobs/job-nextchat")
	require.NotContains(t, recorder.Body.String(), "/api/v1/image-studio/jobs/job-nextchat")
}

func TestNextChatImageStudioReferenceUploadForcesOneDayRetention(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	imageStudio := &nextChatRouteImageStudioStub{}
	router := newNextChatRouteTestRouterWithImageStudio(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, imageStudio, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := postNextChatBFF(router, "/api/v1/nextchat/image-studio/references", "server-secret", 42, 123, `{}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, 24*time.Hour, imageStudio.referenceTTL)
}

func TestNextChatImageStudioRewritesNestedAssetURLs(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	imageStudio := &nextChatRouteImageStudioStub{}
	router := newNextChatRouteTestRouterWithImageStudio(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, imageStudio, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/image-studio/jobs", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "/api/v1/nextchat/image-studio/assets/asset-nextchat/content")
	require.Contains(t, recorder.Body.String(), "/api/v1/nextchat/image-studio/assets/asset-nextchat/thumbnail")
	require.Contains(t, recorder.Body.String(), "/api/v1/nextchat/image-studio/assets/asset-nextchat/download")
	require.NotContains(t, recorder.Body.String(), "/api/v1/image-studio/assets/asset-nextchat")
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

func TestNextChatPromptsUsePublicPromptLibraryWhenAvailable(t *testing.T) {
	_, rdb := newNextChatRouteRedis(t)
	promptProvider := &nextChatRoutePromptProviderStub{
		list: []service.PublicPrompt{
			{
				ID:          88,
				Title:       "爆款短视频脚本",
				Description: "把商品卖点改写成短视频脚本",
				Purpose:     "marketing",
				Version:     3,
			},
		},
		detailByID: map[int64]service.PublicPrompt{
			88: {
				ID:          88,
				Title:       "爆款短视频脚本",
				Description: "把商品卖点改写成短视频脚本",
				Purpose:     "marketing",
				Version:     3,
				PromptText:  "请根据商品卖点输出 30 秒短视频脚本。",
			},
		},
	}
	router := newNextChatRouteTestRouterWithPromptProvider(t, nextChatRouteGateStub{enabled: true}, &nextChatRouteIssuerStub{}, promptProvider, nil, &config.Config{
		NextChat: config.NextChatConfig{ExchangeSecret: "server-secret"},
	}, rdb)

	recorder := getNextChatBFF(router, "/api/v1/nextchat/prompts", "server-secret", 42, 123)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	got := decodeNextChatRouteResponse[nextChatPromptsResponse](t, recorder)
	require.Equal(t, []service.NextChatPrompt{
		{
			ID:          "prompt-88-v3",
			Title:       "爆款短视频脚本",
			Description: "把商品卖点改写成短视频脚本",
			Content:     "请根据商品卖点输出 30 秒短视频脚本。",
			Category:    "marketing",
		},
	}, got.ChatPrompts)
	require.NotEmpty(t, got.ImageTemplates.Intents)
	require.NotContains(t, recorder.Body.String(), "通用助手")
}
