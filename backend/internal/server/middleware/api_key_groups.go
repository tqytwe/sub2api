package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func isMultiGroupBillingInfoRequest(c *gin.Context, key *service.APIKey) bool {
	return c.Request.URL.Path == "/v1/sub2api/billing" && len(key.ConfiguredGroupIDs()) > 1
}

const deferredAPIKeyRequestGroup = "deferred_api_key_request_group"

// SetDeferredAPIKeyRequestGroup delays model-dependent selection until the first
// WebSocket response.create frame, while identity authentication stays in auth.
func SetDeferredAPIKeyRequestGroup(c *gin.Context, resolve func([]string) (*service.APIKey, error)) {
	c.Set(deferredAPIKeyRequestGroup, resolve)
}

func ResolveDeferredAPIKeyRequestGroup(c *gin.Context, models []string) (*service.APIKey, error) {
	if value, ok := c.Get(deferredAPIKeyRequestGroup); ok {
		resolve, _ := value.(func([]string) (*service.APIKey, error))
		selected, err := resolve(models)
		if err != nil {
			return nil, err
		}
		delete(c.Keys, deferredAPIKeyRequestGroup)
		return selected, nil
	}
	key, _ := GetAPIKeyFromContext(c)
	return key, nil
}

const deferredAPIKeyRequestGroupByID = "deferred_api_key_request_group_by_id"

func ResolveDeferredAPIKeyRequestGroupByID(c *gin.Context, id int64) (*service.APIKey, error) {
	resolve, ok := c.Get(deferredAPIKeyRequestGroupByID)
	if !ok {
		return nil, infraerrors.Forbidden("API_KEY_GROUP_CHANGED", "Live call group is no longer available")
	}
	resolver, _ := resolve.(func(int64) (*service.APIKey, error))
	key, err := resolver(id)
	if err == nil {
		delete(c.Keys, deferredAPIKeyRequestGroupByID)
	}
	return key, err
}

func hasDeferredAPIKeyRequestGroup(c *gin.Context) bool {
	_, ok := c.Get(deferredAPIKeyRequestGroup)
	_, pinned := c.Get(deferredAPIKeyRequestGroupByID)
	return ok || pinned
}

func isAPIKeyResponsesWebSocket(c *gin.Context) bool {
	return c.Request.Method == http.MethodGet && strings.EqualFold(c.GetHeader("Upgrade"), "websocket") && strings.HasSuffix(c.Request.URL.Path, "/responses")
}

func selectAPIKeyRequestGroup(c *gin.Context, keys *service.APIKeyService, subscriptions *service.SubscriptionService, cfg *config.Config, key *service.APIKey, google bool) (*service.APIKey, error) {
	if len(key.ConfiguredGroupIDs()) < 2 || isMultiGroupBillingInfoRequest(c, key) || c.Request.URL.Path == "/v1/usage" || isAsyncImageTaskRead(c.Request.Method, c.Request.URL.Path) {
		return key, nil
	}
	if c.Request.Method == http.MethodGet && c.Param("call_id") != "" && (strings.Contains(c.FullPath(), "/live/") || strings.Contains(c.FullPath(), "/realtime/calls/") || c.FullPath() == "/backend-api/codex/:call_id") {
		c.Set(deferredAPIKeyRequestGroupByID, func(id int64) (*service.APIKey, error) {
			selected, err := keys.PinAPIKeyGroup(c.Request.Context(), key, id)
			if err != nil {
				return nil, err
			}
			var subscription *service.UserSubscription
			if cfg.RunMode != config.RunModeSimple {
				subscription, err = validateSelectedGroupBilling(c.Request.Context(), selected, subscriptions, cfg)
				if err != nil {
					return nil, err
				}
			}
			c.Set(string(ContextKeyAPIKey), selected)
			if subscription != nil {
				c.Set(string(ContextKeySubscription), subscription)
			} else {
				delete(c.Keys, string(ContextKeySubscription))
			}
			setGroupContext(c, selected.Group)
			SetOpsFallbackAPIKey(c, selected)
			return selected, nil
		})
		return key, nil
	}
	var models []string
	var body []byte
	if model := groupModelAllowlistModelFromParams(c); model != "" {
		models = []string{model}
	}
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
		var ok bool
		body, ok = readAdmissionRequestBody(c)
		if !ok {
			return nil, nil
		}
		if len(models) == 0 {
			models = requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body)
		}
		for _, model := range models {
			if service.IsExplicitImageGenerationIntent(c.Request.URL.Path, model, body) {
				c.Request = c.Request.WithContext(service.WithOpenAIImageGenerationIntent(c.Request.Context()))
				break
			}
		}
	}
	if isAPIKeyResponsesWebSocket(c) {
		SetDeferredAPIKeyRequestGroup(c, func(models []string) (*service.APIKey, error) {
			selected, err := resolveAPIKeyRequestGroup(c, keys, subscriptions, cfg, key, google, models, cfg.RunMode != config.RunModeSimple)
			if err != nil {
				return nil, err
			}
			var subscription *service.UserSubscription
			if cfg.RunMode != config.RunModeSimple {
				subscription, err = validateSelectedGroupBilling(c.Request.Context(), selected, subscriptions, cfg)
				if err != nil {
					return nil, err
				}
			}
			c.Set(string(ContextKeyAPIKey), selected)
			if subscription != nil {
				c.Set(string(ContextKeySubscription), subscription)
			} else {
				delete(c.Keys, string(ContextKeySubscription))
			}
			setGroupContext(c, selected.Group)
			SetOpsFallbackAPIKey(c, selected)
			return selected, nil
		})
		return key, nil
	}
	if len(models) == 0 && c.Query("model") != "" {
		models = []string{c.Query("model")}
	}
	checkBilling := cfg.RunMode != config.RunModeSimple && c.Request.Method != http.MethodGet && !strings.HasSuffix(c.Request.URL.Path, "/count_tokens")
	selected, err := resolveAPIKeyRequestGroup(c, keys, subscriptions, cfg, key, google, models, checkBilling)
	if err != nil {
		status := infraerrors.Code(err)
		message, reason := infraerrors.Message(err), infraerrors.Reason(err)
		if status >= 500 {
			message = "No selected group is currently available for this request"
		}
		if google {
			abortWithGoogleError(c, status, message)
		} else {
			AbortWithError(c, status, reason, message)
		}
		return nil, err
	}
	SetOpsFallbackAPIKey(c, selected)
	return selected, nil
}

func resolveAPIKeyRequestGroup(c *gin.Context, keys *service.APIKeyService, subscriptions *service.SubscriptionService, cfg *config.Config, key *service.APIKey, google bool, models []string, checkBilling bool) (*service.APIKey, error) {
	kind := "token"
	path := c.Request.URL.Path
	endpoint := apiKeyGroupRequestEndpoint(path)
	if strings.Contains(path, "/images/") {
		kind = "image"
	}
	if strings.Contains(path, "/videos") {
		kind = "video"
	}
	accepts := func(candidate *service.APIKey) bool {
		group := candidate.Group
		switch endpoint {
		case "audio", "audio_realtime":
			if group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformGrok {
				return false
			}
		case "audio_openai":
			if group.Platform != service.PlatformOpenAI {
				return false
			}
		case "voice_grok":
			if group.Platform != service.PlatformGrok {
				return false
			}
		}
		if strings.HasSuffix(path, "/live") && (!group.AllowLive || (group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformComposite)) {
			return false
		}
		// Match the existing endpoint handlers before ranking by multiplier.
		// Composite targets are checked by the endpoint-aware scheduler probe.
		if group.Platform != service.PlatformComposite {
			if isAPIKeyResponsesWebSocket(c) && group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformGrok {
				return false
			}
			if kind == "image" && group.Platform != service.PlatformOpenAI && group.Platform != service.PlatformGrok {
				return false
			}
			if strings.HasSuffix(path, "/embeddings") && group.Platform != service.PlatformOpenAI {
				return false
			}
			if kind == "video" && group.Platform != service.PlatformGrok && (!strings.HasSuffix(path, "/videos") || group.Platform != service.PlatformOpenAI) {
				return false
			}
		}
		if group.Platform == service.PlatformOpenAI && (strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/messages/count_tokens")) && !group.AllowMessagesDispatch {
			return false
		}
		if checkBilling && !group.IsSubscriptionType() && apiKeyBalanceBelowAuthThreshold(candidate.User.Balance, cfg) {
			return false
		}
		if google && group.Platform != service.PlatformGemini && group.Platform != service.PlatformAntigravity && group.Platform != service.PlatformComposite {
			return false
		}
		if strings.HasPrefix(path, "/antigravity/") && group.Platform != service.PlatformAntigravity {
			return false
		}
		if (kind == "image" || service.OpenAIImageGenerationIntentFromContext(c.Request.Context())) && !service.GroupAllowsImageGeneration(group) {
			return false
		}
		return true
	}
	ctx := context.WithValue(c.Request.Context(), ctxkey.UserID, key.UserID)
	ctx = service.WithAPIKeyGroupEndpoint(ctx, endpoint)
	if isAPIKeyResponsesWebSocket(c) {
		ctx = service.WithAPIKeyGroupWebSocket(ctx)
	}
	return keys.SelectRequestGroup(ctx, key, models, kind, subscriptions, accepts, checkBilling)
}

func apiKeyGroupRequestEndpoint(path string) string {
	switch {
	case strings.HasSuffix(path, "/audio/translations"):
		return "audio_openai"
	case strings.HasSuffix(path, "/realtime"):
		return "audio_realtime"
	case strings.Contains(path, "/audio/"):
		return "audio"
	case strings.HasSuffix(path, "/tts") || strings.HasSuffix(path, "/stt") || strings.Contains(path, "/custom-voices"):
		return "voice_grok"
	case strings.HasSuffix(path, "/messages/count_tokens"):
		return service.CompositeRouteEndpointCountTokens
	case strings.HasSuffix(path, "/messages"):
		return service.CompositeRouteEndpointMessages
	case strings.Contains(path, "/responses"):
		return service.CompositeRouteEndpointResponses
	case strings.HasSuffix(path, "/chat/completions"):
		return service.CompositeRouteEndpointChatCompletions
	case strings.HasSuffix(path, "/embeddings"):
		return service.CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/"):
		return service.CompositeRouteEndpointImages
	case strings.HasSuffix(path, "/live"):
		return "live"
	case strings.HasSuffix(path, "/videos"):
		return "videos"
	case strings.Contains(path, "/videos/"):
		return "videos_grok"
	case strings.Contains(path, "/models/") && strings.Contains(path, ":"):
		return service.CompositeRouteEndpointGemini
	default:
		return service.CompositeRouteEndpointAny
	}
}

func validateSelectedGroupBilling(ctx context.Context, key *service.APIKey, subscriptions *service.SubscriptionService, cfg *config.Config) (*service.UserSubscription, error) {
	if key.Group.IsSubscriptionType() && subscriptions != nil {
		sub, err := subscriptions.GetActiveSubscription(ctx, key.UserID, key.Group.ID)
		if err != nil {
			return nil, err
		}
		maintenance, err := subscriptions.ValidateAndCheckLimits(sub, key.Group)
		if maintenance {
			sub, err = subscriptions.EnsureWindowMaintenance(ctx, sub)
			if err != nil {
				return nil, err
			}
			_, err = subscriptions.ValidateAndCheckLimits(sub, key.Group)
		}
		return sub, err
	}
	if apiKeyBalanceBelowAuthThreshold(key.User.Balance, cfg) {
		return nil, infraerrors.Forbidden("INSUFFICIENT_BALANCE", "Insufficient account balance")
	}
	return nil, nil
}

func readAdmissionRequestBody(c *gin.Context) ([]byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, true
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		status := http.StatusBadRequest
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status = http.StatusRequestEntityTooLarge
		}
		c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Failed to read request body"}})
		return nil, false
	}
	requestmodel.ResetRequestBody(c.Request, body)
	return body, true
}
