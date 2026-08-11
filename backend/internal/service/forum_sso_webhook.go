package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Outbound webhooks to the forum.
//
// Endpoints (root-mounted on the forum, deliberately outside /api/v3 because
// NodeBB auto-enforces CSRF there and a server-to-server caller cannot satisfy
// it):
//
//	POST <forum>/sub2api/webhook                   balance/vip/role changes
//	POST <forum>/sub2api/forum-payment-callback     entitlement grant after payment
//
// Every request carries x-sub2api-timestamp, x-sub2api-nonce and
// x-sub2api-signature over the canonical form in forum_sso_sign.go. The forum
// rejects a timestamp more than 300s from its own clock and remembers nonces for
// 900s, so retries must mint a fresh nonce and timestamp rather than replay.
const (
	forumWebhookPath        = "/sub2api/webhook"
	forumPaymentCallbackURL = "/sub2api/forum-payment-callback"

	forumWebhookTimeout = 10 * time.Second
	// Two attempts total. The forum's nonce cache makes a replay a hard 401, so
	// each attempt re-signs; more than a couple of tries on a user-facing path
	// just adds latency.
	forumWebhookMaxAttempts = 2

	ForumEventBalanceChanged = "balance.changed"
	ForumEventVIPChanged     = "vip.changed"
	ForumEventRoleChanged    = "role.changed"
)

// forumWebhookHTTPClient is package-level so tests can substitute a transport.
var forumWebhookHTTPClient = &http.Client{Timeout: forumWebhookTimeout}

func forumNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// webhookConfigured reports whether outbound delivery is possible. Callers treat
// a false result as "skip silently": the forum is an optional satellite and a
// platform action must never fail because the forum is unconfigured.
func (s *ForumSSOService) webhookConfigured() bool {
	c := s.conf()
	return c.Enabled && c.WebhookSecret != "" && strings.TrimSpace(c.ForumBaseURL) != ""
}

// postSignedWebhook delivers one signed payload. Returns nil when the forum
// accepted it, or when delivery is not configured.
func (s *ForumSSOService) postSignedWebhook(ctx context.Context, path string, payload map[string]any) error {
	if !s.webhookConfigured() {
		return nil
	}
	c := s.conf()
	endpoint := strings.TrimRight(c.ForumBaseURL, "/") + path

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("forum webhook marshal: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= forumWebhookMaxAttempts; attempt++ {
		// Fresh timestamp and nonce per attempt: the forum's replay cache would
		// reject a byte-identical retry as replayed_nonce.
		timestamp := strconv.FormatInt(s.now().Unix(), 10)
		nonce, err := forumNonce()
		if err != nil {
			return fmt.Errorf("forum webhook nonce: %w", err)
		}
		signature := ForumSSOSign(payload, timestamp, nonce, c.WebhookSecret)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("forum webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-sub2api-timestamp", timestamp)
		req.Header.Set("x-sub2api-nonce", nonce)
		req.Header.Set("x-sub2api-signature", signature)

		resp, err := forumWebhookHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		status := resp.StatusCode
		_ = resp.Body.Close()

		if status >= 200 && status < 300 {
			return nil
		}
		// 4xx other than 429 is a contract problem (bad signature, unbound user);
		// retrying cannot fix it and would only double the noise.
		if status >= 400 && status < 500 && status != http.StatusTooManyRequests {
			return fmt.Errorf("forum webhook rejected: %s %d", path, status)
		}
		lastErr = fmt.Errorf("forum webhook status %d", status)
	}
	return lastErr
}

// deliverAsync fires a webhook without blocking the caller's request.
//
// A forum sync must never fail or slow a platform write: the balance change is
// already committed, and the forum reconciles on the user's next login through
// userinfo. Uses a detached context so it survives the request's cancellation.
func (s *ForumSSOService) deliverAsync(path string, payload map[string]any, label string) {
	if s == nil || !s.webhookConfigured() {
		return
	}
	go func() {
		// Budget for all attempts: each attempt gets its own HTTP timeout, plus
		// a small margin for network setup.  Without this, a first-attempt
		// timeout leaves only ~2 s for the second attempt, making the retry
		// effectively useless.
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(forumWebhookMaxAttempts)*(forumWebhookTimeout+2*time.Second))
		defer cancel()
		if err := s.postSignedWebhook(ctx, path, payload); err != nil {
			log.Printf("[forum-sso] %s webhook failed: %v", label, err)
		}
	}()
}

// NotifyBalanceChanged tells the forum a wallet balance moved.
// Money values are strings so the canonical signature matches the forum's
// stringification of the parsed JSON.
func (s *ForumSSOService) NotifyBalanceChanged(userID int64, newBalance, change float64, reason string) {
	s.deliverAsync(forumWebhookPath, map[string]any{
		"event": ForumEventBalanceChanged,
		"data": map[string]any{
			"user_id": userID,
			"balance": formatForumMoney(newBalance),
			"change":  formatForumMoney(change),
			"reason":  reason,
		},
	}, "balance.changed")
}

// NotifyVIPChanged tells the forum a user's VIP tier moved, which drives their
// NodeBB group membership and badge.
func (s *ForumSSOService) NotifyVIPChanged(userID int64, vip PlayVIPStatus, role string) {
	s.deliverAsync(forumWebhookPath, map[string]any{
		"event": ForumEventVIPChanged,
		"data": map[string]any{
			"user_id":            userID,
			"tier":               vip.Tier,
			"label":              vip.Label,
			"recharge_bonus_pct": vip.RechargeBonusPct,
			"role":               role,
		},
	}, "vip.changed")
}

// NotifyRoleChanged promotes or demotes the forum account alongside the platform.
func (s *ForumSSOService) NotifyRoleChanged(userID int64, newRole string) {
	s.deliverAsync(forumWebhookPath, map[string]any{
		"event": ForumEventRoleChanged,
		"data": map[string]any{
			"user_id":  userID,
			"new_role": newRole,
		},
	}, "role.changed")
}

// SendForumPaymentCallback grants the purchased entitlement on the forum.
//
// Synchronous and error-returning, unlike the sync events: the user has already
// been charged, so the caller needs to know whether the grant landed. The forum
// side is idempotent on order_id (incrObjectField returns >1 on replay), so a
// retry cannot double-grant.
func (s *ForumSSOService) SendForumPaymentCallback(ctx context.Context, userID int64, order *ForumOrderResult, itemID, itemType string) error {
	if !s.webhookConfigured() {
		return nil
	}
	return s.postSignedWebhook(ctx, forumPaymentCallbackURL, map[string]any{
		"order_id":  order.OrderID,
		"user_id":   userID,
		"item_id":   itemID,
		"item_type": itemType,
		"amount":    order.Amount,
	})
}
