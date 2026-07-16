package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type ImageStudioHandler struct {
	studio        *service.ImageStudioService
	gateway       *OpenAIGatewayHandler
	billingCache  *service.BillingCacheService
	apiKeyService *service.APIKeyService
}

func NewImageStudioHandler(
	studio *service.ImageStudioService,
	gateway *OpenAIGatewayHandler,
	billingCache *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
) *ImageStudioHandler {
	return &ImageStudioHandler{
		studio:        studio,
		gateway:       gateway,
		billingCache:  billingCache,
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
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, got)
}

func (h *ImageStudioHandler) Generate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req service.ImageStudioGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	job, body, err := h.studio.CreatePendingJob(c.Request.Context(), subject.UserID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if job.APIKeyID == nil {
		response.ErrorFrom(c, service.ErrImageStudioAPIKey)
		return
	}
	userID := subject.UserID
	jobID := job.ID
	apiKeyID := *job.APIKeyID
	estimatedCost := job.EstimatedCost
	baseCtx := c.Request.Context()

	go h.runGenerateJob(baseCtx, userID, jobID, apiKeyID, estimatedCost, body)

	response.Success(c, gin.H{
		"job":   job,
		"async": true,
		"poll":  fmt.Sprintf("/api/v1/image-studio/jobs/%s", jobID),
	})
}

func (h *ImageStudioHandler) ActiveJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	job, err := h.studio.GetActiveJob(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"job": job})
}

func (h *ImageStudioHandler) runGenerateJob(parent context.Context, userID int64, jobID string, apiKeyID int64, estimatedCost float64, body string) {
	ctx := context.WithoutCancel(parent)
	_ = h.studio.MarkJobRunning(ctx, jobID)

	storedKey, err := h.apiKeyService.GetByID(ctx, apiKeyID)
	if err != nil {
		_, _ = h.studio.CompleteJob(ctx, userID, jobID, nil, 0, service.ErrImageStudioAPIKey.Error())
		return
	}
	apiKey, err := h.apiKeyService.GetByKey(ctx, storedKey.Key)
	if err != nil {
		_, _ = h.studio.CompleteJob(ctx, userID, jobID, nil, 0, service.ErrImageStudioAPIKey.Error())
		return
	}
	images, actualCost, genErr := h.invokeGatewayImages(ctx, apiKey, body, estimatedCost)
	if genErr != nil {
		h.studio.RecordGenerateFailure(
			gjson.Get(body, "model").String(),
			gjson.Get(body, "size").String(),
			errString(genErr),
		)
	}
	if _, completeErr := h.studio.CompleteJob(ctx, userID, jobID, images, actualCost, errString(genErr)); completeErr != nil {
		// Fall back to an explicit failed job so polling never sees orphan "completed" rows.
		_, _ = h.studio.CompleteJob(ctx, userID, jobID, nil, 0, completeErr.Error())
	}
}

func (h *ImageStudioHandler) ListJobs(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	limit, _ := parseIntQueryDefault(c, "limit", 20)
	jobs, err := h.studio.ListJobs(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"jobs": jobs})
}

func (h *ImageStudioHandler) GetJob(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	job, err := h.studio.GetJob(c.Request.Context(), subject.UserID, c.Param("id"))
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
	if err := h.studio.DeleteJob(c.Request.Context(), subject.UserID, c.Param("id")); err != nil {
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
	data, contentType, err := h.studio.OpenAssetContent(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(data) == 0 && contentType != "" && strings.HasPrefix(contentType, "http") {
		c.Redirect(http.StatusFound, contentType)
		return
	}
	if contentType == "" {
		contentType = "image/png"
	}
	c.Data(http.StatusOK, contentType, data)
}

func (h *ImageStudioHandler) AssetDownload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	data, contentType, err := h.studio.OpenAssetContent(c.Request.Context(), subject.UserID, c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
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
	if id := c.Param("id"); len(id) >= 8 {
		filename += "-" + id[:8]
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s%s\"", filename, ext))
	c.Data(http.StatusOK, contentType, data)
}

func (h *ImageStudioHandler) invokeGatewayImages(ctx context.Context, apiKey *service.APIKey, body string, estimatedCost float64) ([]service.ImageStudioImagePayload, float64, error) {
	authKey, err := h.apiKeyService.GetByKey(ctx, apiKey.Key)
	if err != nil {
		return nil, 0, service.ErrImageStudioAPIKey
	}
	apiKey = authKey
	if service.ValidateImageStudioAPIKey(apiKey) != nil {
		return nil, 0, service.ErrImageStudioAPIKey
	}
	if h.gateway == nil {
		return nil, 0, service.ErrImageStudioDisabled
	}
	rec := httptest.NewRecorder()
	gwCtx, _ := gin.CreateTestContext(rec)
	gwCtx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(body))
	gwCtx.Request = gwCtx.Request.WithContext(ctx)
	gwCtx.Request.Header.Set("Content-Type", "application/json")
	gwCtx.Request.Header.Set("Authorization", "Bearer "+apiKey.Key)
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
	h.gateway.Images(gwCtx)
	respBody, _ := io.ReadAll(rec.Body)
	if rec.Code >= 400 {
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = "image generation failed"
		}
		return nil, 0, fmt.Errorf("%s", msg)
	}
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
	actualCost := estimatedCost
	if usageCost := gjson.GetBytes(respBody, "usage.total_cost").Float(); usageCost > 0 {
		actualCost = usageCost
	}
	return normalized, actualCost, nil
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

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
