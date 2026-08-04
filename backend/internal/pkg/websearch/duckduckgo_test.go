package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDuckDuckGoProviderSearchMapsInstantAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "golang" || r.URL.Query().Get("format") != "json" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Heading":"Go","AbstractText":"Go language","AbstractURL":"https://go.dev","RelatedTopics":[{"Text":"Packages","FirstURL":"https://pkg.go.dev"}]}`))
	}))
	defer server.Close()

	provider := newDuckDuckGoProviderWithEndpoint(server.URL, server.Client())
	result, err := provider.Search(context.Background(), SearchRequest{Query: "golang", MaxResults: 2})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if provider.Name() != duckDuckGoProvider || len(result.Results) != 2 {
		t.Fatalf("unexpected provider/results: %s %#v", provider.Name(), result.Results)
	}
	if result.Results[0].URL != "https://go.dev" || result.Results[1].URL != "https://pkg.go.dev" {
		t.Fatalf("unexpected URLs: %#v", result.Results)
	}
}

func TestDuckDuckGoProviderSearchPreservesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := newDuckDuckGoProviderWithEndpoint(server.URL, server.Client()).Search(context.Background(), SearchRequest{Query: "test"})
	statusErr, ok := err.(*UpstreamStatusError)
	if !ok || statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("error = %#v, want upstream status 502", err)
	}
}
