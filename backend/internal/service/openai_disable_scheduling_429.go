package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	OpenAIDisableSchedulingOn429ExtraKey                  = "openai_disable_scheduling_on_429"
	OpenAIDisableSchedulingOn4297dThresholdExtraKey       = "openai_disable_scheduling_on_429_7d_threshold_percent"
	DefaultOpenAIDisableSchedulingOn4297dThresholdPercent = 98.0
	OpenAI429WeeklyEvidenceKey                            = "codex_7d_permanent_disable_evidence"
	openAI429LegacyWeeklySupersededKey                    = "codex_7d_permanent_disable_legacy_superseded"
)

func openAI429WeeklyThreshold(account *Account) (float64, bool) {
	if _, exists := account.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey]; !exists {
		return DefaultOpenAIDisableSchedulingOn4297dThresholdPercent, true
	}
	threshold, ok := resolveAccountExtraNumber(account.Extra, OpenAIDisableSchedulingOn4297dThresholdExtraKey)
	return threshold, ok && validOpenAI429Percent(threshold)
}

func validOpenAI429Percent(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100
}

// Read this response directly: persistence happens later and may be throttled.
// A present but invalid/low weekly observation must never fall back to an older
// high snapshot. The five-hour window does not participate in this policy.
func openAI429WeeklyHeaderEvidence(headers http.Header) (used float64, reset time.Duration, present, valid bool) {
	// A permanent pause must not rely on Normalize's legacy primary=7d
	// assumption. Require exactly one explicitly identified weekly window.
	prefix := ""
	for _, window := range []string{"primary", "secondary"} {
		values := headers.Values("x-codex-" + window + "-window-minutes")
		claimsWeekly := false
		for _, value := range values {
			for _, part := range strings.Split(value, ",") {
				if minutes, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && minutes == 7*24*60 {
					claimsWeekly = true
				}
			}
		}
		if !claimsWeekly {
			continue
		}
		if prefix != "" || len(values) != 1 {
			return 0, 0, true, false
		}
		if minutes, err := strconv.Atoi(values[0]); err != nil || minutes != 7*24*60 {
			return 0, 0, true, false
		}
		prefix = "x-codex-" + window + "-"
	}
	if prefix == "" {
		return 0, 0, false, false
	}
	values := headers.Values(prefix + "used-percent")
	if len(values) != 1 {
		return 0, 0, true, false
	}
	weekly, err := strconv.ParseFloat(values[0], 64)
	if err != nil || !validOpenAI429Percent(weekly) {
		return 0, 0, true, false
	}
	if values, exists := headers[http.CanonicalHeaderKey(prefix+"reset-after-seconds")]; exists {
		if len(values) != 1 {
			return 0, 0, true, false
		}
		seconds, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil || seconds <= 0 || seconds > int64((7*24*time.Hour)/time.Second) {
			return 0, 0, true, false
		}
		reset = time.Duration(seconds) * time.Second
	}
	return weekly, reset, true, true
}

func openAI429SnapshotTime(extra map[string]any) time.Time {
	observed, _ := parseTime(fmt.Sprint(extra["codex_usage_updated_at"]))
	return observed
}

func openAI429FreshObservation(observed, now time.Time) bool {
	return !observed.IsZero() && !observed.After(now) && now.Sub(observed) < openAICodexAutoPauseStaleAfter
}

func openAI429Evidence(used float64, observed, reset time.Time, valid bool) map[string]any {
	evidence := map[string]any{
		"observed_at":    observed.UTC().Format(time.RFC3339Nano),
		"observed_at_ns": strconv.FormatInt(observed.UnixNano(), 10),
	}
	if valid {
		evidence["used_percent"] = used
		evidence["window_minutes"] = 7 * 24 * 60
		if !reset.IsZero() {
			evidence["reset_at"] = reset.UTC().Format(time.RFC3339Nano)
		}
	}
	return evidence
}

func openAI429WeeklyUsage(account *Account, headers http.Header, now time.Time, receipts ...time.Time) (float64, bool) {
	if used, reset, present, valid := openAI429WeeklyHeaderEvidence(headers); present {
		observed := now
		if len(receipts) > 0 {
			observed = receipts[0]
		}
		return used, valid && openAI429FreshObservation(observed, now) && (reset == 0 || observed.Add(reset).After(now))
	}
	if account == nil || !openAICodexSnapshotIdentityTrusted(account) {
		return 0, false
	}
	extra := account.Extra
	if raw, exists := extra[OpenAI429WeeklyEvidenceKey]; exists {
		evidence, ok := raw.(map[string]any)
		if !ok {
			return 0, false
		}
		observed, _ := parseTime(fmt.Sprint(evidence["observed_at"]))
		version, err := strconv.ParseInt(fmt.Sprint(evidence["observed_at_ns"]), 10, 64)
		minutes, known := resolveAccountExtraNumber(evidence, "window_minutes")
		used, valid := resolveAccountExtraNumber(evidence, "used_percent")
		if err != nil || version <= 0 || !observed.Equal(time.Unix(0, version)) ||
			!openAI429FreshObservation(observed, now) || !known || minutes != 7*24*60 || !valid || !validOpenAI429Percent(used) {
			return 0, false
		}
		if rawReset, exists := evidence["reset_at"]; exists {
			reset, err := parseTime(fmt.Sprint(rawReset))
			if err != nil || !reset.After(now) {
				return 0, false
			}
		}
		return used, true
	}
	if _, superseded := extra[openAI429LegacyWeeklySupersededKey]; superseded {
		return 0, false
	}
	minutes, ok := resolveAccountExtraNumber(extra, "codex_7d_window_minutes")
	if !ok || minutes != 7*24*60 {
		return 0, false
	}
	used, ok := resolveAccountExtraNumber(extra, "codex_7d_used_percent")
	if !ok || !validOpenAI429Percent(used) {
		return 0, false
	}
	observed := openAI429SnapshotTime(extra)
	if versions, exists := extra[CodexUsageFieldVersionsKey]; exists {
		fields, ok := versions.(map[string]any)
		if !ok {
			return 0, false
		}
		stamp, err := strconv.ParseInt(fmt.Sprint(fields["codex_7d_used_percent"]), 10, 64)
		windowStamp, windowErr := strconv.ParseInt(fmt.Sprint(fields["codex_7d_window_minutes"]), 10, 64)
		if err != nil || windowErr != nil || stamp <= 0 || stamp != windowStamp {
			return 0, false
		}
		observed = time.Unix(0, stamp)
	}
	// Permanent pauses require a known, recent observation, unlike temporary
	// scheduling guards that may retain legacy snapshots without timestamps.
	if !openAI429FreshObservation(observed, now) {
		return 0, false
	}
	if raw, exists := extra["codex_7d_reset_at"]; exists {
		reset, err := parseTime(fmt.Sprint(raw))
		if err != nil || !reset.After(now) {
			return 0, false
		}
	} else if raw, exists := extra["codex_7d_reset_after_seconds"]; exists {
		seconds, err := strconv.ParseInt(fmt.Sprint(raw), 10, 64)
		if err != nil || seconds <= 0 || seconds > int64((7*24*time.Hour)/time.Second) {
			return 0, false
		}
		resetObserved := openAI429SnapshotTime(extra)
		if fields, ok := extra[CodexUsageFieldVersionsKey].(map[string]any); ok {
			stamp, err := strconv.ParseInt(fmt.Sprint(fields["codex_7d_reset_after_seconds"]), 10, 64)
			if err != nil || stamp <= 0 {
				return 0, false
			}
			resetObserved = time.Unix(0, stamp)
		}
		if resetObserved.IsZero() || resetObserved.After(now) || !resetObserved.Add(time.Duration(seconds)*time.Second).After(now) {
			return 0, false
		}
	}
	return used, true
}

func openAIDisableSchedulingOn429Enabled(account *Account) bool {
	// Evaluate the persisted owner, not a managed connection's projected type.
	if account == nil || account.ID <= 0 || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth ||
		account.IsShadow() {
		return false
	}
	enabled, _ := account.Extra[OpenAIDisableSchedulingOn429ExtraKey].(bool)
	return enabled
}

type openAIDisableScheduling429ObservedKey struct{}
type openAI429ObservedAtKey struct{}
type openAI429HeadersKey struct{}

func bindOpenAI429ResponseObservation(ctx context.Context, c *gin.Context, resp *http.Response) context.Context {
	var observed time.Time
	if resp != nil && resp.Request != nil {
		observed, _ = resp.Request.Context().Value(openAI429ObservedAtKey{}).(time.Time)
	}
	if observed.IsZero() {
		return ctx
	}
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), openAI429ObservedAtKey{}, observed))
	}
	return context.WithValue(ctx, openAI429ObservedAtKey{}, observed)
}

func openAI429LeaseObservation(ctx context.Context, lease *openAIWSConnLease) context.Context {
	var observed time.Time
	if lease != nil && lease.conn != nil {
		observed = lease.conn.createdAt()
	}
	return context.WithValue(ctx, openAI429ObservedAtKey{}, observed)
}

func (s *RateLimitService) persistOpenAI429WeeklyEvidence(ctx context.Context, account *Account, headers http.Header) {
	if s == nil || s.accountRepo == nil || account == nil || account.Platform != PlatformOpenAI || account.IsShadow() {
		return
	}
	if original, ok := ctx.Value(openAI429HeadersKey{}).(http.Header); ok {
		headers = original
	}
	observed, hasReceipt := ctx.Value(openAI429ObservedAtKey{}).(time.Time)
	if !hasReceipt {
		observed = time.Now()
	}
	used, resetAfter, present, valid := openAI429WeeklyHeaderEvidence(headers)
	if !present || observed.IsZero() {
		return
	}
	var reset time.Time
	if resetAfter > 0 {
		reset = observed.Add(resetAfter)
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	updates := map[string]any{OpenAI429WeeklyEvidenceKey: openAI429Evidence(used, observed, reset, valid)}
	if err := s.accountRepo.UpdateExtra(stateCtx, account.ID, updates); err != nil {
		slog.Warn("openai_429_weekly_evidence_persist_failed", "account_id", account.ID)
	}
}

type openAIDisableScheduling429Observation struct {
	accountID int64
	disabled  bool
}

// Persist the normal scheduling switch, rather than a temporary cooldown. A
// successful test or an elapsed 429 window must not reopen the account. The
// context marker shares the result across gateway and common error handlers.
func (s *RateLimitService) maybeDisableOpenAISchedulingOn429(ctx context.Context, account *Account, status int, headers http.Header) (context.Context, bool) {
	if s == nil || s.accountRepo == nil || status != http.StatusTooManyRequests || account == nil ||
		account.ID <= 0 || account.Platform != PlatformOpenAI || account.IsShadow() {
		return ctx, false
	}
	if observed, ok := ctx.Value(openAIDisableScheduling429ObservedKey{}).(openAIDisableScheduling429Observation); ok && observed.accountID == account.ID {
		return ctx, observed.disabled
	}
	stateCtx, cancel := openAIAccountStateContext(ctx)
	defer cancel()
	// A projected api_key connection can belong to an OAuth account. Conversely,
	// a projected OAuth type/Extra must not opt an API Key owner into this policy.
	// Resolve the owning account authoritatively, retaining one observation even
	// on failure so the common handler does not repeat the read for this error.
	ctx = context.WithValue(ctx, openAIDisableScheduling429ObservedKey{}, openAIDisableScheduling429Observation{accountID: account.ID})
	owner := account
	if account.AuthConnectionID != "" {
		if !account.HasManagedOpenAIConnections() {
			slog.Warn("openai_429_disable_scheduling_owner_unverified", "account_id", account.ID)
			return ctx, false
		}
		var err error
		owner, err = s.accountRepo.GetByID(stateCtx, account.ID)
		if err != nil || owner == nil || owner.ID != account.ID {
			slog.Warn("openai_429_disable_scheduling_owner_read_failed", "account_id", account.ID)
			return ctx, false
		}
	}
	if !openAIDisableSchedulingOn429Enabled(owner) {
		return ctx, false
	}
	threshold, thresholdValid := openAI429WeeklyThreshold(owner)
	if unfiltered, ok := ctx.Value(openAI429HeadersKey{}).(http.Header); ok {
		headers = unfiltered
	}
	var receipts []time.Time
	if receipt, ok := ctx.Value(openAI429ObservedAtKey{}).(time.Time); ok {
		receipts = []time.Time{receipt}
	}
	used, usageValid := openAI429WeeklyUsage(owner, headers, time.Now(), receipts...)
	if !thresholdValid || !usageValid || used <= threshold {
		return ctx, false
	}
	err := s.accountRepo.SetSchedulable(stateCtx, owner.ID, false)
	if err != nil {
		// Never log an upstream response, request body or credential here.
		slog.Warn("openai_429_disable_scheduling_failed", "account_id", account.ID)
	} else {
		// SetSchedulable refreshes the shared scheduler snapshot; bridge stale
		// in-flight candidates immediately and stop same-account 429 retries.
		s.notifyAccountSchedulingBlocked(owner, time.Time{}, "openai_429_scheduling_disabled")
		slog.Info("openai_429_scheduling_disabled", "account_id", account.ID, "weekly_used_percent", used, "threshold_percent", threshold)
	}
	ctx = context.WithValue(ctx, openAIDisableScheduling429ObservedKey{}, openAIDisableScheduling429Observation{accountID: account.ID, disabled: err == nil})
	return ctx, err == nil
}
