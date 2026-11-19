package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	excelBPSImagesEndpoint      = "/basispoints/api/images/generations"
	excelBPSImagesEditsEndpoint = "/basispoints/api/images/edits"
)

// excelBPSImagesSupportedModel lists the models the BPS image endpoints accept.
// Every other direct-image model got 422 for 'model' in testing, so unlisted
// models stay on Codex instead of wasting a BPS round trip per request.
func excelBPSImagesSupportedModel(model string) bool {
	return strings.TrimSpace(model) == "gpt-image-2"
}

// excelBPSImagesUnsupportedReason reports why a request must stay on Codex.
// BPS answers 422 for any field outside model, prompt, size, quality, n,
// background and output_format; only png output and opaque/auto backgrounds
// were accepted in testing.
func excelBPSImagesUnsupportedReason(parsed *OpenAIImagesRequest) string {
	switch {
	case parsed == nil:
		return "missing request"
	case parsed.Stream:
		return "stream"
	case parsed.PartialImages != nil:
		return "partial_images"
	case parsed.OutputCompression != nil:
		return "output_compression"
	case strings.TrimSpace(parsed.Style) != "":
		return "style"
	case strings.TrimSpace(parsed.InputFidelity) != "":
		return "input_fidelity"
	}
	switch strings.ToLower(strings.TrimSpace(parsed.OutputFormat)) {
	case "", "png":
	default:
		return "output_format"
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Moderation)) {
	case "", "auto":
	default:
		return "moderation"
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Background)) {
	case "", "opaque", "auto":
	default:
		return "background"
	}
	if parsed.IsEdits() {
		return excelBPSImagesEditsUnsupportedReason(parsed)
	}
	return ""
}

// The BPS edit endpoint requires exactly one image file; masks and remote
// image URLs (which the gateway would have to download) stay on Codex.
func excelBPSImagesEditsUnsupportedReason(parsed *OpenAIImagesRequest) string {
	if parsed.HasMask || parsed.MaskUpload != nil || strings.TrimSpace(parsed.MaskImageURL) != "" {
		return "mask"
	}
	images := len(parsed.Uploads)
	for _, imageURL := range parsed.InputImageURLs {
		if imageURL = strings.TrimSpace(imageURL); imageURL == "" {
			continue
		}
		images++
		if _, _, err := decodeAccountTestDataURL(imageURL); err != nil {
			return "remote image_url"
		}
	}
	if images != 1 {
		return "image count"
	}
	return ""
}

// buildExcelBPSImagesPayload reuses the Codex generations body. moderation=auto
// is the upstream default, and BPS rejects the field itself.
func buildExcelBPSImagesPayload(parsed *OpenAIImagesRequest, model string) ([]byte, error) {
	body, _, err := buildOpenAIImagesOAuthPayload(parsed, model)
	if err != nil {
		return nil, err
	}
	return sjson.DeleteBytes(body, "moderation")
}

var excelBPSImagesQuoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, `\"`)

// buildExcelBPSImagesEditsBody builds the multipart edit request. Unlike the
// Codex edit endpoint, which takes JSON data URLs, BPS wants one file part
// plus the same form fields the generations endpoint accepts.
func buildExcelBPSImagesEditsBody(parsed *OpenAIImagesRequest, model string) ([]byte, string, error) {
	prompt := parsed.Prompt
	if !parsed.Multipart && gjson.ValidBytes(parsed.Body) {
		if rawPrompt := gjson.GetBytes(parsed.Body, "prompt").String(); rawPrompt != "" {
			prompt = rawPrompt
		}
	}
	if strings.TrimSpace(prompt) == "" {
		return nil, "", fmt.Errorf("prompt is required")
	}
	fileName, contentType, data := "", "", []byte(nil)
	if len(parsed.Uploads) > 0 {
		upload := parsed.Uploads[0]
		fileName, contentType, data = upload.FileName, upload.ContentType, upload.Data
	} else {
		for _, imageURL := range parsed.InputImageURLs {
			if imageURL = strings.TrimSpace(imageURL); imageURL == "" {
				continue
			}
			decoded, mimeType, err := decodeAccountTestDataURL(imageURL)
			if err != nil {
				return nil, "", err
			}
			contentType, data = mimeType, decoded
			break
		}
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("image input is required")
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(data)
	}
	if strings.TrimSpace(fileName) == "" {
		// BPS reads the part's Content-Type; the name is only cosmetic.
		fileName = "image.png"
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fields := [][2]string{{"model", model}, {"prompt", prompt},
		{"size", strings.TrimSpace(parsed.Size)}, {"quality", strings.TrimSpace(parsed.Quality)},
		{"background", strings.TrimSpace(parsed.Background)}, {"output_format", strings.TrimSpace(parsed.OutputFormat)}}
	if parsed.N > 1 {
		fields = append(fields, [2]string{"n", strconv.Itoa(parsed.N)})
	}
	for _, field := range fields {
		if field[1] == "" {
			continue
		}
		if err := writer.WriteField(field[0], field[1]); err != nil {
			return nil, "", err
		}
	}
	// CreateFormFile would hardcode application/octet-stream; keep the image MIME.
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="image"; filename="`+excelBPSImagesQuoteEscaper.Replace(fileName)+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(data); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

// The images endpoint checks the full Excel client identity, not only the
// product and agent profile that the Responses endpoint needs.
func newExcelBPSImagesRequest(ctx context.Context, upstreamURL, contentType string, body []byte, token, accountID string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	applyExcelBPSRequestHeaders(req, token, accountID)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range map[string]string{
		"X-Openai-Internal-Basispoints-Client-Platform":       "excel",
		"X-Openai-Internal-Basispoints-Client-Editor":         "excel",
		"X-Openai-Internal-Basispoints-Client-Host":           "office",
		"X-Openai-Internal-Basispoints-Client-Runtime":        "desktop",
		"X-Openai-Internal-Basispoints-Client-Platform-Class": "PC",
		"X-Openai-Internal-Basispoints-Office-Host":           "Excel",
		"X-Openai-Internal-Basispoints-Office-Platform":       "PC",
	} {
		req.Header.Set(key, value)
	}
	return req, nil
}

// excelBPSImagesFallbackStatus lists rejections of the request format. Nothing
// was generated or written yet, so the same request can still go to Codex.
// 413 covers edit uploads above whatever limit BPS enforces.
func excelBPSImagesFallbackStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusNotFound, http.StatusMethodNotAllowed,
		http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return true
	}
	return false
}

// forwardExcelBPSImages sends a text-to-image request to BPS. fallback=true
// means BPS rejected the format before generating anything; the caller then
// uses Codex. BPS failures never write Codex account state, and a request that
// may have reached BPS is never replayed.
func (s *OpenAIGatewayService) forwardExcelBPSImages(ctx context.Context, c *gin.Context, account *Account, parsed *OpenAIImagesRequest, requestModel, upstreamModel string, startTime time.Time) (*OpenAIForwardResult, bool, error) {
	endpoint, upstreamURL := excelBPSImagesEndpoint, basispoints.ImagesGenerationsURL
	if parsed.IsEdits() {
		endpoint, upstreamURL = excelBPSImagesEditsEndpoint, basispoints.ImagesEditsURL
	}
	fail := func(status int, code, message string) (*OpenAIForwardResult, bool, error) {
		SetActualOpenAIUpstreamEndpoint(c, endpoint)
		errorType := "invalid_request_error"
		if status >= 500 {
			errorType = "server_error"
		}
		upstreamErr := &OpenAIImagesUpstreamError{StatusCode: status, ErrorType: errorType, Code: code, Message: message}
		writeOpenAIImagesUpstreamErrorResponse(c, upstreamErr)
		return nil, false, upstreamErr
	}
	var body []byte
	var contentType string
	var err error
	if parsed.IsEdits() {
		body, contentType, err = buildExcelBPSImagesEditsBody(parsed, upstreamModel)
	} else {
		body, err = buildExcelBPSImagesPayload(parsed, upstreamModel)
	}
	if err != nil {
		return nil, false, err
	}
	credential, err := s.excelBPSRuntimeCredential(ctx, account)
	if err != nil {
		var typed *UpstreamFailoverError
		if errors.As(err, &typed) {
			return nil, false, err
		}
		return fail(http.StatusServiceUnavailable, "basispoints_auth_unavailable", "Excel BPS authorization is unavailable")
	}
	if credential.ChatGPTAccountID == "" {
		return fail(http.StatusBadRequest, "basispoints_account_id_missing", "Excel BPS requires a ChatGPT account ID")
	}
	requestCtx := WithHTTPUpstreamRedirectsDisabled(WithHTTPUpstreamProfile(ctx, HTTPUpstreamProfileLongStream))
	req, err := newExcelBPSImagesRequest(requestCtx, upstreamURL, contentType, body, credential.AccessToken, credential.ChatGPTAccountID)
	if err != nil {
		return nil, false, err
	}
	if err := s.fenceAuthConnectionEgress(requestCtx, req, account); err != nil {
		return nil, false, err
	}
	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	sent := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(sent).Milliseconds())
	if err != nil {
		return fail(http.StatusBadGateway, "basispoints_transport_error", "Excel BPS image request failed after sending")
	}
	defer func() { _ = resp.Body.Close() }()
	if excelBPSImagesFallbackStatus(resp.StatusCode) {
		return nil, true, nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		s.applyExcelBPSCooldown(ctx, account, resp.Header)
		return nil, false, newExcelBPSRateLimitedFailoverError(resp.Header.Get("Retry-After"))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
		code := gjson.GetBytes(raw, "error.code").String()
		message := s.handleExcelBPSAccessRejection(ctx, account, resp.StatusCode, code, upstreamModel)
		if !isExcelBPSModelPermissionError(resp.StatusCode, code) {
			code = "basispoints_upstream_error"
		}
		return fail(resp.StatusCode, code, message)
	}
	SetActualOpenAIUpstreamEndpoint(c, endpoint)
	s.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, resp.Header)
	usage, imageCount, imageOutputSizes, err := s.handleCodexDirectImagesNonStreamingResponse(resp, c, parsed, requestModel, upstreamModel)
	if err != nil {
		if !c.Writer.Written() {
			var upstreamErr *OpenAIImagesUpstreamError
			if errors.As(err, &upstreamErr) {
				writeOpenAIImagesUpstreamErrorResponse(c, upstreamErr)
			}
		}
		return nil, false, err
	}
	return &OpenAIForwardResult{
		RequestID: resp.Header.Get("x-request-id"), UpstreamHeaders: resp.Header.Clone(), ResponseHeaders: resp.Header.Clone(),
		Usage: usage, Model: requestModel, UpstreamModel: upstreamModel, UpstreamEndpoint: endpoint,
		UpstreamResponseModel: observedUpstreamResponseModel(c), UpstreamResponseModelConflict: observedUpstreamResponseModelConflict(c),
		Duration: time.Since(startTime), ImageCount: imageCount, ImageSize: parsed.SizeTier, ImageInputSize: parsed.Size, ImageOutputSizes: imageOutputSizes,
	}, false, nil
}
