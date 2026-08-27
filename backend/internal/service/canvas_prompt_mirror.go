package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	CanvasPromptMirrorSourceKey  = "canvas.jisudeng.com"
	CanvasPromptMirrorSourceURL  = "https://canvas.jisudeng.com/api/prompts"
	canvasPromptMirrorPageSize   = 500
	canvasPromptMirrorMaxPages   = 10
	canvasPromptMirrorMaxItems   = 5000
	canvasPromptMirrorMaxBody    = 8 << 20
	canvasPromptMirrorTimeout    = 15 * time.Second
	canvasPromptMirrorMaxID      = 255
	canvasPromptMirrorMaxTitle   = 4096
	canvasPromptMirrorMaxPrompt  = 512 << 10
	canvasPromptMirrorMaxURL     = 4096
	canvasPromptMirrorMaxPreview = 1 << 20
)

var (
	ErrCanvasPromptMirrorUnavailable = errors.New("canvas prompt mirror is unavailable")
	ErrCanvasPromptMirrorInvalidPage = errors.New("canvas prompt source returned an invalid page")
	ErrCanvasPromptMirrorLimit       = errors.New("canvas prompt source exceeded mirror limits")
)

// CanvasPromptMirrorItem is the normalized, image-only source record stored in
// the mirror. It deliberately has no media_type input: every item returned by
// this module is an image prompt, and a source value can never turn it into a
// video prompt by accident.
type CanvasPromptMirrorItem struct {
	ExternalID      string
	Title           string
	Prompt          string
	CoverURL        string
	SourceURL       string
	Preview         string
	Category        string
	Tags            []string
	ContentHash     string
	SourceCreatedAt *time.Time
	SourceUpdatedAt *time.Time
}

type CanvasPromptMirrorSnapshot struct {
	Items          []CanvasPromptMirrorItem
	Categories     []string
	Revision       string
	CategoriesHash string
	RefreshedAt    time.Time
}

type CanvasPromptMirrorManifest struct {
	MediaType  string    `json:"media_type"`
	Revision   string    `json:"revision"`
	Total      int64     `json:"total"`
	Categories []string  `json:"categories"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CanvasPromptMirrorCatalogItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// PromptText follows the platform prompt catalog contract. Prompt remains
	// as a compatibility alias for older mobile builds using the Canvas field.
	PromptText     string        `json:"prompt_text"`
	Prompt         string        `json:"prompt,omitempty"`
	CoverURL       string        `json:"cover_url,omitempty"`
	CoverSourceURL string        `json:"-"`
	SourceURL      string        `json:"source_url,omitempty"`
	Preview        string        `json:"preview,omitempty"`
	Category       string        `json:"category,omitempty"`
	Categories     []string      `json:"categories,omitempty"`
	Tags           []string      `json:"tags"`
	MediaType      string        `json:"media_type"`
	Featured       bool          `json:"featured"`
	Version        int           `json:"version"`
	Description    string        `json:"description,omitempty"`
	Purpose        string        `json:"purpose,omitempty"`
	Style          string        `json:"style,omitempty"`
	Subject        string        `json:"subject,omitempty"`
	Media          []PromptMedia `json:"media"`
	CreatedAt      time.Time     `json:"created_at,omitempty"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type CanvasPromptMirrorCatalog struct {
	Items      []CanvasPromptMirrorCatalogItem `json:"items"`
	Total      int64                           `json:"total"`
	Page       int                             `json:"page"`
	PageSize   int                             `json:"page_size"`
	Pages      int                             `json:"pages"`
	Revision   string                          `json:"revision"`
	Categories []string                        `json:"categories"`
}

type CanvasPromptMirrorDelta struct {
	Cursor     string                          `json:"cursor"`
	Revision   string                          `json:"revision"`
	Version    string                          `json:"version"`
	ETag       string                          `json:"etag"`
	Items      []CanvasPromptMirrorCatalogItem `json:"items"`
	DeletedIDs []string                        `json:"deleted_ids"`
	Categories *[]string                       `json:"categories,omitempty"`
}

// CanvasPromptMirrorRepository is intentionally narrower than PromptLibrary.
// The mirror has its own tombstones and revision, so an upstream refresh cannot
// publish or alter administrator-owned prompt records.
type CanvasPromptMirrorRepository interface {
	Refresh(context.Context, CanvasPromptMirrorSnapshot) (*CanvasPromptMirrorManifest, error)
	Manifest(context.Context) (*CanvasPromptMirrorManifest, error)
	Catalog(context.Context, int, int) (*CanvasPromptMirrorCatalog, error)
	Delta(context.Context, time.Time) (*CanvasPromptMirrorDelta, error)
	Get(context.Context, string) (*CanvasPromptMirrorCatalogItem, error)
}

type canvasPromptMirrorSourcePage struct {
	Code int `json:"code"`
	Data struct {
		Items []canvasPromptMirrorSourceItem `json:"items"`
		Total int64                          `json:"total"`
	} `json:"data"`
}

type canvasPromptMirrorSourceItem struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Prompt    string   `json:"prompt"`
	CoverURL  string   `json:"coverUrl"`
	Tags      []string `json:"tags"`
	Category  string   `json:"category"`
	GithubURL string   `json:"githubUrl"`
	Preview   string   `json:"preview"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type canvasPromptMirrorSource interface {
	FetchPage(context.Context, int, int) (canvasPromptMirrorSourcePage, error)
}

type canvasPromptMirrorHTTPSource struct {
	client  *http.Client
	baseURL string
}

func newCanvasPromptMirrorHTTPSource(client *http.Client) *canvasPromptMirrorHTTPSource {
	if client == nil {
		client = &http.Client{Timeout: canvasPromptMirrorTimeout}
	}
	return &canvasPromptMirrorHTTPSource{client: client, baseURL: CanvasPromptMirrorSourceURL}
}

func (s *canvasPromptMirrorHTTPSource) FetchPage(ctx context.Context, page, pageSize int) (canvasPromptMirrorSourcePage, error) {
	var out canvasPromptMirrorSourcePage
	parsed, err := url.Parse(s.baseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "canvas.jisudeng.com" || parsed.Path != "/api/prompts" {
		return out, ErrCanvasPromptMirrorUnavailable
	}
	query := parsed.Query()
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))
	parsed.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return out, fmt.Errorf("create canvas prompt request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return out, fmt.Errorf("fetch canvas prompt page %d: %w", page, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("fetch canvas prompt page %d: http status %d", page, resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, canvasPromptMirrorMaxBody+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return out, fmt.Errorf("read canvas prompt page %d: %w", page, err)
	}
	if len(body) > canvasPromptMirrorMaxBody {
		return out, ErrCanvasPromptMirrorLimit
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("decode canvas prompt page %d: %w", page, err)
	}
	if out.Code != 0 {
		return out, fmt.Errorf("canvas prompt page %d returned code %d", page, out.Code)
	}
	return out, nil
}

type CanvasPromptMirrorService struct {
	repo   CanvasPromptMirrorRepository
	source canvasPromptMirrorSource
	now    func() time.Time
}

func NewCanvasPromptMirrorService(repo CanvasPromptMirrorRepository) *CanvasPromptMirrorService {
	return &CanvasPromptMirrorService{
		repo:   repo,
		source: newCanvasPromptMirrorHTTPSource(nil),
		now:    time.Now,
	}
}

// NewCanvasPromptMirrorServiceWithSource is used by focused tests and keeps
// the production constructor pinned to the fixed Canvas source URL.
func NewCanvasPromptMirrorServiceWithSource(repo CanvasPromptMirrorRepository, source canvasPromptMirrorSource) *CanvasPromptMirrorService {
	return &CanvasPromptMirrorService{repo: repo, source: source, now: time.Now}
}

func (s *CanvasPromptMirrorService) Refresh(ctx context.Context) (*CanvasPromptMirrorManifest, error) {
	if s == nil || s.repo == nil || s.source == nil {
		return nil, ErrCanvasPromptMirrorUnavailable
	}
	items, categories, err := s.fetchAll(ctx)
	if err != nil {
		return nil, err
	}
	revision := canvasPromptMirrorRevision(items, categories)
	categoriesHash := canvasPromptMirrorCategoriesHash(categories)
	snapshot := CanvasPromptMirrorSnapshot{
		Items: items, Categories: categories, Revision: revision,
		CategoriesHash: categoriesHash, RefreshedAt: s.now().UTC(),
	}
	return s.repo.Refresh(ctx, snapshot)
}

func (s *CanvasPromptMirrorService) Manifest(ctx context.Context) (*CanvasPromptMirrorManifest, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCanvasPromptMirrorUnavailable
	}
	return s.repo.Manifest(ctx)
}

func (s *CanvasPromptMirrorService) Catalog(ctx context.Context, page, pageSize int) (*CanvasPromptMirrorCatalog, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCanvasPromptMirrorUnavailable
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repo.Catalog(ctx, page, pageSize)
}

func (s *CanvasPromptMirrorService) Delta(ctx context.Context, since time.Time) (*CanvasPromptMirrorDelta, error) {
	if s == nil || s.repo == nil {
		return nil, ErrCanvasPromptMirrorUnavailable
	}
	return s.repo.Delta(ctx, since)
}

func (s *CanvasPromptMirrorService) Get(ctx context.Context, id string) (*CanvasPromptMirrorCatalogItem, error) {
	if s == nil || s.repo == nil || strings.TrimSpace(id) == "" {
		return nil, ErrCanvasPromptMirrorUnavailable
	}
	return s.repo.Get(ctx, strings.TrimSpace(id))
}

func (s *CanvasPromptMirrorService) fetchAll(ctx context.Context) ([]CanvasPromptMirrorItem, []string, error) {
	var items []CanvasPromptMirrorItem
	var expected int64 = -1
	seen := make(map[string]struct{})
	categorySet := make(map[string]struct{})
	for page := 1; page <= canvasPromptMirrorMaxPages; page++ {
		result, err := s.source.FetchPage(ctx, page, canvasPromptMirrorPageSize)
		if err != nil {
			return nil, nil, err
		}
		if result.Data.Total < 0 || result.Data.Total > canvasPromptMirrorMaxItems {
			return nil, nil, ErrCanvasPromptMirrorLimit
		}
		if expected < 0 {
			expected = result.Data.Total
		} else if expected != result.Data.Total {
			return nil, nil, fmt.Errorf("%w: total changed between pages", ErrCanvasPromptMirrorInvalidPage)
		}
		for _, raw := range result.Data.Items {
			item, err := normalizeCanvasPromptMirrorItem(raw)
			if err != nil {
				return nil, nil, err
			}
			if _, ok := seen[item.ExternalID]; ok {
				return nil, nil, fmt.Errorf("%w: duplicate id %q", ErrCanvasPromptMirrorInvalidPage, item.ExternalID)
			}
			seen[item.ExternalID] = struct{}{}
			if item.Category != "" {
				categorySet[item.Category] = struct{}{}
			}
			items = append(items, item)
		}
		if int64(len(items)) > expected || int64(len(items)) > canvasPromptMirrorMaxItems {
			return nil, nil, ErrCanvasPromptMirrorLimit
		}
		if int64(len(items)) == expected {
			break
		}
		if len(result.Data.Items) == 0 {
			return nil, nil, fmt.Errorf("%w: page %d was empty before total", ErrCanvasPromptMirrorInvalidPage, page)
		}
		if page == canvasPromptMirrorMaxPages {
			return nil, nil, ErrCanvasPromptMirrorLimit
		}
	}
	if expected < 0 || int64(len(items)) != expected {
		return nil, nil, fmt.Errorf("%w: expected %d items, got %d", ErrCanvasPromptMirrorInvalidPage, expected, len(items))
	}
	categories := make([]string, 0, len(categorySet))
	for category := range categorySet {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	sort.Slice(items, func(i, j int) bool { return items[i].ExternalID < items[j].ExternalID })
	return items, categories, nil
}

func normalizeCanvasPromptMirrorItem(raw canvasPromptMirrorSourceItem) (CanvasPromptMirrorItem, error) {
	item := CanvasPromptMirrorItem{
		ExternalID: strings.TrimSpace(raw.ID), Title: strings.TrimSpace(raw.Title),
		Prompt:   strings.TrimSpace(strings.ReplaceAll(raw.Prompt, "\r\n", "\n")),
		CoverURL: strings.TrimSpace(raw.CoverURL), SourceURL: strings.TrimSpace(raw.GithubURL),
		Preview: strings.TrimSpace(raw.Preview), Category: strings.TrimSpace(raw.Category),
		Tags: normalizeCanvasPromptMirrorTags(raw.Tags),
	}
	if item.ExternalID == "" || len(item.ExternalID) > canvasPromptMirrorMaxID || item.Title == "" || len(item.Title) > canvasPromptMirrorMaxTitle || item.Prompt == "" || len(item.Prompt) > canvasPromptMirrorMaxPrompt {
		return CanvasPromptMirrorItem{}, fmt.Errorf("%w: invalid id, title, or prompt", ErrCanvasPromptMirrorInvalidPage)
	}
	if item.Category != "" && len(item.Category) > 255 {
		return CanvasPromptMirrorItem{}, fmt.Errorf("%w: category too long", ErrCanvasPromptMirrorInvalidPage)
	}
	for _, candidate := range []struct {
		name, value string
		max         int
	}{
		{"cover_url", item.CoverURL, canvasPromptMirrorMaxURL}, {"source_url", item.SourceURL, canvasPromptMirrorMaxURL},
	} {
		if candidate.value != "" {
			if err := validateCanvasPromptMirrorURL(candidate.value); err != nil {
				return CanvasPromptMirrorItem{}, fmt.Errorf("%w: %s: %v", ErrCanvasPromptMirrorInvalidPage, candidate.name, err)
			}
			if len(candidate.value) > candidate.max {
				return CanvasPromptMirrorItem{}, fmt.Errorf("%w: %s too long", ErrCanvasPromptMirrorInvalidPage, candidate.name)
			}
		}
	}
	// Canvas preview is descriptive Markdown, often ![](https://...). It is
	// never fetched by this service, so validate its bounded text shape rather
	// than incorrectly treating it as an absolute URL.
	if len(item.Preview) > canvasPromptMirrorMaxPreview {
		return CanvasPromptMirrorItem{}, fmt.Errorf("%w: preview too long", ErrCanvasPromptMirrorInvalidPage)
	}
	created, err := parseCanvasPromptMirrorTime(raw.CreatedAt)
	if err != nil {
		return CanvasPromptMirrorItem{}, fmt.Errorf("%w: createdAt: %v", ErrCanvasPromptMirrorInvalidPage, err)
	}
	updated, err := parseCanvasPromptMirrorTime(raw.UpdatedAt)
	if err != nil {
		return CanvasPromptMirrorItem{}, fmt.Errorf("%w: updatedAt: %v", ErrCanvasPromptMirrorInvalidPage, err)
	}
	item.SourceCreatedAt, item.SourceUpdatedAt = created, updated
	item.ContentHash = canvasPromptMirrorItemHash(item)
	return item, nil
}

func normalizeCanvasPromptMirrorTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || len(tag) > 128 {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func validateCanvasPromptMirrorURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return errors.New("must be an absolute http(s) URL")
	}
	return nil
}

func parseCanvasPromptMirrorTime(raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func canvasPromptMirrorItemHash(item CanvasPromptMirrorItem) string {
	payload := struct {
		ID, Title, Prompt, CoverURL, SourceURL, Preview, Category string
		Tags                                                      []string
	}{item.ExternalID, item.Title, item.Prompt, item.CoverURL, item.SourceURL, item.Preview, item.Category, item.Tags}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func canvasPromptMirrorCategoriesHash(categories []string) string {
	copyCategories := append([]string(nil), categories...)
	sort.Strings(copyCategories)
	encoded, _ := json.Marshal(copyCategories)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func canvasPromptMirrorRevision(items []CanvasPromptMirrorItem, categories []string) string {
	type entry struct{ ID, Hash string }
	entries := make([]entry, 0, len(items))
	for _, item := range items {
		entries = append(entries, entry{item.ExternalID, item.ContentHash})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	payload := struct {
		Items      []entry
		Categories []string
	}{entries, append([]string(nil), categories...)}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
