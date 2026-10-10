package service

import (
	"github.com/Wei-Shaw/sub2api/internal/requestledger"
	"net/http"
)

// Clone the client, preserving redirects/timeouts without mutating pooled clients.
func batchImageLedgerHTTP(client *http.Client, req *http.Request) (*http.Response, error) {
	if requestledger.FromContext(req.Context()) == nil {
		return client.Do(req)
	}
	clone := *client
	base := clone.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	clone.Transport = requestledger.Transport(base, 0)
	return clone.Do(req)
}
