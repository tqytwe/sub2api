package websearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	duckDuckGoEndpoint = "https://api.duckduckgo.com/"
	duckDuckGoProvider = "duckduckgo"
)

// DuckDuckGoProvider uses the public Instant Answer endpoint. It is intended
// as a keyless, server-side fallback for mobile search; the client never calls
// DuckDuckGo directly, so rate limiting, privacy and request correlation stay
// under the platform's control.
type DuckDuckGoProvider struct {
	httpClient *http.Client
	endpoint   string
}

func NewDuckDuckGoProvider(httpClient *http.Client) *DuckDuckGoProvider {
	return newDuckDuckGoProviderWithEndpoint(duckDuckGoEndpoint, httpClient)
}

func newDuckDuckGoProviderWithEndpoint(endpoint string, httpClient *http.Client) *DuckDuckGoProvider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &DuckDuckGoProvider{endpoint: strings.TrimSpace(endpoint), httpClient: httpClient}
}

func (d *DuckDuckGoProvider) Name() string { return duckDuckGoProvider }

func (d *DuckDuckGoProvider) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, fmt.Errorf("duckduckgo: empty search query")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	u, err := url.Parse(d.endpoint)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: build request: %w", err)
	}
	values := u.Query()
	values.Set("q", query)
	values.Set("format", "json")
	values.Set("no_html", "1")
	values.Set("no_redirect", "1")
	values.Set("skip_disambig", "1")
	u.RawQuery = values.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: build request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &UpstreamStatusError{StatusCode: resp.StatusCode, Err: fmt.Errorf("duckduckgo response status %d", resp.StatusCode)}
	}
	var raw duckDuckGoResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("duckduckgo: decode response: %w", err)
	}
	results := make([]SearchResult, 0, maxResults)
	appendResult := func(item duckDuckGoTopic) {
		if len(results) >= maxResults {
			return
		}
		link := strings.TrimSpace(item.FirstURL)
		parsed, parseErr := url.Parse(link)
		if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return
		}
		title := strings.TrimSpace(item.Text)
		if title == "" {
			title = strings.TrimSpace(raw.Heading)
		}
		if title == "" {
			return
		}
		results = append(results, SearchResult{URL: parsed.String(), Title: title, Snippet: strings.TrimSpace(item.Result), PageAge: ""})
	}
	if raw.AbstractURL != "" {
		appendResult(duckDuckGoTopic{FirstURL: raw.AbstractURL, Text: raw.Heading, Result: raw.AbstractText})
	}
	var walk func([]duckDuckGoTopic)
	walk = func(topics []duckDuckGoTopic) {
		for _, topic := range topics {
			appendResult(topic)
			walk(topic.Topics)
			if len(results) >= maxResults {
				return
			}
		}
	}
	walk(raw.RelatedTopics)
	return &SearchResponse{Results: results, Query: query}, nil
}

type duckDuckGoResponse struct {
	Heading       string            `json:"Heading"`
	AbstractText  string            `json:"AbstractText"`
	AbstractURL   string            `json:"AbstractURL"`
	RelatedTopics []duckDuckGoTopic `json:"RelatedTopics"`
}

type duckDuckGoTopic struct {
	FirstURL string            `json:"FirstURL"`
	Text     string            `json:"Text"`
	Result   string            `json:"Result"`
	Topics   []duckDuckGoTopic `json:"Topics"`
}
