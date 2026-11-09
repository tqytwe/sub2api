package service

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const aliyunAudioTestBase = "https://workspace.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"

func aliyunAudioJSONResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestAliyunFiletransUploadsMultipartWithoutLeakingCredentials(t *testing.T) {
	body, contentType := buildOpenAIAudioMultipart(t, "qwen-audio-3.1-asr-flash-filetrans", []byte("audio-file"))
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		aliyunAudioJSONResponse(`{"data":{"upload_host":"https://dashscope-upload.oss-cn-beijing.aliyuncs.com","upload_dir":"upload-dir","oss_access_key_id":"test-policy-key","signature":"test-signature","policy":"test-policy","x_oss_object_acl":"private","x_oss_forbid_overwrite":"true"}}`),
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))},
		aliyunAudioJSONResponse(`{"request_id":"submit-1","output":{"task_id":"task-1","task_status":"PENDING"}}`),
		aliyunAudioJSONResponse(`{"output":{"task_status":"SUCCEEDED","results":[{"subtask_status":"SUCCEEDED","transcription_url":"https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/result.json"}]},"usage":{"duration":1}}`),
		aliyunAudioJSONResponse(`{"transcripts":[{"text":"hello"}]}`),
	}}
	s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", bytes.NewReader(body))
	_, err := s.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(1, aliyunAudioTestBase), OpenAIAudioOperationTranscriptions, body, contentType, "")
	require.NoError(t, err)
	require.Equal(t, "/api/v1/uploads", upstream.requests[0].URL.Path)
	require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
	fields, file := readMultipartFields(t, upstream.bodies[1], upstream.requests[1].Header.Get("Content-Type"))
	require.Equal(t, []byte("audio-file"), file)
	require.Equal(t, "test-signature", fields["Signature"])
	require.True(t, strings.HasPrefix(gjson.GetBytes(upstream.bodies[2], "input.file_urls.0").String(), "oss://upload-dir/"))
	require.Equal(t, "enable", upstream.requests[2].Header.Get("X-DashScope-OssResourceResolve"))
}

func TestAliyunAudioRejectsPrivateArtifactsAndDoesNotRetryAcceptedTasks(t *testing.T) {
	for _, failed := range []string{
		`{"output":{"task_status":"FAILED"}}`,
		`{"output":{"task_status":"SUCCEEDED","results":[{"subtask_status":"FAILED"}]}}`,
		`{"output":{"task_status":"SUCCEEDED","results":[{"subtask_status":"SUCCEEDED","transcription_url":"http://127.0.0.1/private"}]}}`,
	} {
		u := &httpUpstreamRecorder{responses: []*http.Response{
			aliyunAudioJSONResponse(`{"output":{"task_id":"task-1"}}`), aliyunAudioJSONResponse(failed),
		}}
		s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: u}
		body := []byte(`{"model":"qwen-audio-3.1-asr-flash-filetrans","file_url":"https://example.com/audio.wav"}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", bytes.NewReader(body))
		_, err := s.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(1, aliyunAudioTestBase), OpenAIAudioOperationTranscriptions, body, "application/json", "")
		require.Error(t, err)
		var failover *UpstreamFailoverError
		require.False(t, errors.As(err, &failover))
		require.Len(t, u.requests, 2)
	}
}

func TestAliyunRealtimeAdaptsOpenAIPCMAndSessionFields(t *testing.T) {
	conn := &aliyunRealtimeConn{openAIWSClientConn: &openAIRealtimeTestConn{}}
	converted, err := conn.clientEvent(json.RawMessage(`{"type":"session.update","session":{"type":"realtime","output_modalities":["audio"],"audio":{"input":{"format":{"type":"audio/pcm","rate":24000},"transcription":{"model":"gpt-4o-transcribe","language":"zh"}},"output":{"format":{"type":"audio/pcm","rate":24000},"voice":"marin"}},"tools":[{"type":"function","name":"weather","parameters":{"type":"object"}}]}}`))
	require.NoError(t, err)
	require.Equal(t, "pcm", gjson.GetBytes(converted, "session.input_audio_format").String())
	require.Equal(t, "longanqian_v3.1", gjson.GetBytes(converted, "session.voice").String())
	require.Equal(t, "weather", gjson.GetBytes(converted, "session.tools.0.function.name").String())
	require.False(t, gjson.GetBytes(converted, "session.input_audio_transcription.model").Exists())
	require.False(t, gjson.GetBytes(converted, "session.audio").Exists())
	response, err := conn.serverEvent([]byte(`{"type":"response.audio.delta","delta":"AA=="}`))
	require.NoError(t, err)
	require.Equal(t, "response.output_audio.delta", gjson.GetBytes(response, "type").String())
}

func TestAliyunRealtimePCMResamplingPreservesFrameBoundaries(t *testing.T) {
	pcm := make([]byte, 24000*2)
	for i := 0; i < 24000; i++ {
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(i%1000-500)))
	}
	whole := (&aliyunPCMResampler{}).convert(pcm)
	var streamed []byte
	r := &aliyunPCMResampler{}
	for start := 0; start < len(pcm); start += 137 {
		streamed = append(streamed, r.convert(pcm[start:min(start+137, len(pcm))])...)
	}
	require.Len(t, whole, 16000*2)
	require.Equal(t, whole, streamed)
}

func TestAliyunRealtimeMapsAgentsDefaultTurnDetection(t *testing.T) {
	a := &aliyunRealtimeConn{}
	data, err := a.clientEvent([]byte(`{"type":"session.update","session":{"input_audio_format":"pcm16","turn_detection":{"type":"semantic_vad","interrupt_response":true,"eagerness":"auto"}}}`))
	require.NoError(t, err)
	require.Equal(t, "smart_turn_v2", gjson.GetBytes(data, "session.turn_detection.type").String())
	require.False(t, gjson.GetBytes(data, "session.turn_detection.eagerness").Exists())
}

func TestAliyunRealtimeInitialGreetingCanBeParsedByAgents(t *testing.T) {
	// The greeting arrives before the SDK has sent its session.update.
	a := &aliyunRealtimeConn{}
	data, err := a.serverEvent([]byte(`{"type":"session.created","session":{"id":"session-1","modalities":["text","audio"],"input_audio_format":"pcm","output_audio_format":"pcm"}}`))
	require.NoError(t, err)
	require.Equal(t, "realtime", gjson.GetBytes(data, "session.type").String())
	require.Equal(t, "pcm", gjson.GetBytes(data, "session.input_audio_format").String(), "native fields stay available")
}

func TestAliyunRealtimeConvertsAssistantContentForAgents(t *testing.T) {
	a := &aliyunRealtimeConn{}
	a.ga.Store(true)
	for _, event := range []string{
		`{"type":"response.output_item.done","item":{"type":"message","role":"assistant","content":[{"type":"text","text":"hello"},{"type":"audio","transcript":"hello"}]}}`,
		`{"type":"response.done","response":{"output":[{"type":"message","role":"assistant","content":[{"type":"text","text":"hello"},{"type":"audio","transcript":"hello"}]}]}}`,
	} {
		data, err := a.serverEvent([]byte(event))
		require.NoError(t, err)
		prefix := "item.content"
		if gjson.GetBytes(data, "type").String() == "response.done" {
			prefix = "response.output.0.content"
		}
		require.Equal(t, "output_text", gjson.GetBytes(data, prefix+".0.type").String())
		require.Equal(t, "output_audio", gjson.GetBytes(data, prefix+".1.type").String())
	}
	data, err := a.serverEvent([]byte(`{"type":"session.updated","session":{"turn_detection":{"type":"smart_turn_v2"}}}`))
	require.NoError(t, err)
	require.Equal(t, "semantic_vad", gjson.GetBytes(data, "session.audio.input.turn_detection.type").String())
}

func TestAliyunAudioSpeechConvertsNativeRequestAndDownloadsAudio(t *testing.T) {
	for _, model := range []string{"qwen-audio-3.1-tts-plus", "qwen-audio-3.1-tts-next"} {
		t.Run(model, func(t *testing.T) {
			body := []byte(`{"model":"` + model + `","input":"你好","voice":"ash","response_format":"wav","speed":1.2}`)
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				aliyunAudioJSONResponse(`{"request_id":"native-1","output":{"audio":{"url":"http://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/audio.wav"}},"usage":{"characters":2}}`),
				{StatusCode: 200, Header: http.Header{"Content-Type": {"audio/wav"}}, Body: io.NopCloser(bytes.NewReader([]byte("RIFF-audio")))},
			}}
			s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/audio/speech", bytes.NewReader(body))
			result, err := s.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(1, aliyunAudioTestBase), OpenAIAudioOperationSpeech, body, "application/json", "")
			require.NoError(t, err)
			require.Equal(t, "/api/v1/services/audio/tts/SpeechSynthesizer", upstream.requests[0].URL.Path)
			field := "input.text"
			if strings.HasSuffix(model, "-next") {
				field = "input.text_prompt"
				require.False(t, gjson.GetBytes(upstream.bodies[0], "input.voice").Exists())
			} else {
				require.Equal(t, "longanhuan_v3.1", gjson.GetBytes(upstream.bodies[0], "input.voice").String())
			}
			require.Equal(t, "你好", gjson.GetBytes(upstream.bodies[0], field).String())
			require.Equal(t, "wav", gjson.GetBytes(upstream.bodies[0], "input.format").String())
			require.Equal(t, int64(24000), gjson.GetBytes(upstream.bodies[0], "input.sample_rate").Int())
			require.InDelta(t, 1.2, gjson.GetBytes(upstream.bodies[0], "input.rate").Float(), 0.001)
			require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
			require.Equal(t, "https", upstream.requests[1].URL.Scheme)
			require.True(t, HTTPUpstreamPublicHostsOnly(upstream.requests[1].Context()))
			require.InDelta(t, 2e-6, result.AudioUsage.DurationOrUnits, 1e-10)
		})
	}
}

func TestAliyunRealtimeUsesNativeWebsocketPath(t *testing.T) {
	dialer := &openAIRealtimeTestDialer{}
	s := &OpenAIGatewayService{cfg: &config.Config{}, openaiWSPassthroughDialer: dialer}
	_, err := s.OpenOpenAIRealtime(context.Background(), newOpenAIAudioTestAccount(1, aliyunAudioTestBase), "qwen-audio-3.1-realtime-flash", http.Header{})
	require.NoError(t, err)
	require.Equal(t, "wss://workspace.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime?model=qwen-audio-3.1-realtime-flash", dialer.lastURL)
}

func TestAliyunFiletransPollsAndReturnsOpenAITranscription(t *testing.T) {
	body := []byte(`{"model":"qwen-audio-3.1-asr-flash-filetrans","file_url":"https://example.com/speech.wav","language":"zh"}`)
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		aliyunAudioJSONResponse(`{"request_id":"submit-1","output":{"task_id":"task-1","task_status":"PENDING"}}`),
		aliyunAudioJSONResponse(`{"output":{"task_status":"SUCCEEDED","results":[{"subtask_status":"SUCCEEDED","transcription_url":"https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/transcript.json"}]},"usage":{"duration":90}}`),
		aliyunAudioJSONResponse(`{"transcripts":[{"text":"你好世界","sentences":[]}],"properties":{"original_duration_in_milliseconds":90000}}`),
	}}
	s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", bytes.NewReader(body))
	result, err := s.ForwardOpenAIAudio(context.Background(), c, newOpenAIAudioTestAccount(1, aliyunAudioTestBase), OpenAIAudioOperationTranscriptions, body, "application/json", "")
	require.NoError(t, err)
	require.JSONEq(t, `{"text":"你好世界"}`, rec.Body.String())
	require.Equal(t, "/api/v1/services/audio/asr/transcription", upstream.requests[0].URL.Path)
	require.Equal(t, "enable", upstream.requests[0].Header.Get("X-DashScope-Async"))
	require.True(t, gjson.GetBytes(upstream.bodies[0], "parameters").IsObject())
	require.Equal(t, "/api/v1/tasks/task-1", upstream.requests[1].URL.Path)
	require.Empty(t, upstream.requests[2].Header.Get("Authorization"))
	require.InDelta(t, 90.0/3600, result.AudioUsage.DurationOrUnits, 1e-9)
}

type aliyunDisconnectUpstream struct {
	*httpUpstreamRecorder
	cancel                  context.CancelFunc
	cancelledDuringDownload bool
}

func (u *aliyunDisconnectUpstream) Do(req *http.Request, proxy string, id int64, concurrency int) (*http.Response, error) {
	if len(u.requests) == 0 {
		u.cancel()
	} else {
		u.cancelledDuringDownload = req.Context().Err() != nil
	}
	return u.httpUpstreamRecorder.Do(req, proxy, id, concurrency)
}

func TestAliyunAcceptedSpeechKeepsUsageAfterClientDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	u := &aliyunDisconnectUpstream{cancel: cancel, httpUpstreamRecorder: &httpUpstreamRecorder{responses: []*http.Response{
		aliyunAudioJSONResponse(`{"output":{"audio":{"url":"https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/audio.wav"}}}`),
		{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("RIFF-audio"))},
	}}}
	s := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: u}
	body := []byte(`{"model":"qwen-audio-3.1-tts-plus","input":"hello"}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/audio/speech", bytes.NewReader(body))
	result, err := s.ForwardOpenAIAudio(ctx, c, newOpenAIAudioTestAccount(1, aliyunAudioTestBase), OpenAIAudioOperationSpeech, body, "application/json", "")
	require.NoError(t, err)
	require.NotNil(t, result.AudioUsage)
	require.False(t, u.cancelledDuringDownload, "a disconnect must not discard the accepted request's billing evidence")
}
