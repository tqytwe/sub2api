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
	"github.com/gin-gonic/gin"
)

type mobileWebSearchProviderStub struct {
	response *websearch.SearchResponse
	err      error
}

func (s mobileWebSearchProviderStub) Search(context.Context, websearch.SearchRequest) (*websearch.SearchResponse, error) {
	return s.response, s.err
}

func TestMobileWebSearchRequiresOptInAndReturnsLocalizedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{response: &websearch.SearchResponse{
		Query:   "golang",
		Results: []websearch.SearchResult{{Title: "Go", URL: "https://go.dev", Snippet: "The Go language"}},
	}}, true, time.Second)

	recorder := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"golang","opt_in":true}`, "en-US", "rid-search-1", 7)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Request-ID") != "rid-search-1" {
		t.Fatalf("request ID header = %q", recorder.Header().Get("X-Request-ID"))
	}
	if !strings.Contains(recorder.Body.String(), `"request_id":"rid-search-1"`) {
		t.Fatalf("response must include request ID: %s", recorder.Body.String())
	}

	withoutOptIn := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"golang"}`, "zh-CN", "rid-search-2", 7)
	if withoutOptIn.Code != http.StatusBadRequest {
		t.Fatalf("opt-in status = %d, want 400", withoutOptIn.Code)
	}
	if !strings.Contains(withoutOptIn.Body.String(), "请先确认启用联网搜索") {
		t.Fatalf("localized opt-in message missing: %s", withoutOptIn.Body.String())
	}
}

func TestMobileWebSearchRejectsBoundsAndDisabledConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, true, time.Second)
	tooLong := `{"query":"` + strings.Repeat("x", mobileWebSearchMaxQueryRunes+1) + `","opt_in":true}`
	tooLongResponse := performMobileWebSearchRequest(h.Search, http.MethodPost, tooLong, "zh-CN", "rid-long", 9)
	if tooLongResponse.Code != http.StatusBadRequest || !strings.Contains(tooLongResponse.Body.String(), "搜索内容过长") {
		t.Fatalf("query bound response = %d %s", tooLongResponse.Code, tooLongResponse.Body.String())
	}

	tooMany := performMobileWebSearchRequest(h.Search, http.MethodPost, `{"query":"ok","max_results":11,"opt_in":true}`, "en", "rid-many", 9)
	if tooMany.Code != http.StatusBadRequest || !strings.Contains(strings.ToLower(tooMany.Body.String()), "maximum") {
		t.Fatalf("result bound response = %d %s", tooMany.Code, tooMany.Body.String())
	}

	disabled := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, false, time.Second)
	disabledResponse := performMobileWebSearchRequest(disabled.Search, http.MethodPost, `{"query":"ok","opt_in":true}`, "en", "rid-off", 9)
	if disabledResponse.Code != http.StatusServiceUnavailable || !strings.Contains(disabledResponse.Body.String(), "not available") {
		t.Fatalf("disabled response = %d %s", disabledResponse.Code, disabledResponse.Body.String())
	}
}

func TestMobileWebSearchClassifiesTimeoutAndUpstreamStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	timeoutHandler := newMobileWebSearchHandler(mobileWebSearchProviderStub{err: context.DeadlineExceeded}, true, time.Second)
	timeoutResponse := performMobileWebSearchRequest(timeoutHandler.Search, http.MethodPost, `{"query":"ok","opt_in":true}`, "zh", "rid-timeout", 9)
	if timeoutResponse.Code != http.StatusGatewayTimeout || !strings.Contains(timeoutResponse.Body.String(), "超时") {
		t.Fatalf("timeout response = %d %s", timeoutResponse.Code, timeoutResponse.Body.String())
	}

	upstreamHandler := newMobileWebSearchHandler(mobileWebSearchProviderStub{err: &websearch.UpstreamStatusError{StatusCode: http.StatusBadGateway}}, true, time.Second)
	upstreamResponse := performMobileWebSearchRequest(upstreamHandler.Search, http.MethodPost, `{"query":"ok","opt_in":true}`, "en", "rid-upstream", 9)
	if upstreamResponse.Code != http.StatusBadGateway || !strings.Contains(upstreamResponse.Body.String(), "upstream") {
		t.Fatalf("upstream response = %d %s", upstreamResponse.Code, upstreamResponse.Body.String())
	}
}

func TestMobileWebSearchRequiresAuthenticatedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newMobileWebSearchHandler(mobileWebSearchProviderStub{}, true, time.Second)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mobile/web-search", strings.NewReader(`{"query":"ok","opt_in":true}`))
	c.Request.Header.Set("Accept-Language", "en")
	c.Header("X-Request-ID", "rid-auth")
	h.Search(c)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func performMobileWebSearchRequest(handler gin.HandlerFunc, method, body, language, requestID string, userID int64) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/v1/mobile/web-search", strings.NewReader(body))
	c.Request.Header.Set("Accept-Language", language)
	c.Request.Header.Set("X-Request-ID", requestID)
	if userID > 0 {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
	}
	handler(c)
	return recorder
}
