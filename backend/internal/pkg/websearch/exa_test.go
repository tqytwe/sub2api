package websearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExaProviderSearchMapsResultsAndKeepsCredentialsOutOfRequests(t *testing.T) {
	var gotAuth string
	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("x-api-key")
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"requestId":"exa-request-1","results":[{"title":"Example","url":"https://example.com","highlights":["A useful result"]},{"title":"Unsafe","url":"javascript:alert(1)"}]}`))
	}))
	defer server.Close()

	provider := newExaProviderWithEndpoint("secret-exa-key", server.URL, server.Client())
	result, err := provider.Search(context.Background(), SearchRequest{Query: "hello", MaxResults: 3})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "secret-exa-key" {
		t.Fatalf("x-api-key = %q, want configured key", gotAuth)
	}
	if len(result.Results) != 1 || result.Results[0].Snippet != "A useful result" {
		t.Fatalf("unexpected result mapping: %#v", result.Results)
	}
	if strings.Contains(result.Results[0].Snippet, "secret-exa-key") {
		t.Fatal("provider result must not contain the API key")
	}
}

func TestExaProviderReturnsTypedUpstreamStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream busy"}`))
	}))
	defer server.Close()

	provider := newExaProviderWithEndpoint("key", server.URL, server.Client())
	_, err := provider.Search(context.Background(), SearchRequest{Query: "hello"})
	if err == nil {
		t.Fatal("expected upstream error")
	}
	statusErr, ok := err.(*UpstreamStatusError)
	if !ok {
		t.Fatalf("error type = %T, want *UpstreamStatusError", err)
	}
	if statusErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", statusErr.StatusCode, http.StatusBadGateway)
	}
}
