package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/websearch"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type mobileWebSearchProviderStub struct {
	response *websearch.SearchResponse
	err      error
}

func (s mobileWebSearchProviderStub) Search(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error) {
	return s.response, s.err
}

type mobileWebSearchProviderFunc func(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error)

func (f mobileWebSearchProviderFunc) Search(ctx context.Context, request websearch.SearchRequest) (*websearch.SearchResponse, error) {
	return f(ctx, request)
}

type mobileWebSearchBudgetStub struct {
	calls      int
	err        error
	retryAfter time.Duration
}

func TestMobileWebSearchEnvironmentHonorsConfiguredProviderAndKeepsDuckDuckGoFallback(t *testing.T) {
	t.Setenv("MOBILE_WEB_SEARCH_ENABLED", "true")
	t.Setenv("EXA_API_KEY", "test-exa-key")
	t.Setenv("MOBILE_WEB_SEARCH_PROVIDER", "duckduckgo")
	duckDuckGo := NewMobileWebSearchHandlerFromEnvironment(unmeteredMobileWebSearchBudget{})
	if !duckDuckGo.enabled || duckDuckGo.providerName != "duckduckgo" {
		t.Fatalf("explicit DuckDuckGo provider = enabled:%v name:%q", duckDuckGo.enabled, duckDuckGo.providerName)
	}

	t.Setenv("MOBILE_WEB_SEARCH_PROVIDER", "exa")
	exa := NewMobileWebSearchHandlerFromEnvironment(unmeteredMobileWebSearchBudget{})
	if !exa.enabled || exa.providerName != "exa" {
		t.Fatalf("Exa provider = enabled:%v name:%q", exa.enabled, exa.providerName)
	}

	t.Setenv("EXA_API_KEY", "")
	noExaKey := NewMobileWebSearchHandlerFromEnvironment(unmeteredMobileWebSearchBudget{})
	if !noExaKey.enabled || noExaKey.providerName != "duckduckgo" {
		t.Fatalf("missing Exa key must fall back to DuckDuckGo = enabled:%v name:%q", noExaKey.enabled, noExaKey.providerName)
	}
}

func (s *mobileWebSearchBudgetStub) Reserve(context.Context, int64) (time.Duration, error) {
	s.calls++
	return s.retryAfter, s.err
}

func configureMobileWebSearchIdempotency(t *testing.T) {
	t.Helper()
	previous := service.DefaultIdempotencyCoordinator()
	cfg := service.DefaultIdempotencyConfig()
	cfg.ObserveOnly = false
	service.SetDefaultIdempotencyCoordinator(service.NewIdempotencyCoordinator(
		newUserMemoryIdempotencyRepoStub(),
		cfg,
	))
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(previous) })
}

func TestMobileWebSearchIsModelToolCallableWithoutClientOptInAndReturnsLocalizedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{response: &websearch.SearchResponse{
		Query:   "golang",
		Results: []websearch.SearchResult{{Title: "Go", URL: "https://go.dev", Snippet: "The Go language"}},
	}}, true, time.Second)

	recorder := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"golang","tool_call_id":"call-golang"}`, "en-US", "rid-search-1", 7)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Request-ID") != "rid-search-1" {
		t.Fatalf("request ID header = %q", recorder.Header().Get("X-Request-ID"))
	}
	if !strings.Contains(recorder.Body.String(), `"request_id":"rid-search-1"`) {
		t.Fatalf("response must include request ID: %s", recorder.Body.String())
	}

	manualOptIn := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"golang","opt_in":true}`, "zh-CN", "rid-search-2", 7)
	if manualOptIn.Code != http.StatusBadRequest || !strings.Contains(manualOptIn.Body.String(), "MOBILE_WEB_SEARCH_TOOL_CALL_REQUIRED") {
		t.Fatalf("manual opt-in request must be rejected = %d, body = %s", manualOptIn.Code, manualOptIn.Body.String())
	}
}

func TestMobileWebSearchFallbackProviderUsesDuckDuckGoAfterExaFailureAndOpensCircuit(t *testing.T) {
	primaryCalls := 0
	fallbackCalls := 0
	now := time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)
	provider := newMobileWebSearchFallbackProvider(
		mobileWebSearchProviderFunc(func(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error) {
			primaryCalls++
			return nil, context.DeadlineExceeded
		}),
		mobileWebSearchProviderFunc(func(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error) {
			fallbackCalls++
			return &websearch.SearchResponse{Query: "query"}, nil
		}),
		"exa",
		"duckduckgo",
	)
	provider.now = func() time.Time { return now }
	provider.primaryTimeout = time.Second
	provider.cooldown = time.Minute

	first, err := provider.Search(context.Background(), websearch.SearchRequest{Query: "query"})
	if err != nil {
		t.Fatalf("first fallback search: %v", err)
	}
	if first.Provider != "duckduckgo" || primaryCalls != 1 || fallbackCalls != 1 {
		t.Fatalf("first fallback = provider:%q primary:%d fallback:%d", first.Provider, primaryCalls, fallbackCalls)
	}

	second, err := provider.Search(context.Background(), websearch.SearchRequest{Query: "query"})
	if err != nil {
		t.Fatalf("circuit fallback search: %v", err)
	}
	if second.Provider != "duckduckgo" || primaryCalls != 1 || fallbackCalls != 2 {
		t.Fatalf("circuit fallback = provider:%q primary:%d fallback:%d", second.Provider, primaryCalls, fallbackCalls)
	}

	now = now.Add(time.Minute + time.Second)
	_, err = provider.Search(context.Background(), websearch.SearchRequest{Query: "query"})
	if err != nil || primaryCalls != 2 || fallbackCalls != 3 {
		t.Fatalf("circuit recovery err:%v primary:%d fallback:%d", err, primaryCalls, fallbackCalls)
	}
}

func TestMobileWebSearchRejectsBoundsAndDisabledConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, true, time.Second)
	tooLong := `{"query":"` + strings.Repeat("x", mobileWebSearchMaxQueryRunes+1) + `","tool_call_id":"call-long"}`
	tooLongResponse := performMobileWebSearchRequest(h.Search, http.MethodPost, tooLong, "zh-CN", "rid-long", 9)
	if tooLongResponse.Code != http.StatusBadRequest || !strings.Contains(tooLongResponse.Body.String(), "搜索内容过长") {
		t.Fatalf("query bound response = %d %s", tooLongResponse.Code, tooLongResponse.Body.String())
	}

	tooMany := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"ok","max_results":11,"tool_call_id":"call-many"}`, "en", "rid-many", 9)
	if tooMany.Code != http.StatusBadRequest || !strings.Contains(strings.ToLower(tooMany.Body.String()), "maximum") {
		t.Fatalf("result bound response = %d %s", tooMany.Code, tooMany.Body.String())
	}

	disabled := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, false, time.Second)
	disabledResponse := performMobileWebSearchRequest(disabled.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-off"}`, "en", "rid-off", 9)
	if disabledResponse.Code != http.StatusServiceUnavailable || !strings.Contains(disabledResponse.Body.String(), "not available") {
		t.Fatalf("disabled response = %d %s", disabledResponse.Code, disabledResponse.Body.String())
	}
}

func TestMobileWebSearchClassifiesTimeoutAndUpstreamStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	timeoutHandler := newMobileWebSearchHandler(mobileWebSearchProviderStub{err: context.DeadlineExceeded}, true, time.Second)
	timeoutResponse := performMobileWebSearchRequest(timeoutHandler.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-timeout"}`, "zh", "rid-timeout", 9)
	if timeoutResponse.Code != http.StatusGatewayTimeout || !strings.Contains(timeoutResponse.Body.String(), "超时") {
		t.Fatalf("timeout response = %d %s", timeoutResponse.Code, timeoutResponse.Body.String())
	}

	upstreamHandler := newMobileWebSearchHandler(mobileWebSearchProviderStub{err: &websearch.UpstreamStatusError{StatusCode: http.StatusBadGateway}}, true, time.Second)
	upstreamResponse := performMobileWebSearchRequest(upstreamHandler.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-upstream"}`, "en", "rid-upstream", 9)
	if upstreamResponse.Code != http.StatusBadGateway || !strings.Contains(upstreamResponse.Body.String(), "upstream") {
		t.Fatalf("upstream response = %d %s", upstreamResponse.Code, upstreamResponse.Body.String())
	}
}

func TestMobileWebSearchRequiresAuthenticatedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, true, time.Second)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mobile/web-search", strings.NewReader(`{"query":"ok","tool_call_id":"call-auth"}`))
	c.Request.Header.Set("Accept-Language", "en")
	c.Header("X-Request-ID", "rid-auth")
	h.Search(c)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestMobileWebSearchRequiresIdempotencyKeyAndReplaysWithoutRepayingBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	budget := &mobileWebSearchBudgetStub{}
	h := newMobileWebSearchHandlerWithBudget(mobileWebSearchProviderStub{response: &websearch.SearchResponse{
		Query:   "latest models",
		Results: []websearch.SearchResult{{Title: "Models", URL: "https://example.com/models"}},
	}}, true, time.Second, budget)

	missingKey := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"latest models","tool_call_id":"call-missing-key"}`, "en", "", 7)
	if missingKey.Code != http.StatusBadRequest || !strings.Contains(missingKey.Body.String(), "MOBILE_WEB_SEARCH_IDEMPOTENCY_KEY_REQUIRED") {
		t.Fatalf("missing key response = %d %s", missingKey.Code, missingKey.Body.String())
	}

	first := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"latest models","tool_call_id":"call-replay"}`, "en", "rid-search-replay", 7)
	second := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"latest models","tool_call_id":"call-replay"}`, "en", "rid-search-replay", 7)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("replay responses = %d/%d: %s / %s", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	if second.Header().Get("X-Idempotency-Replayed") != "true" {
		t.Fatalf("replayed response missing header: %v", second.Header())
	}
	if budget.calls != 1 {
		t.Fatalf("budget reserve calls = %d, want 1", budget.calls)
	}
}

func TestMobileWebSearchReportsBudgetAvailabilityAndLimitTruthfully(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)

	unavailable := newMobileWebSearchHandlerWithBudget(mobileWebSearchProviderStub{response: &websearch.SearchResponse{}}, true, time.Second, &mobileWebSearchBudgetStub{err: errMobileWebSearchBudgetUnavailable})
	unavailableResponse := performMobileWebSearchRequest(unavailable.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-budget-unavailable"}`, "zh", "rid-budget-unavailable", 9)
	if unavailableResponse.Code != http.StatusServiceUnavailable || !strings.Contains(unavailableResponse.Body.String(), "MOBILE_WEB_SEARCH_BUDGET_UNAVAILABLE") || !strings.Contains(unavailableResponse.Body.String(), "rid-budget-unavailable") {
		t.Fatalf("unavailable budget response = %d %s", unavailableResponse.Code, unavailableResponse.Body.String())
	}

	exceeded := newMobileWebSearchHandlerWithBudget(mobileWebSearchProviderStub{response: &websearch.SearchResponse{}}, true, time.Second, &mobileWebSearchBudgetStub{
		err:        newMobileWebSearchBudgetExceededError(17 * time.Second),
		retryAfter: 17 * time.Second,
	})
	exceededResponse := performMobileWebSearchRequest(exceeded.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-budget-exceeded"}`, "en", "rid-budget-exceeded", 9)
	if exceededResponse.Code != http.StatusTooManyRequests || !strings.Contains(exceededResponse.Body.String(), "MOBILE_WEB_SEARCH_BUDGET_EXCEEDED") {
		t.Fatalf("exceeded budget response = %d %s", exceededResponse.Code, exceededResponse.Body.String())
	}
	if exceededResponse.Header().Get("Retry-After") != "17" {
		t.Fatalf("retry-after = %q, want 17", exceededResponse.Header().Get("Retry-After"))
	}
}

func TestMobileWebSearchCountsAmbiguousUpstreamAttemptsAgainstBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	budget := &mobileWebSearchBudgetStub{}
	h := newMobileWebSearchHandlerWithBudget(mobileWebSearchProviderStub{err: context.DeadlineExceeded}, true, time.Second, budget)

	response := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"ok","tool_call_id":"call-upstream-attempt"}`, "en", "rid-upstream-attempt", 9)
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("upstream timeout response = %d %s", response.Code, response.Body.String())
	}
	if budget.calls != 1 {
		t.Fatalf("budget calls after ambiguous upstream timeout = %d, want 1", budget.calls)
	}
}

func TestMobileWebSearchReportsTheProviderThatActuallyReturnedResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	h := newMobileWebSearchHandlerWithBudgetAndName(mobileWebSearchProviderStub{response: &websearch.SearchResponse{
		Query:    "fallback query",
		Provider: "duckduckgo",
		Results:  []websearch.SearchResult{{Title: "Result", URL: "https://example.com"}},
	}}, true, "exa", time.Second, &mobileWebSearchBudgetStub{})

	response := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"fallback query","tool_call_id":"call-provider-fallback"}`, "en", "rid-provider-fallback", 9)
	if response.Code != http.StatusOK {
		t.Fatalf("fallback provider response = %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"provider":"duckduckgo"`) {
		t.Fatalf("response must report the provider that served results: %s", response.Body.String())
	}
}

func TestMobileWebSearchUsesShortRecoveryBackoffAndPreservesRetryMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	configureMobileWebSearchIdempotency(t)
	budget := &mobileWebSearchBudgetStub{}
	providerCalls := 0
	h := newMobileWebSearchHandlerWithBudget(mobileWebSearchProviderFunc(func(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error) {
		providerCalls++
		if providerCalls == 1 {
			return nil, &websearch.UpstreamStatusError{StatusCode: http.StatusBadGateway}
		}
		return &websearch.SearchResponse{
			Query:   "recoverable search",
			Results: []websearch.SearchResult{{Title: "Recovered", URL: "https://example.com/recovered"}},
		}, nil
	}), true, time.Second, budget)

	first := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"recoverable search","tool_call_id":"call-recovery"}`, "en", "rid-search-recovery", 9)
	if first.Code != http.StatusBadGateway {
		t.Fatalf("first upstream response = %d %s", first.Code, first.Body.String())
	}

	immediateRetry := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"recoverable search","tool_call_id":"call-recovery"}`, "en", "rid-search-recovery", 9)
	if immediateRetry.Code != http.StatusConflict || !strings.Contains(immediateRetry.Body.String(), "MOBILE_WEB_SEARCH_RETRY_BACKOFF") {
		t.Fatalf("immediate retry response = %d %s", immediateRetry.Code, immediateRetry.Body.String())
	}
	if immediateRetry.Header().Get("Retry-After") == "" || !strings.Contains(immediateRetry.Body.String(), `"retry_after"`) {
		t.Fatalf("retry backoff must expose retry metadata: headers=%v body=%s", immediateRetry.Header(), immediateRetry.Body.String())
	}

	time.Sleep(mobileWebSearchRetryBackoff + 40*time.Millisecond)
	recovered := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"recoverable search","tool_call_id":"call-recovery"}`, "en", "rid-search-recovery", 9)
	if recovered.Code != http.StatusOK {
		t.Fatalf("recovered retry response = %d %s", recovered.Code, recovered.Body.String())
	}
	if providerCalls != 2 || budget.calls != 2 {
		t.Fatalf("provider/budget calls = %d/%d, want 2/2", providerCalls, budget.calls)
	}
}

func performMobileWebSearchRequest(handler gin.HandlerFunc, method, body, language, requestID string, userID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/v1/mobile/web-search", strings.NewReader(body))
	c.Request.Header.Set("Accept-Language", language)
	if requestID != "" {
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Request.Header.Set("Idempotency-Key", requestID)
	}
	if userID > 0 {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	handler(c)
	return recorder
}
