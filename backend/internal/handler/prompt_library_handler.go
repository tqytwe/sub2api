package handler

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type PromptLibraryHandler struct {
	service *service.PromptLibraryService
}

func NewPromptLibraryHandler(promptService *service.PromptLibraryService) *PromptLibraryHandler {
	return &PromptLibraryHandler{service: promptService}
}

func (h *PromptLibraryHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	filter := service.PromptListFilter{
		Query:      c.Query("q"),
		Purpose:    c.Query("purpose"),
		Style:      c.Query("style"),
		Subject:    c.Query("subject"),
		Model:      c.Query("model"),
		Size:       c.Query("size"),
		Sort:       normalizePromptSort(c.Query("sort")),
		Pagination: pagination.PaginationParams{Page: page, PageSize: pageSize},
	}
	filter.ReferenceRequirement = promptReferenceRequirement(c.Query("reference"))
	filter.Featured = optionalQueryBool(c.Query("featured"))
	if favorite := optionalQueryBool(c.Query("favorite")); favorite != nil {
		filter.FavoritedOnly = *favorite
	}

	var userID *int64
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		id := subject.UserID
		userID = &id
	}
	rows, result, err := h.service.ListPublic(c.Request.Context(), filter, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, rows, result.Total, result.Page, result.PageSize)
}

func (h *PromptLibraryHandler) Get(c *gin.Context) {
	id, ok := promptPathID(c)
	if !ok {
		return
	}
	var userID *int64
	if subject, authenticated := middleware.GetAuthSubjectFromContext(c); authenticated {
		value := subject.UserID
		userID = &value
	}
	prompt, err := h.service.GetPublic(c.Request.Context(), id, userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, prompt)
}

func (h *PromptLibraryHandler) Categories(c *gin.Context) {
	rows, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rows)
}

func (h *PromptLibraryHandler) Favorite(c *gin.Context) {
	h.setFavorite(c, true)
}

func (h *PromptLibraryHandler) Unfavorite(c *gin.Context) {
	h.setFavorite(c, false)
}

func (h *PromptLibraryHandler) setFavorite(c *gin.Context, favorite bool) {
	id, ok := promptPathID(c)
	if !ok {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	state, err := h.service.SetFavorite(c.Request.Context(), id, subject.UserID, favorite)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"prompt_id": id, "favorited": state})
}

func (h *PromptLibraryHandler) Use(c *gin.Context) {
	id, ok := promptPathID(c)
	if !ok {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	result, err := h.service.UsePrompt(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *PromptLibraryHandler) Report(c *gin.Context) {
	id, ok := promptPathID(c)
	if !ok {
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	var input struct {
		Reason string `json:"reason" binding:"required"`
		Detail string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	report, err := h.service.ReportPrompt(
		c.Request.Context(), id, subject.UserID, input.Reason, input.Detail,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, report)
}

type promptSitemapURL struct {
	Location   string                   `xml:"loc"`
	Modified   string                   `xml:"lastmod,omitempty"`
	ChangeFreq string                   `xml:"changefreq,omitempty"`
	Priority   string                   `xml:"priority,omitempty"`
	Alternates []promptSitemapAlternate `xml:"xhtml:link,omitempty"`
}

type promptSitemapAlternate struct {
	Rel      string `xml:"rel,attr"`
	Hreflang string `xml:"hreflang,attr"`
	Href     string `xml:"href,attr"`
}

type promptSitemapURLSet struct {
	XMLName xml.Name           `xml:"urlset"`
	XMLNS   string             `xml:"xmlns,attr"`
	XHTML   string             `xml:"xmlns:xhtml,attr"`
	URLs    []promptSitemapURL `xml:"url"`
}

type promptSitemapStaticPath struct {
	Path       string
	ChangeFreq string
	Priority   string
	Alternates []promptSitemapAlternatePath
}

type promptSitemapAlternatePath struct {
	Hreflang string
	Path     string
}

var promptSitemapStaticPaths = []promptSitemapStaticPath{
	{
		Path:       "/",
		ChangeFreq: "daily",
		Priority:   "1.00",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "zh-CN", Path: "/"},
			{Hreflang: "en", Path: "/en/"},
			{Hreflang: "x-default", Path: "/en/"},
		},
	},
	{
		Path:       "/models",
		ChangeFreq: "daily",
		Priority:   "0.95",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "zh-CN", Path: "/models"},
			{Hreflang: "en", Path: "/en/models"},
			{Hreflang: "x-default", Path: "/en/models"},
		},
	},
	{
		Path:       "/docs",
		ChangeFreq: "weekly",
		Priority:   "0.90",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "zh-CN", Path: "/docs"},
			{Hreflang: "en", Path: "/en/docs"},
			{Hreflang: "x-default", Path: "/en/docs"},
		},
	},
	{
		Path:       "/en/",
		ChangeFreq: "daily",
		Priority:   "0.95",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "en", Path: "/en/"},
			{Hreflang: "zh-CN", Path: "/"},
			{Hreflang: "x-default", Path: "/en/"},
		},
	},
	{
		Path:       "/en/models",
		ChangeFreq: "daily",
		Priority:   "0.90",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "en", Path: "/en/models"},
			{Hreflang: "zh-CN", Path: "/models"},
			{Hreflang: "x-default", Path: "/en/models"},
		},
	},
	{
		Path:       "/en/docs",
		ChangeFreq: "weekly",
		Priority:   "0.85",
		Alternates: []promptSitemapAlternatePath{
			{Hreflang: "en", Path: "/en/docs"},
			{Hreflang: "zh-CN", Path: "/docs"},
			{Hreflang: "x-default", Path: "/en/docs"},
		},
	},
	{Path: "/about", ChangeFreq: "monthly", Priority: "0.60", Alternates: []promptSitemapAlternatePath{
		{Hreflang: "zh-CN", Path: "/about"},
		{Hreflang: "x-default", Path: "/about"},
	}},
	{Path: "/contact", ChangeFreq: "monthly", Priority: "0.60", Alternates: []promptSitemapAlternatePath{
		{Hreflang: "zh-CN", Path: "/contact"},
		{Hreflang: "x-default", Path: "/contact"},
	}},
	{Path: "/download/android", ChangeFreq: "weekly", Priority: "0.55", Alternates: []promptSitemapAlternatePath{
		{Hreflang: "zh-CN", Path: "/download/android"},
		{Hreflang: "x-default", Path: "/download/android"},
	}},
	{Path: "/image-studio", ChangeFreq: "weekly", Priority: "0.70"},
	{Path: "/prompts", ChangeFreq: "daily", Priority: "0.75"},
}

func (h *PromptLibraryHandler) Sitemap(c *gin.Context) {
	prompts := make([]service.PublicPrompt, 0)
	for page := 1; ; page++ {
		rows, result, err := h.service.ListPublic(c.Request.Context(), service.PromptListFilter{
			Sort: "latest",
			Pagination: pagination.PaginationParams{
				Page:     page,
				PageSize: 500,
			},
		}, nil)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		prompts = append(prompts, rows...)
		if result == nil || page >= result.Pages {
			break
		}
	}
	body, err := buildPromptLibrarySitemap(promptRequestOrigin(c.Request), prompts)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
}

func (h *PromptLibraryHandler) Robots(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(buildRobotsTxt(promptRequestOrigin(c.Request))))
}

func (h *PromptLibraryHandler) LLMSTxt(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(buildLLMSTxt(promptRequestOrigin(c.Request))))
}

func buildPromptLibrarySitemap(origin string, prompts []service.PublicPrompt) ([]byte, error) {
	origin = strings.TrimRight(origin, "/")
	urls := make([]promptSitemapURL, 0, len(promptSitemapStaticPaths)+len(prompts))
	for _, path := range promptSitemapStaticPaths {
		urls = append(urls, promptSitemapURL{
			Location:   origin + path.Path,
			ChangeFreq: path.ChangeFreq,
			Priority:   path.Priority,
			Alternates: promptSitemapAlternates(origin, path.Alternates),
		})
	}
	for _, prompt := range prompts {
		entry := promptSitemapURL{
			Location:   fmt.Sprintf("%s/prompts/%d", origin, prompt.ID),
			ChangeFreq: "weekly",
			Priority:   "0.50",
		}
		if prompt.PublishedAt != nil {
			entry.Modified = prompt.PublishedAt.UTC().Format(time.DateOnly)
		}
		urls = append(urls, entry)
	}
	body, err := xml.Marshal(promptSitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		XHTML: "http://www.w3.org/1999/xhtml",
		URLs:  urls,
	})
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
}

func promptSitemapAlternates(origin string, alternates []promptSitemapAlternatePath) []promptSitemapAlternate {
	if len(alternates) == 0 {
		return nil
	}
	links := make([]promptSitemapAlternate, 0, len(alternates))
	for _, alternate := range alternates {
		links = append(links, promptSitemapAlternate{
			Rel:      "alternate",
			Hreflang: alternate.Hreflang,
			Href:     origin + alternate.Path,
		})
	}
	return links
}

func buildRobotsTxt(origin string) string {
	origin = strings.TrimRight(origin, "/")
	return fmt.Sprintf(`User-agent: *
Allow: /
Disallow: /api/
Disallow: /v1/
Disallow: /v1beta/
Disallow: /backend-api/
Disallow: /admin/
Disallow: /setup/
Sitemap: %s/sitemap.xml
`, origin)
}

func buildLLMSTxt(origin string) string {
	origin = strings.TrimRight(origin, "/")
	return fmt.Sprintf(`# Jisudeng

> Access DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and more through one OpenAI-compatible API with unified keys, public model pricing, image APIs, billing, and docs.

Jisudeng is an AI API gateway for developers, teams, and AI tool users. It helps users compare model access, review usage-based pricing, create API keys, connect existing OpenAI SDK clients, use image generation APIs, and read implementation docs from one site.

## Key Links

- English homepage: %s/en/
- Model catalog and pricing: %s/en/models
- API docs: %s/en/docs
- Chinese homepage: %s/
- 中文模型目录与价格: %s/models
- 中文 API 文档: %s/docs
- Support contact: %s/contact
- Sitemap: %s/sitemap.xml

## Core Topics

- OpenAI-compatible API gateway
- DeepSeek, Qwen, Kimi, GLM, GPT, Claude, Gemini and other model access
- Public model catalog, usage-based pricing, and account-specific effective pricing after login
- API Key setup, base URL migration, SDK configuration, image generation, Batch Image, billing, and troubleshooting

## 中文摘要

极速蹬为开发者、团队和 AI 工具用户提供 OpenAI 兼容 API 网关、模型目录、公开价格、接入文档、图像生成、API Key 管理和提示词库。中文页面默认使用中文，英文页面仅在 /en 路径下提供。
`, origin, origin, origin, origin, origin, origin, origin, origin)
}

func promptRequestOrigin(request *http.Request) string {
	_ = request
	return "https://www.jisudeng.com"
}

func promptPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid id")
		return 0, false
	}
	return id, true
}

func optionalQueryBool(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		result := true
		return &result
	case "false", "0":
		result := false
		return &result
	default:
		return nil
	}
}

func promptReferenceRequirement(value string) service.PromptReferenceRequirement {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return service.PromptReferenceNone
	case "optional":
		return service.PromptReferenceOptional
	case "required":
		return service.PromptReferenceRequired
	default:
		return ""
	}
}

func normalizePromptSort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "latest", "popular":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "featured"
	}
}
