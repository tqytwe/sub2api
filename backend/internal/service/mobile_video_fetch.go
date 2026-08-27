package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const mobileVideoMaxDownloadBytes int64 = 512 << 20 // 512 MiB

// FetchMobileVideoRemoteURL copies a provider-owned, short-lived artifact into
// server storage. It reuses the image-studio HTTPS, DNS and redirect policy so
// an upstream response cannot turn the worker into an SSRF proxy.
func FetchMobileVideoRemoteURL(ctx context.Context, rawURL string) ([]byte, string, error) {
	validatedURL, err := validateImageStudioRemoteURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validatedURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := newImageStudioRemoteHTTPClient()
	client.Timeout = 5 * time.Minute
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("fetch video failed: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, mobileVideoMaxDownloadBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > mobileVideoMaxDownloadBytes {
		return nil, "", fmt.Errorf("video exceeds %d bytes", mobileVideoMaxDownloadBytes)
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(strings.ToLower(contentType), "video/") {
		contentType = "video/mp4"
	}
	return data, contentType, nil
}
