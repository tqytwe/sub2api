package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	ImageStudioGenerateRequestBodyLimit  int64 = 128 << 10
	ImageStudioReferenceRequestBodyLimit int64 = 21 << 20
	imageStudioPrivateCacheControl             = "private, no-store"
)

type ImageStudioHandler struct {
	studio        *service.ImageStudioService
	gateway       imageStudioGateway
	apiKeyService *service.APIKeyService
}

type imageStudioGateway interface {
	Images(c *gin.Context)
	GrokImages(c *gin.Context)
}

func NewImageStudioHandler(
	studio *service.ImageStudioService,
	gateway *OpenAIGatewayHandler,
	apiKeyService *service.APIKeyService,
) *ImageStudioHandler {
	return &ImageStudioHandler{
		studio:        studio,
		gateway:       gateway,
		apiKeyService: apiKeyService,
	}
}

func (h *ImageStudioHandler) Templates(c *gin.Context) {
	if !h.studio.IsEnabled(c.Request.Context()) {
		response.ErrorFrom(c, service.ErrImageStudioDisabled)
		return
	}
	response.Success(c, h.studio.ListTemplates())
}

func (h *ImageStudioHandler) Capabilities(c *gin.Context) {
	if !h.studio.IsEnabled(c.Request.Context()) {
		response.ErrorFrom(c, service.ErrImageStudioDisabled)
		return
	}
	response.Success(c, h.studio.ListCapabilities())
}

func (h *ImageStudioHandler) Models(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	apiKeyID, _ := parseInt64Query(c, "api_key_id")
	models, err := h.studio.ListModels(c.Request.Context(), subject.UserID, apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"models": models})
}

func (h *ImageStudioHandler) Estimate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	apiKeyID, _ := parseInt64Query(c, "api_key_id")
	count, _ := parseIntQueryDefault(c, "count", 1)
	got, err := h.studio.Estimate(
		c.Request.Context(),
		subject.UserID,
		c.Query("template_id"),
		c.Query("size"),
		count,
		apiKeyID,
		c.Query("model"),
		parseImageStudioEstimateReferenceIDs(c),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, got)
}

func parseImageStudioEstimateReferenceIDs(c *gin.Context) []string {
	if c == nil {
		return nil
	}
	values := append([]string(nil), c.QueryArray("reference_ids")...)
	values = append(values, c.QueryArray("reference_ids[]")...)
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if id := strings.TrimSpace(part); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

func (h *ImageStudioHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.ImageStudioGenerateRequest
	if !bindImageStudioGenerateRequest(c, &req) {
		return
	}
	if err := prepareImageStudioGenerateIdempotency(c, subject.UserID, &req); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeUserIdempotentJSON(
		c,
		"user.image_studio.generate",
		req,
		service.DefaultWriteIdempotencyTTL(),
		func(ctx context.Context) (any, error) {
			job, _, err := h.studio.CreatePendingJob(ctx, subject.UserID, req)
			if err != nil {
				return nil, err
			}
			if job.APIKeyID == nil {
				return nil, service.ErrImageStudioAPIKey
			}
			return gin.H{
				"job":   job,
				"async": true,
				"poll":  fmt.Sprintf("/api/v1/image-studio/jobs/%s", job.ID),
			}, nil
		},
	)
}

func prepareImageStudioGenerateIdempotency(
	c *gin.Context,
	userID int64,
	req *service.ImageStudioGenerateRequest,
) error {
	if c == nil || c.Request == nil || req == nil {
		return service.ErrIdempotencyKeyRequired
	}
	key, err := service.NormalizeIdempotencyKey(c.GetHeader("Idempotency-Key"))
	if err != nil {
		return err
	}
	if key == "" {
		return service.ErrIdempotencyKeyRequired
	}
	fingerprint, err := service.BuildIdempotencyFingerprint(
		http.MethodPost,
		"/api/v1/image-studio/generate",
		fmt.Sprintf("user:%d", userID),
		req,
	)
	if err != nil {
		return err
	}
	req.IdempotencyKeyHash = service.HashIdempotencyKey(key)
	req.IdempotencyFingerprint = fingerprint
	return nil
}

func (h *ImageStudioHandler) UploadReference(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	release, err := h.studio.AcquireReferenceUpload(
		c.Request.Context(),
		subject.UserID,
		time.Now().UTC(),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer release()
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.ErrorFrom(c, service.ErrImageStudioReferenceInvalid)
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, service.ImageStudioReferenceUploadMaxBytes+1))
	if err != nil {
		response.ErrorFrom(c, service.ErrImageStudioReferenceInvalid)
		return
	}
	if int64(len(data)) > service.ImageStudioReferenceUploadMaxBytes {
		response.ErrorFrom(c, service.ErrImageStudioReferenceTooLarge)
		return
	}
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	reference, err := h.studio.CreateReference(
		c.Request.Context(),
		subject.UserID,
		header.Filename,
		contentType,
		data,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"reference": reference})
}

func (h *ImageStudioHandler) DeleteReference(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	referenceID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	if err := h.studio.DeleteReference(c.Request.Context(), subject.UserID, referenceID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func bindImageStudioGenerateRequest(c *gin.Context, req *service.ImageStudioGenerateRequest) bool {
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(req); err != nil {
		writeImageStudioGenerateBindError(c, err)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeImageStudioGenerateBindError(c, err)
		return false
	}
	return true
}

func writeImageStudioGenerateBindError(c *gin.Context, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		response.ErrorWithDetails(
			c,
			http.StatusRequestEntityTooLarge,
			"Image studio request body is too large",
			"IMAGE_STUDIO_REQUEST_TOO_LARGE",
			nil,
		)
		return
	}
	response.BadRequest(c, "Invalid request body")
}

func requireImageStudioUUIDParam(c *gin.Context) (string, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.ErrorFrom(c, service.ErrImageStudioInvalidID)
		return "", false
	}
	return id.String(), true
}

func (h *ImageStudioHandler) ActiveJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobs, err := h.studio.ListActiveJobs(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var job *service.ImageStudioJob
	if len(jobs) > 0 {
		job = &jobs[0]
	}
	response.Success(c, gin.H{"job": job, "jobs": jobs})
}

func (h *ImageStudioHandler) CancelJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	job, err := h.studio.CancelJob(c.Request.Context(), subject.UserID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *ImageStudioHandler) processWorkerItem(
	ctx context.Context,
	job *service.ImageStudioJob,
	item *service.ImageStudioItem,
	body string,
) (*service.ImageStudioImagePayload, float64, error) {
	if job == nil || item == nil || job.APIKeyID == nil {
		return nil, 0, service.ErrImageStudioAPIKey
	}
	storedKey, err := h.apiKeyService.GetByID(ctx, *job.APIKeyID)
	if err != nil {
		return nil, 0, service.ErrImageStudioAPIKey
	}
	apiKey, err := h.apiKeyService.GetByKey(ctx, storedKey.Key)
	if err != nil {
		return nil, 0, service.ErrImageStudioAPIKey
	}
	requestID := fmt.Sprintf("image-studio:%s:%s", job.ID, item.ID)
	ctx = context.WithValue(ctx, ctxkey.ClientRequestID, requestID)
	ctx = service.WithImageStudioBillingActualCostCap(
		ctx,
		service.ImageStudioPerItemBillingCap(job),
	)
	workerReq, err := h.studio.BuildWorkerRequest(ctx, job, body)
	if err != nil {
		h.studio.RecordGenerateFailure(job.Model, job.Size, err.Error())
		return nil, 0, err
	}
	images, actualCost, err := h.invokeGatewayImagesOnce(ctx, apiKey, workerReq)
	if err != nil {
		h.studio.RecordGenerateFailure(job.Model, job.Size, err.Error())
		return nil, 0, err
	}
	if len(images) == 0 {
		return nil, 0, errors.New("image studio gateway returned no image")
	}
	return &images[0], actualCost, nil
}

func ProvideImageStudioWorkerRuntime(
	studio *service.ImageStudioService,
	handler *ImageStudioHandler,
) *ImageStudioWorkerRuntime {
	runtime := NewImageStudioWorkerRuntime(
		studio,
		handler.processWorkerItem,
		ImageStudioWorkerRuntimeOptions{},
	)
	runtime.Start()
	return runtime
}

func (h *ImageStudioHandler) ListJobs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, _ := parseIntQueryDefault(c, "page", 1)
	pageSize, _ := parseIntQueryDefault(c, "page_size", 12)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}
	jobs, total, err := h.studio.ListJobsPage(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	if total > 0 && len(jobs) == 0 && page > pages {
		page = pages
		jobs, total, err = h.studio.ListJobsPage(c.Request.Context(), subject.UserID, page, pageSize)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		pages = int(math.Ceil(float64(total) / float64(pageSize)))
	}
	response.Success(c, gin.H{
		"jobs":      jobs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"pages":     pages,
	})
}

func (h *ImageStudioHandler) GetJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	job, err := h.studio.GetJob(c.Request.Context(), subject.UserID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, job)
}

func (h *ImageStudioHandler) DeleteJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	if err := h.studio.DeleteJob(c.Request.Context(), subject.UserID, jobID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *ImageStudioHandler) AssetContent(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	data, contentType, err := h.studio.OpenAssetContent(c.Request.Context(), subject.UserID, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", imageStudioPrivateCacheControl)
	if len(data) == 0 && contentType != "" && strings.HasPrefix(contentType, "http") {
		c.Redirect(http.StatusFound, contentType)
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	c.Data(http.StatusOK, contentType, data)
}

func (h *ImageStudioHandler) AssetThumbnail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	data, contentType, err := h.studio.OpenAssetThumbnail(c.Request.Context(), subject.UserID, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", imageStudioPrivateCacheControl)
	c.Data(http.StatusOK, contentType, data)
}

func (h *ImageStudioHandler) AssetDownload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	assetID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	data, contentType, err := h.studio.OpenAssetContent(c.Request.Context(), subject.UserID, assetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", imageStudioPrivateCacheControl)
	if len(data) == 0 && contentType != "" && strings.HasPrefix(contentType, "http") {
		c.Redirect(http.StatusFound, contentType)
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	ext := ".png"
	switch contentType {
	case "image/jpeg":
		ext = ".jpg"
	case "image/webp":
		ext = ".webp"
	}
	filename := "image-studio"
	if len(assetID) >= 8 {
		filename += "-" + assetID[:8]
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s%s\"", filename, ext))
	c.Data(http.StatusOK, contentType, data)
}

func (h *ImageStudioHandler) JobDownload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	jobID, ok := requireImageStudioUUIDParam(c)
	if !ok {
		return
	}
	data, filename, err := h.studio.OpenJobArchive(c.Request.Context(), subject.UserID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", imageStudioPrivateCacheControl)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "application/zip", data)
}

func imageStudioGenerationCount(body string) int {
	count := int(gjson.Get(body, "n").Int())
	if count <= 0 {
		return 1
	}
	return count
}

func imageStudioSingleImageRequestBody(body string) (string, error) {
	if !gjson.Valid(body) {
		return "", fmt.Errorf("invalid image generation request")
	}
	return sjson.Set(body, "n", 1)
}

func (h *ImageStudioHandler) invokeGatewayImagesOnce(ctx context.Context, apiKey *service.APIKey, workerReq *service.ImageStudioWorkerRequest) ([]service.ImageStudioImagePayload, float64, error) {
	if workerReq == nil || len(workerReq.Body) == 0 {
		return nil, 0, errors.New("image studio worker request is empty")
	}
	rec := httptest.NewRecorder()
	gwCtx, _ := gin.CreateTestContext(rec)
	endpoint := strings.TrimSpace(workerReq.Endpoint)
	if endpoint == "" {
		endpoint = "/v1/images/generations"
	}
	gwCtx.Request = httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(workerReq.Body))
	costCapture := service.NewImageStudioBillingCapture()
	gwCtx.Request = gwCtx.Request.WithContext(service.WithImageStudioBillingCapture(ctx, costCapture))
	contentType := strings.TrimSpace(workerReq.ContentType)
	if contentType == "" {
		contentType = "application/json"
	}
	gwCtx.Request.Header.Set("Content-Type", contentType)
	gwCtx.Request.Header.Set("Authorization", "Bearer "+apiKey.Key)
	if requestID, _ := ctx.Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(requestID) != "" {
		gwCtx.Request.Header.Set("Idempotency-Key", strings.TrimSpace(requestID))
	}
	gwCtx.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	if apiKey.Group != nil && service.IsGroupContextValid(apiKey.Group) {
		gwCtx.Request = gwCtx.Request.WithContext(context.WithValue(gwCtx.Request.Context(), ctxkey.Group, apiKey.Group))
	}
	concurrency := 1
	if apiKey.User != nil && apiKey.User.Concurrency > 0 {
		concurrency = apiKey.User.Concurrency
	}
	gwCtx.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{
		UserID:      apiKey.UserID,
		Concurrency: concurrency,
	})
	if err := h.dispatchImageStudioGateway(gwCtx, workerReq); err != nil {
		return nil, 0, err
	}
	if rec.Code >= 400 {
		return nil, 0, fmt.Errorf("image generation provider request failed with status %d", rec.Code)
	}
	respBody, _ := io.ReadAll(rec.Body)
	var parsed struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, 0, err
	}
	out := make([]service.ImageStudioImagePayload, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		switch {
		case item.B64JSON != "":
			data, err := base64.StdEncoding.DecodeString(item.B64JSON)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, service.ImageStudioImagePayload{Data: data, ContentType: "image/png"})
		case item.URL != "":
			if strings.HasPrefix(item.URL, "data:") {
				data, ct, err := service.DecodeImageStudioDataURL(item.URL)
				if err != nil {
					return nil, 0, err
				}
				out = append(out, service.ImageStudioImagePayload{Data: data, ContentType: ct})
				continue
			}
			data, ct, err := service.FetchImageStudioRemoteURL(ctx, item.URL)
			if err != nil {
				return nil, 0, err
			}
			out = append(out, service.ImageStudioImagePayload{Data: data, ContentType: ct})
		}
	}
	normalized, err := service.NormalizeImageStudioPayloads(ctx, out)
	if err != nil {
		return nil, 0, err
	}
	costCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if capturedCost, ok := costCapture.Wait(costCtx); ok {
		return normalized, capturedCost, nil
	}
	actualCostResult := gjson.GetBytes(respBody, "usage.total_cost")
	if actualCostResult.Exists() && actualCostResult.Float() >= 0 {
		return normalized, actualCostResult.Float(), nil
	}
	return nil, 0, errors.New("image studio authoritative usage cost is unavailable")
}

func (h *ImageStudioHandler) dispatchImageStudioGateway(
	gwCtx *gin.Context,
	workerReq *service.ImageStudioWorkerRequest,
) error {
	if h == nil || h.gateway == nil || gwCtx == nil || workerReq == nil {
		return errors.New("image studio gateway is unavailable")
	}
	switch strings.ToLower(strings.TrimSpace(workerReq.Platform)) {
	case service.PlatformOpenAI:
		h.gateway.Images(gwCtx)
	case service.PlatformGrok:
		h.gateway.GrokImages(gwCtx)
	default:
		return service.ErrImageStudioProviderNotSupported
	}
	return nil
}

func parseInt64Query(c *gin.Context, key string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, false
	}
	var v int64
	if _, err := fmt.Sscan(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

func parseIntQueryDefault(c *gin.Context, key string, def int) (int, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def, true
	}
	var v int
	if _, err := fmt.Sscan(raw, &v); err != nil {
		return def, true
	}
	return v, true
}
