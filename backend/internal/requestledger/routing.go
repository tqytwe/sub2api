package requestledger

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Coverage is deliberately narrower than ledger admission. It describes only
// OpenAI-compatible HTTP text routing, never provider-independent or WS evidence.
const RoutingCoverageVersion = "openai_compatible_http_text_v1"
const routingMaxEvents = 128
const routingMaxBytes = 32 * 1024
const routingWriteTimeout = 200 * time.Millisecond
const routingPendingWindow = 20 * time.Second

// RoutingEvent is a value-only allowlist. No account, request, error, map or
// caller-owned slice can escape into the asynchronous writer.
type RoutingEvent struct {
	SelectionNo int       `json:"selection_no"`
	Stage       string    `json:"stage"`
	Reason      string    `json:"reason"`
	AccountID   int64     `json:"account_id,omitempty"`
	Value       int       `json:"value"`
	At          time.Time `json:"at"`
}

type RoutingSummary struct {
	Status         string `json:"status"`
	SelectionCount int    `json:"selection_count"`
	EventCount     int    `json:"event_count"`
	VetoCount      int    `json:"veto_count"`
	OmittedCount   int    `json:"omitted_count"`
	Incomplete     bool   `json:"incomplete"`
	// Sources have already filtered accounts. Zero returned candidates cannot
	// establish that a group contains no accounts, nor why invisible ones failed.
	SourcePopulationComplete bool `json:"source_population_complete"`
}

type routingCapture struct {
	mu      sync.Mutex
	summary RoutingSummary
	events  []RoutingEvent
	bytes   int
	final   string
	ready   chan struct{}
}
type routingContextKey struct{}
type routingPosition struct {
	number int
	stage  string
	muted  bool
}

func routingEligible(route, method, kind string) bool {
	if kind != "http" || method != "POST" {
		return false
	}
	switch route {
	case "/v1/responses", "/v1/responses/*subpath", "/responses", "/responses/*subpath", "/backend-api/codex/responses", "/backend-api/codex/responses/*subpath", "/v1/messages", "/v1/chat/completions", "/chat/completions":
		return true
	}
	return false
}

// BeginRoutingSelection numbers scheduler selections independently of upstream
// attempts. The caller supplies only a coverage decision, never a platform name.
func BeginRoutingSelection(ctx context.Context, supported bool) (context.Context, func(int64, bool, bool)) {
	h := FromContext(ctx)
	if h == nil || !routingEligible(h.route, h.method, h.kind) {
		return ctx, func(int64, bool, bool) {}
	}
	h.routing.mu.Lock()
	if !supported || h.routing.summary.Status == "unsupported_coverage" {
		if h.routing.summary.SelectionCount == 0 {
			h.routing.summary.Status = "unsupported_coverage"
		} else {
			h.routing.summary.Incomplete = true
		}
		h.routing.mu.Unlock()
		return context.WithValue(ctx, routingContextKey{}, routingPosition{muted: true}), func(int64, bool, bool) {}
	}
	h.routing.summary.Status = "pending"
	h.routing.summary.SelectionCount++
	number := h.routing.summary.SelectionCount
	h.routing.mu.Unlock()
	ctx = context.WithValue(ctx, routingContextKey{}, routingPosition{number: number, stage: "initial_filter"})
	routingRecord(ctx, "selection", 0, "selection_started", 0, false)
	return ctx, func(account int64, wait, failed bool) {
		reason := "selection_selected"
		if failed || account <= 0 {
			reason = "selection_failed"
		} else if wait {
			reason = "selection_wait"
		}
		routingRecord(ctx, "selection", account, reason, 0, false)
	}
}

func RoutingStage(ctx context.Context, stage string) context.Context {
	position, _ := ctx.Value(routingContextKey{}).(routingPosition)
	position.stage = stage
	return context.WithValue(ctx, routingContextKey{}, position)
}

// WithoutRoutingEvidence suppresses a pre-existing diagnostic re-evaluation;
// only the original scheduling decision is evidence.
func WithoutRoutingEvidence(ctx context.Context) context.Context {
	position, _ := ctx.Value(routingContextKey{}).(routingPosition)
	position.muted = true
	return context.WithValue(ctx, routingContextKey{}, position)
}

func RoutingVeto(ctx context.Context, stage string, accountID int64, reason string) {
	routingRecord(ctx, stage, accountID, reason, 0, true)
}
func RoutingObservation(ctx context.Context, stage string, accountID int64, reason string) {
	routingRecord(ctx, stage, accountID, reason, 0, false)
}

func RoutingPool(ctx context.Context, count int, failed bool) {
	reason := "prefiltered_schedulable_pool"
	if failed {
		reason = "pool_read_failed"
	}
	routingRecord(ctx, "pool_source", 0, reason, count, false)
}
func routingRecord(ctx context.Context, stage string, account int64, reason string, value int, veto bool) {
	h := FromContext(ctx)
	if h == nil {
		return
	}
	position, _ := ctx.Value(routingContextKey{}).(routingPosition)
	if position.muted {
		return
	}
	h.routing.mu.Lock()
	defer h.routing.mu.Unlock()
	r := &h.routing
	if r.summary.SelectionCount == 0 || r.final != "" {
		return
	}
	if position.number == 0 {
		position.number = r.summary.SelectionCount
	}
	if stage == "" {
		stage = position.stage
		if stage == "" {
			stage = "admission"
		}
	}
	if !routingStageAllowed(stage) || !routingReasonAllowed(reason) || account < 0 || value < 0 {
		r.summary.Incomplete = true
		r.summary.OmittedCount++
		return
	}
	if veto {
		r.summary.VetoCount++
	}
	event := RoutingEvent{SelectionNo: position.number, Stage: stage, Reason: reason, AccountID: account, Value: value, At: time.Now().UTC()}
	encoded, _ := json.Marshal(event)
	if len(r.events) >= routingMaxEvents || r.bytes+len(encoded)+1+2 > routingMaxBytes {
		r.summary.OmittedCount++
		return
	}
	r.bytes += len(encoded) + 1
	r.events = append(r.events, event)
	r.summary.EventCount = len(r.events)
}

func routingStageAllowed(stage string) bool {
	switch stage {
	case "selection", "pool_source", "initial_filter", "sticky", "previous_response", "fresh_recheck", "db_recheck", "slot", "fallback", "admission", "scheduling_threshold":
		return true
	}
	return false
}
func routingReasonAllowed(reason string) bool {
	switch reason {
	case "selection_started", "selection_selected", "selection_wait", "selection_failed", "prefiltered_schedulable_pool", "pool_read_failed",
		"grok_team_model_rate_limit", "grok_model_quota_block", "grok_free_quota_soft_gate", "quota_auto_reset_pending_5h", "quota_auto_reset_pending_7d", "quota_auto_reset_credit_check_5h", "quota_auto_reset_credit_check_7d", "profit_threshold", "profit_invalid_account_rate",
		"account_nil", "account_missing", "account_read_failed", "account_model_not_owned", "platform_mismatch", "not_schedulable", "model_rate_limited", "quota_auto_pause", "quota_auto_pause_5h", "quota_auto_pause_7d", "quota_auto_pause_retry_after", "quota_auto_pause_requests", "quota_auto_pause_tokens", "model_not_supported", "model_not_allowed_in_group", "model_not_supported_in_group", "responses_tools_protocol_mismatch", "capability_mismatch", "compact_unsupported", "excluded", "runtime_blocked", "privacy_not_set", "transport_incompatible", "shadow_parent_unhealthy", "channel_upstream_restricted", "channel_restricted", "channel_pricing_restricted", "proxy_stream_quarantined", "group_mismatch", "sticky_unhealthy", "sticky_escape_ttft", "sticky_escape_error_rate", "known_full", "acquire_busy", "acquire_failed", "probe_budget_exhausted", "db_recheck_budget_exhausted", "wait_queue_full", "wait_failed", "wait_counter_failed", "wait_plan_missing", "selection_missing", "previous_response_http_incompatible", "scheduling_threshold", "load_read_failed":
		return true
	}
	return false
}

func (h *Handle) routingSnapshot() (RoutingSummary, []RoutingEvent) {
	h.routing.mu.Lock()
	defer h.routing.mu.Unlock()
	summary := h.routing.summary
	if summary.Status == "" {
		summary.Status = "not_recorded"
		if !routingEligible(h.route, h.method, h.kind) {
			summary.Status = "unsupported_coverage"
		}
	}
	return summary, append([]RoutingEvent(nil), h.routing.events...)
}

type routingBatch struct {
	ready <-chan struct{}
	id    string
	data  string
	count int
}

// freezeRouting runs inside Finish, with no SQL or queue wait. The request's
// existing final write durably declares the expected batch or its known loss.
func (h *Handle) freezeRouting() string {
	h.routing.mu.Lock()
	defer h.routing.mu.Unlock()
	r := &h.routing
	if r.final != "" {
		return r.final
	}
	if r.summary.Status == "" {
		r.summary.Status = "not_recorded"
		if !routingEligible(h.route, h.method, h.kind) {
			r.summary.Status = "unsupported_coverage"
		}
	}
	if r.summary.SelectionCount > 0 {
		encoded, _ := json.Marshal(r.events)
		r.ready = make(chan struct{})
		batch := routingBatch{id: h.ID, data: string(encoded), count: len(r.events), ready: r.ready}
		if !h.ledger.enqueueRouting(batch) {
			r.summary.Status = "incomplete"
			r.summary.Incomplete = true
		}
	}
	encoded, _ := json.Marshal(r.summary)
	r.final = string(encoded)
	r.events = nil
	return r.final
}

func (h *Handle) releaseRouting() {
	h.routing.mu.Lock()
	defer h.routing.mu.Unlock()
	if h.routing.ready != nil {
		close(h.routing.ready)
		h.routing.ready = nil
	}
}

func (l *Ledger) enqueueRouting(batch routingBatch) bool {
	if l == nil || l.db == nil {
		return false
	}
	select {
	case <-l.stop:
		return false
	default:
	}
	l.routingStart.Do(func() { go l.routingWriter() })
	select {
	case l.routingQueue <- batch:
		return true
	default:
		return false
	}
}
func (l *Ledger) routingWriter() {
	for {
		select {
		case <-l.stop:
			return
		case batch := <-l.routingQueue:
			l.persistRouting(batch)
		}
	}
}
func (l *Ledger) persistRouting(batch routingBatch) {
	// Evidence failures (including an unexpected writer panic) cannot affect
	// admission or the existing durable request/billing lifecycle.
	defer func() { _ = recover() }()
	if batch.ready != nil {
		timer := time.NewTimer(writeTimeout)
		defer timer.Stop()
		select {
		case <-batch.ready:
		case <-timer.C:
			return
		case <-l.stop:
			return
		}
	}
	stats := l.db.Stats()
	if stats.MaxOpenConnections > 0 && stats.InUse >= stats.MaxOpenConnections-1 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), routingWriteTimeout)
	defer cancel()
	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL lock_timeout='50ms'; SET LOCAL statement_timeout='150ms'`); err != nil {
		return
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO gateway_request_routing(request_id,events,event_count) VALUES($1,$2::jsonb,$3) ON CONFLICT(request_id) DO NOTHING`, batch.id, batch.data, batch.count); err != nil {
		return
	}
	_ = tx.Commit()
}

// RoutingUnsupported marks the actual dispatch into a provider outside this
// coverage version. Pre-auth rejections retain not_recorded instead.
func RoutingUnsupported(ctx context.Context) {
	h := FromContext(ctx)
	if h == nil {
		return
	}
	h.routing.mu.Lock()
	defer h.routing.mu.Unlock()
	if h.routing.final != "" {
		return
	}
	if h.routing.summary.SelectionCount > 0 {
		h.routing.summary.Incomplete = true
		return
	}
	h.routing.summary.Status = "unsupported_coverage"
}
