package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const imageStudioMaxDownloadBytes = 32 << 20 // 32MB

func FetchImageStudioRemoteURL(ctx context.Context, rawURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("fetch image failed: status %d", resp.StatusCode)
	}
	limited := io.LimitReader(resp.Body, imageStudioMaxDownloadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > imageStudioMaxDownloadBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = "image/png"
	}
	return data, ct, nil
}

func DecodeImageStudioDataURL(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "data:") {
		return nil, "", fmt.Errorf("invalid data url")
	}
	parts := strings.SplitN(raw, ",", 2)
	if len(parts) != 2 {
		return nil, "", fmt.Errorf("invalid data url")
	}
	meta := parts[0]
	payload := parts[1]
	ct := "image/png"
	if semi := strings.Index(meta, ";"); semi > 5 {
		ct = meta[5:semi]
	} else if len(meta) > 5 {
		ct = meta[5:]
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > imageStudioMaxDownloadBytes {
		return nil, "", fmt.Errorf("image too large")
	}
	return data, ct, nil
}

func NormalizeImageStudioPayloads(ctx context.Context, images []ImageStudioImagePayload) ([]ImageStudioImagePayload, error) {
	out := make([]ImageStudioImagePayload, 0, len(images))
	for _, img := range images {
		if len(img.Data) == 0 {
			continue
		}
		ct := img.ContentType
		if ct == "" {
			ct = "image/png"
		}
		out = append(out, ImageStudioImagePayload{Data: img.Data, ContentType: ct})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no image data")
	}
	return out, nil
}
