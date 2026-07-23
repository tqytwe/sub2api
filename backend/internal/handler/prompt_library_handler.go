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
	Location string `xml:"loc"`
	Modified string `xml:"lastmod,omitempty"`
}

type promptSitemapURLSet struct {
	XMLName xml.Name           `xml:"urlset"`
	XMLNS   string             `xml:"xmlns,attr"`
	URLs    []promptSitemapURL `xml:"url"`
}

var promptSitemapStaticPaths = []string{
	"/",
	"/models",
	"/docs",
	"/en/",
	"/en/models",
	"/en/docs",
	"/image-studio",
	"/prompts",
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

func buildPromptLibrarySitemap(origin string, prompts []service.PublicPrompt) ([]byte, error) {
	origin = strings.TrimRight(origin, "/")
	urls := make([]promptSitemapURL, 0, len(promptSitemapStaticPaths)+len(prompts))
	for _, path := range promptSitemapStaticPaths {
		urls = append(urls, promptSitemapURL{Location: origin + path})
	}
	for _, prompt := range prompts {
		entry := promptSitemapURL{
			Location: fmt.Sprintf("%s/prompts/%d", origin, prompt.ID),
		}
		if prompt.PublishedAt != nil {
			entry.Modified = prompt.PublishedAt.UTC().Format(time.DateOnly)
		}
		urls = append(urls, entry)
	}
	body, err := xml.Marshal(promptSitemapURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	})
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), body...), nil
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
