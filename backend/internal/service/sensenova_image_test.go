package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
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

func TestSenseNovaImageCapabilitiesAreExactAndProviderBound(t *testing.T) {
	u15, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "sensenova-u1.5-lite")
	require.True(t, ok)
	require.Equal(t, PlatformOpenAI, u15.Platform)
	require.Equal(t, "sensenova", u15.ProviderID)
	require.Equal(t, "sensenova:sensenova-u1.5-lite:v1", u15.ProfileID)
	require.Equal(t, []string{"create", "edit"}, u15.Operations)
	require.Equal(t, "custom_dimensions", u15.SizingKind)
	require.Equal(t, 512, u15.MinDimension)
	require.Equal(t, 4096, u15.MaxDimension)
	require.Equal(t, 32, u15.DimensionStep)
	require.Equal(t, 3.0, u15.MaxAspectRatio)
	require.Equal(t, []string{"png", "jpeg", "webp"}, u15.SupportedOutputFormats)
	require.Equal(t, 4, u15.MaxReferenceImages)

	fast, ok := ResolveImageStudioProviderCapability(PlatformOpenAI, "sensenova-u1-fast")
	require.True(t, ok)
	require.Equal(t, "sensenova:sensenova-u1-fast:v1", fast.ProfileID)
	require.Equal(t, []string{"create"}, fast.Operations)
	require.Equal(t, "fixed", fast.SizingKind)
	require.Equal(t, "2752x1536", fast.DefaultSize)
	require.Equal(t, []string{
		"1664x2496", "2496x1664", "1760x2368", "2368x1760", "1824x2272", "2272x1824",
		"2048x2048", "2752x1536", "1536x2752", "3072x1376", "1344x3136",
	}, fast.SupportedSizes)

	for _, model := range []string{
		"sensenova-u1.5-lite-preview",
		"sensenova-u1-fast-preview",
		"sensenova-u1",
	} {
		t.Run(model, func(t *testing.T) {
			_, ok := ResolveImageStudioModelCapability(model)
			require.False(t, ok)
		})
	}

	for _, platform := range []string{PlatformGemini, PlatformGrok, "anthropic"} {
		t.Run(platform, func(t *testing.T) {
			_, ok := ResolveImageStudioProviderCapability(platform, "sensenova-u1.5-lite")
			require.False(t, ok)
		})
	}
}

func TestSenseNovaU15ValidatesDocumentedCustomDimensions(t *testing.T) {
	svc := &ImageStudioService{}
	groupID := int64(7)
	apiKey := &APIKey{
		GroupID: &groupID,
		Group:   &Group{ID: groupID, Platform: PlatformOpenAI},
	}

	for _, size := range []string{"512x512", "2720x1536", "4096x4096"} {
		t.Run(size, func(t *testing.T) {
			require.NoError(t, svc.ValidateSizeForModel(apiKey, "sensenova-u1.5-lite", size))
		})
	}
	for _, size := range []string{"auto", "500x500", "1025x1024", "2048x512", "4096x4128"} {
		t.Run(size, func(t *testing.T) {
			require.ErrorIs(t, svc.ValidateSizeForModel(apiKey, "sensenova-u1.5-lite", size), ErrImageStudioSizeNotSupported)
		})
	}
}

func TestSenseNovaU1FastUsesCapabilityDefaultWhenImageStudioSizeIsOmitted(t *testing.T) {
	svc := &ImageStudioService{}
	groupID := int64(7)
	apiKey := &APIKey{
		GroupID: &groupID,
		Group:   &Group{ID: groupID, Platform: PlatformOpenAI},
	}

	size, err := svc.resolveGenerateSize(
		apiKey,
		senseNovaU1FastModelID,
		ImageStudioGenerateRequest{},
		ImageStudioTemplate{},
	)

	require.NoError(t, err)
	require.Equal(t, defaultSenseNovaU1FastSize, size)
}

func TestSenseNovaOpenAIImagesRequestsUseDedicatedImageProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}

	for _, tt := range []struct {
		model string
		size  string
	}{
		{model: senseNovaU15LiteModelID, size: "1024x1024"},
		{model: senseNovaU1FastModelID, size: defaultSenseNovaU1FastSize},
	} {
		t.Run("parse "+tt.model, func(t *testing.T) {
			body := []byte(`{"model":"` + tt.model + `","prompt":"draw a diagram","size":"` + tt.size + `"}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, strings.NewReader(string(body)))
			c.Request.Header.Set("Content-Type", "application/json")

			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			require.Equal(t, tt.model, parsed.Model)
		})
	}

	for _, body := range [][]byte{
		[]byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":2}`),
		[]byte(`{"model":"sensenova-u1-fast","prompt":"draw","reference_image":{"url":"data:image/png;base64,aGVsbG8="}}`),
	} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")

		_, err := svc.ParseOpenAIImagesRequest(c, body)
		require.Error(t, err)
	}

	// SenseNova documents U1 Fast n as an integer without publishing a maximum.
	// Keep the ordinary OpenAI-compatible limit for other models, but do not make
	// up an upper bound for this exact provider model.
	fastCountBody := []byte(`{"model":"sensenova-u1-fast","prompt":"draw","n":11,"size":"2752x1536"}`)
	fastCountRec := httptest.NewRecorder()
	fastCountCtx, _ := gin.CreateTestContext(fastCountRec)
	fastCountCtx.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(fastCountBody))
	fastCountCtx.Request.Header.Set("Content-Type", "application/json")
	fastCount, err := svc.ParseOpenAIImagesRequest(fastCountCtx, fastCountBody)
	require.NoError(t, err)
	require.Equal(t, 11, fastCount.N)

	u15Body := []byte(`{"model":"public-image-alias","prompt":"edit the product","n":1,"size":"2720x1536","response_format":"url","output_format":"webp","watermark":false,"prompt_extend":false}`)
	u15Parsed := &OpenAIImagesRequest{
		Endpoint:       openAIImagesGenerationsEndpoint,
		ContentType:    "application/json",
		Model:          "public-image-alias",
		Prompt:         "edit the product",
		N:              1,
		Size:           "2720x1536",
		ResponseFormat: "url",
		OutputFormat:   "webp",
	}
	rewritten, contentType, handled, err := rewriteAdaptedOpenAIImagesBody(
		u15Body,
		"application/json",
		u15Parsed,
		"sensenova-u1.5-lite",
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "application/json", contentType)
	require.Equal(t, "sensenova-u1.5-lite", gjson.GetBytes(rewritten, "model").String())
	require.Equal(t, "2720x1536", gjson.GetBytes(rewritten, "size").String())
	require.Equal(t, "url", gjson.GetBytes(rewritten, "response_format").String())
	require.Equal(t, "webp", gjson.GetBytes(rewritten, "output_format").String())
	require.False(t, gjson.GetBytes(rewritten, "watermark").Bool())
	require.False(t, gjson.GetBytes(rewritten, "prompt_extend").Bool())

	u15Parsed.N = 2
	_, _, handled, err = rewriteAdaptedOpenAIImagesBody(u15Body, "application/json", u15Parsed, "sensenova-u1.5-lite")
	require.True(t, handled)
	require.ErrorContains(t, err, "n=1")

	fastParsed := &OpenAIImagesRequest{
		Endpoint:       openAIImagesGenerationsEndpoint,
		ContentType:    "application/json",
		Model:          "sensenova-u1-fast",
		Prompt:         "draw a diagram",
		N:              3,
		Size:           "2752x1536",
		ResponseFormat: "url",
		OutputFormat:   "webp",
	}
	fastBody := []byte(`{"model":"sensenova-u1-fast","prompt":"draw a diagram","n":3,"size":"2752x1536","watermark":true,"response_format":"url","output_format":"webp","prompt_extend":true,"quality":"high","background":"transparent","moderation":"auto","input_fidelity":"high","style":"vivid","output_compression":80,"partial_images":1,"extra_body":{"response_format":"url","output_format":"webp","prompt_extend":true}}`)
	rewritten, _, handled, err = rewriteAdaptedOpenAIImagesBody(fastBody, "application/json", fastParsed, "sensenova-u1-fast")
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, int64(3), gjson.GetBytes(rewritten, "n").Int())
	for _, path := range []string{
		"response_format",
		"output_format",
		"prompt_extend",
		"quality",
		"background",
		"moderation",
		"input_fidelity",
		"style",
		"output_compression",
		"partial_images",
		"extra_body",
	} {
		require.False(t, gjson.GetBytes(rewritten, path).Exists(), path)
	}

	fastParsed.Endpoint = openAIImagesEditsEndpoint
	_, _, handled, err = rewriteAdaptedOpenAIImagesBody(fastBody, "application/json", fastParsed, "sensenova-u1-fast")
	require.True(t, handled)
	require.ErrorIs(t, err, ErrImageStudioOperationNotSupported)

	for _, field := range []string{"image", "image_url", "input_image", "input_images", "reference_images"} {
		t.Run("fast rejects reference input "+field, func(t *testing.T) {
			body := []byte(`{"model":"sensenova-u1-fast","prompt":"draw","n":1,"` + field + `":{"url":"data:image/png;base64,aGVsbG8="}}`)
			parsed := &OpenAIImagesRequest{
				Endpoint: openAIImagesGenerationsEndpoint,
				N:        1,
			}

			_, _, handled, err := rewriteAdaptedOpenAIImagesBody(body, "application/json", parsed, "sensenova-u1-fast")
			require.True(t, handled)
			require.ErrorIs(t, err, ErrImageStudioOperationNotSupported)
		})
	}

	t.Run("fast rejects nested reference input", func(t *testing.T) {
		body := []byte(`{"model":"sensenova-u1-fast","prompt":"draw","n":1,"extra_body":{"reference_image":{"url":"data:image/png;base64,aGVsbG8="}}}`)
		parsed := &OpenAIImagesRequest{Endpoint: openAIImagesGenerationsEndpoint, N: 1}

		_, _, handled, err := rewriteAdaptedOpenAIImagesBody(body, "application/json", parsed, senseNovaU1FastModelID)
		require.True(t, handled)
		require.ErrorIs(t, err, ErrImageStudioOperationNotSupported)
	})

	for _, tt := range []struct {
		name          string
		body          []byte
		parsed        *OpenAIImagesRequest
		upstreamModel string
		want          error
	}{
		{
			name: "u15 rejects unsupported custom size",
			body: []byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":1,"size":"1025x1024"}`),
			parsed: &OpenAIImagesRequest{
				Endpoint: openAIImagesGenerationsEndpoint, N: 1, Size: "1025x1024",
			},
			upstreamModel: "sensenova-u1.5-lite",
			want:          ErrImageStudioSizeNotSupported,
		},
		{
			name: "u15 rejects unsupported output format",
			body: []byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":1,"output_format":"gif"}`),
			parsed: &OpenAIImagesRequest{
				Endpoint: openAIImagesGenerationsEndpoint, N: 1, OutputFormat: "gif",
			},
			upstreamModel: "sensenova-u1.5-lite",
			want:          ErrImageStudioOutputFormatNotSupported,
		},
		{
			name: "fast rejects undocumented size",
			body: []byte(`{"model":"sensenova-u1-fast","prompt":"draw","n":1,"size":"1024x1024"}`),
			parsed: &OpenAIImagesRequest{
				Endpoint: openAIImagesGenerationsEndpoint, N: 1, Size: "1024x1024",
			},
			upstreamModel: "sensenova-u1-fast",
			want:          ErrImageStudioSizeNotSupported,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, _, handled, err := rewriteAdaptedOpenAIImagesBody(tt.body, "application/json", tt.parsed, tt.upstreamModel)
			require.True(t, handled)
			require.ErrorIs(t, err, tt.want)
		})
	}

	autoParsed := &OpenAIImagesRequest{
		Endpoint: openAIImagesGenerationsEndpoint,
		N:        1,
		Size:     "auto",
	}
	_, _, handled, err = rewriteAdaptedOpenAIImagesBody(
		[]byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":1,"size":"auto"}`),
		"application/json",
		autoParsed,
		"sensenova-u1.5-lite",
	)
	require.True(t, handled)
	require.NoError(t, err)
}

func TestSenseNovaU15DirectMultipartEditConvertsToJSONBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	image := encodeImageStudioReferencePNG(t, 2, 2)
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	require.NoError(t, writer.WriteField("model", "sensenova-u1.5-lite"))
	require.NoError(t, writer.WriteField("prompt", "replace the background"))
	require.NoError(t, writer.WriteField("n", "1"))
	require.NoError(t, writer.WriteField("size", "2048x2048"))
	require.NoError(t, writer.WriteField("output_format", "webp"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
	require.NoError(t, writer.WriteField("watermark", "false"))
	require.NoError(t, writer.WriteField("prompt_extend", "false"))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	_, err = part.Write(image)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, openAIImagesEditsEndpoint, bytes.NewReader(multipartBody.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	svc := &OpenAIGatewayService{}
	parsed, err := svc.ParseOpenAIImagesRequest(c, multipartBody.Bytes())
	require.NoError(t, err)
	require.True(t, parsed.Multipart)

	rewritten, contentType, handled, err := rewriteAdaptedOpenAIImagesBody(
		multipartBody.Bytes(),
		writer.FormDataContentType(),
		parsed,
		"sensenova-u1.5-lite",
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, "application/json", contentType)
	require.Equal(t, "sensenova-u1.5-lite", gjson.GetBytes(rewritten, "model").String())
	require.Equal(t, "replace the background", gjson.GetBytes(rewritten, "prompt").String())
	require.Equal(t, int64(1), gjson.GetBytes(rewritten, "n").Int())
	require.Equal(t, "2048x2048", gjson.GetBytes(rewritten, "size").String())
	require.Equal(t, "webp", gjson.GetBytes(rewritten, "output_format").String())
	require.Equal(t, "b64_json", gjson.GetBytes(rewritten, "response_format").String())
	require.False(t, gjson.GetBytes(rewritten, "watermark").Bool())
	require.False(t, gjson.GetBytes(rewritten, "prompt_extend").Bool())
	require.Equal(t,
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(image),
		gjson.GetBytes(rewritten, "images.0.image_url").String(),
	)
	require.False(t, gjson.GetBytes(rewritten, "image").Exists())

	_, err = buildSenseNovaU15MultipartEditJSON(&OpenAIImagesRequest{
		Uploads: []OpenAIImagesUpload{{
			ContentType: "image/png",
			Data:        []byte("plain text disguised as an image"),
		}},
	}, "sensenova-u1.5-lite")
	require.ErrorIs(t, err, ErrImageStudioReferenceInvalid)

	// Exact U1.5 IDs are rejected while parsing, before the scheduler, billing,
	// credentials, or transport can be reached.
	badBody := []byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":2}`)
	badRec := httptest.NewRecorder()
	badCtx, _ := gin.CreateTestContext(badRec)
	badCtx.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(badBody))
	badCtx.Request.Header.Set("Content-Type", "application/json")
	_, err = svc.ParseOpenAIImagesRequest(badCtx, badBody)
	require.ErrorContains(t, err, "n=1")
	upstream := &httpUpstreamRecorder{}
	svc.httpUpstream = upstream
	require.Nil(t, upstream.lastReq)

	for _, tt := range []struct {
		name string
		body []byte
		want error
	}{
		{
			name: "u15 size",
			body: []byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":1,"size":"1025x1024"}`),
			want: ErrImageStudioSizeNotSupported,
		},
		{
			name: "u15 output format",
			body: []byte(`{"model":"sensenova-u1.5-lite","prompt":"draw","n":1,"output_format":"gif"}`),
			want: ErrImageStudioOutputFormatNotSupported,
		},
		{
			name: "fast size",
			body: []byte(`{"model":"sensenova-u1-fast","prompt":"draw","n":1,"size":"1024x1024"}`),
			want: ErrImageStudioSizeNotSupported,
		},
	} {
		t.Run(tt.name+" is rejected before scheduling and upstream", func(t *testing.T) {
			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			_, err := svc.ParseOpenAIImagesRequest(ctx, tt.body)
			require.ErrorIs(t, err, tt.want)
			upstream.lastReq = nil
			require.Nil(t, upstream.lastReq)
		})
	}
}

func TestSenseNovaFastDirectEditsAreRejectedBeforeUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"sensenova-u1-fast","prompt":"edit","images":[{"image_url":"data:image/png;base64,aGVsbG8="}]}`)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, openAIImagesEditsEndpoint, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	svc := &OpenAIGatewayService{httpUpstream: &httpUpstreamRecorder{}}
	_, err := svc.ParseOpenAIImagesRequest(c, body)
	require.ErrorIs(t, err, ErrImageStudioOperationNotSupported)
	upstream, ok := svc.httpUpstream.(*httpUpstreamRecorder)
	require.True(t, ok)
	require.Nil(t, upstream.lastReq)
}

func TestImageStudioCreatePendingJobBuildsSenseNovaPayloads(t *testing.T) {
	u15Repo := &imageStudioCreateRepoStub{}
	u15Encryptor := &imageStudioEncryptorStub{}
	u15Service := newImageStudioProviderCreateServiceForTest(
		u15Repo,
		u15Encryptor,
		PlatformOpenAI,
		[]string{"sensenova-u1.5-lite"},
	)
	u15Job, _, err := u15Service.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID:   "free-create",
		UserPrompt:   "product launch artwork",
		Size:         "2720x1536",
		Count:        1,
		Model:        "sensenova-u1.5-lite",
		OutputFormat: "webp",
		APIKeyID:     20,
	})
	require.NoError(t, err)
	require.NotNil(t, u15Job)
	require.Equal(t, "sensenova:sensenova-u1.5-lite:v1", gjson.Get(u15Encryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, openAIImagesGenerationsEndpoint, gjson.Get(u15Encryptor.plaintext, "endpoint").String())
	require.Equal(t, "sensenova-u1.5-lite", gjson.Get(u15Encryptor.plaintext, "body.model").String())
	require.Equal(t, "2720x1536", gjson.Get(u15Encryptor.plaintext, "body.size").String())
	require.Equal(t, "webp", gjson.Get(u15Encryptor.plaintext, "body.output_format").String())
	require.Equal(t, "b64_json", gjson.Get(u15Encryptor.plaintext, "body.response_format").String())
	require.True(t, gjson.Get(u15Encryptor.plaintext, "body.watermark").Bool())
	require.True(t, gjson.Get(u15Encryptor.plaintext, "body.prompt_extend").Bool())

	fastRepo := &imageStudioCreateRepoStub{}
	fastEncryptor := &imageStudioEncryptorStub{}
	fastService := newImageStudioProviderCreateServiceForTest(
		fastRepo,
		fastEncryptor,
		PlatformOpenAI,
		[]string{"sensenova-u1-fast"},
	)
	fastJob, _, err := fastService.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID: "free-create",
		UserPrompt: "product launch artwork",
		Size:       "2752x1536",
		Count:      1,
		Model:      "sensenova-u1-fast",
		APIKeyID:   20,
	})
	require.NoError(t, err)
	require.NotNil(t, fastJob)
	require.Equal(t, "sensenova:sensenova-u1-fast:v1", gjson.Get(fastEncryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, "2752x1536", gjson.Get(fastEncryptor.plaintext, "body.size").String())
	require.True(t, gjson.Get(fastEncryptor.plaintext, "body.watermark").Bool())
	require.False(t, gjson.Get(fastEncryptor.plaintext, "body.response_format").Exists())
}

func TestImageStudioBuildWorkerRequestBuildsSenseNovaU15EditDataURLs(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	image := encodeImageStudioReferencePNG(t, 2, 2)
	storageKey, err := store.Save(10, "sensenova-job-ref", "image/png", image)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{jobRefs: []ImageStudioJobReference{{
		ID:          "sensenova-job-ref",
		JobID:       "sensenova-job",
		StorageKey:  storageKey,
		ContentType: "image/png",
		ByteSize:    int64(len(image)),
	}}}
	svc := &ImageStudioService{repo: repo, assetStore: store}
	decrypted := `{
		"platform":"openai",
		"operation":"edit",
		"capability_profile_id":"sensenova:sensenova-u1.5-lite:v1",
		"capability_revision":"` + imageStudioCapabilityRevision + `",
		"endpoint":"/v1/images/edits",
		"body":{
			"model":"sensenova-u1.5-lite",
			"prompt":"replace the background",
			"n":2,
			"size":"2048x2048",
			"watermark":true,
			"image_studio_job_reference_ids":["sensenova-job-ref"]
		}
	}`

	req, err := svc.BuildWorkerRequest(context.Background(), &ImageStudioJob{
		ID: "sensenova-job", UserID: 10, Model: "sensenova-u1.5-lite",
	}, decrypted)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, req.Platform)
	require.Equal(t, "edit", req.Operation)
	require.Equal(t, openAIImagesEditsEndpoint, req.Endpoint)
	require.Equal(t, "application/json", req.ContentType)
	require.Equal(t, int64(1), gjson.GetBytes(req.Body, "n").Int())
	require.Equal(t,
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(image),
		gjson.GetBytes(req.Body, "images.0.image_url").String(),
	)
	require.False(t, gjson.GetBytes(req.Body, "image_studio_job_reference_ids").Exists())
	require.NotContains(t, string(req.Body), storageKey)

	fastEdit := `{
		"platform":"openai",
		"operation":"edit",
		"capability_profile_id":"sensenova:sensenova-u1-fast:v1",
		"capability_revision":"` + imageStudioCapabilityRevision + `",
		"endpoint":"/v1/images/edits",
		"body":{"model":"sensenova-u1-fast","prompt":"replace","n":1}
	}`
	_, err = (&ImageStudioService{}).BuildWorkerRequest(context.Background(), &ImageStudioJob{Model: "sensenova-u1-fast"}, fastEdit)
	require.ErrorIs(t, err, ErrImageStudioOperationNotSupported)
}

func TestAccountTestServiceSenseNovaModelsUseImagesGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("u15 mapped model sends documented image payload", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"b64_json":"aGVsbG8="}]}`)),
		}}
		svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
		account := &Account{
			ID:       77,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "test-api-key",
				"base_url": "https://token.sensenova.cn/v1",
				"model_mapping": map[string]any{
					"public-u15": "sensenova-u1.5-lite",
				},
			},
		}

		err := svc.testOpenAIAccountConnection(c, account, "public-u15", "", "")
		require.NoError(t, err)
		require.NotNil(t, upstream.lastReq)
		require.Equal(t, "https://token.sensenova.cn/v1/images/generations", upstream.lastReq.URL.String())
		require.Equal(t, "Bearer test-api-key", upstream.lastReq.Header.Get("Authorization"))
		body, err := io.ReadAll(upstream.lastReq.Body)
		require.NoError(t, err)
		require.Equal(t, "sensenova-u1.5-lite", gjson.GetBytes(body, "model").String())
		require.Equal(t, "1024x1024", gjson.GetBytes(body, "size").String())
		require.Equal(t, "png", gjson.GetBytes(body, "output_format").String())
		require.True(t, gjson.GetBytes(body, "watermark").Bool())
		require.True(t, gjson.GetBytes(body, "prompt_extend").Bool())
	})

	t.Run("fast model accepts provider URL result without unsupported response_format", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/1/test", nil)
		upstream := &httpUpstreamRecorder{resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[{"url":"https://cdn.sensenova.example/generated.png"}]}`)),
		}}
		svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
		account := &Account{
			ID:       78,
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"api_key":  "test-api-key",
				"base_url": "https://token.sensenova.cn/v1",
			},
		}

		err := svc.testOpenAIAccountConnection(c, account, "sensenova-u1-fast", "", "")
		require.NoError(t, err)
		require.NotNil(t, upstream.lastReq)
		body, err := io.ReadAll(upstream.lastReq.Body)
		require.NoError(t, err)
		require.Equal(t, "sensenova-u1-fast", gjson.GetBytes(body, "model").String())
		require.Equal(t, "2752x1536", gjson.GetBytes(body, "size").String())
		require.False(t, gjson.GetBytes(body, "response_format").Exists())
		require.Contains(t, rec.Body.String(), "https://cdn.sensenova.example/generated.png")
	})
}
