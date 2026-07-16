package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestImageStudioSingleImageRequestBodyForcesOneImage(t *testing.T) {
	body := `{"model":"gpt-image-2","prompt":"draw a cat","n":4,"size":"1024x1024"}`

	require.Equal(t, 4, imageStudioGenerationCount(body))

	got, err := imageStudioSingleImageRequestBody(body)
	require.NoError(t, err)
	require.Equal(t, int64(1), gjson.Get(got, "n").Int())
	require.Equal(t, "gpt-image-2", gjson.Get(got, "model").String())
	require.Equal(t, "draw a cat", gjson.Get(got, "prompt").String())
	require.Equal(t, "1024x1024", gjson.Get(got, "size").String())
}

func TestImageStudioGenerationCountDefaultsToOne(t *testing.T) {
	require.Equal(t, 1, imageStudioGenerationCount(`{"model":"gpt-image-2","prompt":"draw a cat"}`))
	require.Equal(t, 1, imageStudioGenerationCount(`{"model":"gpt-image-2","prompt":"draw a cat","n":0}`))
}

func TestBindImageStudioGenerateRequestReturnsStructured413(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/image-studio/generate",
		strings.NewReader(`{"user_prompt":"`+strings.Repeat("x", 200)+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Body = http.MaxBytesReader(recorder, req.Body, 64)
	ctx.Request = req

	var got service.ImageStudioGenerateRequest
	require.False(t, bindImageStudioGenerateRequest(ctx, &got))
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.Equal(t, "IMAGE_STUDIO_REQUEST_TOO_LARGE", gjson.Get(recorder.Body.String(), "reason").String())
}

func TestBindImageStudioGenerateRequestRejectsTrailingJSONValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/image-studio/generate",
		strings.NewReader(`{"user_prompt":"first"}{"user_prompt":"second"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	var got service.ImageStudioGenerateRequest
	require.False(t, bindImageStudioGenerateRequest(ctx, &got))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
