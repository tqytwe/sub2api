package service

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/basispoints"
	"github.com/stretchr/testify/require"
)

func TestExcelBPSDirectImageUsesSelectedCredential(t *testing.T) {
	account := excelAccount()
	account.Extra["openai_excel_bps_models"] = []string{"gpt-image-2"}
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jv5sAAAAASUVORK5CYII="}],"usage":{"input_tokens":20,"output_tokens":10}}`)),
	}}
	svc := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-image-2","available":true}]}}`, true, upstream)
	c, recorder := newExcelBPSContext()
	parsed := &OpenAIImagesRequest{Endpoint: "/v1/images/generations", Model: "gpt-image-2", Prompt: "Draw a circle", N: 1, Size: "1024x1024", ResponseFormat: "b64_json"}
	result, err := svc.ForwardImages(c.Request.Context(), c, account, nil, parsed, "")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, basispoints.ImagesGenerationsURL, upstream.request.URL.String())
	require.Equal(t, "Bearer excel-only-access", upstream.request.Header.Get("Authorization"))
	require.Empty(t, upstream.request.Header.Get("Cookie"))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "/basispoints/api/images/generations", result.UpstreamEndpoint)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 20, result.Usage.InputTokens)
}

func TestExcelBPSDirectImageEditRoutesAndBillsOneResult(t *testing.T) {
	const fixtureImage = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jv5sAAAAASUVORK5CYII="
	input, err := base64.StdEncoding.DecodeString(fixtureImage)
	require.NoError(t, err)
	account := excelAccount()
	account.Extra["openai_excel_bps_models"] = []string{"gpt-image-2"}
	upstream := &excelBPSHTTPUpstreamStub{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"` + fixtureImage + `"}],"usage":{"input_tokens":28,"output_tokens":10}}`)),
	}}
	svc := newExcelBPSRuntime(t, account, `{"allowed":true,"model_catalog":{"models":[{"id":"gpt-image-2","available":true}]}}`, true, upstream)
	c, recorder := newExcelBPSContext()
	parsed := &OpenAIImagesRequest{
		Endpoint: "/v1/images/edits", Model: "gpt-image-2", Prompt: "Add a border", N: 1, ResponseFormat: "b64_json",
		Uploads: []OpenAIImagesUpload{{FileName: "source.png", ContentType: "image/png", Data: input}},
	}
	result, err := svc.ForwardImages(c.Request.Context(), c, account, nil, parsed, "")
	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, basispoints.ImagesEditsURL, upstream.request.URL.String())
	mediaType, params, err := mime.ParseMediaType(upstream.request.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(bytes.NewReader(upstream.body), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, form.RemoveAll()) })
	require.Equal(t, []string{"Add a border"}, form.Value["prompt"])
	require.Len(t, form.File["image"], 1)
	require.Equal(t, "image/png", form.File["image"][0].Header.Get("Content-Type"))
	file, err := form.File["image"][0].Open()
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	delivered, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, input, delivered)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "/basispoints/api/images/edits", result.UpstreamEndpoint)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, 28, result.Usage.InputTokens)
}
