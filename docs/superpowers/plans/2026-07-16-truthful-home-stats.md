# Truthful Home Statistics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace fabricated public-home counters with exact production metrics and establish repeatable database/API/UI reconciliation for public, admin, and user dashboards.

**Architecture:** Add a dedicated public stats repository/service that counts successful usage and computes real ops availability and weighted TTFT. Replace the browser-side synthetic state machine with a last-real-snapshot cache and verify all three surfaces against captured production queries after deployment.

**Tech Stack:** Go, PostgreSQL, Gin, Vue 3, TypeScript, Vitest, Playwright/browser acceptance.

---

### Task 1: Public Stats Query Domain

**Files:**
- Create: `backend/internal/service/public_home_stats_service.go`
- Create: `backend/internal/service/public_home_stats_service_test.go`
- Create: `backend/internal/repository/public_home_stats_repo.go`
- Create: `backend/internal/repository/public_home_stats_repo_test.go`
- Modify: `backend/internal/repository/wire.go`
- Modify: `backend/internal/service/wire.go`

- [ ] **Step 1: Write failing weighted-metric tests**

```go
func TestPublicHomeStatsUsesWeightedTTFTAndSLACounts(t *testing.T) {
    repo := &publicStatsRepoStub{raw: PublicHomeStatsRaw{
        TotalRequests: 120,
        Success30d: 99, ErrorSLA30d: 1,
        TTFTWeightedSum24h: 6000, TTFTSamples24h: 10,
    }}
    got, err := NewPublicHomeStatsService(repo).Get(t.Context())
    require.NoError(t, err)
    require.Equal(t, int64(120), got.TotalRequests)
    require.InEpsilon(t, 99.0, *got.AvailabilityPct, 1e-9)
    require.InEpsilon(t, 600.0, *got.AvgTTFTMs, 1e-9)
}
```

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/repository -run PublicHomeStats -count=1 -p 1`

Expected: FAIL because the dedicated service and repository do not exist.

- [ ] **Step 3: Implement authoritative SQL and null behavior**

The repository performs an exact `COUNT(*) FROM usage_logs` and aggregates only
overall `ops_metrics_hourly` rows (`platform IS NULL AND group_id IS NULL`). It
returns `SUM(success_count)`, `SUM(error_count_sla)`, and
`SUM(ttft_avg_ms * ttft_sample_count) / SUM(ttft_sample_count)` inputs for the
documented windows.

```go
type PublicHomeStats struct {
    TotalRequests int64      `json:"total_requests"`
    AvailabilityPct *float64 `json:"availability_pct"`
    AvgTTFTMs      *float64  `json:"avg_ttft_ms"`
    OpsDataThrough *time.Time `json:"ops_data_through"`
    ComputedAt      time.Time `json:"computed_at"`
}
```

Set `OpsDataThrough` from `MAX(bucket_start)` over the overall rows used by the
ops query. It is null when no overall aggregate row exists, so clients can
distinguish missing metrics from a stale aggregate watermark.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/repository -run PublicHomeStats -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit the stats domain**

```powershell
git add backend/internal/service/public_home_stats_service.go backend/internal/service/public_home_stats_service_test.go backend/internal/repository/public_home_stats_repo.go backend/internal/repository/public_home_stats_repo_test.go backend/internal/repository/wire.go backend/internal/service/wire.go
git commit -m "feat(stats): query truthful public metrics"
```

### Task 2: Handler, Cache, And Wiring

**Files:**
- Create: `backend/internal/handler/public_home_stats_test.go`
- Modify: `backend/internal/handler/public_home_stats.go`
- Modify: `backend/internal/server/router.go`
- Modify: `backend/cmd/server/wire.go`
- Regenerate: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: Write failing handler tests**

Assert exact JSON mapping, null metrics when samples are absent, and fallback to
the last real cached snapshot after a repository failure. Assert no heuristic
`99.97` value is manufactured.

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler -run PublicHomeStats -count=1 -p 1`

Expected: FAIL because the handler still accepts `DashboardService` and uses heuristics.

- [ ] **Step 3: Wire the dedicated service and bounded real cache**

Replace the handler dependency with `*service.PublicHomeStatsService`. Cache only
successful snapshots for 60 seconds; on refresh failure return the cached real
snapshot if present, otherwise return a server error. Include `computed_at` and
`ops_data_through` unchanged from the real snapshot.

- [ ] **Step 4: Regenerate Wire and run backend tests**

Run: `& 'C:\Program Files\Go\bin\go.exe' generate ./cmd/server`

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler ./internal/server ./cmd/server -run 'PublicHomeStats|Wire' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit handler and wiring**

```powershell
git add backend/internal/handler/public_home_stats.go backend/internal/handler/public_home_stats_test.go backend/internal/server/router.go backend/cmd/server/wire.go backend/cmd/server/wire_gen.go
git commit -m "fix(stats): expose real public home metrics"
```

### Task 3: Remove Synthetic Frontend Statistics

**Files:**
- Modify: `frontend/src/utils/homeLiveStats.ts`
- Modify: `frontend/src/composables/useHomeLiveStats.ts`
- Modify: `frontend/src/composables/__tests__/homeLiveStats.spec.ts`
- Modify: `frontend/src/api/publicHomeStats.ts`
- Modify: `frontend/src/i18n/locales/jisudeng-home.zh.ts`
- Modify: `frontend/src/i18n/locales/jisudeng-home.en.ts`

- [ ] **Step 1: Replace existing tests with failing truthful-value tests**

```ts
it('never advances a real snapshot with elapsed time', () => {
  vi.useFakeTimers()
  const snapshot = toHomeStatsValues({
    total_requests: 11336,
    availability_pct: 99.5,
    avg_ttft_ms: 600,
    computed_at: '2026-07-16T02:00:00Z',
  })
  expect(snapshot.requests).toBe(11336)
  vi.advanceTimersByTime(60 * 60 * 1000)
  expect(snapshot.requests).toBe(11336)
  vi.useRealTimers()
})

it('returns unavailable values without a real snapshot', () => {
  expect(emptyHomeStats()).toEqual({ requests: null, uptimePct: null, latencyMs: null })
})
```

- [ ] **Step 2: Run and verify RED**

Run: `pnpm --dir frontend exec vitest run src/composables/__tests__/homeLiveStats.spec.ts`

Expected: FAIL because synthetic helpers and baseline behavior remain.

- [ ] **Step 3: Implement last-real-snapshot behavior**

Delete `HOME_STATS_BASE_*`, request-rate, online-credit, and wave functions. The
composable loads a previously persisted real snapshot, refreshes every minute,
and never mutates metric values between responses. Format null as `--`. Keep
number animation only as a presentation transition ending at the exact value.

- [ ] **Step 4: Run tests, search, and build**

Run: `pnpm --dir frontend exec vitest run src/composables/__tests__/homeLiveStats.spec.ts`

Run: `rg -n "12_847_360|HOME_STATS_BASE|syntheticRequests|creditedMs" frontend/src`

Expected: no matches.

Run: `pnpm --dir frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit the frontend correction**

```powershell
git add frontend/src/utils/homeLiveStats.ts frontend/src/composables/useHomeLiveStats.ts frontend/src/composables/__tests__/homeLiveStats.spec.ts frontend/src/api/publicHomeStats.ts frontend/src/i18n/locales/jisudeng-home.zh.ts frontend/src/i18n/locales/jisudeng-home.en.ts
git commit -m "fix(frontend): remove fabricated home statistics"
```

### Task 4: Dashboard Reconciliation Tests

**Files:**
- Create: `backend/internal/repository/home_stats_reconciliation_test.go`
- Modify: `backend/internal/repository/usage_log_repo_dashboard.go`
- Modify only if test proves mismatch: `backend/internal/handler/usage_handler.go`
- Modify only if test proves mismatch: `backend/internal/handler/admin/dashboard_snapshot_v2_handler.go`

- [ ] **Step 1: Write database-backed reconciliation tests**

Seed usage rows with input, output, cache creation/read, actual cost, and models.
Assert public total, admin total/today, and user total/today use their documented
formulas and model sums. Capture one `now` value for all assertions.

- [ ] **Step 2: Run and verify RED or establish existing behavior**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository -run HomeStatsReconciliation -count=1 -p 1`

Expected: tests must fail for any undocumented mismatch; if all existing user/admin formulas already match, keep the passing characterization and do not alter production code.

- [ ] **Step 3: Fix only demonstrated mismatches**

Align repository queries or DTO mapping to the documented token and cost
formulas. Do not change formulas merely to share code when existing contracts
are already correct.

- [ ] **Step 4: Run repository and handler statistics tests**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository ./internal/handler -run 'Dashboard|Usage|HomeStatsReconciliation|PublicHomeStats' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit reconciliation coverage**

```powershell
git add backend/internal/repository/home_stats_reconciliation_test.go backend/internal/repository/usage_log_repo_dashboard.go backend/internal/handler/usage_handler.go backend/internal/handler/admin/dashboard_snapshot_v2_handler.go
git diff --cached --quiet || git commit -m "test(stats): reconcile dashboard totals"
```

### Task 5: Full Verification And Production Acceptance

- [ ] **Step 1: Run backend protection and build**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/repository ./internal/handler ./internal/server ./cmd/server -count=1 -p 1`

Run: `$env:CGO_ENABLED='0'; & 'C:\Program Files\Go\bin\go.exe' build -p 1 ./cmd/server`

- [ ] **Step 2: Run frontend tests and production build**

Run: `pnpm --dir frontend exec vitest run src/composables/__tests__/homeLiveStats.spec.ts`

Run: `pnpm --dir frontend run build`

- [ ] **Step 3: Deploy through the repository's existing production workflow**

Push the reviewed branch, wait for CI and Zeabur deployment success, and record
the deployed commit before acceptance. Do not mutate production statistics.

- [ ] **Step 4: Capture database truth at one reference time**

Run read-only PostgreSQL queries for total `usage_logs`, today's global and user
counts/tokens/cost, overall 30-day ops success/SLA errors, weighted 24-hour TTFT,
model distribution, and aggregation watermarks. Record lower/upper timestamps.

- [ ] **Step 5: Compare APIs and rendered pages**

Compare `/api/v1/public/home-stats`, admin dashboard APIs, and user dashboard APIs
to the captured SQL. Then inspect public, admin, and user pages and verify their
visible values equal API values. Traffic arriving between bounds must explain
the only permitted count difference.

- [ ] **Step 6: Verify reward isolation**

Confirm blindbox/team reward ledger and balance changes do not change usage-log
request, Token, or cost totals.

- [ ] **Step 7: Save acceptance evidence and commit corrections only**

Append the deployed commit, SQL/API/UI values, and watermarks to the existing
production acceptance document. Commit only evidence or corrections required by
failed acceptance checks.
