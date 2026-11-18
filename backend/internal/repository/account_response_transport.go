package repository

import (
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/upstreamstats"
)

type accountResponseTransport struct {
	base      http.RoundTripper
	accountID int64
}

func (t accountResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err == nil && resp != nil && req.Context().Err() == nil && isAccountInferenceRequest(req) {
		upstreamstats.Default.Record(t.accountID, resp.StatusCode, time.Now())
	}
	return resp, err
}

func isAccountInferenceRequest(req *http.Request) bool {
	if req.Method != http.MethodPost || req.URL == nil {
		return false
	}
	path := strings.TrimRight(req.URL.Path, "/")
	for _, suffix := range []string{"/messages", "/responses", "/responses/compact", "/chat/completions", "/completions", ":generateContent", ":streamGenerateContent"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}
