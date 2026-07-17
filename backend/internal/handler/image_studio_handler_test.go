package handler

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type imageStudioUploadRepoStub struct {
	service.ImageStudioRepository
	created *service.ImageStudioReference
}

func (s *imageStudioUploadRepoStub) CreateReference(_ context.Context, reference *service.ImageStudioReference) error {
	copyReference := *reference
	s.created = &copyReference
	return nil
}

func (s *imageStudioUploadRepoStub) AcquireImageStudioUploadSlot(
	context.Context,
	int64,
	time.Time,
	time.Duration,
	int,
	int,
	time.Duration,
) (string, bool, error) {
	return "upload-slot", true, nil
}

func (s *imageStudioUploadRepoStub) ReleaseImageStudioUploadSlot(
	context.Context,
	int64,
	string,
	time.Time,
) error {
	return nil
}

type imageStudioUploadSettingRepoStub struct {
	service.SettingRepository
}

func (*imageStudioUploadSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	values[service.SettingKeyImageStudioEnabled] = "true"
	return values, nil
}

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

func TestRequireImageStudioUUIDParamRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/image-studio/jobs/not-a-uuid", nil)
	ctx.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	id, ok := requireImageStudioUUIDParam(ctx)

	require.False(t, ok)
	require.Empty(t, id)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "IMAGE_STUDIO_INVALID_ID", gjson.Get(recorder.Body.String(), "reason").String())
}

func TestPrepareImageStudioGenerateIdempotencyRequiresKey(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/image-studio/generate", nil)
	req := service.ImageStudioGenerateRequest{TemplateID: "free-create", UserPrompt: "draw"}

	err := prepareImageStudioGenerateIdempotency(ctx, 42, &req)

	require.ErrorIs(t, err, service.ErrIdempotencyKeyRequired)
}

func TestPrepareImageStudioGenerateIdempotencyIsStableAndPayloadSensitive(t *testing.T) {
	newContext := func(key string) *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/image-studio/generate", nil)
		ctx.Request.Header.Set("Idempotency-Key", key)
		return ctx
	}
	first := service.ImageStudioGenerateRequest{TemplateID: "free-create", UserPrompt: "draw"}
	retry := first
	changed := service.ImageStudioGenerateRequest{TemplateID: "free-create", UserPrompt: "draw something else"}

	require.NoError(t, prepareImageStudioGenerateIdempotency(newContext("same-submit"), 42, &first))
	require.NoError(t, prepareImageStudioGenerateIdempotency(newContext("same-submit"), 42, &retry))
	require.NoError(t, prepareImageStudioGenerateIdempotency(newContext("same-submit"), 42, &changed))

	require.NotEmpty(t, first.IdempotencyKeyHash)
	require.Equal(t, first.IdempotencyKeyHash, retry.IdempotencyKeyHash)
	require.Equal(t, first.IdempotencyFingerprint, retry.IdempotencyFingerprint)
	require.NotEqual(t, first.IdempotencyFingerprint, changed.IdempotencyFingerprint)
}

func TestParseImageStudioEstimateReferenceIDsAcceptsArrayAndCommaForms(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/image-studio/estimate?reference_ids=ref-1&reference_ids=ref-2%2Cref-3&reference_ids%5B%5D=ref-4",
		nil,
	)

	require.Equal(
		t,
		[]string{"ref-1", "ref-2", "ref-3", "ref-4"},
		parseImageStudioEstimateReferenceIDs(ctx),
	)
}

func TestImageStudioUploadReferenceRequiresAuthentication(t *testing.T) {
	handler, _ := newImageStudioUploadHandlerForTest(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = newImageStudioReferenceRequest(t, realImageStudioPNGFixture(t))

	handler.UploadReference(ctx)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestImageStudioUploadReferenceStoresPrivateObjectAndMetadata(t *testing.T) {
	handler, repo := newImageStudioUploadHandlerForTest(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = newImageStudioReferenceRequest(t, realImageStudioPNGFixture(t))
	ctx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.UploadReference(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, repo.created)
	require.Equal(t, int64(42), repo.created.UserID)
	require.Equal(t, "image/png", repo.created.ContentType)
	require.NotEmpty(t, repo.created.StorageKey)
	require.NotContains(t, recorder.Body.String(), repo.created.StorageKey)
	require.Equal(t, repo.created.ID, gjson.Get(recorder.Body.String(), "data.reference.id").String())
}

func newImageStudioUploadHandlerForTest(t *testing.T) (*ImageStudioHandler, *imageStudioUploadRepoStub) {
	t.Helper()
	repo := &imageStudioUploadRepoStub{}
	settingService := service.NewSettingService(&imageStudioUploadSettingRepoStub{}, &config.Config{})
	playService := service.NewPlayService(nil, nil, nil, settingService, nil, nil)
	studio := service.NewImageStudioService(
		repo,
		service.NewImageStudioAssetStore(t.TempDir()),
		nil,
		nil,
		settingService,
		playService,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	return NewImageStudioHandler(studio, nil, nil, nil), repo
}

func newImageStudioReferenceRequest(t *testing.T, image []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", "reference.png")
	require.NoError(t, err)
	_, err = part.Write(image)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/image-studio/references", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func realImageStudioPNGFixture(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 230, G: 30, B: 80, A: 255})
	img.Set(1, 0, color.NRGBA{R: 20, G: 150, B: 220, A: 255})
	img.Set(0, 1, color.NRGBA{R: 90, G: 210, B: 60, A: 255})
	img.Set(1, 1, color.NRGBA{R: 240, G: 180, B: 40, A: 255})
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return out.Bytes()
}
