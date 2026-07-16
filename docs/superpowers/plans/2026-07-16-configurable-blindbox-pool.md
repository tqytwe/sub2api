# Configurable Blindbox Pool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the approved seven-tier blindbox pool as a validated, versioned setting used by backend draws, APIs, admin configuration, and the user preview.

**Architecture:** Add a focused pool parser/validator in the Play service, pass an explicit draw source for deterministic tests, and make `play_blindbox_pool_json` authoritative. Persist pool audit fields on each open and expose the same sanitized pool through public, authenticated, and admin APIs so the frontend has no independent reward constants.

**Tech Stack:** Go, Gin, PostgreSQL migrations, Vue 3, TypeScript, Vitest.

---

### Task 1: Pool Domain And Runtime

**Files:**
- Create: `backend/internal/service/play_blindbox_pool.go`
- Create: `backend/internal/service/play_blindbox_pool_test.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/play_models.go`
- Modify: `backend/internal/service/play_service.go`
- Modify: `backend/internal/service/setting_play_runtime.go`

- [ ] **Step 1: Write the failing pool tests**

```go
func TestDefaultBlindboxPoolIsApprovedPool(t *testing.T) {
    pool := defaultBlindboxPool()
    require.NoError(t, ValidateBlindboxPool(pool))
    require.Equal(t, "season-1-v1", pool.Version)
    require.Equal(t, []float64{0.05, 0.2, 0.5, 1, 3, 10, 20}, tierAmounts(pool))
    require.InEpsilon(t, 0.45, pool.ExpectedReward(), 1e-9)
}

func TestPickBlindboxRewardAtCoversBoundaries(t *testing.T) {
    pool := defaultBlindboxPool()
    require.Equal(t, 0.05, pickBlindboxRewardAt(pool, 0))
    require.Equal(t, 0.20, pickBlindboxRewardAt(pool, 4000))
    require.Equal(t, 20.00, pickBlindboxRewardAt(pool, 9999))
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service -run 'Test(DefaultBlindboxPool|PickBlindboxRewardAt)' -count=1 -p 1`

Expected: FAIL because `PlayBlindboxPool`, `defaultBlindboxPool`, and validation do not exist.

- [ ] **Step 3: Implement the pool types and validation**

```go
const blindboxWeightTotal int64 = 10_000

type PlayBlindboxTier struct {
    Amount float64 `json:"amount"`
    Weight int64   `json:"weight"`
}

type PlayBlindboxPool struct {
    Version string             `json:"version"`
    Cost    float64            `json:"cost"`
    RTPCap  float64            `json:"rtp_cap"`
    Tiers   []PlayBlindboxTier `json:"tiers"`
}

func pickBlindboxRewardAt(pool PlayBlindboxPool, draw int64) float64 {
    var cumulative int64
    for _, tier := range pool.Tiers {
        cumulative += tier.Weight
        if draw < cumulative { return tier.Amount }
    }
    return pool.Tiers[len(pool.Tiers)-1].Amount
}
```

Add `SettingKeyPlayBlindboxPoolJSON`, parse the setting with fallback to the
approved pool, and add `BlindboxPool PlayBlindboxPool` to `PlayRuntime` and
`PlayBlindboxStatus`. Add `PoolVersion` and `OpenSource` string fields to
`PlayBlindboxOpenResult`.

Add a package-private `blindboxDrawSource func(max int64) (int64, error)` field to
`PlayService`, initialize it to a crypto-random implementation in
`NewPlayService`, and let focused tests replace it with deterministic boundary
or error functions. Reject draws outside `[0, max)` before any balance change.

- [ ] **Step 4: Run the focused service tests and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service -run 'BlindboxPool|PickBlindboxRewardAt' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit the domain change**

```powershell
git add backend/internal/service/play_blindbox_pool.go backend/internal/service/play_blindbox_pool_test.go backend/internal/service/domain_constants.go backend/internal/service/play_models.go backend/internal/service/play_service.go backend/internal/service/setting_play_runtime.go
git commit -m "feat(play): restore configurable blindbox pool"
```

### Task 2: Forward Migration And Open Audit

**Files:**
- Create: `backend/migrations/190_restore_configurable_blindbox_pool.sql`
- Create: `backend/migrations/blindbox_pool_restore_migration_test.go`
- Modify: `backend/internal/repository/play_repo_extended.go`
- Modify: `backend/internal/service/play_models.go`

- [ ] **Step 1: Write the failing migration contract test**

```go
func TestBlindboxPoolRestoreMigrationIsForwardOnly(t *testing.T) {
    raw, err := FS.ReadFile("190_restore_configurable_blindbox_pool.sql")
    require.NoError(t, err)
    sql := string(raw)
    require.Contains(t, sql, "play_blindbox_pool_json")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS pool_version")
    require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS open_source")
    require.NotContains(t, strings.ToUpper(sql), "DROP TABLE")
}
```

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./migrations -run BlindboxPoolRestore -count=1 -p 1`

Expected: FAIL because migration 190 is absent.

- [ ] **Step 3: Add the idempotent migration and audited record API**

The migration inserts the approved JSON only when the key is absent, adds the two columns with defaults, and backfills existing rows as `legacy-v1`/`paid` without changing amounts.

```go
type PlayBlindboxOpenRecord struct {
    UserID         int64
    Date           time.Time
    Cost           float64
    Reward         float64
    IdempotencyKey string
    PoolVersion    string
    OpenSource     string
}
```

The repository insert must persist `pool_version` and `open_source` from this
record in the same transaction as the open row.

- [ ] **Step 4: Run migration and repository tests**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./migrations ./internal/repository -run 'Blindbox|PlayRepository' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit the migration**

```powershell
git add backend/migrations/190_restore_configurable_blindbox_pool.sql backend/migrations/blindbox_pool_restore_migration_test.go backend/internal/repository/play_repo_extended.go backend/internal/service/play_models.go
git commit -m "feat(play): audit blindbox pool versions"
```

### Task 3: Backend Draw And API Contracts

**Files:**
- Create: `backend/internal/service/play_blindbox_open_test.go`
- Create: `backend/internal/handler/play_handler_blindbox_test.go`
- Modify: `backend/internal/service/play_extended.go`
- Modify: `backend/internal/handler/play_handler_extended.go`
- Modify: `backend/internal/server/routes/play.go`

- [ ] **Step 1: Write failing service and handler tests**

Assert that `OpenBlindbox` uses `rt.BlindboxPool.Cost`, returns `pool_version`, records the configured tier, and that public `/play/blindbox/pool` and authenticated status return identical pool JSON.

```go
require.Equal(t, "season-1-v1", result.PoolVersion)
require.Equal(t, 20.0, public.Pool.Tiers[6].Amount)
require.Equal(t, public.Pool, status.Pool)
```

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/handler -run 'Blindbox(Open|Pool|Status)' -count=1 -p 1`

Expected: FAIL because service and DTOs still use legacy fields.

- [ ] **Step 3: Replace the hard-coded draw and expose the pool**

```go
func (s *PlayService) pickBlindboxReward(pool PlayBlindboxPool) (float64, error) {
    draw, err := s.blindboxDrawSource(blindboxWeightTotal)
    if err != nil { return 0, err }
    if draw < 0 || draw >= blindboxWeightTotal {
        return 0, fmt.Errorf("blindbox draw out of range: %d", draw)
    }
    return pickBlindboxRewardAt(pool, draw), nil
}
```

Remove the legacy five-tier function. Fail before charging if pool loading or randomness fails. Add `GET /play/blindbox/pool` outside JWT auth and map the pool in all DTOs.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/handler ./internal/server -run 'Blindbox|PlayRoutes' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit backend behavior**

```powershell
git add backend/internal/service/play_extended.go backend/internal/service/play_blindbox_open_test.go backend/internal/handler/play_handler_extended.go backend/internal/handler/play_handler_blindbox_test.go backend/internal/server/routes/play.go
git commit -m "fix(play): use configured blindbox rewards"
```

### Task 4: User Prize Preview

**Files:**
- Create: `frontend/src/views/public/__tests__/BlindboxView.spec.ts`
- Modify: `frontend/src/api/play.ts`
- Modify: `frontend/src/views/public/BlindboxView.vue`
- Modify: `frontend/src/i18n/locales/jisudeng-pages.zh.ts`
- Modify: `frontend/src/i18n/locales/jisudeng-pages.en.ts`

- [ ] **Step 1: Write a failing Vitest rendering test**

Mock a pool containing `$20/0.1%`, mount the view, and assert `$20.00` and `0.1%` render while `$2.00` does not.

- [ ] **Step 2: Run and verify RED**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/BlindboxView.spec.ts`

Expected: FAIL because `prizeTiers` is hard-coded.

- [ ] **Step 3: Render the API pool and remove misleading VIP copy**

```ts
const prizeTiers = computed(() => (status.value?.pool?.tiers ?? []).map((tier) => ({
  amount: tier.amount.toFixed(2),
  rate: `${tier.weight / 100}%`,
})))
```

For guests, load the public pool endpoint. Disable opening when no effective pool is available. Replace the VIP upgrade note with live-pool wording.

- [ ] **Step 4: Run the frontend tests and typecheck**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/BlindboxView.spec.ts`

Run: `pnpm --dir frontend run typecheck`

Expected: PASS.

- [ ] **Step 5: Commit the user UI**

```powershell
git add frontend/src/api/play.ts frontend/src/views/public/BlindboxView.vue frontend/src/views/public/__tests__/BlindboxView.spec.ts frontend/src/i18n/locales/jisudeng-pages.zh.ts frontend/src/i18n/locales/jisudeng-pages.en.ts
git commit -m "fix(frontend): show the live blindbox pool"
```

### Task 5: Admin Pool Editor

**Files:**
- Create: `frontend/src/components/admin/play/BlindboxPoolEditor.vue`
- Create: `frontend/src/components/admin/play/__tests__/BlindboxPoolEditor.spec.ts`
- Modify: `backend/internal/handler/admin/play_handler.go`
- Modify: `backend/internal/server/routes/admin.go`
- Modify: `frontend/src/api/admin/play.ts`
- Modify: `frontend/src/views/admin/SettingsView.vue`

- [ ] **Step 1: Write failing backend and frontend validation tests**

Assert total weight `9999` and RTP above cap are rejected, and that saving a valid seven-tier pool calls `PUT /admin/play/blindbox/pool`.

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler/admin -run BlindboxPool -count=1 -p 1`

Run: `pnpm --dir frontend exec vitest run src/components/admin/play/__tests__/BlindboxPoolEditor.spec.ts`

Expected: FAIL because admin endpoints and editor do not exist.

- [ ] **Step 3: Implement admin get/update and the focused editor component**

Expose `GET` and `PUT /api/v1/admin/play/blindbox/pool` through `PlayService`.
Add `SettingService.SetPlayBlindboxPool`, validate and marshal before calling
`SettingRepository.SetMultiple`, and leave the previous value untouched on
validation or persistence failure. `GetPlayRuntime` reads the setting store on
each call, so the next read observes the committed value without a separate
cache invalidation API. Keep the giant Settings view limited to mounting the
focused component.

- [ ] **Step 4: Run focused tests and production builds**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler/admin ./internal/service -run BlindboxPool -count=1 -p 1`

Run: `pnpm --dir frontend exec vitest run src/components/admin/play/__tests__/BlindboxPoolEditor.spec.ts`

Run: `pnpm --dir frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit the admin editor**

```powershell
git add backend/internal/handler/admin/play_handler.go backend/internal/server/routes/admin.go frontend/src/api/admin/play.ts frontend/src/components/admin/play/BlindboxPoolEditor.vue frontend/src/components/admin/play/__tests__/BlindboxPoolEditor.spec.ts frontend/src/views/admin/SettingsView.vue
git commit -m "feat(admin): manage the blindbox pool"
```

### Task 6: Blindbox Verification

- [ ] **Step 1: Run all Play and migration tests**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/repository ./internal/handler ./internal/server ./migrations -run 'Play|Blindbox|Growth' -count=1 -p 1`

- [ ] **Step 2: Run frontend tests and build**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/BlindboxView.spec.ts src/components/admin/play/__tests__/BlindboxPoolEditor.spec.ts`

Run: `pnpm --dir frontend run build`

- [ ] **Step 3: Confirm no legacy pool remains**

Run: `rg -n "0\.05.*40|amount: '2\.00'|\{2\.0, 2\}" backend/internal frontend/src`

Expected: no legacy hard-coded prize array outside test fixtures or migration history.

- [ ] **Step 4: Commit any verification-only corrections**

Only commit corrections required by failed checks; do not create an empty commit.
