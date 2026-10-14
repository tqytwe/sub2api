//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type transportTempUnschedRepoStub struct {
	AccountRepository
	calls      int
	lastID     int64
	lastUntil  time.Time
	lastReason string
}

func (r *transportTempUnschedRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.calls++
	r.lastID = id
	r.lastUntil = until
	r.lastReason = reason
	return nil
}

func newTransportErrorTestGin(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c
}

// TestHandleUpstreamTransportError_TransientFailsOverWithoutEviction pins the
// contract for transient transport blips (EOF / connection reset): the request
// fails over to another account, the current account stays schedulable, and
// nothing is written to the response (the handler owns it).
func TestHandleUpstreamTransportError_TransientFailsOverWithoutEviction(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic}

	err := s.handleUpstreamTransportError(context.Background(), c, account,
		errors.New(`Post "http://upstream/v1/messages?beta=true": EOF`), OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("expected *UpstreamFailoverError, got %T: %v", err, err)
	}
	if failoverErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode = %d, want 502", failoverErr.StatusCode)
	}
	if string(failoverErr.ResponseBody) != string(gatewayTransportFailoverBody) {
		t.Fatalf("ResponseBody = %s, want legacy 502 body", failoverErr.ResponseBody)
	}
	if !failoverErr.ShouldRetryNextAccount() {
		t.Fatal("transient transport error must allow retrying the next account")
	}
	if repo.calls != 0 {
		t.Fatalf("SetTempUnschedulable called %d times for a transient error, want 0", repo.calls)
	}
	if c.Writer.Written() {
		t.Fatal("handler owns the response; service must not write on transport failover")
	}
}

// TestHandleUpstreamTransportError_AttributesEventTimeProxy pins that the
// shared helper stamps proxy attribution from the same account snapshot the
// transport used, so every Anthropic/Bedrock caller inherits it.
func TestHandleUpstreamTransportError_AttributesEventTimeProxy(t *testing.T) {
	proxyID := int64(10060)
	tests := []struct {
		name     string
		account  *Account
		wantID   *int64
		wantName string
	}{
		{
			name:     "managed proxy",
			account:  &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic, ProxyID: &proxyID, Proxy: &Proxy{ID: proxyID, Name: "wldsg82-ipv6-10060"}},
			wantID:   &proxyID,
			wantName: "wldsg82-ipv6-10060",
		},
		{
			name:     "direct",
			account:  &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic},
			wantName: opsProxyNameDirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &GatewayService{accountRepo: &transportTempUnschedRepoStub{}}
			c := newTransportErrorTestGin(t)
			_ = s.handleUpstreamTransportError(context.Background(), c, tt.account,
				errors.New("EOF"), OpsUpstreamErrorEvent{UpstreamURL: "https://api.anthropic.com/v1/messages", Passthrough: true})
			raw, ok := c.Get(OpsUpstreamErrorsKey)
			if !ok {
				t.Fatal("no upstream events recorded")
			}
			events := raw.([]*OpsUpstreamErrorEvent)
			if len(events) != 1 {
				t.Fatalf("events = %d, want 1", len(events))
			}
			ev := events[0]
			if tt.wantID == nil {
				if ev.ProxyID != nil {
					t.Fatalf("proxy_id = %d, want null", *ev.ProxyID)
				}
			} else if ev.ProxyID == nil || *ev.ProxyID != *tt.wantID {
				t.Fatalf("proxy_id = %v, want %d", ev.ProxyID, *tt.wantID)
			}
			if ev.ProxyName != tt.wantName {
				t.Fatalf("proxy_name = %q, want %q", ev.ProxyName, tt.wantName)
			}
			if !ev.Passthrough || ev.UpstreamURL == "" || ev.Kind != "request_error" {
				t.Fatalf("caller-supplied fields lost: %+v", ev)
			}
		})
	}
}

// TestHandleUpstreamTransportError_PersistentEvictsAccount pins the contract
// for durable faults (dead endpoint / DNS / proxy credentials): fail over AND
// temporarily unschedule the account for the transport cooldown.
func TestHandleUpstreamTransportError_PersistentEvictsAccount(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic}

	before := time.Now()
	err := s.handleUpstreamTransportError(context.Background(), c, account,
		errors.New(`dial tcp 1.2.3.4:443: connect: connection refused`), OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("expected *UpstreamFailoverError, got %T: %v", err, err)
	}
	if repo.calls != 1 {
		t.Fatalf("SetTempUnschedulable called %d times for a persistent error, want 1", repo.calls)
	}
	if repo.lastID != account.ID {
		t.Fatalf("unscheduled account = %d, want %d", repo.lastID, account.ID)
	}
	wantUntil := before.Add(gatewayTransportErrorTempUnschedDuration)
	if repo.lastUntil.Before(wantUntil.Add(-time.Minute)) || repo.lastUntil.After(wantUntil.Add(time.Minute)) {
		t.Fatalf("until = %v, want ~%v", repo.lastUntil, wantUntil)
	}
	if !strings.HasPrefix(repo.lastReason, "upstream transport error (proxy/network): ") {
		t.Fatalf("reason = %q, want transport-error prefix", repo.lastReason)
	}
}

// TestHandleUpstreamTransportError_ClientCanceledNoFailover pins that a
// canceled client neither fails over nor evicts: the upstream never had a
// chance to exhibit a fault.
func TestHandleUpstreamTransportError_ClientCanceledNoFailover(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic}

	inErr := context.Canceled
	err := s.handleUpstreamTransportError(context.Background(), c, account, inErr, OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if errors.As(err, &failoverErr) {
		t.Fatal("canceled client must not fail over to another account")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled passthrough", err)
	}
	if repo.calls != 0 {
		t.Fatalf("SetTempUnschedulable called %d times on client cancel, want 0", repo.calls)
	}
}

// TestHandleUpstreamTransportError_UpstreamDeadlineStillFailsOver pins that an
// upstream-side timeout (request context still alive) is treated as a
// transient fault: fail over, no eviction.
func TestHandleUpstreamTransportError_UpstreamDeadlineStillFailsOver(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{ID: 149, Name: "acc", Platform: PlatformAnthropic}

	err := s.handleUpstreamTransportError(context.Background(), c, account,
		context.DeadlineExceeded, OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("upstream deadline with live request context must fail over, got %T: %v", err, err)
	}
	if repo.calls != 0 {
		t.Fatalf("SetTempUnschedulable called %d times for upstream deadline, want 0", repo.calls)
	}
}

// --- P3-2 transport-retry (v2.10.5 port, adapted to fork pool helpers) ---
// Upstream contract: a direct pool-mode account that opted in by listing 503
// in pool_mode_retry_status_codes treats a transport failure like an upstream
// 503 — same-account retry with request-scoped backoff, no temp-unschedule.
// Accounts behind a proxy and accounts that did not opt in keep the existing
// eviction behavior.

// TestHandleUpstreamTransportError_PoolMode503OptInSkipsEviction pins the
// opt-in contract: pool mode + no proxy + 503 listed => 503 failover marked
// retryable-on-same-account and request-scoped-transient, zero eviction calls.
func TestHandleUpstreamTransportError_PoolMode503OptInSkipsEviction(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{
		ID:       149,
		Name:     "acc",
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(401), float64(403), float64(429), float64(503)},
		},
	}

	err := s.handleUpstreamTransportError(context.Background(), c, account,
		errors.New(`dial tcp 1.2.3.4:443: connect: connection refused`), OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("expected *UpstreamFailoverError, got %T: %v", err, err)
	}
	if failoverErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503 for opted-in pool mode", failoverErr.StatusCode)
	}
	if !failoverErr.RetryableOnSameAccount {
		t.Fatal("opted-in pool transport error must be retryable on the same account")
	}
	if !failoverErr.RequestScopedTransient {
		t.Fatal("opted-in pool transport error must not trigger account eviction downstream")
	}
	if string(failoverErr.ResponseBody) != string(gatewayTransportFailoverBody) {
		t.Fatalf("ResponseBody = %s, want legacy failover body", failoverErr.ResponseBody)
	}
	if repo.calls != 0 {
		t.Fatalf("SetTempUnschedulable called %d times for opted-in pool mode, want 0", repo.calls)
	}
}

// TestHandleUpstreamTransportError_PoolModeWithoutOptInKeepsEviction pins that
// the default retry list (no 503) keeps the durable-fault eviction behavior.
func TestHandleUpstreamTransportError_PoolModeWithoutOptInKeepsEviction(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	account := &Account{
		ID:       149,
		Name:     "acc",
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode": true,
		},
	}

	err := s.handleUpstreamTransportError(context.Background(), c, account,
		errors.New(`dial tcp 1.2.3.4:443: connect: connection refused`), OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("expected *UpstreamFailoverError, got %T: %v", err, err)
	}
	if failoverErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode = %d, want legacy 502 without opt-in", failoverErr.StatusCode)
	}
	if failoverErr.RetryableOnSameAccount {
		t.Fatal("non-opted-in pool transport error must not be retryable on the same account")
	}
	if repo.calls != 1 {
		t.Fatalf("SetTempUnschedulable called %d times for non-opted-in pool mode, want 1", repo.calls)
	}
}

// TestHandleUpstreamTransportError_PoolMode503OptInBehindProxyKeepsEviction
// pins that a refused dial behind a proxy is still attributed to the proxy —
// a durable fault the temp-unschedule exists for — even with 503 opted in.
func TestHandleUpstreamTransportError_PoolMode503OptInBehindProxyKeepsEviction(t *testing.T) {
	repo := &transportTempUnschedRepoStub{}
	s := &GatewayService{accountRepo: repo}
	c := newTransportErrorTestGin(t)
	proxyID := int64(10060)
	account := &Account{
		ID:       149,
		Name:     "acc",
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		ProxyID:  &proxyID,
		Proxy:    &Proxy{ID: proxyID, Name: "wldsg82-ipv6-10060"},
		Credentials: map[string]any{
			"pool_mode":                    true,
			"pool_mode_retry_status_codes": []any{float64(401), float64(403), float64(429), float64(503)},
		},
	}

	err := s.handleUpstreamTransportError(context.Background(), c, account,
		errors.New(`dial tcp 1.2.3.4:443: connect: connection refused`), OpsUpstreamErrorEvent{})

	var failoverErr *UpstreamFailoverError
	if !errors.As(err, &failoverErr) {
		t.Fatalf("expected *UpstreamFailoverError, got %T: %v", err, err)
	}
	if failoverErr.RetryableOnSameAccount {
		t.Fatal("proxied pool transport error must not be retryable on the same account")
	}
	if repo.calls != 1 {
		t.Fatalf("SetTempUnschedulable called %d times for proxied pool account, want 1", repo.calls)
	}
}
