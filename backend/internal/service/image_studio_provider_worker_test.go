package service

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImageStudioCreatePendingJobPinsOpenAIProviderProfile(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	encryptor := &imageStudioEncryptorStub{}
	svc := newImageStudioProviderCreateServiceForTest(
		repo,
		encryptor,
		PlatformOpenAI,
		[]string{"gpt-image-1.5"},
	)
	compression := 82

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID:        "free-create",
		UserPrompt:        "transparent product render",
		Size:              "1024x1024",
		Count:             1,
		Model:             "gpt-image-1.5",
		Quality:           "high",
		APIKeyID:          20,
		Background:        "transparent",
		OutputFormat:      "webp",
		OutputCompression: &compression,
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, PlatformOpenAI, gjson.Get(encryptor.plaintext, "platform").String())
	require.Equal(t, "create", gjson.Get(encryptor.plaintext, "operation").String())
	require.Equal(t, "openai:gpt-image-1.5:v1", gjson.Get(encryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, imageStudioCapabilityRevision, gjson.Get(encryptor.plaintext, "capability_revision").String())
	require.Equal(t, openAIImagesGenerationsEndpoint, gjson.Get(encryptor.plaintext, "endpoint").String())
	require.Equal(t, "gpt-image-1.5", gjson.Get(encryptor.plaintext, "body.model").String())
	require.Equal(t, "transparent", gjson.Get(encryptor.plaintext, "body.background").String())
	require.Equal(t, "webp", gjson.Get(encryptor.plaintext, "body.output_format").String())
	require.Equal(t, int64(82), gjson.Get(encryptor.plaintext, "body.output_compression").Int())
}

func TestImageStudioCreatePendingJobRejectsProviderOptionMismatch(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	svc := newImageStudioProviderCreateServiceForTest(
		repo,
		&imageStudioEncryptorStub{},
		PlatformOpenAI,
		[]string{"gpt-image-2"},
	)

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID:   "free-create",
		UserPrompt:   "transparent product render",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gpt-image-2",
		APIKeyID:     20,
		Background:   "transparent",
		OutputFormat: "png",
	})

	require.Nil(t, job)
	require.ErrorIs(t, err, ErrImageStudioBackgroundNotSupported)
	require.Nil(t, repo.created)
}

func TestImageStudioCreatePendingJobBuildsGrokNativePayload(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	encryptor := &imageStudioEncryptorStub{}
	svc := newImageStudioProviderCreateServiceForTest(
		repo,
		encryptor,
		PlatformGrok,
		[]string{"grok-imagine-image-quality"},
	)

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID: "free-create",
		UserPrompt: "wide launch artwork",
		Size:       "3584x2048",
		Count:      1,
		Model:      "grok-imagine-image-quality",
		Quality:    "standard",
		APIKeyID:   20,
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, PlatformGrok, gjson.Get(encryptor.plaintext, "platform").String())
	require.Equal(t, "create", gjson.Get(encryptor.plaintext, "operation").String())
	require.Equal(t, "grok:grok-imagine-image-quality:v1", gjson.Get(encryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, "16:9", gjson.Get(encryptor.plaintext, "body.aspect_ratio").String())
	require.Equal(t, "2k", gjson.Get(encryptor.plaintext, "body.resolution").String())
	for _, field := range []string{
		"body.size",
		"body.quality",
		"body.background",
		"body.output_format",
		"body.output_compression",
		"body.input_fidelity",
		"body.style",
		"body.response_format",
	} {
		require.False(t, gjson.Get(encryptor.plaintext, field).Exists(), field)
	}
}

func TestImageStudioCreatePendingJobBuildsGeminiNativePayload(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	encryptor := &imageStudioEncryptorStub{}
	svc := newImageStudioProviderCreateServiceForTest(
		repo,
		encryptor,
		PlatformGemini,
		[]string{"gemini-3.1-flash-image"},
	)

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID:   "free-create",
		UserPrompt:   "wide launch artwork",
		Size:         "3584x2048",
		Count:        1,
		Model:        "gemini-3.1-flash-image",
		OutputFormat: "png",
		APIKeyID:     20,
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, PlatformGemini, gjson.Get(encryptor.plaintext, "platform").String())
	require.Equal(t, "create", gjson.Get(encryptor.plaintext, "operation").String())
	require.Equal(t, "gemini:gemini-3.1-flash-image:v1", gjson.Get(encryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, "/v1beta/models/gemini-3.1-flash-image:generateContent", gjson.Get(encryptor.plaintext, "endpoint").String())
	require.Contains(t, gjson.Get(encryptor.plaintext, "body.contents.0.parts.0.text").String(), "wide launch artwork")
	require.Equal(t, "TEXT", gjson.Get(encryptor.plaintext, "body.generationConfig.responseModalities.0").String())
	require.Equal(t, "IMAGE", gjson.Get(encryptor.plaintext, "body.generationConfig.responseModalities.1").String())
}

func TestImageStudioCreatePendingJobKeepsOpenAICompatibleTransportForGeminiModel(t *testing.T) {
	repo := &imageStudioCreateRepoStub{}
	encryptor := &imageStudioEncryptorStub{}
	svc := newImageStudioProviderCreateServiceForTest(
		repo,
		encryptor,
		PlatformOpenAI,
		[]string{"gemini-3.1-flash-image"},
	)

	job, _, err := svc.CreatePendingJob(context.Background(), 10, ImageStudioGenerateRequest{
		TemplateID:   "free-create",
		UserPrompt:   "wide launch artwork",
		Size:         "1024x1024",
		Count:        1,
		Model:        "gemini-3.1-flash-image",
		OutputFormat: "png",
		APIKeyID:     20,
	})

	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, PlatformOpenAI, gjson.Get(encryptor.plaintext, "platform").String())
	require.Equal(t, "gemini:gemini-3.1-flash-image:v1", gjson.Get(encryptor.plaintext, "capability_profile_id").String())
	require.Equal(t, openAIImagesGenerationsEndpoint, gjson.Get(encryptor.plaintext, "endpoint").String())
	require.Equal(t, "gemini-3.1-flash-image", gjson.Get(encryptor.plaintext, "body.model").String())
	require.Equal(t, "1024x1024", gjson.Get(encryptor.plaintext, "body.size").String())
	require.False(t, gjson.Get(encryptor.plaintext, "body.response_format").Exists())
	require.False(t, gjson.Get(encryptor.plaintext, "body.contents").Exists())
}

func TestImageStudioBuildWorkerRequestRejectsPinnedProfileDrift(t *testing.T) {
	svc := &ImageStudioService{}
	decrypted := `{
		"platform":"openai",
		"operation":"create",
		"capability_profile_id":"openai:gpt-image-1.5:v1",
		"capability_revision":"stale-revision",
		"endpoint":"/v1/images/generations",
		"body":{"model":"gpt-image-1.5","prompt":"draw","n":1,"size":"1024x1024"}
	}`

	req, err := svc.BuildWorkerRequest(context.Background(), &ImageStudioJob{Model: "gpt-image-1.5"}, decrypted)

	require.Nil(t, req)
	require.ErrorIs(t, err, ErrImageStudioCapabilityProfileChanged)
}

func TestImageStudioBuildWorkerRequestKeepsOpenAICompatibleTransportForGeminiModel(t *testing.T) {
	svc := &ImageStudioService{}
	decrypted := `{
		"platform":"openai",
		"operation":"create",
		"capability_profile_id":"gemini:gemini-3.1-flash-image:v1",
		"capability_revision":"` + imageStudioCapabilityRevision + `",
		"endpoint":"/v1/images/generations",
		"body":{"model":"gemini-3.1-flash-image","prompt":"draw","n":1,"size":"1024x1024","response_format":"b64_json"}
	}`

	req, err := svc.BuildWorkerRequest(context.Background(), &ImageStudioJob{Model: "gemini-3.1-flash-image"}, decrypted)

	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, req.Platform)
	require.Equal(t, "create", req.Operation)
	require.Equal(t, openAIImagesGenerationsEndpoint, req.Endpoint)
	require.Equal(t, "application/json", req.ContentType)
	require.Equal(t, "gemini-3.1-flash-image", gjson.GetBytes(req.Body, "model").String())
}

func TestImageStudioBuildWorkerRequestKeepsLegacyPayloadCompatible(t *testing.T) {
	svc := &ImageStudioService{}

	req, err := svc.BuildWorkerRequest(
		context.Background(),
		&ImageStudioJob{Model: "gpt-image-1"},
		`{"model":"gpt-image-1","prompt":"legacy request","n":4,"size":"1024x1024"}`,
	)

	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, req.Platform)
	require.Equal(t, "create", req.Operation)
	require.Equal(t, openAIImagesGenerationsEndpoint, req.Endpoint)
	require.Equal(t, int64(1), gjson.GetBytes(req.Body, "n").Int())
}

func TestImageStudioBuildWorkerRequestBuildsGrokMultiImageEditJSON(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	first := encodeImageStudioReferencePNG(t, 2, 2)
	second := encodeImageStudioReferencePNG(t, 3, 2)
	firstKey, err := store.Save(10, "grok-job-ref-1", "image/png", first)
	require.NoError(t, err)
	secondKey, err := store.Save(10, "grok-job-ref-2", "image/png", second)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		jobRefs: []ImageStudioJobReference{
			{
				ID:          "grok-job-ref-1",
				JobID:       "grok-job",
				StorageKey:  firstKey,
				ContentType: "image/png",
				ByteSize:    int64(len(first)),
			},
			{
				ID:          "grok-job-ref-2",
				JobID:       "grok-job",
				StorageKey:  secondKey,
				ContentType: "image/png",
				ByteSize:    int64(len(second)),
			},
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}
	decrypted := `{
		"platform":"grok",
		"operation":"edit",
		"capability_profile_id":"grok:grok-imagine-image-quality:v1",
		"capability_revision":"` + imageStudioCapabilityRevision + `",
		"endpoint":"/v1/images/edits",
		"body":{
			"model":"grok-imagine-image-quality",
			"prompt":"combine references",
			"n":2,
			"aspect_ratio":"1:1",
			"resolution":"1k",
			"image_studio_job_reference_ids":["grok-job-ref-1","grok-job-ref-2"]
		}
	}`

	req, err := svc.BuildWorkerRequest(
		context.Background(),
		&ImageStudioJob{ID: "grok-job", UserID: 10, Model: "grok-imagine-image-quality"},
		decrypted,
	)

	require.NoError(t, err)
	require.Equal(t, PlatformGrok, req.Platform)
	require.Equal(t, "edit", req.Operation)
	require.Equal(t, openAIImagesEditsEndpoint, req.Endpoint)
	require.Equal(t, "application/json", req.ContentType)
	require.Equal(t, int64(1), gjson.GetBytes(req.Body, "n").Int())
	require.Equal(t, "image_url", gjson.GetBytes(req.Body, "images.0.type").String())
	require.Equal(t, "image_url", gjson.GetBytes(req.Body, "images.1.type").String())
	require.Equal(
		t,
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(first),
		gjson.GetBytes(req.Body, "images.0.url").String(),
	)
	require.Equal(
		t,
		"data:image/png;base64,"+base64.StdEncoding.EncodeToString(second),
		gjson.GetBytes(req.Body, "images.1.url").String(),
	)
	require.False(t, gjson.GetBytes(req.Body, "image_studio_job_reference_ids").Exists())
}

func TestImageStudioBuildWorkerRequestBuildsGeminiMultiImageEditJSON(t *testing.T) {
	store := NewImageStudioAssetStore(t.TempDir())
	first := encodeImageStudioReferencePNG(t, 2, 2)
	second := encodeImageStudioReferencePNG(t, 3, 2)
	firstKey, err := store.Save(10, "gemini-job-ref-1", "image/png", first)
	require.NoError(t, err)
	secondKey, err := store.Save(10, "gemini-job-ref-2", "image/png", second)
	require.NoError(t, err)
	repo := &imageStudioReferenceRepoStub{
		jobRefs: []ImageStudioJobReference{
			{
				ID:          "gemini-job-ref-1",
				JobID:       "gemini-job",
				StorageKey:  firstKey,
				ContentType: "image/png",
				ByteSize:    int64(len(first)),
			},
			{
				ID:          "gemini-job-ref-2",
				JobID:       "gemini-job",
				StorageKey:  secondKey,
				ContentType: "image/png",
				ByteSize:    int64(len(second)),
			},
		},
	}
	svc := &ImageStudioService{repo: repo, assetStore: store}
	decrypted := `{
		"platform":"gemini",
		"operation":"edit",
		"capability_profile_id":"gemini:gemini-3.1-flash-image:v1",
		"capability_revision":"` + imageStudioCapabilityRevision + `",
		"endpoint":"/v1beta/models/gemini-3.1-flash-image:generateContent",
		"body":{
			"contents":[{"parts":[{"text":"combine references"}]}],
			"generationConfig":{"responseModalities":["TEXT","IMAGE"]},
			"image_studio_job_reference_ids":["gemini-job-ref-1","gemini-job-ref-2"]
		}
	}`

	req, err := svc.BuildWorkerRequest(
		context.Background(),
		&ImageStudioJob{ID: "gemini-job", UserID: 10, Model: "gemini-3.1-flash-image"},
		decrypted,
	)

	require.NoError(t, err)
	require.Equal(t, PlatformGemini, req.Platform)
	require.Equal(t, "edit", req.Operation)
	require.Equal(t, "/v1beta/models/gemini-3.1-flash-image:generateContent", req.Endpoint)
	require.Equal(t, "application/json", req.ContentType)
	require.Equal(t, "combine references", gjson.GetBytes(req.Body, "contents.0.parts.0.text").String())
	require.Equal(t, "image/png", gjson.GetBytes(req.Body, "contents.0.parts.1.inlineData.mimeType").String())
	require.Equal(t, base64.StdEncoding.EncodeToString(first), gjson.GetBytes(req.Body, "contents.0.parts.1.inlineData.data").String())
	require.Equal(t, "image/png", gjson.GetBytes(req.Body, "contents.0.parts.2.inlineData.mimeType").String())
	require.Equal(t, base64.StdEncoding.EncodeToString(second), gjson.GetBytes(req.Body, "contents.0.parts.2.inlineData.data").String())
	require.False(t, gjson.GetBytes(req.Body, "image_studio_job_reference_ids").Exists())
}

func TestParseGrokMediaRequestRecognizesOfficialMultiImageSchema(t *testing.T) {
	body := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"combine references",
		"images":[
			{"type":"image_url","url":"data:image/png;base64,QQ=="},
			{"type":"image_url","url":"data:image/jpeg;base64,Qg=="}
		]
	}`)

	info, err := ParseGrokMediaRequestWithError("application/json", body)

	require.NoError(t, err)
	require.Equal(t, []string{
		"data:image/png;base64,QQ==",
		"data:image/jpeg;base64,Qg==",
	}, info.InputImageURLs)
	require.JSONEq(t, `{
		"prompt":"combine references",
		"images":[
			{"image_url":"data:image/png;base64,QQ=="},
			{"image_url":"data:image/jpeg;base64,Qg=="}
		]
	}`, string(info.ModerationBody()))
}

func TestGrokImageResolutionFeedsImageBillingTier(t *testing.T) {
	info, err := ParseGrokMediaRequestWithError(
		"application/json",
		[]byte(`{
			"model":"grok-imagine-image-quality",
			"prompt":"draw",
			"aspect_ratio":"1:1",
			"resolution":"1k"
		}`),
	)
	require.NoError(t, err)

	usage := grokMediaUsageFromResponse(
		GrokMediaEndpointImagesGenerations,
		info,
		[]byte(`{"data":[{"url":"data:image/jpeg;base64,QQ=="}]}`),
	)

	require.Equal(t, ImageBillingSize1K, usage.ImageSize)
	require.Equal(t, "1k", usage.ImageInputSize)
}

func TestGrokImageEditUsageFallsBackToConservativeImageInputTokens(t *testing.T) {
	reference := encodeImageStudioReferencePNG(t, 1024, 512)
	body := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"edit",
		"images":[{"type":"image_url","url":"data:image/png;base64,` +
		base64.StdEncoding.EncodeToString(reference) + `"}]
	}`)
	info, err := ParseGrokMediaRequestWithError("application/json", body)
	require.NoError(t, err)

	usage := grokMediaUsageFromResponse(
		GrokMediaEndpointImagesEdits,
		info,
		[]byte(`{"data":[{"url":"data:image/png;base64,QQ=="}]}`),
	)

	want := imageStudioReferenceInputTokenUpperBound(1024, 512)
	require.Equal(t, want, usage.Usage.ImageInputTokens)
	require.GreaterOrEqual(t, usage.Usage.InputTokens, want)
}

func TestGrokImageEditUsageKeepsAuthoritativeImageInputTokens(t *testing.T) {
	reference := encodeImageStudioReferencePNG(t, 1024, 512)
	body := []byte(`{
		"model":"grok-imagine-image-quality",
		"prompt":"edit",
		"images":[{"type":"image_url","url":"data:image/png;base64,` +
		base64.StdEncoding.EncodeToString(reference) + `"}]
	}`)
	info, err := ParseGrokMediaRequestWithError("application/json", body)
	require.NoError(t, err)

	usage := grokMediaUsageFromResponse(
		GrokMediaEndpointImagesEdits,
		info,
		[]byte(`{
			"data":[{"url":"data:image/png;base64,QQ=="}],
			"usage":{
				"input_tokens":91,
				"input_tokens_details":{"image_tokens":77}
			}
		}`),
	)

	require.Equal(t, 77, usage.Usage.ImageInputTokens)
	require.Equal(t, 91, usage.Usage.InputTokens)
}

func TestGatewayServiceEstimatesImageStudioInputCost(t *testing.T) {
	pricing := &PricingService{pricingData: map[string]*LiteLLMModelPricing{
		"gpt-image-edit-test": {
			InputCostPerToken:      0.000001,
			InputCostPerImageToken: 0.00001,
		},
	}}
	gateway := &GatewayService{
		cfg:            &config.Config{},
		billingService: NewBillingService(&config.Config{}, pricing),
	}
	groupID := int64(30)
	apiKey := &APIKey{
		UserID:  10,
		GroupID: &groupID,
		Group: &Group{
			ID:             groupID,
			RateMultiplier: 1.5,
		},
	}

	cost, err := gateway.EstimateImageStudioInputCost(
		context.Background(),
		"gpt-image-edit-test",
		apiKey,
		100,
	)

	require.NoError(t, err)
	require.InDelta(t, 0.0015, cost, 0.000001)
}

func newImageStudioProviderCreateServiceForTest(
	repo ImageStudioRepository,
	encryptor SecretEncryptor,
	platform string,
	models []string,
) *ImageStudioService {
	settingService := NewSettingService(&imageStudioSettingRepoStub{values: map[string]string{
		SettingKeyImageStudioEnabled: "true",
	}}, &config.Config{})
	playService := NewPlayService(nil, nil, nil, settingService, nil, nil)
	userRepo := &imageStudioCreateUserRepoStub{user: &User{
		ID:             10,
		Balance:        100,
		TotalRecharged: 100,
	}}
	groupID := int64(30)
	apiKey := &APIKey{
		ID:      20,
		UserID:  10,
		GroupID: &groupID,
		Status:  StatusAPIKeyActive,
		Group: &Group{
			ID:                   groupID,
			Platform:             platform,
			AllowImageGeneration: true,
		},
	}
	apiKeyService := NewAPIKeyService(
		&imageStudioCreateAPIKeyRepoStub{key: apiKey},
		userRepo,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)
	return NewImageStudioService(
		repo,
		nil,
		apiKeyService,
		userRepo,
		settingService,
		playService,
		nil,
		&imageStudioModelResolverStub{models: models},
		encryptor,
		&imageStudioCreateBillingStub{},
		nil,
	)
}
