package handler

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageStudioGatewayDispatchStub struct {
	openAICalls int
	grokCalls   int
	lastPath    string
}

func (s *imageStudioGatewayDispatchStub) Images(c *gin.Context) {
	s.openAICalls++
	s.lastPath = c.Request.URL.Path
}

func (s *imageStudioGatewayDispatchStub) GrokImages(c *gin.Context) {
	s.grokCalls++
	s.lastPath = c.Request.URL.Path
}

type imageStudioGatewayCostStub struct {
	responseCost float64
	capturedCost float64
	imageData    []byte
}

func (s *imageStudioGatewayCostStub) Images(c *gin.Context) {
	service.RecordImageStudioManagedBillingCost(c.Request.Context(), s.capturedCost)
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{{
			"b64_json": base64.StdEncoding.EncodeToString(s.imageData),
		}},
		"usage": gin.H{"total_cost": s.responseCost},
	})
}

func (s *imageStudioGatewayCostStub) GrokImages(c *gin.Context) {
	s.Images(c)
}

type imageStudioGatewayErrorStub struct {
	status int
	body   string
}

func (s *imageStudioGatewayErrorStub) Images(c *gin.Context) {
	c.Data(s.status, "application/json", []byte(s.body))
}

func (s *imageStudioGatewayErrorStub) GrokImages(c *gin.Context) {
	s.Images(c)
}

func TestImageStudioGatewayDispatchUsesPinnedProviderForCreateAndEdit(t *testing.T) {
	tests := []struct {
		name       string
		platform   string
		operation  string
		endpoint   string
		wantOpenAI int
		wantGrok   int
	}{
		{
			name:       "OpenAI create",
			platform:   service.PlatformOpenAI,
			operation:  "create",
			endpoint:   "/v1/images/generations",
			wantOpenAI: 1,
		},
		{
			name:       "OpenAI edit",
			platform:   service.PlatformOpenAI,
			operation:  "edit",
			endpoint:   "/v1/images/edits",
			wantOpenAI: 1,
		},
		{
			name:      "Grok create",
			platform:  service.PlatformGrok,
			operation: "create",
			endpoint:  "/v1/images/generations",
			wantGrok:  1,
		},
		{
			name:      "Grok edit",
			platform:  service.PlatformGrok,
			operation: "edit",
			endpoint:  "/v1/images/edits",
			wantGrok:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &imageStudioGatewayDispatchStub{}
			handler := &ImageStudioHandler{gateway: stub}
			gwCtx := &gin.Context{
				Request: httptest.NewRequest(http.MethodPost, tt.endpoint, nil),
			}

			require.NoError(t, handler.dispatchImageStudioGateway(
				gwCtx,
				&service.ImageStudioWorkerRequest{
					Platform:  tt.platform,
					Operation: tt.operation,
					Endpoint:  tt.endpoint,
				},
			))
			require.Equal(t, tt.wantOpenAI, stub.openAICalls)
			require.Equal(t, tt.wantGrok, stub.grokCalls)
			require.Equal(t, tt.endpoint, stub.lastPath)
		})
	}
}

func TestImageStudioGatewayDispatchRejectsUnknownPlatform(t *testing.T) {
	stub := &imageStudioGatewayDispatchStub{}
	handler := &ImageStudioHandler{gateway: stub}

	err := handler.dispatchImageStudioGateway(
		&gin.Context{Request: httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)},
		&service.ImageStudioWorkerRequest{Platform: "anthropic"},
	)

	require.Error(t, err)
	require.Zero(t, stub.openAICalls)
	require.Zero(t, stub.grokCalls)
}

func TestImageStudioGatewayUsesManagedBillingCaptureAsAuthoritativeCost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &imageStudioGatewayCostStub{
		responseCost: 0,
		capturedCost: 0.25,
		imageData:    realImageStudioPNGFixture(t),
	}
	handler := &ImageStudioHandler{gateway: stub}
	apiKey := &service.APIKey{
		ID:     22,
		UserID: 10,
		Key:    "sk-test-redacted",
		User:   &service.User{ID: 10},
		Group: &service.Group{
			ID:                   30,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}

	images, actualCost, err := handler.invokeGatewayImagesOnce(
		context.Background(),
		apiKey,
		&service.ImageStudioWorkerRequest{
			Platform:    service.PlatformOpenAI,
			Endpoint:    "/v1/images/generations",
			ContentType: "application/json",
			Body:        []byte(`{"model":"gpt-image-2","prompt":"draw","n":1}`),
		},
	)

	require.NoError(t, err)
	require.Len(t, images, 1)
	require.InDelta(t, 0.25, actualCost, 0.000001)
}

func TestParseGeminiImageStudioPayloadsExtractsInlineImages(t *testing.T) {
	raw := []byte(`{
		"candidates":[{
			"content":{
				"parts":[
					{"text":"done"},
					{"inlineData":{"mimeType":"image/png","data":"` + base64.StdEncoding.EncodeToString(realImageStudioPNGFixture(t)) + `"}}
				]
			}
		}]
	}`)

	images, err := parseGeminiImageStudioPayloads(context.Background(), raw)

	require.NoError(t, err)
	require.Len(t, images, 1)
	require.Equal(t, "image/png", images[0].ContentType)
	require.NotEmpty(t, images[0].Data)
}

func TestImageStudioGatewayDoesNotReturnSensitiveUpstreamErrorBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "sk-sensitive-upstream-value"
	const prompt = "private user prompt that must not be persisted"
	handler := &ImageStudioHandler{gateway: &imageStudioGatewayErrorStub{
		status: http.StatusBadGateway,
		body:   `{"error":{"message":"authorization=` + secret + ` prompt=` + prompt + `"}}`,
	}}
	apiKey := &service.APIKey{
		ID:     22,
		UserID: 10,
		Key:    "sk-test-redacted",
		User:   &service.User{ID: 10},
		Group: &service.Group{
			ID:                   30,
			Platform:             service.PlatformOpenAI,
			AllowImageGeneration: true,
		},
	}

	_, _, err := handler.invokeGatewayImagesOnce(
		context.Background(),
		apiKey,
		&service.ImageStudioWorkerRequest{
			Platform:    service.PlatformOpenAI,
			Endpoint:    "/v1/images/generations",
			ContentType: "application/json",
			Body:        []byte(`{"model":"gpt-image-2","prompt":"draw","n":1}`),
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "502")
	require.NotContains(t, err.Error(), secret)
	require.NotContains(t, err.Error(), prompt)
	require.LessOrEqual(t, len(err.Error()), 128)
}
