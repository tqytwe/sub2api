package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const aliyunAudioMaxArtifactBytes = 64 << 20

// Detect the provider by the configured host, never by the account's display
// name or the public model alias. Other OpenAI-compatible relays stay untouched.
func isAliyunAudioBase(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "dashscope.aliyuncs.com" || host == "dashscope-intl.aliyuncs.com" ||
		host == "dashscope-us.aliyuncs.com" || strings.HasSuffix(host, ".maas.aliyuncs.com")
}

func aliyunAudioURL(base, endpoint string) string {
	u, _ := url.Parse(base) // caller has already validated the configured base
	u.Path, u.RawPath, u.RawQuery, u.Fragment = endpoint, "", "", ""
	return u.String()
}

func aliyunAudioClientError(c *gin.Context, message string) error {
	writeOpenAIEmbeddingsError(c, http.StatusBadRequest, "invalid_request_error", message)
	return errors.New(message)
}

func (s *OpenAIGatewayService) forwardAliyunAudio(ctx context.Context, c *gin.Context, account *Account, op OpenAIAudioOperation, body []byte, contentType, base, originalModel, billingModel, model string) (*OpenAIForwardResult, error) {
	started := time.Now()
	// Preserve the existing audio gateway's post-disconnect usage collection.
	// The native task may already be billable; do not abandon its final result.
	ctx, release := detachUpstreamContext(ctx)
	defer release()
	// Filetrans is asynchronous upstream; bound the synchronous wrapper.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var requestID string
	var usage *AudioUsage
	var err error
	switch {
	case op == OpenAIAudioOperationSpeech && (model == "qwen-audio-3.1-tts-plus" || model == "qwen-audio-3.1-tts-next"):
		_, requestID, usage, err = s.aliyunSpeech(ctx, c, account, base, model, body)
	case op == OpenAIAudioOperationTranscriptions && model == "qwen-audio-3.1-asr-flash-filetrans":
		_, requestID, usage, err = s.aliyunFiletrans(ctx, c, account, base, model, body, contentType)
	default:
		return nil, aliyunAudioClientError(c, "This Aliyun model does not support the requested audio operation")
	}
	if err != nil {
		var failover *UpstreamFailoverError
		if !errors.As(err, &failover) && !c.Writer.Written() {
			writeOpenAIEmbeddingsError(c, http.StatusBadGateway, "upstream_error", "Aliyun audio request failed")
		}
		return nil, err
	}
	return &OpenAIForwardResult{
		RequestID: StableOpenAIAudioBillingRequestID(requestID), Model: originalModel,
		BillingModel: billingModel, UpstreamModel: model, Duration: time.Since(started), AudioUsage: usage,
	}, nil
}

// Only native API requests carry the account credential. Storage requests use
// an independent, auth-free request with public-address checks on every hop.
func (s *OpenAIGatewayService) aliyunAudioRequest(ctx context.Context, c *gin.Context, account *Account, method, target, model string, body []byte, headers http.Header, allowFailover bool) ([]byte, error) {
	ctx = WithHTTPUpstreamRedirectsDisabled(ctx)
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("build Aliyun audio request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.GetOpenAIProtocolAPIKey())
	account.ApplyHeaderOverrides(req.Header)
	for k, v := range headers {
		req.Header[k] = v
	}
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	started := time.Now()
	resp, err := s.doOpenAIUpstream(req, proxy, account)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, errors.New("aliyun audio transport failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 && allowFailover {
		return nil, s.handleOpenAIAudioErrorResponse(ctx, c, account, resp, model)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// After task submission never fail over and create another paid task.
		return nil, fmt.Errorf("aliyun audio returned status %d", resp.StatusCode)
	}
	data, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, errors.New("read Aliyun audio response failed")
	}
	if !gjson.ValidBytes(data) || gjson.GetBytes(data, "code").String() != "" {
		return nil, errors.New("aliyun audio returned an invalid result")
	}
	return data, nil
}

func (s *OpenAIGatewayService) aliyunAudioArtifact(ctx context.Context, account *Account, rawURL string, method, contentType string, body []byte) ([]byte, error) {
	// DashScope still returns HTTP OSS links. OSS serves the same signed
	// object via TLS; never weaken the platform's outbound HTTPS policy.
	if u, err := url.Parse(rawURL); err == nil && u.Scheme == "http" && strings.HasSuffix(strings.ToLower(u.Hostname()), ".aliyuncs.com") {
		u.Scheme = "https"
		rawURL = u.String()
	}
	target, err := s.validateOutboundURL(rawURL)
	if err != nil {
		return nil, errors.New("invalid Aliyun artifact URL")
	}
	if err := rejectPrivateImageHost(target); err != nil {
		return nil, errors.New("aliyun artifact host is not public")
	}
	ctx = WithHTTPUpstreamPublicHostsOnly(ctx)
	// Uploads contain signed policy fields: do not forward them via redirects.
	if method != http.MethodGet {
		ctx = WithHTTPUpstreamRedirectsDisabled(ctx)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("build Aliyun artifact request failed")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	if err != nil {
		return nil, errors.New("aliyun artifact transport failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("aliyun artifact returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, aliyunAudioMaxArtifactBytes+1))
	if err != nil || len(data) > aliyunAudioMaxArtifactBytes {
		return nil, errors.New("aliyun artifact is unreadable or exceeds 64 MiB")
	}
	return data, nil
}

func (s *OpenAIGatewayService) aliyunSpeech(ctx context.Context, c *gin.Context, account *Account, base, model string, body []byte) ([]byte, string, *AudioUsage, error) {
	text := ExtractOpenAIAudioSpeechInput(body)
	if !gjson.ValidBytes(body) || gjson.GetBytes(body, "input").Type != gjson.String || text == "" {
		return nil, "", nil, aliyunAudioClientError(c, "input must be a non-empty string")
	}
	format := gjson.GetBytes(body, "response_format").String()
	if format == "" {
		format = "mp3"
	}
	ct := map[string]string{"mp3": "audio/mpeg", "wav": "audio/wav", "pcm": "audio/pcm", "opus": "audio/ogg"}[format]
	if ct == "" || (model == "qwen-audio-3.1-tts-next" && format == "opus") {
		return nil, "", nil, aliyunAudioClientError(c, "Unsupported response_format for this Aliyun model; use mp3, wav or pcm")
	}
	if v := gjson.GetBytes(body, "stream_format"); v.Exists() && v.String() != "audio" {
		return nil, "", nil, aliyunAudioClientError(c, "This Aliyun adapter supports stream_format=audio")
	}
	input := map[string]any{"format": format, "sample_rate": 24000}
	if model == "qwen-audio-3.1-tts-next" {
		input["text_prompt"] = text
	} else {
		input["text"] = text
		voice := gjson.GetBytes(body, "voice").String()
		if voice == "" || isOpenAIAudioVoice(voice) {
			voice = "longanhuan_v3.1"
		}
		input["voice"] = voice
	}
	if v := gjson.GetBytes(body, "speed"); v.Exists() {
		input["rate"] = v.Value()
	}
	if v := gjson.GetBytes(body, "instructions"); v.Exists() && model != "qwen-audio-3.1-tts-next" {
		input["instruction"] = v.Value()
	}
	for _, k := range []string{"sample_rate", "volume", "pitch", "seed", "language_hints", "enable_ssml", "references", "channels"} {
		if v := gjson.GetBytes(body, k); v.Exists() {
			input[k] = v.Value()
		}
	}
	native, err := json.Marshal(map[string]any{"model": model, "input": input})
	if err != nil {
		return nil, "", nil, aliyunAudioClientError(c, "Invalid speech request")
	}
	data, err := s.aliyunAudioRequest(ctx, c, account, http.MethodPost, aliyunAudioURL(base, "/api/v1/services/audio/tts/SpeechSynthesizer"), model, native, nil, true)
	if err != nil {
		return nil, "", nil, err
	}
	audioURL := gjson.GetBytes(data, "output.audio.url").String()
	if audioURL == "" {
		return nil, "", nil, errors.New("aliyun synthesis returned no audio URL")
	}
	audio, err := s.aliyunAudioArtifact(ctx, account, audioURL, http.MethodGet, "", nil)
	if err != nil {
		return nil, "", nil, err
	}
	if len(audio) == 0 {
		return nil, "", nil, errors.New("aliyun synthesis returned empty audio")
	}
	c.Data(http.StatusOK, ct, audio)
	return data, gjson.GetBytes(data, "request_id").String(), estimateOpenAIAudioUsage(OpenAIAudioOperationSpeech, body, nil), nil
}

type aliyunTranscriptionInput struct {
	URL, Filename, Format, Language, Prompt string
	File                                    []byte
}

func parseAliyunTranscription(body []byte, contentType string) (aliyunTranscriptionInput, error) {
	var input aliyunTranscriptionInput
	media, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return input, errors.New("invalid transcription Content-Type")
	}
	fields := map[string]string{}
	if media == "multipart/form-data" {
		if params["boundary"] == "" {
			return input, errors.New("missing multipart boundary")
		}
		r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := r.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return input, errors.New("invalid multipart audio request")
			}
			data, err := io.ReadAll(part)
			_ = part.Close()
			if err != nil {
				return input, errors.New("invalid multipart audio field")
			}
			if part.FormName() == "file" && part.FileName() != "" {
				if input.Filename != "" {
					return input, errors.New("exactly one audio file is supported")
				}
				input.File, input.Filename = data, path.Base(part.FileName())
			} else {
				fields[part.FormName()] = string(data)
			}
		}
	} else if media == "application/json" && gjson.ValidBytes(body) {
		for _, key := range []string{"file_url", "response_format", "language", "prompt"} {
			fields[key] = gjson.GetBytes(body, key).String()
		}
	} else {
		return input, errors.New("use multipart/form-data with file or application/json with file_url")
	}
	input.URL, input.Format, input.Language, input.Prompt = fields["file_url"], fields["response_format"], fields["language"], fields["prompt"]
	if input.Format == "" {
		input.Format = "json"
	}
	switch input.Format {
	case "json", "verbose_json", "text", "srt", "vtt":
	default:
		return input, errors.New("unsupported transcription response_format")
	}
	if (input.URL == "") == (len(input.File) == 0) {
		return input, errors.New("provide exactly one non-empty file or file_url")
	}
	if input.URL != "" {
		u, err := url.Parse(input.URL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
			return input, errors.New("file_url must be a public HTTP or HTTPS audio URL")
		}
		if err := rejectPrivateImageHost(input.URL); err != nil {
			return input, errors.New("file_url must be public")
		}
	}
	return input, nil
}

func (s *OpenAIGatewayService) aliyunUploadAudio(ctx context.Context, c *gin.Context, account *Account, base, model string, input aliyunTranscriptionInput) (string, error) {
	policyURL := aliyunAudioURL(base, "/api/v1/uploads") + "?action=getPolicy&model=" + url.QueryEscape(model)
	data, err := s.aliyunAudioRequest(ctx, c, account, http.MethodGet, policyURL, model, nil, nil, true)
	if err != nil {
		return "", err
	}
	cert := gjson.GetBytes(data, "data")
	for _, k := range []string{"upload_host", "upload_dir", "oss_access_key_id", "signature", "policy"} {
		if cert.Get(k).String() == "" {
			return "", errors.New("aliyun upload policy is incomplete")
		}
	}
	u, err := url.Parse(cert.Get("upload_host").String())
	if err != nil || u.User != nil || !strings.HasSuffix(strings.ToLower(u.Hostname()), ".aliyuncs.com") || u.Scheme != "https" {
		return "", errors.New("aliyun upload policy host is invalid")
	}
	// Unique filenames avoid cross-user collisions under a reusable policy.
	filename := generateRequestID() + strings.ToLower(path.Ext(input.Filename))
	key := strings.TrimRight(cert.Get("upload_dir").String(), "/") + "/" + filename
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := map[string]string{
		"OSSAccessKeyId": cert.Get("oss_access_key_id").String(), "Signature": cert.Get("signature").String(),
		"policy": cert.Get("policy").String(), "key": key, "success_action_status": "200",
		"x-oss-object-acl": cert.Get("x_oss_object_acl").String(), "x-oss-forbid-overwrite": cert.Get("x_oss_forbid_overwrite").String(),
		"x-oss-content-type": mime.TypeByExtension(path.Ext(filename)),
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return "", errors.New("encode Aliyun upload failed")
		}
	}
	p, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", errors.New("encode Aliyun file failed")
	}
	if _, err := p.Write(input.File); err != nil {
		return "", errors.New("encode Aliyun file failed")
	}
	if err := w.Close(); err != nil {
		return "", errors.New("encode Aliyun upload failed")
	}
	if _, err = s.aliyunAudioArtifact(ctx, account, u.String(), http.MethodPost, w.FormDataContentType(), buf.Bytes()); err != nil {
		return "", err
	}
	return "oss://" + key, nil
}

func (s *OpenAIGatewayService) aliyunFiletrans(ctx context.Context, c *gin.Context, account *Account, base, model string, body []byte, contentType string) ([]byte, string, *AudioUsage, error) {
	input, err := parseAliyunTranscription(body, contentType)
	if err != nil {
		return nil, "", nil, aliyunAudioClientError(c, err.Error())
	}
	headers := http.Header{"X-Dashscope-Async": {"enable"}}
	if len(input.File) > 0 {
		input.URL, err = s.aliyunUploadAudio(ctx, c, account, base, model, input)
		if err != nil {
			return nil, "", nil, err
		}
		headers.Set("X-DashScope-OssResourceResolve", "enable")
	}
	parameters := map[string]any{}
	if input.Language != "" {
		parameters["language_hints"] = []string{input.Language}
	}
	nativeInput := map[string]any{"file_urls": []string{input.URL}}
	if input.Prompt != "" {
		nativeInput["context"] = []any{map[string]any{"role": "user", "content": []any{map[string]string{"type": "input_text", "text": input.Prompt}}}}
	}
	native, err := json.Marshal(map[string]any{"model": model, "input": nativeInput, "parameters": parameters})
	if err != nil {
		return nil, "", nil, aliyunAudioClientError(c, "Invalid transcription request")
	}
	data, err := s.aliyunAudioRequest(ctx, c, account, http.MethodPost, aliyunAudioURL(base, "/api/v1/services/audio/asr/transcription"), model, native, headers, true)
	if err != nil {
		return nil, "", nil, err
	}
	id := gjson.GetBytes(data, "request_id").String()
	taskID := gjson.GetBytes(data, "output.task_id").String()
	if taskID == "" || strings.ContainsAny(taskID, "/\\?#%") {
		return nil, "", nil, errors.New("aliyun returned an invalid task ID")
	}
	for {
		data, err = s.aliyunAudioRequest(ctx, c, account, http.MethodGet, aliyunAudioURL(base, "/api/v1/tasks/"+taskID), model, nil, nil, false)
		if err != nil {
			return nil, "", nil, err
		}
		switch gjson.GetBytes(data, "output.task_status").String() {
		case "SUCCEEDED":
			results := gjson.GetBytes(data, "output.results").Array()
			if len(results) != 1 || results[0].Get("subtask_status").String() != "SUCCEEDED" {
				return nil, "", nil, errors.New("aliyun transcription subtask failed")
			}
			transcript, err := s.aliyunAudioArtifact(ctx, account, results[0].Get("transcription_url").String(), http.MethodGet, "", nil)
			if err != nil {
				return nil, "", nil, err
			}
			secs := gjson.GetBytes(data, "usage.duration").Float()
			if secs <= 0 {
				secs = gjson.GetBytes(transcript, "properties.original_duration_in_milliseconds").Float() / 1000
			}
			if err := writeAliyunTranscript(c, input, transcript, secs); err != nil {
				return nil, "", nil, err
			}
			return data, id, &AudioUsage{Mode: "stt", DurationOrUnits: secs / 3600}, nil
		case "PENDING", "RUNNING":
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, "", nil, errors.New("aliyun transcription timed out or caller disconnected")
			case <-timer.C:
			}
		default:
			return nil, "", nil, errors.New("aliyun transcription task failed")
		}
	}
}

func writeAliyunTranscript(c *gin.Context, input aliyunTranscriptionInput, data []byte, seconds float64) error {
	if !gjson.ValidBytes(data) || !gjson.GetBytes(data, "transcripts").IsArray() {
		return errors.New("aliyun returned an invalid transcript")
	}
	var text strings.Builder
	var segments []map[string]any
	var subtitles strings.Builder
	if input.Format == "vtt" {
		_, _ = subtitles.WriteString("WEBVTT\n\n")
	}
	index := 0
	for _, transcript := range gjson.GetBytes(data, "transcripts").Array() {
		if text.Len() > 0 {
			_ = text.WriteByte('\n')
		}
		_, _ = text.WriteString(transcript.Get("text").String())
		for _, sentence := range transcript.Get("sentences").Array() {
			begin, end := sentence.Get("begin_time").Int(), sentence.Get("end_time").Int()
			line := sentence.Get("text").String()
			segments = append(segments, map[string]any{"id": index, "start": float64(begin) / 1000, "end": float64(end) / 1000, "text": line})
			index++
			separator := ","
			if input.Format == "vtt" {
				separator = "."
			}
			fmt.Fprintf(&subtitles, "%d\n%s --> %s\n%s\n\n", index, aliyunSubtitleTime(begin, separator), aliyunSubtitleTime(end, separator), line)
		}
	}
	switch input.Format {
	case "text":
		c.Data(200, "text/plain; charset=utf-8", []byte(text.String()))
	case "srt":
		c.Data(200, "application/x-subrip; charset=utf-8", []byte(subtitles.String()))
	case "vtt":
		c.Data(200, "text/vtt; charset=utf-8", []byte(subtitles.String()))
	case "verbose_json":
		if segments == nil {
			segments = []map[string]any{}
		}
		c.JSON(200, gin.H{"task": "transcribe", "language": input.Language, "duration": seconds, "text": text.String(), "segments": segments})
	default:
		c.JSON(200, gin.H{"text": text.String()})
	}
	return nil
}

func aliyunSubtitleTime(ms int64, separator string) string {
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3600000, ms/60000%60, ms/1000%60, separator, ms%1000)
}
