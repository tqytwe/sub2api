package routes

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const nextChatLaunchTokenKeyPrefix = "nextchat:launch:"

const (
	nextChatImageStudioInternalAPIPrefix = "/api/v1/image-studio/"
	nextChatImageStudioBFFAPIPrefix      = "/api/v1/nextchat/image-studio/"
	nextChatImageStudioAssetRetention    = 24 * time.Hour
)

type nextChatSessionIssuer interface {
	IssueNextChatManagedSession(ctx context.Context, userID int64) (*service.NextChatManagedSession, error)
}

type nextChatScopedSessionIssuer interface {
	IssueNextChatManagedSessions(ctx context.Context, userID int64) (*service.NextChatManagedSessions, error)
	IssueNextChatManagedSessionForPurpose(ctx context.Context, userID int64, purpose string) (*service.NextChatManagedSession, error)
	SetNextChatManagedSessionGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*service.NextChatWorkspaceIdentity, error)
}

// nextChatImmutableScopedSessionSwitcher is implemented by the production
// issuer. It lets a trusted Canvas BFF replace a purpose/group session with a
// new immutable key instead of mutating the key that another request may be
// using concurrently.
type nextChatImmutableScopedSessionSwitcher interface {
	SwitchNextChatManagedSessionGroup(ctx context.Context, userID, currentKeyID int64, purpose string, groupID int64) (*service.NextChatManagedSession, *service.NextChatWorkspaceIdentity, error)
}

// nextChatGroupPinnedSessionIssuer issues a fresh purpose/group-bound session
// for first-party mobile clients. Mobile authentication already establishes
// the user, while Canvas additionally presents the current key ID and uses the
// stricter switcher above.
type nextChatGroupPinnedSessionIssuer interface {
	IssueNextChatManagedSessionForPurposeAndGroup(ctx context.Context, userID int64, purpose string, groupID int64) (*service.NextChatManagedSession, error)
}

type nextChatFeatureGate interface {
	IsNextChatEnabled(ctx context.Context) bool
}

type nextChatWorkspaceIdentityProvider interface {
	GetNextChatWorkspaceIdentity(ctx context.Context, userID, apiKeyID int64) (*service.NextChatWorkspaceIdentity, error)
	SetNextChatManagedKeyGroup(ctx context.Context, userID, apiKeyID, groupID int64) (*service.NextChatWorkspaceIdentity, error)
}

type nextChatWorkspaceModelProvider interface {
	GetNextChatWorkspaceModels(ctx context.Context, userID, apiKeyID int64) (*service.NextChatWorkspaceModels, error)
}

type nextChatPromptProvider interface {
	ListPublic(ctx context.Context, filter service.PromptListFilter, userID *int64) ([]service.PublicPrompt, *pagination.PaginationResult, error)
	GetPublic(ctx context.Context, id int64, userID *int64) (*service.PublicPrompt, error)
	SetFavorite(ctx context.Context, promptID, userID int64, favorite bool) (bool, error)
	UsePrompt(ctx context.Context, promptID, userID int64) (*service.PromptUseResult, error)
}

type nextChatImageStudioBFFHandler interface {
	Models(c *gin.Context)
	Estimate(c *gin.Context)
	Generate(c *gin.Context)
	UploadReference(c *gin.Context)
	DeleteReference(c *gin.Context)
	ActiveJob(c *gin.Context)
	ListJobs(c *gin.Context)
	GetJob(c *gin.Context)
	JobDownload(c *gin.Context)
	CancelJob(c *gin.Context)
	DeleteJob(c *gin.Context)
	AssetThumbnail(c *gin.Context)
	AssetContent(c *gin.Context)
	AssetDownload(c *gin.Context)
}

type nextChatPublicSettingsProvider interface {
	GetPublicSettings(ctx context.Context) (*service.PublicSettings, error)
	GetFrontendURL(ctx context.Context) string
}

type nextChatLaunchTokenRecord struct {
	UserID     int64     `json:"user_id"`
	IssuedAt   time.Time `json:"issued_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	RemoteAddr string    `json:"remote_addr,omitempty"`
}

type nextChatLaunchIntent struct {
	Type          string `json:"type"`
	PromptID      int64  `json:"prompt_id"`
	PromptVersion int    `json:"prompt_version,omitempty"`
}

type nextChatLaunchRequest struct {
	Intent *nextChatLaunchIntent `json:"intent"`
}

type nextChatExchangeRequest struct {
	LaunchToken string `json:"launch_token"`
}

type nextChatGroupSwitchRequest struct {
	GroupID int64 `json:"group_id"`
}

func RegisterNextChatRoutes(
	v1 *gin.RouterGroup,
	jwtAuth middleware.JWTAuthMiddleware,
	apiKeyService *service.APIKeyService,
	modelCatalogService *service.ModelCatalogService,
	promptProvider nextChatPromptProvider,
	imageStudio nextChatImageStudioBFFHandler,
	settingService *service.SettingService,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	registerNextChatRoutes(v1, jwtAuth, apiKeyService, modelCatalogService, promptProvider, imageStudio, settingService, cfg, redisClient)
}

func RegisterMobileNextChatSessionRoutes(
	v1 *gin.RouterGroup,
	jwtAuth middleware.JWTAuthMiddleware,
	apiKeyService *service.APIKeyService,
	modelCatalogService *service.ModelCatalogService,
	settingService *service.SettingService,
	cfg *config.Config,
) {
	mobile := v1.Group("/mobile")
	mobile.Use(gin.HandlerFunc(jwtAuth))
	{
		mobile.GET("/sessions", func(c *gin.Context) {
			handleNextChatMobileBootstrap(c, apiKeyService, modelCatalogService, settingService, cfg)
		})
		mobile.POST("/sessions/:purpose/switch-group", func(c *gin.Context) {
			handleNextChatMobileGroupSwitch(c, apiKeyService, modelCatalogService, settingService, cfg, c.Param("purpose"))
		})
	}
}

func registerNextChatRoutes(
	v1 *gin.RouterGroup,
	jwtAuth middleware.JWTAuthMiddleware,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	promptProvider nextChatPromptProvider,
	imageStudio nextChatImageStudioBFFHandler,
	gate nextChatFeatureGate,
	cfg *config.Config,
	redisClient *redis.Client,
) {
	nextchat := v1.Group("/nextchat")
	{
		nextchat.POST("/session", func(c *gin.Context) {
			handleNextChatSessionExchange(c, issuer, gate, cfg, redisClient)
		})
		nextchat.POST("/identity", func(c *gin.Context) {
			handleNextChatIdentityExchange(c, gate, cfg, redisClient)
		})
		nextchat.GET("/bootstrap", func(c *gin.Context) {
			handleNextChatBootstrap(c, issuer, modelProvider, gate, cfg)
		})
		nextchat.GET("/prompts", func(c *gin.Context) {
			handleNextChatPrompts(c, promptProvider, gate, cfg)
		})
		nextchat.GET("/image-prompts", func(c *gin.Context) {
			handleNextChatImagePrompts(c, promptProvider, gate, cfg)
		})
		nextchat.GET("/image-prompts/:id", func(c *gin.Context) {
			handleNextChatImagePrompt(c, promptProvider, gate, cfg)
		})
		nextchat.POST("/image-prompts/:id/favorite", func(c *gin.Context) {
			handleNextChatImagePromptFavorite(c, promptProvider, gate, cfg, true)
		})
		nextchat.DELETE("/image-prompts/:id/favorite", func(c *gin.Context) {
			handleNextChatImagePromptFavorite(c, promptProvider, gate, cfg, false)
		})
		nextchat.POST("/image-prompts/:id/use", func(c *gin.Context) {
			handleNextChatImagePromptUse(c, promptProvider, gate, cfg)
		})
		nextchat.POST("/group", func(c *gin.Context) {
			handleNextChatGroupSwitch(c, issuer, modelProvider, gate, cfg)
		})
		nextchat.POST("/sessions/:purpose/group", func(c *gin.Context) {
			handleNextChatScopedGroupSwitch(c, issuer, modelProvider, gate, cfg, c.Param("purpose"))
		})
		registerNextChatImageStudioRoutes(nextchat, imageStudio, gate, cfg)
	}

	authenticated := nextchat.Group("")
	authenticated.Use(gin.HandlerFunc(jwtAuth))
	{
		authenticated.POST("/launch", func(c *gin.Context) {
			handleNextChatLaunch(c, gate, cfg, redisClient)
		})
		authenticated.GET("/mobile/bootstrap", func(c *gin.Context) {
			handleNextChatMobileBootstrap(c, issuer, modelProvider, gate, cfg)
		})
		authenticated.POST("/mobile/group", func(c *gin.Context) {
			handleNextChatMobileGroupSwitch(c, issuer, modelProvider, gate, cfg, service.NextChatSessionPurposeChat)
		})
		authenticated.POST("/mobile/sessions/:purpose/group", func(c *gin.Context) {
			handleNextChatMobileGroupSwitch(c, issuer, modelProvider, gate, cfg, c.Param("purpose"))
		})
	}
}

func registerNextChatImageStudioRoutes(
	nextchat *gin.RouterGroup,
	imageStudio nextChatImageStudioBFFHandler,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if imageStudio == nil {
		return
	}
	studio := nextchat.Group("/image-studio")
	{
		studio.GET("/models", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, true, true, imageStudio.Models)
		})
		studio.GET("/estimate", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, true, true, imageStudio.Estimate)
		})
		studio.POST(
			"/generate",
			middleware.RequestBodyLimit(handler.ImageStudioGenerateRequestBodyLimit),
			func(c *gin.Context) {
				handleNextChatImageStudioGenerate(c, imageStudio, gate, cfg)
			},
		)
		studio.POST(
			"/references",
			middleware.RequestBodyLimit(handler.ImageStudioReferenceRequestBodyLimit),
			func(c *gin.Context) {
				handleNextChatImageStudioUploadReference(c, imageStudio, gate, cfg)
			},
		)
		studio.DELETE("/references/:id", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.DeleteReference)
		})
		studio.GET("/jobs/active", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.ActiveJob)
		})
		studio.GET("/jobs", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.ListJobs)
		})
		studio.GET("/jobs/:id", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.GetJob)
		})
		studio.GET("/jobs/:id/download", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, false, imageStudio.JobDownload)
		})
		studio.POST("/jobs/:id/cancel", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.CancelJob)
		})
		studio.DELETE("/jobs/:id", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, true, imageStudio.DeleteJob)
		})
		studio.GET("/assets/:id/thumbnail", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, false, imageStudio.AssetThumbnail)
		})
		studio.GET("/assets/:id/content", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, false, imageStudio.AssetContent)
		})
		studio.GET("/assets/:id/download", func(c *gin.Context) {
			handleNextChatImageStudio(c, imageStudio, gate, cfg, false, false, imageStudio.AssetDownload)
		})
	}
}

func handleNextChatImageStudio(
	c *gin.Context,
	_ nextChatImageStudioBFFHandler,
	gate nextChatFeatureGate,
	cfg *config.Config,
	forceAPIKeyQuery bool,
	rewriteJSON bool,
	handle func(*gin.Context),
) {
	if !prepareNextChatImageStudioBFF(c, gate, cfg, forceAPIKeyQuery) {
		return
	}
	if rewriteJSON {
		rewriteNextChatImageStudioJSONResponse(c, handle)
		return
	}
	handle(c)
}

func handleNextChatImageStudioGenerate(
	c *gin.Context,
	imageStudio nextChatImageStudioBFFHandler,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	_, apiKeyID, ok := prepareNextChatImageStudioBFFSession(c, gate, cfg)
	if !ok {
		return
	}
	var req service.ImageStudioGenerateRequest
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	req.APIKeyID = apiKeyID
	req.RetainDays = nil
	raw, err := json.Marshal(req)
	if err != nil {
		response.InternalError(c, "Failed to prepare image studio request")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	c.Request.ContentLength = int64(len(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	rewriteNextChatImageStudioJSONResponse(c, imageStudio.Generate)
}

func handleNextChatImageStudioUploadReference(
	c *gin.Context,
	imageStudio nextChatImageStudioBFFHandler,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if !prepareNextChatImageStudioBFF(c, gate, cfg, false) {
		return
	}
	c.Request = c.Request.WithContext(
		service.WithImageStudioReferenceTTL(c.Request.Context(), nextChatImageStudioAssetRetention),
	)
	rewriteNextChatImageStudioJSONResponse(c, imageStudio.UploadReference)
}

type nextChatImageStudioJSONRewriteWriter struct {
	gin.ResponseWriter
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func (w *nextChatImageStudioJSONRewriteWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
}

func (w *nextChatImageStudioJSONRewriteWriter) WriteHeaderNow() {
	if w.wroteHeader {
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (w *nextChatImageStudioJSONRewriteWriter) Write(data []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(data)
}

func (w *nextChatImageStudioJSONRewriteWriter) WriteString(data string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(data)
}

func (w *nextChatImageStudioJSONRewriteWriter) Status() int {
	if w.status != 0 {
		return w.status
	}
	return http.StatusOK
}

func (w *nextChatImageStudioJSONRewriteWriter) Size() int {
	return w.body.Len()
}

func (w *nextChatImageStudioJSONRewriteWriter) Written() bool {
	return w.wroteHeader
}

func rewriteNextChatImageStudioJSONResponse(c *gin.Context, handle func(*gin.Context)) {
	if c == nil || handle == nil {
		return
	}
	originalWriter := c.Writer
	recorder := &nextChatImageStudioJSONRewriteWriter{ResponseWriter: originalWriter}
	c.Writer = recorder
	handle(c)
	c.Writer = originalWriter

	if !recorder.Written() {
		return
	}
	body := recorder.body.Bytes()
	if shouldRewriteNextChatImageStudioJSON(originalWriter.Header().Get("Content-Type"), body) {
		body = []byte(strings.ReplaceAll(
			string(body),
			nextChatImageStudioInternalAPIPrefix,
			nextChatImageStudioBFFAPIPrefix,
		))
	}
	originalWriter.WriteHeader(recorder.Status())
	if len(body) > 0 {
		_, _ = originalWriter.Write(body)
	}
}

func shouldRewriteNextChatImageStudioJSON(contentType string, body []byte) bool {
	if len(body) == 0 || !strings.Contains(string(body), nextChatImageStudioInternalAPIPrefix) {
		return false
	}
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	return json.Valid(body)
}

func prepareNextChatImageStudioBFF(
	c *gin.Context,
	gate nextChatFeatureGate,
	cfg *config.Config,
	forceAPIKeyQuery bool,
) bool {
	_, apiKeyID, ok := prepareNextChatImageStudioBFFSession(c, gate, cfg)
	if !ok {
		return false
	}
	if forceAPIKeyQuery {
		query := c.Request.URL.Query()
		query.Set("api_key_id", strconv.FormatInt(apiKeyID, 10))
		c.Request.URL.RawQuery = query.Encode()
	}
	return true
}

func prepareNextChatImageStudioBFFSession(
	c *gin.Context,
	gate nextChatFeatureGate,
	cfg *config.Config,
) (int64, int64, bool) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return 0, 0, false
	}
	userID, apiKeyID, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return 0, 0, false
	}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{
		UserID:      userID,
		Concurrency: 1,
	})
	return userID, apiKeyID, true
}

func handleNextChatBootstrap(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, apiKeyID, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}

	payload, err := buildNextChatBootstrapPayload(c.Request.Context(), issuer, modelProvider, gate, userID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, payload)
}

func handleNextChatMobileBootstrap(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	if issuer == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat session issuer is unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	payload, err := buildNextChatMobileBootstrapPayload(c.Request.Context(), issuer, modelProvider, gate, cfg, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, payload)
}

// buildNextChatMobileBootstrapPayload is the complete mobile session contract.
// Scoped group switching returns this shape as well, so clients never mistake a
// single workspace's models for the full chat/image/video bootstrap state.
func buildNextChatMobileBootstrapPayload(
	ctx context.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
	userID int64,
) (gin.H, error) {
	scoped, scopedOK := issuer.(nextChatScopedSessionIssuer)
	var sessions *service.NextChatManagedSessions
	var session *service.NextChatManagedSession
	var err error
	if scopedOK {
		sessions, err = scoped.IssueNextChatManagedSessions(ctx, userID)
		if err != nil {
			return nil, err
		}
		session = &sessions.Chat
	} else {
		session, err = issuer.IssueNextChatManagedSession(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	payload, err := buildNextChatBootstrapPayload(ctx, issuer, modelProvider, gate, session.UserID, session.KeyID)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(nextChatSessionTTL(cfg))
	payload["session"] = nextChatSessionPayload(session, expiresAt)
	if !scopedOK {
		return payload, nil
	}
	payload["session"] = nextChatSessionPayload(&sessions.Chat, expiresAt)
	payload["sessions"] = gin.H{
		service.NextChatSessionPurposeChat:  nextChatSessionPayload(&sessions.Chat, expiresAt),
		service.NextChatSessionPurposeImage: nextChatSessionPayload(&sessions.Image, expiresAt),
		service.NextChatSessionPurposeVideo: nextChatSessionPayload(&sessions.Video, expiresAt),
	}
	imagePayload, err := buildNextChatBootstrapPayload(ctx, issuer, modelProvider, gate, sessions.Image.UserID, sessions.Image.KeyID)
	if err != nil {
		return nil, err
	}
	videoPayload, err := buildNextChatBootstrapPayload(ctx, issuer, modelProvider, gate, sessions.Video.UserID, sessions.Video.KeyID)
	if err != nil {
		return nil, err
	}
	payload["managed_api_keys"] = gin.H{
		service.NextChatSessionPurposeChat:  payload["managed_api_key"],
		service.NextChatSessionPurposeImage: imagePayload["managed_api_key"],
		service.NextChatSessionPurposeVideo: videoPayload["managed_api_key"],
	}
	payload["workspaces"] = gin.H{
		service.NextChatSessionPurposeChat:  gin.H{"models": payload["models"]},
		service.NextChatSessionPurposeImage: gin.H{"models": imagePayload["models"]},
		service.NextChatSessionPurposeVideo: gin.H{"models": videoPayload["models"]},
	}
	return payload, nil
}

func handleNextChatMobileGroupSwitch(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
	purpose string,
) {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	switch purpose {
	case service.NextChatSessionPurposeChat, service.NextChatSessionPurposeImage, service.NextChatSessionPurposeVideo:
	default:
		response.ErrorFrom(c, infraerrors.BadRequest("NEXTCHAT_INVALID_SESSION_PURPOSE", "session purpose must be chat, image, or video"))
		return
	}
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	if issuer == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat session issuer is unavailable")
		return
	}
	identityProvider, ok := issuer.(nextChatWorkspaceIdentityProvider)
	if !ok || identityProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat group switch service is unavailable")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req nextChatGroupSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID <= 0 {
		response.BadRequest(c, "group_id is required")
		return
	}
	var (
		err              error
		replacement      *service.NextChatManagedSession
		groupPinnedReply bool
	)
	if pinned, pinnedOK := issuer.(nextChatGroupPinnedSessionIssuer); pinnedOK && pinned != nil {
		replacement, err = pinned.IssueNextChatManagedSessionForPurposeAndGroup(c.Request.Context(), subject.UserID, purpose, req.GroupID)
		groupPinnedReply = err == nil
	} else if scoped, scopedOK := issuer.(nextChatScopedSessionIssuer); scopedOK {
		_, err = scoped.SetNextChatManagedSessionGroup(c.Request.Context(), subject.UserID, purpose, req.GroupID)
	} else if purpose != service.NextChatSessionPurposeChat {
		// A legacy issuer can only produce the chat key. Never let image/video
		// group switching silently rebind that key, because a subsequent worker
		// could execute against the wrong purpose or group.
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("NEXTCHAT_PURPOSE_SESSION_UNAVAILABLE", "scoped image/video session service is unavailable"))
		return
	} else {
		var session *service.NextChatManagedSession
		session, err = issuer.IssueNextChatManagedSession(c.Request.Context(), subject.UserID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if session == nil || session.UserID != subject.UserID || strings.TrimSpace(session.APIKey) == "" || session.KeyID <= 0 || session.Purpose != purpose {
			response.ErrorFrom(c, infraerrors.ServiceUnavailable("NEXTCHAT_PURPOSE_SESSION_UNAVAILABLE", "scoped image/video session service is unavailable"))
			return
		}
		_, err = identityProvider.SetNextChatManagedKeyGroup(c.Request.Context(), subject.UserID, session.KeyID, req.GroupID)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	payload, err := buildNextChatMobileBootstrapPayload(c.Request.Context(), issuer, modelProvider, gate, cfg, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if groupPinnedReply {
		payload, err = replaceNextChatMobileBootstrapPurpose(c.Request.Context(), payload, issuer, modelProvider, gate, cfg, purpose, replacement)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		payload["session_binding"] = service.NextChatGroupPinnedSessionBinding
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, payload)
}

func nextChatSessionPayload(session *service.NextChatManagedSession, expiresAt time.Time) gin.H {
	payload := gin.H{
		"user_id":    session.UserID,
		"api_key":    session.APIKey,
		"api_key_id": session.KeyID,
		"purpose":    session.Purpose,
		"expires_at": expiresAt,
	}
	if session.GroupID != nil && *session.GroupID > 0 {
		payload["group_id"] = *session.GroupID
	}
	if binding := strings.TrimSpace(session.Binding); binding != "" {
		payload["binding"] = binding
	}
	return payload
}

// replaceNextChatMobileBootstrapPurpose keeps the mobile response in its
// established full-bootstrap shape while replacing exactly one purpose's key,
// identity, and workspace. A subsequent generic bootstrap must not overwrite
// the newly issued group-pinned session.
func replaceNextChatMobileBootstrapPurpose(
	ctx context.Context,
	payload gin.H,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
	purpose string,
	session *service.NextChatManagedSession,
) (gin.H, error) {
	if session == nil || session.UserID <= 0 || session.KeyID <= 0 || strings.TrimSpace(session.APIKey) == "" ||
		!strings.EqualFold(strings.TrimSpace(session.Purpose), strings.TrimSpace(purpose)) ||
		strings.TrimSpace(session.Binding) != service.NextChatGroupPinnedSessionBinding {
		return nil, infraerrors.ServiceUnavailable("NEXTCHAT_GROUP_PINNED_SESSION_UNAVAILABLE", "group-pinned session is unavailable")
	}
	replacement, err := buildNextChatBootstrapPayload(ctx, issuer, modelProvider, gate, session.UserID, session.KeyID)
	if err != nil {
		return nil, err
	}
	return replaceNextChatMobileBootstrapPurposeWithExpiry(payload, replacement, purpose, session, time.Now().UTC().Add(nextChatSessionTTL(cfg))), nil
}

func replaceNextChatMobileBootstrapPurposeWithExpiry(
	payload gin.H,
	replacement gin.H,
	purpose string,
	session *service.NextChatManagedSession,
	expiresAt time.Time,
) gin.H {
	if sessions, ok := payload["sessions"].(gin.H); ok {
		sessions[purpose] = nextChatSessionPayload(session, expiresAt)
	} else {
		payload["sessions"] = gin.H{purpose: nextChatSessionPayload(session, expiresAt)}
	}
	if keys, ok := payload["managed_api_keys"].(gin.H); ok {
		keys[purpose] = replacement["managed_api_key"]
	} else {
		payload["managed_api_keys"] = gin.H{purpose: replacement["managed_api_key"]}
	}
	if workspaces, ok := payload["workspaces"].(gin.H); ok {
		workspaces[purpose] = gin.H{"models": replacement["models"]}
	} else {
		payload["workspaces"] = gin.H{purpose: gin.H{"models": replacement["models"]}}
	}
	if purpose == service.NextChatSessionPurposeChat {
		payload["session"] = nextChatSessionPayload(session, expiresAt)
		payload["managed_api_key"] = replacement["managed_api_key"]
		payload["models"] = replacement["models"]
	}
	return payload
}

func buildNextChatBootstrapPayload(
	ctx context.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	userID int64,
	apiKeyID int64,
) (gin.H, error) {
	identityProvider, ok := issuer.(nextChatWorkspaceIdentityProvider)
	if !ok || identityProvider == nil {
		return nil, infraerrors.ServiceUnavailable("NEXTCHAT_BOOTSTRAP_UNAVAILABLE", "NextChat bootstrap service is unavailable")
	}
	identity, err := identityProvider.GetNextChatWorkspaceIdentity(ctx, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	workspaceModels, err := getNextChatWorkspaceModels(ctx, modelProvider, userID, apiKeyID)
	if err != nil {
		return nil, err
	}
	settings, err := getNextChatPublicSettings(ctx, gate)
	if err != nil {
		return nil, err
	}
	siteURL := firstNonEmptyNextChat(getNextChatFrontendURL(ctx, gate), "https://www.jisudeng.com")
	returnURL := joinNextChatURL(siteURL, "/dashboard")
	rechargeURL := joinNextChatURL(siteURL, "/purchase")

	return gin.H{
		"user":            identity.User,
		"managed_api_key": identity.APIKey,
		"brand": gin.H{
			"site_name":      firstNonEmptyNextChat(settings.SiteName, "极速蹬"),
			"site_logo":      settings.SiteLogo,
			"workspace_name": "极速蹬 AI 工作台",
		},
		"features": gin.H{
			"chat":           true,
			"image_studio":   settings.ImageStudioEnabled,
			"prompts":        true,
			"history_export": true,
			"cloud_sync":     false,
		},
		"models": workspaceModels,
		"urls": gin.H{
			"return_url":   returnURL,
			"recharge_url": rechargeURL,
			"profile_url":  joinNextChatURL(siteURL, "/profile"),
		},
		"support_contact": settings.SupportContact,
		"retention": gin.H{
			"text_session_days":     7,
			"image_job_days":        7,
			"image_asset_hours":     24,
			"image_reference_hours": 24,
			"server_chat_log":       false,
		},
	}, nil
}

func handleNextChatPrompts(
	c *gin.Context,
	promptProvider nextChatPromptProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, _, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	response.Success(c, buildNextChatPromptCatalog(c.Request.Context(), promptProvider, userID))
}

func handleNextChatImagePrompts(
	c *gin.Context,
	promptProvider nextChatPromptProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, _, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	if promptProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat image prompt service is unavailable")
		return
	}
	page, pageSize := response.ParsePagination(c)
	userIDPtr := &userID
	rows, result, err := promptProvider.ListPublic(c.Request.Context(), service.PromptListFilter{
		Query:                c.Query("q"),
		Purpose:              c.Query("purpose"),
		Style:                c.Query("style"),
		Subject:              c.Query("subject"),
		Model:                c.Query("model"),
		Size:                 c.Query("size"),
		ReferenceRequirement: nextChatPromptReferenceRequirement(c.Query("reference")),
		ImageOnly:            true,
		Featured:             nextChatOptionalBool(c.Query("featured")),
		FavoritedOnly:        nextChatBoolQuery(c.Query("favorite")),
		Sort:                 c.DefaultQuery("sort", "featured"),
		Pagination:           pagination.PaginationParams{Page: page, PageSize: pageSize},
	}, userIDPtr)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func handleNextChatImagePrompt(
	c *gin.Context,
	promptProvider nextChatPromptProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, _, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	if promptProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat image prompt service is unavailable")
		return
	}
	id, ok := nextChatPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := promptProvider.GetPublic(c.Request.Context(), id, &userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if prompt == nil || !service.IsNextChatPublicImagePrompt(*prompt) {
		response.NotFound(c, "NextChat image prompt not found")
		return
	}
	response.Success(c, prompt)
}

func handleNextChatImagePromptFavorite(
	c *gin.Context,
	promptProvider nextChatPromptProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
	favorite bool,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, _, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	if promptProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat image prompt service is unavailable")
		return
	}
	id, ok := nextChatPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := promptProvider.GetPublic(c.Request.Context(), id, &userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if prompt == nil || !service.IsNextChatPublicImagePrompt(*prompt) {
		response.NotFound(c, "NextChat image prompt not found")
		return
	}
	state, err := promptProvider.SetFavorite(c.Request.Context(), id, userID, favorite)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"prompt_id": id, "favorited": state})
}

func handleNextChatImagePromptUse(
	c *gin.Context,
	promptProvider nextChatPromptProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	userID, _, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	if promptProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat image prompt service is unavailable")
		return
	}
	id, ok := nextChatPromptPathID(c)
	if !ok {
		return
	}
	prompt, err := promptProvider.GetPublic(c.Request.Context(), id, &userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if prompt == nil || !service.IsNextChatPublicImagePrompt(*prompt) {
		response.NotFound(c, "NextChat image prompt not found")
		return
	}
	result, err := promptProvider.UsePrompt(c.Request.Context(), id, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result == nil || !nextChatPromptUseResultIsImage(result) {
		response.NotFound(c, "NextChat image prompt not found")
		return
	}
	response.Success(c, result)
}

func buildNextChatPromptCatalog(ctx context.Context, promptProvider nextChatPromptProvider, userID int64) service.NextChatPromptCatalog {
	if promptProvider == nil {
		return service.BuildNextChatPromptCatalog()
	}
	userIDPtr := &userID
	rows, _, err := promptProvider.ListPublic(ctx, service.PromptListFilter{
		Sort:       "featured",
		Pagination: pagination.PaginationParams{Page: 1, PageSize: 48},
	}, userIDPtr)
	if err != nil || len(rows) == 0 {
		return service.BuildNextChatPromptCatalog()
	}
	prompts := make([]service.PublicPrompt, 0, len(rows))
	for _, row := range rows {
		prompt := row
		if strings.TrimSpace(prompt.PromptText) == "" {
			if detail, detailErr := promptProvider.GetPublic(ctx, row.ID, userIDPtr); detailErr == nil && detail != nil {
				prompt = *detail
			}
		}
		prompts = append(prompts, prompt)
	}
	return service.BuildNextChatPromptCatalogFromPublicPrompts(prompts)
}

func nextChatPromptPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid prompt id")
		return 0, false
	}
	return id, true
}

func nextChatPromptReferenceRequirement(raw string) service.PromptReferenceRequirement {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(service.PromptReferenceNone):
		return service.PromptReferenceNone
	case string(service.PromptReferenceOptional):
		return service.PromptReferenceOptional
	case string(service.PromptReferenceRequired):
		return service.PromptReferenceRequired
	default:
		return ""
	}
}

func nextChatPromptUseResultIsImage(result *service.PromptUseResult) bool {
	if result == nil {
		return false
	}
	if len(result.Sizes) > 0 || result.RequiresReference {
		return true
	}
	if requirement := strings.TrimSpace(string(result.ReferenceRequirement)); requirement != "" && requirement != string(service.PromptReferenceNone) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(result.Purpose)) {
	case "image", "image_studio", "image-studio":
		return true
	}
	for _, model := range result.Models {
		if _, ok := service.ResolveImageStudioModelCapability(model); ok {
			return true
		}
	}
	return false
}

func nextChatOptionalBool(raw string) *bool {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	value := nextChatBoolQuery(raw)
	return &value
}

func nextChatBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func handleNextChatGroupSwitch(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	identityProvider, ok := issuer.(nextChatWorkspaceIdentityProvider)
	if !ok || identityProvider == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat group switch service is unavailable")
		return
	}
	userID, apiKeyID, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	var req nextChatGroupSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID <= 0 {
		response.BadRequest(c, "group_id is required")
		return
	}
	identity, err := identityProvider.SetNextChatManagedKeyGroup(c.Request.Context(), userID, apiKeyID, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	workspaceModels, err := getNextChatWorkspaceModels(c.Request.Context(), modelProvider, userID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"managed_api_key": identity.APIKey,
		"models":          workspaceModels,
	})
}

// handleNextChatScopedGroupSwitch is the Canvas BFF variant of the mobile
// purpose-scoped group switch. A Canvas server authenticates with the exchange
// secret and may change only the managed key whose ID it presented. This keeps
// image and video group selection independent from the chat key.
func handleNextChatScopedGroupSwitch(
	c *gin.Context,
	issuer nextChatSessionIssuer,
	modelProvider nextChatWorkspaceModelProvider,
	gate nextChatFeatureGate,
	cfg *config.Config,
	purpose string,
) {
	if gate == nil || !gate.IsNextChatEnabled(c.Request.Context()) {
		response.NotFound(c, "NextChat is disabled")
		return
	}
	scoped, ok := issuer.(nextChatScopedSessionIssuer)
	if !ok || scoped == nil {
		response.Error(c, http.StatusServiceUnavailable, "NextChat scoped group switch service is unavailable")
		return
	}
	if !isNextChatSessionPurpose(purpose) {
		response.BadRequest(c, "session purpose must be chat, image, or video")
		return
	}
	userID, apiKeyID, ok := requireNextChatBFFSession(c, cfg)
	if !ok {
		return
	}
	var req nextChatGroupSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.GroupID <= 0 {
		response.BadRequest(c, "group_id is required")
		return
	}
	if switcher, immutable := issuer.(nextChatImmutableScopedSessionSwitcher); immutable && switcher != nil {
		session, identity, err := switcher.SwitchNextChatManagedSessionGroup(c.Request.Context(), userID, apiKeyID, purpose, req.GroupID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if session == nil || session.UserID != userID || session.KeyID <= 0 || strings.TrimSpace(session.APIKey) == "" || session.Purpose != purpose || identity == nil || identity.APIKey.ID != session.KeyID {
			response.Error(c, http.StatusServiceUnavailable, "NextChat scoped group session is unavailable")
			return
		}
		workspaceModels, err := getNextChatWorkspaceModels(c.Request.Context(), modelProvider, userID, session.KeyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		response.Success(c, gin.H{
			"purpose":         purpose,
			"session_binding": service.NextChatGroupPinnedSessionBinding,
			"session":         nextChatSessionPayload(session, time.Now().UTC().Add(nextChatSessionTTL(cfg))),
			"managed_api_key": identity.APIKey,
			"models":          workspaceModels,
		})
		return
	}
	session, err := scoped.IssueNextChatManagedSessionForPurpose(c.Request.Context(), userID, purpose)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if session == nil || session.UserID != userID || session.KeyID != apiKeyID || session.Purpose != purpose {
		response.Unauthorized(c, "Managed session does not match the requested purpose")
		return
	}
	identity, err := scoped.SetNextChatManagedSessionGroup(c.Request.Context(), userID, purpose, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if identity == nil || identity.APIKey.ID != apiKeyID {
		response.Unauthorized(c, "Managed session changed during group switch")
		return
	}
	workspaceModels, err := getNextChatWorkspaceModels(c.Request.Context(), modelProvider, userID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"purpose":         purpose,
		"managed_api_key": identity.APIKey,
		"models":          workspaceModels,
	})
}

func isNextChatSessionPurpose(purpose string) bool {
	switch purpose {
	case service.NextChatSessionPurposeChat, service.NextChatSessionPurposeImage, service.NextChatSessionPurposeVideo:
		return true
	default:
		return false
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
	var req nextChatLaunchRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.BadRequest(c, "Invalid launch request")
		return
	}
	if req.Intent != nil && !validNextChatLaunchIntent(*req.Intent) {
		response.BadRequest(c, "Invalid creation intent")
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
		"launch_url":  nextChatLaunchURL(cfg, token, req.Intent),
		"expires_at":  record.ExpiresAt,
		"ttl_seconds": int(ttl.Seconds()),
	})
}

func validNextChatLaunchIntent(intent nextChatLaunchIntent) bool {
	return intent.Type == "image_prompt" && intent.PromptID > 0 && intent.PromptVersion >= 0
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

	expiresAt := time.Now().UTC().Add(nextChatSessionTTL(cfg))
	if scoped, ok := issuer.(nextChatScopedSessionIssuer); ok {
		sessions, err := scoped.IssueNextChatManagedSessions(c.Request.Context(), record.UserID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		payload := nextChatSessionPayload(&sessions.Chat, expiresAt)
		payload["sessions"] = gin.H{
			service.NextChatSessionPurposeChat:  nextChatSessionPayload(&sessions.Chat, expiresAt),
			service.NextChatSessionPurposeImage: nextChatSessionPayload(&sessions.Image, expiresAt),
			service.NextChatSessionPurposeVideo: nextChatSessionPayload(&sessions.Video, expiresAt),
		}
		response.Success(c, payload)
		return
	}

	session, err := issuer.IssueNextChatManagedSession(c.Request.Context(), record.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nextChatSessionPayload(session, expiresAt))
}

// handleNextChatIdentityExchange consumes a launch token without creating or
// returning a managed model key. Canvas uses this endpoint for SSO only.
func handleNextChatIdentityExchange(
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

	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"user_id": record.UserID})
}

func requireNextChatBFFSession(c *gin.Context, cfg *config.Config) (int64, int64, bool) {
	if !validNextChatExchangeSecret(c.GetHeader("X-NextChat-Secret"), cfg) {
		response.Unauthorized(c, "Invalid NextChat exchange secret")
		return 0, 0, false
	}
	userID, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader("X-NextChat-User-ID")), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "X-NextChat-User-ID is required")
		return 0, 0, false
	}
	apiKeyID, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader("X-NextChat-API-Key-ID")), 10, 64)
	if err != nil || apiKeyID <= 0 {
		response.BadRequest(c, "X-NextChat-API-Key-ID is required")
		return 0, 0, false
	}
	return userID, apiKeyID, true
}

func getNextChatPublicSettings(ctx context.Context, gate nextChatFeatureGate) (*service.PublicSettings, error) {
	provider, ok := gate.(nextChatPublicSettingsProvider)
	if !ok {
		return &service.PublicSettings{SiteName: "极速蹬", NextChatEnabled: true}, nil
	}
	settings, err := provider.GetPublicSettings(ctx)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

func getNextChatFrontendURL(ctx context.Context, gate nextChatFeatureGate) string {
	provider, ok := gate.(nextChatPublicSettingsProvider)
	if !ok {
		return ""
	}
	return provider.GetFrontendURL(ctx)
}

func getNextChatWorkspaceModels(ctx context.Context, provider nextChatWorkspaceModelProvider, userID, apiKeyID int64) (*service.NextChatWorkspaceModels, error) {
	if provider == nil {
		return &service.NextChatWorkspaceModels{
			Source: "/v1/models",
			Groups: []service.NextChatWorkspaceGroup{},
		}, nil
	}
	return provider.GetNextChatWorkspaceModels(ctx, userID, apiKeyID)
}

func firstNonEmptyNextChat(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func joinNextChatURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	if base == "" {
		return path
	}
	return base + path
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

func nextChatLaunchURL(cfg *config.Config, token string, intent *nextChatLaunchIntent) string {
	base := "https://canvas.jisudeng.com"
	if cfg != nil {
		if strings.TrimSpace(cfg.AICreationSpace.PublicURL) != "" {
			base = strings.TrimSpace(cfg.AICreationSpace.PublicURL)
		} else if strings.TrimSpace(cfg.NextChat.PublicURL) != "" {
			// Keep older test fixtures and deployments working until the
			// dedicated AI creation space URL is configured.
			base = strings.TrimSpace(cfg.NextChat.PublicURL)
		}
	}
	parsed, err := url.Parse(base)
	if err != nil {
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		value := base + separator + "launch_token=" + url.QueryEscape(token)
		if intent != nil {
			value += "&creation_prompt=" + strconv.FormatInt(intent.PromptID, 10)
			if intent.PromptVersion > 0 {
				value += "&creation_prompt_version=" + strconv.Itoa(intent.PromptVersion)
			}
		}
		return value
	}
	query := parsed.Query()
	query.Set("launch_token", token)
	if intent != nil {
		query.Set("creation_prompt", strconv.FormatInt(intent.PromptID, 10))
		if intent.PromptVersion > 0 {
			query.Set("creation_prompt_version", strconv.Itoa(intent.PromptVersion))
		}
	}
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
