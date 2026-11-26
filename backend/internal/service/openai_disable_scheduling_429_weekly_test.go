//go:build unit

package service

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func weekly429Headers(weekly, fiveHour string, reverse bool) http.Header {
	headers := http.Header{}
	week, short := "primary", "secondary"
	if reverse {
		week, short = short, week
	}
	headers.Set("x-codex-"+week+"-window-minutes", "10080")
	headers.Set("x-codex-"+short+"-window-minutes", "300")
	if weekly != "" {
		headers.Set("x-codex-"+week+"-used-percent", weekly)
	}
	if fiveHour != "" {
		headers.Set("x-codex-"+short+"-used-percent", fiveHour)
	}
	return headers
}

func TestWeekly429EvidenceRequiresAnUnambiguousExplicitWindow(t *testing.T) {
	now := time.Now()
	account := scheduling429Account()
	delete(account.Extra, "codex_7d_window_minutes")
	_, valid := openAI429WeeklyUsage(account, nil, now)
	require.False(t, valid, "legacy normalized fields without a window identity are not enough")
	account.Extra["codex_7d_window_minutes"] = 7 * 24 * 60
	for _, tc := range []struct {
		name, key, value string
		duplicate, valid bool
	}{
		{"duplicate usage", "x-codex-primary-used-percent", "20", true, false},
		{"duplicate reset", "x-codex-primary-reset-after-seconds", "100", true, false},
		{"hidden weekly claim", "x-codex-primary-window-minutes", "300, 10080", false, false},
		{"malformed short identity", "x-codex-secondary-window-minutes", "invalid", false, true},
		{"short identity in weekly slot uses independent cache", "x-codex-primary-window-minutes", "60", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-reset-after-seconds", "300")
			if tc.duplicate {
				h.Add(tc.key, tc.value)
			} else {
				h.Set(tc.key, tc.value)
			}
			used, valid := openAI429WeeklyUsage(account, h, now)
			require.Equal(t, tc.valid, valid)
			if valid {
				require.Equal(t, 99.0, used)
			}
		})
	}
}

func TestWeekly429SnapshotEvidenceSurvivesUnrelatedUpdatesAndRejectsInvalidWeek(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	account := scheduling429Account()
	apply := func(headers http.Header, observed time.Time) {
		snapshot := ParseCodexRateLimitHeaders(headers)
		require.NotNil(t, snapshot)
		snapshot.UpdatedAt = observed.Format(time.RFC3339Nano)
		for key, value := range buildCodexUsageExtraUpdates(snapshot, observed) {
			account.Extra[key] = value
		}
	}
	apply(weekly429Headers("99", "15", false), now.Add(-time.Minute))
	unrelated := http.Header{}
	unrelated.Set("x-codex-primary-used-percent", "10")
	apply(unrelated, now.Add(-30*time.Second))
	used, valid := openAI429WeeklyUsage(account, nil, now)
	require.True(t, valid)
	require.Equal(t, 99.0, used, "unidentified observations must not overwrite weekly evidence")
	invalid := weekly429Headers("99", "15", false)
	invalid.Add("x-codex-primary-used-percent", "20")
	apply(invalid, now.Add(-20*time.Second))
	_, valid = openAI429WeeklyUsage(account, nil, now)
	require.False(t, valid, "a more recent ambiguous weekly packet must invalidate the old high")
	apply(weekly429Headers("20", "99", true), now.Add(-10*time.Second))
	used, valid = openAI429WeeklyUsage(account, nil, now)
	require.True(t, valid)
	require.Equal(t, 20.0, used)
}

func TestWeekly429LegacyCacheCannotBorrowNewObservation(t *testing.T) {
	account := scheduling429Account()
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "99")
	for key, value := range buildCodexUsageExtraUpdates(ParseCodexRateLimitHeaders(headers), time.Now()) {
		account.Extra[key] = value
	}
	_, valid := openAI429WeeklyUsage(account, nil, time.Now())
	require.False(t, valid)
}

func TestWeekly429ExcelBPSPreservesTemporaryCooldown(t *testing.T) {
	for _, used := range []string{"98", "99"} {
		t.Run(used, func(t *testing.T) {
			account := scheduling429Account()
			repo := &scheduling429Repo{}
			rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			gateway := &OpenAIGatewayService{rateLimitService: rates}
			gateway.coolDownExcelBPS(t.Context(), account, "60", weekly429Headers(used, "100", false))
			require.Equal(t, used == "99", len(repo.ids) != 0)
			until, ok := gateway.excelBPSCooldownUntil.Load(account.ID)
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(time.Minute), until.(time.Time), time.Second)
		})
	}
}

func TestWeekly429ObservationUsesReceiptTime(t *testing.T) {
	now := time.Now()
	headers := weekly429Headers("99", "10", false)
	headers.Set("x-codex-primary-reset-after-seconds", "60")
	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"live", now.Add(-30 * time.Second), true},
		{"expired", now.Add(-2 * time.Minute), false},
		{"missing", time.Time{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := scheduling429Account()
			repo := &scheduling429Repo{}
			rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			ctx := context.WithValue(t.Context(), openAI429ObservedAtKey{}, tc.at)
			ctx = context.WithValue(ctx, openAI429HeadersKey{}, headers)
			_, disabled := rates.maybeDisableOpenAISchedulingOn429(ctx, account, 429, nil)
			require.Equal(t, tc.want, disabled)
		})
	}
}

func TestWeekly429PersistenceKeepsReceiptTimeAndFilteredHeaders(t *testing.T) {
	now := time.Now()
	observed := now.Add(-2 * time.Minute)
	headers := weekly429Headers("99", "20", false)
	headers.Set("x-codex-primary-reset-after-seconds", "60")
	ctx := context.WithValue(t.Context(), openAI429ObservedAtKey{}, observed)
	ctx = context.WithValue(ctx, openAI429HeadersKey{}, headers)
	account := scheduling429Account()
	repo := &scheduling429Repo{}
	rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rates.persistOpenAICodexSnapshot(ctx, account, nil)
	evidence, ok := repo.lastExtraUpdates[OpenAI429WeeklyEvidenceKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, observed.UTC().Format(time.RFC3339Nano), evidence["observed_at"])
	require.NotContains(t, repo.lastExtraUpdates, "codex_7d_used_percent")
	account.Extra[OpenAI429WeeklyEvidenceKey] = evidence
	_, valid := openAI429WeeklyUsage(account, nil, now)
	require.False(t, valid, "writing an expired observation cannot renew it")
}

func TestDisableScheduling429RequiresWeeklyUsageStrictlyAboveThreshold(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*Account)
		headers http.Header
		want    bool
	}{
		{name: "legacy account defaults to 98", want: true},
		{name: "exactly 98 does not disable", mutate: func(a *Account) { a.Extra["codex_7d_used_percent"] = 98.0 }},
		{name: "fraction above default", mutate: func(a *Account) { a.Extra["codex_7d_used_percent"] = 98.01 }, want: true},
		{name: "custom fractional threshold", mutate: func(a *Account) { a.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey] = 99.5 }},
		{name: "custom lower threshold", mutate: func(a *Account) { a.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey] = 97.5 }, want: true},
		{name: "zero is a valid configured threshold", mutate: func(a *Account) { a.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey] = 0.0 }, want: true},
		{name: "100 never exceeded by valid usage", mutate: func(a *Account) { a.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey] = 100.0 }, headers: weekly429Headers("100", "100", false)},
		{name: "five hour exhaustion alone", mutate: func(a *Account) { a.Extra["codex_7d_used_percent"] = 20.0; a.Extra["codex_5h_used_percent"] = 100.0 }},
		{name: "missing weekly usage", mutate: func(a *Account) { delete(a.Extra, "codex_7d_used_percent"); a.Extra["codex_5h_used_percent"] = 100.0 }},
		{name: "cached short window mislabeled as weekly", mutate: func(a *Account) { a.Extra["codex_7d_window_minutes"] = 300 }},
		{name: "missing timestamp", mutate: func(a *Account) { delete(a.Extra, "codex_usage_updated_at") }},
		{name: "invalid timestamp", mutate: func(a *Account) { a.Extra["codex_usage_updated_at"] = "invalid" }},
		{name: "future observation", mutate: func(a *Account) {
			a.Extra["codex_usage_updated_at"] = time.Now().Add(time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "stale snapshot", mutate: func(a *Account) {
			a.Extra["codex_usage_updated_at"] = time.Now().Add(-3 * time.Hour).Format(time.RFC3339Nano)
		}},
		{name: "stale weekly field with fresh five hour update", mutate: func(a *Account) {
			a.Extra[CodexUsageFieldVersionsKey] = map[string]any{"codex_7d_used_percent": strconv.FormatInt(time.Now().Add(-3*time.Hour).UnixNano(), 10)}
		}},
		{name: "unknown weekly field version", mutate: func(a *Account) { a.Extra[CodexUsageFieldVersionsKey] = map[string]any{} }},
		{name: "already reset", mutate: func(a *Account) { a.Extra["codex_7d_reset_at"] = time.Now().Add(-time.Minute).Format(time.RFC3339Nano) }},
		{name: "invalid reset", mutate: func(a *Account) { a.Extra["codex_7d_reset_at"] = "invalid" }},
		{name: "relative reset already passed", mutate: func(a *Account) { a.Extra["codex_7d_reset_after_seconds"] = 1 }},
		{name: "identity mismatch", mutate: func(a *Account) {
			a.Credentials = map[string]any{"chatgpt_account_id": "owner"}
			a.Extra["chatgpt_account_id"] = "other"
		}},
		{name: "new low header overrides old high snapshot", headers: weekly429Headers("20", "100", false)},
		{name: "new high header overrides old low snapshot", mutate: func(a *Account) { a.Extra["codex_7d_used_percent"] = 20.0 }, headers: weekly429Headers("99", "20", false), want: true},
		{name: "latest equality overrides old high", headers: weekly429Headers("98", "100", false)},
		{name: "reversed windows do not confuse five hour quota", headers: weekly429Headers("20", "100", true)},
		{name: "reversed weekly quota", headers: weekly429Headers("99", "20", true), want: true},
		{name: "five hour header alone cannot confirm cached weekly exhaustion", headers: weekly429Headers("", "100", false)},
		{name: "new header validates otherwise stale snapshot", mutate: func(a *Account) { a.Extra["codex_usage_updated_at"] = "invalid" }, headers: weekly429Headers("99", "20", false), want: true},
		{name: "invalid current weekly header blocks fallback", headers: weekly429Headers("invalid", "100", false)},
		{name: "negative current weekly usage", headers: weekly429Headers("-1", "100", false)},
		{name: "over 100 current weekly usage", headers: weekly429Headers("101", "100", false)},
		{name: "NaN current weekly usage", headers: weekly429Headers("NaN", "100", false)},
		{name: "infinite current weekly usage", headers: weekly429Headers("+Inf", "100", false)},
		{name: "current reset already elapsed", headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-reset-after-seconds", "0")
			return h
		}()},
		{name: "empty current reset", headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-reset-after-seconds", "")
			return h
		}()},
		{name: "reset without window identity permits weekly cache", want: true, headers: func() http.Header { h := http.Header{}; h.Set("x-codex-primary-reset-after-seconds", "0"); return h }()},
		{name: "weekly reset and window without usage cannot reuse cached high", headers: func() http.Header {
			h := http.Header{}
			h.Set("x-codex-primary-reset-after-seconds", "0")
			h.Set("x-codex-primary-window-minutes", "10080")
			return h
		}()},
		{name: "weekly window alone cannot reuse cached high", headers: func() http.Header { h := http.Header{}; h.Set("x-codex-primary-window-minutes", "10080"); return h }()},
		{name: "short window alone permits weekly cache", want: true, headers: func() http.Header { h := http.Header{}; h.Set("x-codex-primary-window-minutes", "300"); return h }()},
		{name: "empty window permits weekly cache", want: true, headers: func() http.Header { h := http.Header{}; h.Set("x-codex-primary-window-minutes", ""); return h }()},
		{name: "empty current window permits weekly cache", want: true, headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-window-minutes", "")
			return h
		}()},
		{name: "nil window values permit weekly cache", want: true, headers: http.Header{http.CanonicalHeaderKey("x-codex-primary-window-minutes"): nil}},
		{name: "invalid current reset", headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-reset-after-seconds", "invalid")
			return h
		}()},
		{name: "two short windows permit weekly cache", want: true, headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-window-minutes", "60")
			return h
		}()},
		{name: "unclassified primary permits weekly cache", want: true, headers: func() http.Header { h := http.Header{}; h.Set("x-codex-primary-used-percent", "100"); return h }()},
		{name: "two weekly windows are ambiguous", headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-secondary-window-minutes", "10080")
			return h
		}()},
		{name: "empty current weekly header", headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-used-percent", "")
			return h
		}()},
		{name: "invalid unidentified current window permits weekly cache", want: true, headers: func() http.Header {
			h := weekly429Headers("99", "100", false)
			h.Set("x-codex-primary-window-minutes", "invalid")
			return h
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := scheduling429Account()
			if tc.mutate != nil {
				tc.mutate(account)
			}
			repo, blocker := &scheduling429Repo{}, &runtimeBlockRecorder{}
			rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			rates.SetAccountRuntimeBlocker(blocker)
			ctx, disabled := rates.maybeDisableOpenAISchedulingOn429(t.Context(), account, http.StatusTooManyRequests, tc.headers)
			_, repeated := rates.maybeDisableOpenAISchedulingOn429(ctx, account, http.StatusTooManyRequests, tc.headers)
			require.Equal(t, tc.want, disabled)
			require.Equal(t, disabled, repeated)
			if tc.want {
				require.Equal(t, []int64{account.ID}, repo.ids)
				require.Equal(t, []bool{false}, repo.values)
			} else {
				require.Empty(t, repo.ids)
				require.Empty(t, blocker.accounts)
			}
		})
	}
}

func TestDisableScheduling429RejectsInvalidStoredThresholdsAndUsage(t *testing.T) {
	for _, key := range []string{OpenAIDisableSchedulingOn4297dThresholdExtraKey, "codex_7d_used_percent"} {
		for _, invalid := range []any{nil, "invalid", -1.0, 101.0, math.NaN(), math.Inf(1), true} {
			account := scheduling429Account()
			account.Extra[key] = invalid
			repo := &scheduling429Repo{}
			rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			_, disabled := rates.maybeDisableOpenAISchedulingOn429(t.Context(), account, 429, nil)
			require.False(t, disabled, "key=%s invalid=%v", key, invalid)
			require.Empty(t, repo.ids)
		}
	}
}

func TestManaged429WeeklyPolicyUsesOwnerThresholdAndSnapshot(t *testing.T) {
	for _, used := range []float64{20, 99} {
		owner := scheduling429Account()
		owner.Extra["codex_7d_used_percent"] = used
		projection := scheduling429Account()
		projection.AuthConnectionID = "connection"
		projection.Extra["openai_connections_managed"] = true
		projection.Extra[OpenAIDisableSchedulingOn4297dThresholdExtraKey] = 0.0
		repo := &scheduling429Repo{owner: owner}
		rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		_, disabled := rates.maybeDisableOpenAISchedulingOn429(t.Context(), projection, 429, nil)
		require.Equal(t, used > 98, disabled)
		require.Equal(t, []int64{owner.ID}, repo.reads)
	}
}

func TestWeekly429ThresholdConfigValidation(t *testing.T) {
	for _, threshold := range []float64{0, 98, 98.5, 100} {
		cfg := DefaultOAuthAutoConfig()
		cfg.OpenAIDisableSchedulingOn4297dThresholdPercent = threshold
		require.NoError(t, ValidateOAuthAutoConfig(cfg))
	}
	for _, threshold := range []float64{-1, 100.01, math.NaN(), math.Inf(1)} {
		cfg := DefaultOAuthAutoConfig()
		cfg.OpenAIDisableSchedulingOn4297dThresholdPercent = threshold
		require.Error(t, ValidateOAuthAutoConfig(cfg))
	}
	for _, raw := range []string{`{"openai_disable_scheduling_on_429_7d_threshold_percent":null}`, `{"openai_disable_scheduling_on_429_7d_threshold_percent":"98"}`} {
		cfg := DefaultOAuthAutoConfig()
		require.Error(t, json.Unmarshal([]byte(raw), &cfg))
	}
}

type scheduling429Repo struct {
	rateLimitAccountRepoStub
	ids        []int64
	values     []bool
	err        error
	ctxErr     error
	owner      *Account
	getErr     error
	reads      []int64
	readCtxErr error
}

func (r *scheduling429Repo) GetByID(ctx context.Context, id int64) (*Account, error) {
	r.reads = append(r.reads, id)
	r.readCtxErr = ctx.Err()
	return r.owner, r.getErr
}

func (r *scheduling429Repo) SetSchedulable(ctx context.Context, id int64, enabled bool) error {
	r.ids = append(r.ids, id)
	r.values = append(r.values, enabled)
	r.ctxErr = ctx.Err()
	return r.err
}

func scheduling429Account() *Account {
	return &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Schedulable: true,
		Extra: map[string]any{OpenAIDisableSchedulingOn429ExtraKey: true, "codex_7d_used_percent": 99.0, "codex_7d_window_minutes": 10080, "codex_usage_updated_at": time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}}
}

func TestWeekly429SuccessfulSnapshotKeepsInvalidEvidenceDuringThrottle(t *testing.T) {
	repo := &openAICodexSnapshotAsyncRepo{updateExtraCh: make(chan map[string]any, 2)}
	gateway := &OpenAIGatewayService{accountRepo: repo, codexSnapshotThrottle: newAccountWriteThrottle(time.Hour)}
	account := scheduling429Account()
	awaitWrite := func() map[string]any {
		t.Helper()
		select {
		case update := <-repo.updateExtraCh:
			return update
		case <-time.After(time.Second):
			t.Fatal("missing weekly observation")
			return nil
		}
	}
	gateway.UpdateCodexUsageSnapshotFromHeaders(t.Context(), account.ID, weekly429Headers("99", "20", false))
	first := awaitWrite()
	require.Contains(t, first, "codex_7d_used_percent")
	invalid := weekly429Headers("99", "20", false)
	invalid.Add("x-codex-primary-used-percent", "0")
	gateway.UpdateCodexUsageSnapshotFromHeaders(t.Context(), account.ID, invalid)
	second := awaitWrite()
	require.Len(t, second, 1, "weekly evidence must not bypass throttling for ordinary quota fields")
	evidence := second[OpenAI429WeeklyEvidenceKey]
	require.NotNil(t, evidence)
	account.Extra[OpenAI429WeeklyEvidenceKey] = evidence
	_, valid := openAI429WeeklyUsage(account, nil, time.Now())
	require.False(t, valid)
}

func TestWeekly429ExcelBPSPersistsInvalidEvidenceWithoutQuotaCooldown(t *testing.T) {
	account := scheduling429Account()
	repo := &scheduling429Repo{}
	rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	gateway := &OpenAIGatewayService{rateLimitService: rates}
	invalid := weekly429Headers("99", "100", false)
	invalid.Add("x-codex-primary-used-percent", "0")
	gateway.coolDownExcelBPS(t.Context(), account, "60", invalid)
	require.Empty(t, repo.ids)
	require.Zero(t, repo.rateLimitedCalls, "a BPS cooldown must remain route-local")
	_, cooling := gateway.excelBPSCooldownUntil.Load(account.ID)
	require.True(t, cooling)
	require.Len(t, repo.lastExtraUpdates, 1)
	evidence := repo.lastExtraUpdates[OpenAI429WeeklyEvidenceKey]
	require.NotNil(t, evidence)
	account.Extra[OpenAI429WeeklyEvidenceKey] = evidence
	_, valid := openAI429WeeklyUsage(account, nil, time.Now())
	require.False(t, valid)
}

type weekly429SameAccountRetry struct{ AccountRuntimeBlocker }

func (weekly429SameAccountRetry) ShouldRetryOpenAIOAuth429(*Account, http.Header, []byte) bool {
	return true
}

func TestWeekly429SameAccountRetryKeepsInvalidEvidence(t *testing.T) {
	account := scheduling429Account()
	repo := &scheduling429Repo{}
	rates := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rates.SetAccountRuntimeBlocker(weekly429SameAccountRetry{})
	invalid := weekly429Headers("99", "20", false)
	invalid.Add("x-codex-primary-used-percent", "0")
	// WS/semantic 429 passes filtered headers to the temporary policy.
	observed := time.Now().Add(-time.Minute)
	ctx := context.WithValue(t.Context(), openAI429ObservedAtKey{}, observed)
	ctx = context.WithValue(ctx, openAI429HeadersKey{}, invalid)
	shouldDisable := rates.HandleUpstreamError(ctx, account, http.StatusTooManyRequests, nil, nil)
	require.False(t, shouldDisable)
	require.Zero(t, repo.rateLimitedCalls)
	require.Equal(t, observed.UTC().Format(time.RFC3339Nano), repo.lastExtraUpdates[OpenAI429WeeklyEvidenceKey].(map[string]any)["observed_at"])
	require.Len(t, repo.lastExtraUpdates, 1)
	evidence := repo.lastExtraUpdates[OpenAI429WeeklyEvidenceKey]
	require.NotNil(t, evidence)
	account.Extra[OpenAI429WeeklyEvidenceKey] = evidence
	_, valid := openAI429WeeklyUsage(account, nil, time.Now())
	require.False(t, valid)
}
