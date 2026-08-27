package handler

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CanvasPromptMirrorHandler struct {
	service    *service.CanvasPromptMirrorService
	httpClient *http.Client
}

const canvasPromptMirrorCoverMaxBytes = 16 << 20

func NewCanvasPromptMirrorHandler(svc *service.CanvasPromptMirrorService) *CanvasPromptMirrorHandler {
	return &CanvasPromptMirrorHandler{
		service:    svc,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (h *CanvasPromptMirrorHandler) ready(c *gin.Context) bool {
	if h == nil || h.service == nil {
		response.InternalError(c, "提示词目录暂不可用")
		return false
	}
	return true
}

// The Creation Space directory is a shared prompt source. Its upstream
// records do not carry a separate media type, so both mobile creation modes
// intentionally read the same cover-and-body mirror. The requested kind is
// still validated to keep arbitrary query values out of the contract.
func requireCanvasPromptMediaType(c *gin.Context) bool {
	mediaType := strings.ToLower(strings.TrimSpace(c.Query("media_type")))
	if mediaType != "" && mediaType != "image" && mediaType != "video" {
		response.BadRequest(c, "media_type must be image or video")
		return false
	}
	return true
}

func (h *CanvasPromptMirrorHandler) Manifest(c *gin.Context) {
	if !requireCanvasPromptMediaType(c) || !h.ready(c) {
		return
	}
	manifest, err := h.service.Manifest(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	etag := `"` + strings.Trim(manifest.Revision, `"`) + `"`
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	c.Header("ETag", etag)
	if strings.TrimSpace(c.GetHeader("If-None-Match")) == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}
	response.Success(c, manifest)
}

func (h *CanvasPromptMirrorHandler) Catalog(c *gin.Context) {
	if !requireCanvasPromptMediaType(c) || !h.ready(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	result, err := h.service.Catalog(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *CanvasPromptMirrorHandler) Delta(c *gin.Context) {
	if !requireCanvasPromptMediaType(c) || !h.ready(c) {
		return
	}
	rawSince := strings.TrimSpace(c.Query("since"))
	if rawSince == "" {
		response.BadRequest(c, "since must be an RFC3339Nano cursor")
		return
	}
	since, err := time.Parse(time.RFC3339Nano, rawSince)
	if err != nil {
		response.BadRequest(c, "since must be an RFC3339Nano cursor")
		return
	}
	result, err := h.service.Delta(c.Request.Context(), since)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	etag := `"` + strings.Trim(strings.TrimSpace(result.ETag), `"`) + `"`
	c.Header("Cache-Control", "private, max-age=0, must-revalidate")
	c.Header("ETag", etag)
	if strings.TrimSpace(c.GetHeader("If-None-Match")) == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}
	response.Success(c, result)
}

func (h *CanvasPromptMirrorHandler) Categories(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	manifest, err := h.service.Manifest(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	categories := make([]service.PromptCategory, 0, len(manifest.Categories))
	for index, name := range manifest.Categories {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		categories = append(categories, service.PromptCategory{
			ID: int64(index + 1), Slug: name, NameZH: name, Dimension: "canvas",
			SortOrder: index, Enabled: true, UpdatedAt: manifest.UpdatedAt,
		})
	}
	response.Success(c, categories)
}

func (h *CanvasPromptMirrorHandler) Get(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		response.BadRequest(c, "prompt id is required")
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if item == nil {
		response.NotFound(c, "canvas prompt not found")
		return
	}
	response.Success(c, item)
}

func canvasPromptMirrorCoverURL(raw string) (*url.URL, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Path == "" {
		return nil, false
	}
	// The mirror normalizer accepts only remote source metadata. The proxy has
	// an additional host allowlist so this route can never be made into a
	// generic server-side request proxy through a poisoned database row.
	switch strings.ToLower(parsed.Hostname()) {
	case "raw.githubusercontent.com", "pbs.twimg.com":
		return parsed, true
	default:
		return nil, false
	}
}

func (h *CanvasPromptMirrorHandler) Cover(c *gin.Context) {
	if !h.ready(c) {
		return
	}
	item, err := h.service.Get(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if item == nil {
		response.NotFound(c, "canvas prompt not found")
		return
	}
	target, ok := canvasPromptMirrorCoverURL(item.CoverSourceURL)
	if !ok {
		response.NotFound(c, "canvas prompt cover is unavailable")
		return
	}
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		response.NotFound(c, "canvas prompt cover is unavailable")
		return
	}
	request.Header.Set("Accept", "image/*")
	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(request)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "canvas prompt cover could not be fetched")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		response.Error(c, http.StatusBadGateway, "canvas prompt cover could not be fetched")
		return
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "image/") {
		response.Error(c, http.StatusBadGateway, "canvas prompt cover is not an image")
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, canvasPromptMirrorCoverMaxBytes+1))
	if err != nil || len(body) == 0 || len(body) > canvasPromptMirrorCoverMaxBytes {
		response.Error(c, http.StatusBadGateway, "canvas prompt cover is too large")
		return
	}
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, contentType, body)
}
