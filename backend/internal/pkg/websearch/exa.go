package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	exaSearchEndpoint = "https://api.exa.ai/search"
	exaProviderName   = "exa"
	exaMaxResults     = 10
)

// UpstreamStatusError preserves the provider HTTP status without exposing the
// provider response body to clients. Handlers can use it to classify a real
// upstream failure while keeping the public error envelope deterministic.
type UpstreamStatusError struct {
	StatusCode int
	Err        error
}

func (e *UpstreamStatusError) Error() string {
	if e == nil {
		return "websearch upstream status error"
	}
	if e.Err == nil {
		return fmt.Sprintf("websearch upstream status %d", e.StatusCode)
	}
	return fmt.Sprintf("websearch upstream status %d: %v", e.StatusCode, e.Err)
}

func (e *UpstreamStatusError) Unwrap() error { return e.Err }

// ExaProvider executes the Exa search API. The API key is held only in this
// server-side provider and is never included in a response or protocol payload.
type ExaProvider struct {
	apiKey     string
	httpClient *http.Client
	endpoint   string
}

// NewExaProvider creates an Exa provider using the production endpoint.
func NewExaProvider(apiKey string, httpClient *http.Client) *ExaProvider {
	return newExaProviderWithEndpoint(apiKey, exaSearchEndpoint, httpClient)
}

func newExaProviderWithEndpoint(apiKey, endpoint string, httpClient *http.Client) *ExaProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &ExaProvider{apiKey: strings.TrimSpace(apiKey), httpClient: httpClient, endpoint: strings.TrimSpace(endpoint)}
}

func (e *ExaProvider) Name() string { return exaProviderName }

func (e *ExaProvider) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("websearch: empty search query")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if maxResults > exaMaxResults {
		maxResults = exaMaxResults
	}

	payload := exaRequest{
		Query:      query,
		Type:       "auto",
		NumResults: maxResults,
		Contents:   exaContents{Highlights: exaHighlights{MaxCharacters: 500}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("exa: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("exa: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("x-api-key", e.apiKey)

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("exa: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("exa: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// Keep provider response details out of errors/logs; the body may contain
		// account-specific diagnostics or echoed credentials.
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Err: fmt.Errorf("exa response status %d", resp.StatusCode)}
	}

	var raw exaResponse
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return nil, fmt.Errorf("exa: decode response: %w", err)
	}
	results := make([]SearchResult, 0, len(raw.Results))
	for _, item := range raw.Results {
		parsedURL, err := url.Parse(strings.TrimSpace(item.URL))
		if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
			continue
		}
		snippet := ""
		if len(item.Highlights) > 0 {
			snippet = strings.TrimSpace(item.Highlights[0])
		}
		results = append(results, SearchResult{
			URL:     parsedURL.String(),
			Title:   strings.TrimSpace(item.Title),
			Snippet: snippet,
			PageAge: strings.TrimSpace(item.PublishedDate),
		})
	}
	return &SearchResponse{Results: results, Query: query}, nil
}

type exaRequest struct {
	Query      string      `json:"query"`
	Type       string      `json:"type"`
	NumResults int         `json:"numResults"`
	Contents   exaContents `json:"contents"`
}

type exaContents struct {
	Highlights exaHighlights `json:"highlights"`
}

type exaHighlights struct {
	MaxCharacters int `json:"maxCharacters"`
}

type exaResponse struct {
	Results []exaResult `json:"results"`
}

type exaResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Highlights    []string `json:"highlights"`
	PublishedDate string   `json:"publishedDate"`
}
