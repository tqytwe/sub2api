package handler

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	mobileWebSearchMaxQueryRunes  = 500
	mobileWebSearchMaxResults     = 10
	mobileWebSearchDefaultResults = 5
	mobileWebSearchMaxBodyBytes   = 16 << 10
	mobileWebSearchTimeout        = 8 * time.Second
)

// mobileWebSearchProvider keeps the handler testable without allowing callers
// to provide an arbitrary upstream endpoint or credential.
type mobileWebSearchProvider interface {
	Search(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error)
}

// MobileWebSearchHandler exposes the one canonical, authenticated mobile web
// search route. Its provider is created from server environment only.
type MobileWebSearchHandler struct {
	provider mobileWebSearchProvider
	enabled  bool
	timeout  time.Duration
}

// NewMobileWebSearchHandlerFromEnvironment is the only production constructor.
// EXA_API_KEY is read here and never passed through an HTTP response or client.
func NewMobileWebSearchHandlerFromEnvironment() *MobileWebSearchHandler {
	apiKey := strings.TrimSpace(os.Getenv("EXA_API_KEY"))
	enabled := mobileWebSearchEnvBool(os.Getenv("MOBILE_WEB_SEARCH_ENABLED")) && apiKey != ""
	var provider mobileWebSearchProvider
	if enabled {
		provider = websearch.NewExaProvider(apiKey, &http.Client{
			Timeout: mobileWebSearchTimeout,
			// Never follow a redirect with x-api-key attached to an unknown host.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		})
	}
	return newMobileWebSearchHandler(provider, enabled, mobileWebSearchTimeout)
}

func newMobileWebSearchHandler(provider mobileWebSearchProvider, enabled bool, timeout time.Duration) *MobileWebSearchHandler {
	if timeout <= 0 {
		timeout = mobileWebSearchTimeout
	}
	return &MobileWebSearchHandler{provider: provider, enabled: enabled && provider != nil, timeout: timeout}
}

func mobileWebSearchEnvBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

type mobileWebSearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	OptIn      bool   `json:"opt_in"`
	Locale     string `json:"locale"`
}

type mobileWebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	PageAge string `json:"page_age,omitempty"`
}

type mobileWebSearchResponse struct {
	RequestID string                  `json:"request_id"`
	Query     string                  `json:"query"`
	Provider  string                  `json:"provider"`
	Results   []mobileWebSearchResult `json:"results"`
}

// Search handles POST /api/v1/mobile/web-search. Search is side-effect free,
// so a retry key is not required; X-Request-ID still correlates the full call.
func (h *MobileWebSearchHandler) Search(c *gin.Context) {
	requestID := mobileWebSearchRequestID(c)
	c.Header("Cache-Control", "private, no-store")
	locale := mobileWebSearchLocale(c.GetHeader("Accept-Language"))
	subject, authenticated := middleware2.GetAuthSubjectFromContext(c)
	if !authenticated || subject.UserID <= 0 {
		writeMobileWebSearchError(c, http.StatusUnauthorized, "MOBILE_WEB_SEARCH_AUTH_REQUIRED", "authentication_required", locale, requestID)
		return
	}
	if h == nil || !h.enabled || h.provider == nil {
		writeMobileWebSearchError(c, http.StatusServiceUnavailable, "MOBILE_WEB_SEARCH_UNAVAILABLE", "web_search_unavailable", locale, requestID)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, mobileWebSearchMaxBodyBytes)
	var input mobileWebSearchRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		writeMobileWebSearchError(c, http.StatusBadRequest, "MOBILE_WEB_SEARCH_INVALID_REQUEST", "invalid_request", locale, requestID)
		return
	}
	if requestLocale := mobileWebSearchLocale(input.Locale); strings.TrimSpace(input.Locale) != "" {
		locale = requestLocale
	}
	if !input.OptIn {
		writeMobileWebSearchError(c, http.StatusBadRequest, "MOBILE_WEB_SEARCH_OPT_IN_REQUIRED", "explicit_opt_in_required", locale, requestID)
		return
	}

	query := strings.TrimSpace(input.Query)
	if query == "" {
		writeMobileWebSearchError(c, http.StatusBadRequest, "MOBILE_WEB_SEARCH_INVALID_QUERY", "query_required", locale, requestID)
		return
	}
	if utf8.RuneCountInString(query) > mobileWebSearchMaxQueryRunes {
		writeMobileWebSearchError(c, http.StatusBadRequest, "MOBILE_WEB_SEARCH_QUERY_TOO_LONG", "query_too_long", locale, requestID)
		return
	}
	maxResults := input.MaxResults
	if maxResults == 0 {
		maxResults = mobileWebSearchDefaultResults
	}
	if maxResults < 1 || maxResults > mobileWebSearchMaxResults {
		writeMobileWebSearchError(c, http.StatusBadRequest, "MOBILE_WEB_SEARCH_RESULTS_LIMIT", "results_limit_exceeded", locale, requestID)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	result, err := h.provider.Search(ctx, websearch.SearchRequest{Query: query, MaxResults: maxResults})
	if err != nil {
		writeMobileWebSearchProviderError(c, err, locale, requestID)
		return
	}
	if result == nil {
		writeMobileWebSearchError(c, http.StatusBadGateway, "MOBILE_WEB_SEARCH_UPSTREAM_ERROR", "upstream_error", locale, requestID)
		return
	}
	items := make([]mobileWebSearchResult, 0, len(result.Results))
	for _, item := range result.Results {
		if len(items) >= maxResults {
			break
		}
		items = append(items, mobileWebSearchResult{Title: item.Title, URL: item.URL, Snippet: item.Snippet, PageAge: item.PageAge})
	}
	executedQuery := strings.TrimSpace(result.Query)
	if executedQuery == "" {
		executedQuery = query
	}
	c.Header("X-Request-ID", requestID)
	response.Success(c, mobileWebSearchResponse{RequestID: requestID, Query: executedQuery, Provider: "exa", Results: items})
}

func writeMobileWebSearchProviderError(c *gin.Context, err error, locale, requestID string) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeMobileWebSearchError(c, http.StatusGatewayTimeout, "MOBILE_WEB_SEARCH_TIMEOUT", "upstream_timeout", locale, requestID)
		return
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		writeMobileWebSearchError(c, http.StatusGatewayTimeout, "MOBILE_WEB_SEARCH_TIMEOUT", "upstream_timeout", locale, requestID)
		return
	}
	if errors.As(err, &networkErr) {
		writeMobileWebSearchError(c, http.StatusBadGateway, "MOBILE_WEB_SEARCH_UNREACHABLE", "upstream_unreachable", locale, requestID)
		return
	}
	metadata := map[string]string{"request_id": requestID}
	var statusErr *websearch.UpstreamStatusError
	if errors.As(err, &statusErr) && statusErr != nil && statusErr.StatusCode > 0 {
		metadata["upstream_status"] = strconv.Itoa(statusErr.StatusCode)
	}
	writeMobileWebSearchErrorWithMetadata(c, http.StatusBadGateway, "MOBILE_WEB_SEARCH_UPSTREAM_ERROR", "upstream_error", locale, metadata)
}

func writeMobileWebSearchError(c *gin.Context, status int, code, reason, locale, requestID string) {
	writeMobileWebSearchErrorWithMetadata(c, status, code, reason, locale, map[string]string{"request_id": requestID})
}

func writeMobileWebSearchErrorWithMetadata(c *gin.Context, status int, code, reason, locale string, metadata map[string]string) {
	if metadata == nil {
		metadata = make(map[string]string, 1)
	}
	metadata["locale"] = locale
	message := mobileWebSearchMessage(code, locale)
	response.ErrorWithDetails(c, status, message, reason, metadata)
}

func mobileWebSearchMessage(code, locale string) string {
	if locale == "en" {
		switch code {
		case "MOBILE_WEB_SEARCH_AUTH_REQUIRED":
			return "Please sign in to use web search"
		case "MOBILE_WEB_SEARCH_UNAVAILABLE":
			return "Web search is not available"
		case "MOBILE_WEB_SEARCH_INVALID_REQUEST":
			return "The search request is invalid"
		case "MOBILE_WEB_SEARCH_OPT_IN_REQUIRED":
			return "Confirm web search before continuing"
		case "MOBILE_WEB_SEARCH_INVALID_QUERY":
			return "Enter a search query"
		case "MOBILE_WEB_SEARCH_QUERY_TOO_LONG":
			return "The search query is too long"
		case "MOBILE_WEB_SEARCH_RESULTS_LIMIT":
			return "Maximum results is 10"
		case "MOBILE_WEB_SEARCH_TIMEOUT":
			return "The search provider timed out"
		case "MOBILE_WEB_SEARCH_UPSTREAM_ERROR":
			return "The search provider is temporarily busy"
		case "MOBILE_WEB_SEARCH_UNREACHABLE":
			return "The search provider could not be reached"
		default:
			return "Web search failed"
		}
	}
	switch code {
	case "MOBILE_WEB_SEARCH_AUTH_REQUIRED":
		return "请先登录后使用联网搜索"
	case "MOBILE_WEB_SEARCH_UNAVAILABLE":
		return "联网搜索暂不可用"
	case "MOBILE_WEB_SEARCH_INVALID_REQUEST":
		return "联网搜索请求无效"
	case "MOBILE_WEB_SEARCH_OPT_IN_REQUIRED":
		return "请先确认启用联网搜索"
	case "MOBILE_WEB_SEARCH_INVALID_QUERY":
		return "请输入搜索内容"
	case "MOBILE_WEB_SEARCH_QUERY_TOO_LONG":
		return "搜索内容过长"
	case "MOBILE_WEB_SEARCH_RESULTS_LIMIT":
		return "最多返回 10 条结果"
	case "MOBILE_WEB_SEARCH_TIMEOUT":
		return "搜索服务响应超时，请稍后重试"
	case "MOBILE_WEB_SEARCH_UPSTREAM_ERROR":
		return "搜索服务暂时繁忙，请稍后重试"
	case "MOBILE_WEB_SEARCH_UNREACHABLE":
		return "暂时无法连接搜索服务，请稍后重试"
	default:
		return "联网搜索失败"
	}
}

func mobileWebSearchLocale(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(value, "en") {
		return "en"
	}
	return "zh"
}

func mobileWebSearchRequestID(c *gin.Context) string {
	if c == nil {
		return uuid.NewString()
	}
	if c.Request != nil {
		if value, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok && strings.TrimSpace(value) != "" {
			c.Header("X-Request-ID", value)
			return value
		}
	}
	if value := strings.TrimSpace(c.GetHeader("X-Request-ID")); value != "" {
		c.Header("X-Request-ID", value)
		return value
	}
	value := uuid.NewString()
	c.Header("X-Request-ID", value)
	return value
}
