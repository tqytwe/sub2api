package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func qualityRuntimeQuotaAccount(now time.Time) *Account {
	return &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true,
		Extra: map[string]any{"codex_usage_updated_at": now.Format(time.RFC3339Nano), "codex_5h_used_percent": 20.0, "codex_7d_used_percent": 30.0}}
}

func TestQualityRuntimeQuotaEvidence(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		change func(*Account)
		want   bool
	}{
		{"available", func(*Account) {}, true},
		{"five_hour_exhausted", func(a *Account) { a.Extra["codex_5h_used_percent"] = 100.0 }, false},
		{"weekly_exhausted", func(a *Account) { a.Extra["codex_7d_used_percent"] = 100.0 }, false},
		{"over_100", func(a *Account) { a.Extra["codex_7d_used_percent"] = 101.0 }, false},
		{"unknown", func(a *Account) { a.Extra = nil }, true},
		{"undated", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			delete(a.Extra, "codex_usage_updated_at")
		}, true},
		{"invalid_date", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["codex_usage_updated_at"] = "unknown"
		}, true},
		{"future_date", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["codex_usage_updated_at"] = now.Add(time.Hour).Format(time.RFC3339)
		}, true},
		{"stale_without_reset", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["codex_usage_updated_at"] = now.Add(-3 * time.Hour).Format(time.RFC3339)
		}, true},
		{"old_but_current_week", func(a *Account) {
			a.Extra["codex_7d_used_percent"] = 100.0
			a.Extra["codex_usage_updated_at"] = now.Add(-3 * time.Hour).Format(time.RFC3339)
			a.Extra["codex_7d_reset_at"] = now.Add(time.Hour).Format(time.RFC3339)
		}, false},
		{"five_hour_reset", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["codex_5h_reset_at"] = now.Format(time.RFC3339)
		}, true},
		{"weekly_reset", func(a *Account) {
			a.Extra["codex_7d_used_percent"] = 100.0
			a.Extra["codex_7d_reset_at"] = now.Add(-time.Second).Format(time.RFC3339)
		}, true},
		{"relative_reset", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["codex_usage_updated_at"] = now.Add(-time.Hour).Format(time.RFC3339)
			a.Extra["codex_5h_reset_after_seconds"] = 3600
		}, true},
		{"nan", func(a *Account) { a.Extra["codex_5h_used_percent"] = math.NaN() }, true},
		{"infinite", func(a *Account) { a.Extra["codex_5h_used_percent"] = math.Inf(1) }, true},
		{"bad_percent", func(a *Account) { a.Extra["codex_5h_used_percent"] = "unknown" }, true},
		{"other_platform", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Platform = PlatformAnthropic
		}, true},
		{"api_key", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Type = AccountTypeAPIKey
		}, true},
		{"other_identity", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Credentials = map[string]any{"chatgpt_account_id": "fixture-a"}
			a.Extra["chatgpt_account_id"] = "fixture-b"
		}, true},
		{"live_429", func(a *Account) {
			reset := now.Add(time.Minute)
			a.RateLimitResetAt = &reset
			a.Extra = nil
		}, false},
		{"expired_429", func(a *Account) {
			reset := now
			a.RateLimitResetAt = &reset
		}, true},
		{"auto_pause_disabled_still_exhausted", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 100.0
			a.Extra["auto_pause_5h_disabled"] = true
		}, false},
		{"below_full_even_with_lower_pause_threshold", func(a *Account) {
			a.Extra["codex_5h_used_percent"] = 99.0
			a.Extra["auto_pause_5h_threshold"] = 50
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := qualityRuntimeQuotaAccount(now)
			tc.change(a)
			require.Equal(t, tc.want, QualityTemplateQuotaAvailable(QualityRuleAccountFilter{Statuses: []string{StatusActive}}, a, now))
		})
	}
	for _, statuses := range [][]string{nil, {"rate_limited"}, {"active", "rate_limited"}, {"active", "error"}, {"temp_unschedulable"}, {"unschedulable"}, {"inactive"}} {
		a := qualityRuntimeQuotaAccount(now)
		a.Extra["codex_5h_used_percent"] = 100.0
		require.True(t, QualityTemplateQuotaAvailable(QualityRuleAccountFilter{Statuses: statuses}, a, now), "explicit status selection %v", statuses)
	}
}

func TestQualityRuntimeQuotaObservationsKeepFieldTimes(t *testing.T) {
	now := time.Now()
	a := qualityRuntimeQuotaAccount(now)
	a.Extra["codex_7d_used_percent"] = 100.0
	a.Extra["codex_usage_field_versions"] = map[string]any{
		"codex_5h_used_percent": now.UnixNano(),
		"codex_7d_used_percent": now.Add(-3 * time.Hour).UnixNano(),
	}
	fieldVersions, ok := a.Extra["codex_usage_field_versions"].(map[string]any)
	require.True(t, ok, "usage field versions must be a map")
	filter := QualityRuleAccountFilter{Statuses: []string{"active"}}
	require.True(t, QualityTemplateQuotaAvailable(filter, a, now), "new 5h observation must not refresh stale weekly exhaustion")
	a.Extra["codex_7d_reset_after_seconds"] = 3600
	fieldVersions["codex_7d_reset_after_seconds"] = now.Add(-3 * time.Hour).UnixNano()
	require.True(t, QualityTemplateQuotaAvailable(filter, a, now), "relative reset must use its original field observation")
	a.Extra["codex_7d_reset_at"] = now.Add(time.Hour).Format(time.RFC3339Nano)
	require.False(t, QualityTemplateQuotaAvailable(filter, a, now), "known current window retains exhaustion until reset")
	delete(fieldVersions, "codex_7d_used_percent")
	require.True(t, QualityTemplateQuotaAvailable(filter, a, now), "missing version in versioned observations is unknown")
}
