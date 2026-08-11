package service

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Payment callback retry queue.
//
// When SendForumPaymentCallback fails the handler returns 200 + grant_pending:true
// so the user sees success (the wallet debit is final). This worker retries the
// callback in the background until the forum accepts it, using an exponential
// back-off capped at 1 hour. Jobs are stored as a Redis sorted set so they
// survive service restarts.
//
// Key: forum_sso:payment_retry
// Score: unix timestamp of next attempt (allows ZRANGEBYSCORE now)
// Member: JSON-encoded forumPaymentRetryJob

const (
	forumPaymentRetryKey     = "forum_sso:payment_retry"
	forumPaymentRetryMaxAge  = 48 * time.Hour // give up after 48 h
	forumPaymentRetryInitial = 30 * time.Second
	forumPaymentRetryMax     = 1 * time.Hour
	forumPaymentRetryPoll    = 30 * time.Second
)

type forumPaymentRetryJob struct {
	UserID    int64  `json:"user_id"`
	OrderID   string `json:"order_id"`
	ItemID    string `json:"item_id"`
	ItemType  string `json:"item_type"`
	Amount    string `json:"amount"`
	Attempt   int    `json:"attempt"`
	EnqueueAt int64  `json:"enqueue_at"` // unix, for max-age check
}

// EnqueuePaymentCallbackRetry is the exported entry point called by the HTTP
// handler when SendForumPaymentCallback fails. It persists a retry job to Redis.
func (s *ForumSSOService) EnqueuePaymentCallbackRetry(order *ForumOrderResult, userID int64, itemID, itemType string) {
	s.enqueuePaymentCallbackRetry(order, userID, itemID, itemType)
}

// enqueuePaymentCallbackRetry persists a retry job to Redis.
// Called when SendForumPaymentCallback fails in the HTTP handler.
func (s *ForumSSOService) enqueuePaymentCallbackRetry(order *ForumOrderResult, userID int64, itemID, itemType string) {
	if s == nil || s.redis == nil {
		return
	}
	job := forumPaymentRetryJob{
		UserID:    userID,
		OrderID:   order.OrderID,
		ItemID:    itemID,
		ItemType:  itemType,
		Amount:    order.Amount,
		Attempt:   1,
		EnqueueAt: s.now().Unix(),
	}
	raw, err := json.Marshal(job)
	if err != nil {
		log.Printf("[forum-sso] failed to marshal payment retry job: %v", err)
		return
	}
	nextRun := s.now().Add(forumPaymentRetryInitial)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.redis.ZAdd(ctx, forumPaymentRetryKey, redis.Z{
		Score:  float64(nextRun.Unix()),
		Member: string(raw),
	}).Err(); err != nil {
		log.Printf("[forum-sso] failed to enqueue payment retry job order_id=%s: %v", job.OrderID, err)
	}
}

// retryBackoff returns the delay before the next attempt, capped at max.
func retryBackoff(attempt int) time.Duration {
	d := forumPaymentRetryInitial
	for i := 1; i < attempt; i++ {
		d *= 2
		if d > forumPaymentRetryMax {
			return forumPaymentRetryMax
		}
	}
	return d
}

// ForumPaymentRetryWorker polls the retry queue and re-delivers failed payment
// callbacks. Wire it like MobilePushWorker: construct, Start(), register Stop()
// in provideCleanup.
type ForumPaymentRetryWorker struct {
	svc  *ForumSSOService
	mu   sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewForumPaymentRetryWorker creates a worker backed by svc. Returns nil if svc
// is nil so callers can guard with a nil check.
func NewForumPaymentRetryWorker(svc *ForumSSOService) *ForumPaymentRetryWorker {
	if svc == nil {
		return nil
	}
	return &ForumPaymentRetryWorker{svc: svc}
}

func (w *ForumPaymentRetryWorker) Start() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		w.run(ctx)
	}()
}

func (w *ForumPaymentRetryWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	cancel, done := w.cancel, w.done
	w.cancel = nil
	w.done = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

func (w *ForumPaymentRetryWorker) run(ctx context.Context) {
	_, _ = w.runOnce(ctx)
	ticker := time.NewTicker(forumPaymentRetryPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.runOnce(ctx)
		}
	}
}

// runOnce processes all jobs whose next-run time has arrived.
func (w *ForumPaymentRetryWorker) runOnce(ctx context.Context) (int, error) {
	if w == nil || w.svc == nil || w.svc.redis == nil {
		return 0, nil
	}
	now := w.svc.now()
	// Fetch all jobs ready to run (score <= now).
	members, err := w.svc.redis.ZRangeByScore(ctx, forumPaymentRetryKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: float64ToString(float64(now.Unix())),
	}).Result()
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, raw := range members {
		if ctx.Err() != nil {
			break
		}
		w.processOne(ctx, raw, now)
		processed++
	}
	return processed, nil
}

func (w *ForumPaymentRetryWorker) processOne(ctx context.Context, raw string, now time.Time) {
	var job forumPaymentRetryJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		log.Printf("[forum-sso] payment retry: malformed job, dropping: %v", err)
		w.svc.redis.ZRem(ctx, forumPaymentRetryKey, raw)
		return
	}

	// Drop jobs that have been retrying longer than forumPaymentRetryMaxAge.
	if now.Unix()-job.EnqueueAt > int64(forumPaymentRetryMaxAge.Seconds()) {
		log.Printf("[forum-sso] payment retry: giving up on order_id=%s after %s", job.OrderID, forumPaymentRetryMaxAge)
		w.svc.redis.ZRem(ctx, forumPaymentRetryKey, raw)
		return
	}

	// Remove from queue before attempting, so a crash doesn't replay immediately.
	w.svc.redis.ZRem(ctx, forumPaymentRetryKey, raw)

	order := &ForumOrderResult{
		OrderID: job.OrderID,
		Amount:  job.Amount,
	}
	callCtx, cancel := context.WithTimeout(ctx, forumWebhookTimeout+2*time.Second)
	defer cancel()
	err := w.svc.SendForumPaymentCallback(callCtx, job.UserID, order, job.ItemID, job.ItemType)
	if err == nil {
		log.Printf("[forum-sso] payment retry: delivered order_id=%s attempt=%d", job.OrderID, job.Attempt)
		return
	}

	log.Printf("[forum-sso] payment retry: attempt %d failed for order_id=%s: %v", job.Attempt, job.OrderID, err)

	// Re-enqueue with exponential backoff.
	job.Attempt++
	nextRaw, marshalErr := json.Marshal(job)
	if marshalErr != nil {
		return
	}
	nextRun := w.svc.now().Add(retryBackoff(job.Attempt))
	w.svc.redis.ZAdd(ctx, forumPaymentRetryKey, redis.Z{
		Score:  float64(nextRun.Unix()),
		Member: string(nextRaw),
	})
}

// float64ToString converts a float64 to its string representation for
// ZRANGEBYSCORE. Uses strconv to avoid a fmt import.
func float64ToString(f float64) string {
	return strconv.FormatFloat(f, 'f', 0, 64)
}
