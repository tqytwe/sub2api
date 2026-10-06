//go:build unit

package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// openAIAudioHTTPUpstream answers per account: statusByAccount overrides the
// default 200 audio response.
type openAIAudioHTTPUpstream struct {
	service.HTTPUpstream
	mu              sync.Mutex
	accountIDs      []int64
	urls            []string
	statusByAccount map[int64]int
	okContentType   string
	okBody          []byte
}

func (u *openAIAudioHTTPUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.urls = append(u.urls, req.URL.String())
	status := u.statusByAccount[accountID]
	u.mu.Unlock()
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
	}
	if status >= 400 {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream unavailable","type":"server_error"}}`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{u.okContentType}, "X-Request-Id": []string{"audio-rid"}},
		Body:       io.NopCloser(bytes.NewReader(u.okBody)),
	}, nil
}

func (u *openAIAudioHTTPUpstream) calls() ([]int64, []string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...), append([]string(nil), u.urls...)
}

func newOpenAIAudioTestHandler(t *testing.T, accounts []service.Account, upstream service.HTTPUpstream) *OpenAIGatewayHandler {
	t.Helper()
	cfg := &config.Config{RunMode: config.RunModeSimple}
	// Local httptest upstreams are plain http/ws.
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	gatewayService := service.NewOpenAIGatewayService(
		openAIImagesFailoverAccountRepo{accounts: accounts},
		nil, nil, nil, nil, nil, nil,
		cfg,
		nil, nil, nil, nil, nil,
		upstream,
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	billingService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingService.Stop)
	h := NewOpenAIGatewayHandler(
		gatewayService,
		service.NewConcurrencyService(nil),
		billingService,
		service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg),
		nil, nil, nil, nil,
		cfg,
	)
	h.maxAccountSwitches = 5
	return h
}

func openAIAudioTestAccount(id int64, accountType string, priority int) service.Account {
	creds := map[string]any{"api_key": "sk-" + string(rune('a'+id))}
	if accountType == service.AccountTypeOAuth {
		creds = map[string]any{"access_token": "oauth-token"}
	}
	return service.Account{
		ID:          id,
		Name:        "audio-account",
		Platform:    service.PlatformOpenAI,
		Type:        accountType,
		Status:      service.StatusActive,
		Schedulable: true,
		Priority:    priority,
		Credentials: creds,
	}
}

func newOpenAIAudioTestContext(method, path, contentType string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	groupID := int64(4242)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
		ID:      77,
		GroupID: &groupID,
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
		User:    &service.User{ID: 88},
	})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 88})
	return c, rec
}

func TestOpenAIAudioSpeech_SkipsOAuthAndFailsOverToHealthyAPIKeyAccount(t *testing.T) {
	audio := []byte{0xff, 0xfb, 0x10, 0x20}
	upstream := &openAIAudioHTTPUpstream{
		statusByAccount: map[int64]int{2: http.StatusInternalServerError},
		okContentType:   "audio/mpeg",
		okBody:          audio,
	}
	h := newOpenAIAudioTestHandler(t, []service.Account{
		openAIAudioTestAccount(1, service.AccountTypeOAuth, 0),
		openAIAudioTestAccount(2, service.AccountTypeAPIKey, 1),
		openAIAudioTestAccount(3, service.AccountTypeAPIKey, 2),
	}, upstream)
	c, rec := newOpenAIAudioTestContext(http.MethodPost, "/v1/audio/speech", "application/json",
		[]byte(`{"model":"tts-1","input":"hello","voice":"alloy"}`))

	h.AudioSpeech(c)

	ids, urls := upstream.calls()
	require.Equal(t, []int64{2, 3}, ids, "OAuth accounts must be skipped and 5xx must fail over")
	require.Equal(t, "https://api.openai.com/v1/audio/speech", urls[len(urls)-1])
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "audio/mpeg", rec.Header().Get("Content-Type"))
	require.Equal(t, audio, rec.Body.Bytes())
}

func TestOpenAIAudioTranscriptions_ForwardsMultipartToOpenAI(t *testing.T) {
	upstream := &openAIAudioHTTPUpstream{okContentType: "application/json", okBody: []byte(`{"text":"hi"}`)}
	h := newOpenAIAudioTestHandler(t, []service.Account{openAIAudioTestAccount(5, service.AccountTypeAPIKey, 0)}, upstream)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("model", "whisper-1"))
	part, err := w.CreateFormFile("file", "a.mp3")
	require.NoError(t, err)
	_, _ = part.Write([]byte("audio-bytes"))
	require.NoError(t, w.Close())
	c, rec := newOpenAIAudioTestContext(http.MethodPost, "/v1/audio/transcriptions", w.FormDataContentType(), buf.Bytes())

	h.AudioTranscriptions(c)

	ids, urls := upstream.calls()
	require.Equal(t, []int64{5}, ids)
	require.Equal(t, "https://api.openai.com/v1/audio/transcriptions", urls[0])
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"text":"hi"}`, rec.Body.String())
}

func TestOpenAIAudioTranslations_NoAPIKeyAccountReturns503WithoutUpstreamCall(t *testing.T) {
	upstream := &openAIAudioHTTPUpstream{okContentType: "application/json", okBody: []byte(`{}`)}
	h := newOpenAIAudioTestHandler(t, []service.Account{openAIAudioTestAccount(6, service.AccountTypeOAuth, 0)}, upstream)
	c, rec := newOpenAIAudioTestContext(http.MethodPost, "/v1/audio/translations", "application/json", []byte(`{"model":"whisper-1"}`))

	h.AudioTranslations(c)

	ids, _ := upstream.calls()
	require.Empty(t, ids)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestOpenAIAudioSpeech_MissingModelReturns400(t *testing.T) {
	upstream := &openAIAudioHTTPUpstream{okContentType: "audio/mpeg"}
	h := newOpenAIAudioTestHandler(t, []service.Account{openAIAudioTestAccount(7, service.AccountTypeAPIKey, 0)}, upstream)
	c, rec := newOpenAIAudioTestContext(http.MethodPost, "/v1/audio/speech", "application/json", []byte(`{"input":"hi"}`))

	h.AudioSpeech(c)

	ids, _ := upstream.calls()
	require.Empty(t, ids)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpenAIRealtime_NoAPIKeyAccountReturns503BeforeUpgrade(t *testing.T) {
	upstream := &openAIAudioHTTPUpstream{}
	h := newOpenAIAudioTestHandler(t, []service.Account{openAIAudioTestAccount(8, service.AccountTypeOAuth, 0)}, upstream)
	c, rec := newOpenAIAudioTestContext(http.MethodGet, "/v1/realtime?model=gpt-realtime", "", nil)
	c.Request.Header.Set("Connection", "Upgrade")
	c.Request.Header.Set("Upgrade", "websocket")
	c.Request.Header.Set("Sec-WebSocket-Version", "13")
	c.Request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	h.Realtime(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
}

func TestOpenAIRealtime_RelaysEventsBetweenClientAndOpenAIUpstream(t *testing.T) {
	type upstreamSeen struct {
		auth, beta, model string
		clientEvent       string
	}
	seenCh := make(chan upstreamSeen, 1)
	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen := upstreamSeen{
			auth:  r.Header.Get("Authorization"),
			beta:  r.Header.Get("OpenAI-Beta"),
			model: r.URL.Query().Get("model"),
		}
		if r.URL.Path != "/v1/realtime" {
			http.NotFound(w, r)
			return
		}
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		_, msg, err := conn.Read(ctx)
		if err != nil {
			return
		}
		seen.clientEvent = string(msg)
		seenCh <- seen
		_ = conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.output_audio.delta","delta":"BBBB"}`))
		_, _, _ = conn.Read(ctx) // wait for the gateway to close
	}))
	defer upstreamSrv.Close()

	account := openAIAudioTestAccount(31, service.AccountTypeAPIKey, 0)
	account.Credentials["base_url"] = upstreamSrv.URL
	h := newOpenAIAudioTestHandler(t, []service.Account{account}, &openAIAudioHTTPUpstream{})

	groupID := int64(4243)
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{
			ID: 78, GroupID: &groupID,
			Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI},
			User:  &service.User{ID: 89},
		})
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 89})
		h.Realtime(c)
	}))
	defer gatewaySrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(gatewaySrv.URL, "http") + "/v1/realtime?model=gpt-realtime"
	client, resp, err := coderws.Dial(ctx, wsURL, &coderws.DialOptions{
		HTTPHeader: http.Header{"OpenAI-Beta": []string{"realtime=v1"}},
	})
	require.NoError(t, err, "gateway should accept the realtime upgrade")
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	defer func() { _ = client.CloseNow() }()

	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"input_audio_buffer.append","audio":"AAAA"}`)))
	_, reply, err := client.Read(ctx)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"response.output_audio.delta","delta":"BBBB"}`, string(reply))

	seen := <-seenCh
	require.Equal(t, "Bearer "+account.Credentials["api_key"].(string), seen.auth)
	require.Equal(t, "realtime=v1", seen.beta)
	require.Equal(t, "gpt-realtime", seen.model)
	require.JSONEq(t, `{"type":"input_audio_buffer.append","audio":"AAAA"}`, seen.clientEvent)
	_ = client.Close(coderws.StatusNormalClosure, "done")
}

func TestOpenAIRealtimeBillingResult(t *testing.T) {
	require.Nil(t, openAIRealtimeBillingResult("gpt-realtime", time.Minute, false))
	require.Nil(t, openAIRealtimeBillingResult("gpt-realtime", 0, true))
	first := openAIRealtimeBillingResult("gpt-realtime", 90*time.Second, true)
	second := openAIRealtimeBillingResult("gpt-realtime", 90*time.Second, true)
	require.NotNil(t, first)
	require.NotEqual(t, first.RequestID, second.RequestID)
	require.True(t, strings.HasPrefix(first.RequestID, "openai_realtime:"))
	require.Equal(t, "realtime", first.AudioUsage.Mode)
	require.InDelta(t, 1.5, first.AudioUsage.DurationOrUnits, 1e-9)
	require.Equal(t, "gpt-realtime", first.Model)
}
