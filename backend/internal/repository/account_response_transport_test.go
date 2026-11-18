package repository

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
	"github.com/stretchr/testify/require"
)

type performanceResponseTransportFunc func(*http.Request) (*http.Response, error)

func (f performanceResponseTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestAccountPerformanceTransportRecordsOnlyRealHTTPResponses(t *testing.T) {
	original := upstreamstats.Default
	now := time.Now()
	upstreamstats.Default = upstreamstats.NewStore(10, now)
	t.Cleanup(func() { upstreamstats.Default = original })
	statuses := []int{200, 429, 503}
	for _, status := range statuses {
		client := &http.Client{Transport: performanceResponseTransportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		})}
		req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/responses", nil)
		require.NoError(t, err)
		resp, err := httpClientWithRequestLedger(client, 7).Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	for _, sample := range []struct{ method, path string }{{http.MethodGet, "/v1/models"}, {http.MethodPost, "/oauth/token"}} {
		client := &http.Client{Transport: performanceResponseTransportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("not inference"))}, nil
		})}
		req, err := http.NewRequest(sample.method, "https://example.test"+sample.path, nil)
		require.NoError(t, err)
		resp, err := httpClientWithRequestLedger(client, 7).Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	client := &http.Client{Transport: performanceResponseTransportFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("no HTTP response")
	})}
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/responses", nil)
	require.NoError(t, err)
	_, err = httpClientWithRequestLedger(client, 7).Do(req)
	require.Error(t, err)
	evidence := upstreamstats.Default.Snapshot([]int64{7, 8}, now, time.Now())
	require.Equal(t, int64(3), evidence[7].Total)
	require.Equal(t, int64(2), evidence[7].Failures)
	require.Equal(t, int64(1), evidence[7].RateLimited)
	require.Equal(t, int64(0), evidence[8].Total)
	require.Nil(t, evidence[8].Percent)
}
