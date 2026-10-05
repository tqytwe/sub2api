//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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

func TestCompatibleImagesGeminiModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, model := range []string{"gemini-2.5-flash-image", "gemini-2.5-flash-image-preview", "gemini-3-pro-image", "gemini-3.1-flash-image"} {
		t.Run(model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw"}`, model))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(body))
			parsed, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			require.True(t, (&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}).SupportsOpenAIImageCapability(parsed.RequiredCapability))
			for _, typ := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
				require.False(t, (&Account{Platform: PlatformOpenAI, Type: typ}).SupportsOpenAIImageCapability(parsed.RequiredCapability))
			}
			require.False(t, isOpenAIImageGenerationModel(model), "compatible image IDs must not enter native Responses normalization")
		})
	}
	// The Fork accepts generic "*image*" IDs for OpenAI-compatible providers
	// (FORK-IMAGE-011), so only non-image IDs are rejected here.
	for _, model := range []string{"gemini-2.5-pro", "gemini-2.5-flash", "gpt-5.5"} {
		t.Run("reject_"+model, func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw"}`, model))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, bytes.NewReader(body))
			_, err := (&OpenAIGatewayService{}).ParseOpenAIImagesRequest(c, body)
			require.ErrorContains(t, err, "images endpoint requires an image model")
		})
	}
}

// The Fork translates compatible Gemini image models to native generateContent
// on API-key accounts instead of passing the OpenAI Images body through.
func TestCompatibleImagesForwardGemini(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{"generation", "json_edit", "multipart_edit", "composite_multipart_alias", "channel_mapping", "account_mapping"} {
		t.Run(kind, func(t *testing.T) {
			model := "gemini-3.1-flash-image"
			endpoint := openAIImagesGenerationsEndpoint
			contentType := "application/json"
			channelModel := ""
			credentials := map[string]any{"api_key": "image-key", "base_url": "https://compatible.example/v1"}
			requestModel := model
			if kind == "channel_mapping" {
				requestModel, channelModel = "gpt-image-2", model
			}
			if kind == "account_mapping" {
				requestModel = "gpt-image-2"
				credentials["model_mapping"] = map[string]any{requestModel: model}
			}
			body := []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw","size":"1024x1024","custom_field":"preserved"}`, requestModel))
			if kind == "json_edit" {
				endpoint = openAIImagesEditsEndpoint
				body = []byte(fmt.Sprintf(`{"model":%q,"prompt":"draw","images":[{"image_url":"data:image/png;base64,b3JpZ2luYWwtaW1hZ2UtYnl0ZXM="}],"custom_field":"preserved"}`, model))
			}
			if strings.Contains(kind, "multipart") {
				endpoint = openAIImagesEditsEndpoint
				if kind == "composite_multipart_alias" {
					requestModel = "public-image"
				}
				var buf bytes.Buffer
				writer := multipart.NewWriter(&buf)
				require.NoError(t, writer.WriteField("model", requestModel))
				require.NoError(t, writer.WriteField("prompt", "draw"))
				require.NoError(t, writer.WriteField("custom_field", "preserved"))
				part, err := writer.CreateFormFile("image", "input.png")
				require.NoError(t, err)
				_, err = part.Write([]byte("original-image-bytes"))
				require.NoError(t, err)
				require.NoError(t, writer.Close())
				body, contentType = buf.Bytes(), writer.FormDataContentType()
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", contentType)
			if kind == "composite_multipart_alias" {
				ctx := WithResolvedTargetPlatform(c.Request.Context(), PlatformOpenAI)
				ctx = WithCompositeRouteDecision(ctx, CompositeRouteDecision{Matched: true, TargetPlatform: PlatformOpenAI, UpstreamModel: model, PublicModel: requestModel})
				c.Request = c.Request.WithContext(ctx)
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}]}`))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			parsed, err := svc.ParseOpenAIImagesRequest(c, body)
			require.NoError(t, err)
			if kind == "channel_mapping" {
				require.Equal(t, OpenAIImagesCapabilityAPIKey, parsed.RequiredCapabilityForModel(channelModel))
			}
			result, err := svc.ForwardImages(c.Request.Context(), c, &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: credentials}, body, parsed, channelModel)
			require.NoError(t, err)
			require.Equal(t, 1, result.ImageCount)
			require.Equal(t, model, result.UpstreamModel)
			require.Equal(t, firstNonEmptyString(channelModel, parsed.Model), result.Model)
			require.Equal(t, "https://compatible.example/v1beta/models/"+model+":generateContent", upstream.lastReq.URL.String())
			require.Equal(t, "Bearer image-key", upstream.lastReq.Header.Get("Authorization"))
			require.Equal(t, "draw", gjson.GetBytes(upstream.lastBody, "contents.0.parts.0.text").String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "custom_field").Exists())
			if kind == "json_edit" || strings.Contains(kind, "multipart") {
				data, err := base64.StdEncoding.DecodeString(gjson.GetBytes(upstream.lastBody, "contents.0.parts.1.inlineData.data").String())
				require.NoError(t, err)
				require.Equal(t, "original-image-bytes", string(data))
			}
		})
	}
}

func TestCompatibleImagesNativeAccountsRejectGeminiBeforeForwarding(t *testing.T) {
	for _, typ := range []string{AccountTypeOAuth, AccountTypeSetupToken} {
		for _, mapping := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/mapping=%t", typ, mapping), func(t *testing.T) {
				model := "gemini-3-pro-image"
				account := &Account{Platform: PlatformOpenAI, Type: typ, Credentials: map[string]any{"access_token": "unused"}}
				if mapping {
					account.Credentials["model_mapping"] = map[string]any{"gpt-image-2": model}
					model = "gpt-image-2"
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, nil)
				upstream := &httpUpstreamRecorder{}
				svc := &OpenAIGatewayService{httpUpstream: upstream}
				_, err := svc.ForwardImages(context.Background(), c, account, nil, &OpenAIImagesRequest{Model: model}, "")
				require.ErrorContains(t, err, "images endpoint requires an image model")
				require.Empty(t, upstream.requests)
			})
		}
	}
}
