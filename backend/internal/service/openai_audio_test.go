package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newOpenAIAudioTestAccount(id int64, baseURL string) *Account {
	return &Account{
		ID:          id,
		Name:        "openai-audio",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"api_key":  "sk-audio-test",
			"base_url": baseURL,
		},
	}
}

func buildOpenAIAudioMultipart(t *testing.T, model string, audio []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("model", model))
	require.NoError(t, w.WriteField("response_format", "json"))
	part, err := w.CreateFormFile("file", "sample.mp3")
	require.NoError(t, err)
	_, err = part.Write(audio)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes(), w.FormDataContentType()
}

func readMultipartFields(t *testing.T, body []byte, contentType string) (map[string]string, []byte) {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)
	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	fields := map[string]string{}
	var file []byte
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		data, err := io.ReadAll(p)
		require.NoError(t, err)
		if p.FileName() != "" {
			file = data
			continue
		}
		fields[p.FormName()] = string(data)
	}
	return fields, file
}

func TestAccountSupportsOpenAIAudioCapability(t *testing.T) {
	apiKey := newOpenAIAudioTestAccount(1, "https://api.openai.com")
	require.True(t, apiKey.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio))

	chatOnly := newOpenAIAudioTestAccount(2, "https://api.openai.com")
	chatOnly.Credentials["openai_capabilities"] = []any{"chat_completions"}
	require.True(t, chatOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio),
		"chat-capable API key upstreams should also serve the standard audio API")

	embeddingsOnly := newOpenAIAudioTestAccount(3, "https://api.jina.ai")
	embeddingsOnly.Credentials["openai_capabilities"] = []any{"embeddings"}
	require.False(t, embeddingsOnly.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio),
		"embeddings-only upstreams must not receive audio requests")

	oauth := &Account{ID: 4, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.False(t, oauth.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio),
		"ChatGPT OAuth upstreams do not expose /v1/audio")

	grok := &Account{ID: 5, Platform: PlatformGrok, Type: AccountTypeAPIKey}
	require.False(t, grok.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityAudio))
}

func TestForwardOpenAIAudio_SpeechPassesThroughBinaryAndBillsCharacters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reqBody := []byte(`{"model":"tts-1","input":"你好世界","voice":"alloy"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")

	audio := []byte{0xff, 0xfb, 0x90, 0x00, 0x01, 0x02}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"audio/mpeg"},
			"X-Request-Id": []string{"tts-rid"},
		},
		Body: io.NopCloser(bytes.NewReader(audio)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := newOpenAIAudioTestAccount(11, "https://api.openai.com")

	result, err := svc.ForwardOpenAIAudio(context.Background(), c, account, OpenAIAudioOperationSpeech, reqBody, "application/json", "")
	require.NoError(t, err)

	require.Equal(t, "https://api.openai.com/v1/audio/speech", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-audio-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "tts-1", gjson.GetBytes(upstream.lastBody, "model").String())

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "audio/mpeg", rec.Header().Get("Content-Type"))
	require.Equal(t, audio, rec.Body.Bytes())

	require.NotNil(t, result)
	require.Equal(t, "tts-1", result.Model)
	require.NotNil(t, result.AudioUsage)
	require.Equal(t, "tts", result.AudioUsage.Mode)
	require.InDelta(t, 4.0/1_000_000.0, result.AudioUsage.DurationOrUnits, 1e-12)
	require.True(t, strings.HasPrefix(result.RequestID, "openai_audio:"), result.RequestID)
}

func TestForwardOpenAIAudio_TranscriptionRewritesMultipartModelAndBillsDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	audio := bytes.Repeat([]byte{0x01}, 1024)
	reqBody, contentType := buildOpenAIAudioMultipart(t, "whisper-1", audio)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(reqBody))
	c.Request.Header.Set("Content-Type", contentType)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"text":"hello","usage":{"type":"duration","seconds":90}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	account := newOpenAIAudioTestAccount(12, "https://relay.example.com/v1")
	account.Credentials["model_mapping"] = map[string]any{"whisper-1": "whisper-large-v3"}

	result, err := svc.ForwardOpenAIAudio(context.Background(), c, account, OpenAIAudioOperationTranscriptions, reqBody, contentType, "")
	require.NoError(t, err)

	require.Equal(t, "https://relay.example.com/v1/audio/transcriptions", upstream.lastReq.URL.String())
	fields, file := readMultipartFields(t, upstream.lastBody, upstream.lastReq.Header.Get("Content-Type"))
	require.Equal(t, "whisper-large-v3", fields["model"])
	require.Equal(t, "json", fields["response_format"])
	require.Equal(t, audio, file)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "hello", gjson.GetBytes(rec.Body.Bytes(), "text").String())
	require.NotNil(t, result.AudioUsage)
	require.Equal(t, "stt", result.AudioUsage.Mode)
	require.InDelta(t, 90.0/3600.0, result.AudioUsage.DurationOrUnits, 1e-9)
	require.Equal(t, "whisper-1", result.Model)
	require.Equal(t, "whisper-large-v3", result.UpstreamModel)
}

func TestForwardOpenAIAudio_TranslationsUsesTranslationsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reqBody, contentType := buildOpenAIAudioMultipart(t, "whisper-1", []byte("abc"))
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/translations", bytes.NewReader(reqBody))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"text":"hi","duration":12.5}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}

	result, err := svc.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(13, ""), OpenAIAudioOperationTranslations, reqBody, contentType, "")
	require.NoError(t, err)
	require.Equal(t, "https://api.openai.com/v1/audio/translations", upstream.lastReq.URL.String())
	require.NotNil(t, result.AudioUsage)
	require.Equal(t, "stt", result.AudioUsage.Mode)
	require.InDelta(t, 12.5/3600.0, result.AudioUsage.DurationOrUnits, 1e-9)
}

func TestForwardOpenAIAudio_MissingModelIsRejectedWithoutUpstreamCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reqBody := []byte(`{"input":"hi"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(reqBody))

	upstream := &httpUpstreamRecorder{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	_, err := svc.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(14, ""), OpenAIAudioOperationSpeech, reqBody, "application/json", "")
	require.Error(t, err)
	require.Nil(t, upstream.lastReq)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestForwardOpenAIAudio_UpstreamServerErrorReturnsFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reqBody := []byte(`{"model":"tts-1","input":"hi","voice":"alloy"}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewReader(reqBody))

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"boom","type":"server_error"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	_, err := svc.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(15, ""), OpenAIAudioOperationSpeech, reqBody, "application/json", "")
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "expected failover error, got %v", err)
	require.Equal(t, http.StatusInternalServerError, failoverErr.StatusCode)
	require.False(t, c.Writer.Written(), "failover must not commit the client response")
}

type openAIRealtimeTestDialer struct {
	lastURL     string
	lastHeaders http.Header
	status      int
	err         error
}

func (d *openAIRealtimeTestDialer) Dial(_ context.Context, wsURL string, headers http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.lastURL = wsURL
	d.lastHeaders = headers.Clone()
	if d.err != nil {
		return nil, d.status, nil, d.err
	}
	return &openAIRealtimeTestConn{}, 0, nil, nil
}

type openAIRealtimeTestConn struct{}

func (c *openAIRealtimeTestConn) WriteJSON(context.Context, any) error { return nil }
func (c *openAIRealtimeTestConn) ReadMessage(context.Context) ([]byte, error) {
	return nil, context.DeadlineExceeded
}
func (c *openAIRealtimeTestConn) Ping(context.Context) error { return nil }
func (c *openAIRealtimeTestConn) Close() error               { return nil }

func TestOpenOpenAIRealtime_DialsWSWithAuthAndModel(t *testing.T) {
	dialer := &openAIRealtimeTestDialer{}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: dialer}
	account := newOpenAIAudioTestAccount(21, "https://relay.example.com/v1")

	clientHeaders := http.Header{"Openai-Beta": []string{"realtime=v1"}}
	upstream, err := svc.OpenOpenAIRealtime(context.Background(), account, "gpt-realtime", clientHeaders)
	require.NoError(t, err)
	require.NotNil(t, upstream)
	require.Equal(t, "wss://relay.example.com/v1/realtime?model=gpt-realtime", dialer.lastURL)
	require.Equal(t, "Bearer sk-audio-test", dialer.lastHeaders.Get("Authorization"))
	require.Equal(t, "realtime=v1", dialer.lastHeaders.Get("OpenAI-Beta"))
}

func TestOpenOpenAIRealtime_DefaultBaseAndDialStatus(t *testing.T) {
	dialer := &openAIRealtimeTestDialer{err: errors.New("unauthorized"), status: http.StatusUnauthorized}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: dialer}

	_, err := svc.OpenOpenAIRealtime(context.Background(), newOpenAIAudioTestAccount(22, ""), "gpt-realtime", nil)
	require.Error(t, err)
	require.Equal(t, "wss://api.openai.com/v1/realtime?model=gpt-realtime", dialer.lastURL)
	var dialErr *GrokRealtimeDialError
	require.True(t, errors.As(err, &dialErr))
	require.Equal(t, http.StatusUnauthorized, dialErr.StatusCode)
}

func TestOpenAIRealtimeEventHasAudio(t *testing.T) {
	require.True(t, grokRealtimeEventHasAudio([]byte(`{"type":"response.output_audio.delta","delta":"AAAA"}`)))
	require.True(t, grokRealtimeEventHasAudio([]byte(`{"type":"input_audio_buffer.append","audio":"AAAA"}`)))
	require.False(t, grokRealtimeEventHasAudio([]byte(`{"type":"response.output_audio_transcript.delta","delta":"hi"}`)))
}

func TestStableOpenAIAudioBillingRequestIDsAreForcedAndUnique(t *testing.T) {
	a := StableOpenAIAudioBillingRequestID("")
	b := StableOpenAIAudioBillingRequestID("")
	require.NotEqual(t, a, b)
	require.True(t, isForcedUsageBillingRequestID(a))
	require.Equal(t, a, StableOpenAIAudioBillingRequestID(a))

	r := StableOpenAIRealtimeBillingRequestID("")
	require.True(t, strings.HasPrefix(r, "openai_realtime:"))
	require.True(t, isForcedUsageBillingRequestID(r))
}
