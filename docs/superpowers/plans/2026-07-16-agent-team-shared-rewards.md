# Agent Team Shared Rewards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add month-end team cashback paid directly to member balances by actual-cost contribution, with auditable membership history, lifecycle operations, and retry-safe settlement.

**Architecture:** Keep active and historical memberships in `play_team_members`, snapshot one immutable settlement per team/month, and pay one idempotent allocation per contributing user. Extend the existing Play runner for automatic previous-month settlement and expose focused user/admin APIs for lifecycle, progress, history, configuration, and retries.

**Tech Stack:** Go, shopspring/decimal, Gin, PostgreSQL, Vue 3, TypeScript, Vitest.

---

### Task 1: Reward Tier And Allocation Domain

**Files:**
- Create: `backend/internal/service/play_team_rewards.go`
- Create: `backend/internal/service/play_team_rewards_test.go`
- Modify: `backend/internal/service/domain_constants.go`
- Modify: `backend/internal/service/play_models.go`
- Modify: `backend/internal/service/setting_play_runtime.go`

- [ ] **Step 1: Write failing tier and allocation tests**

```go
func TestResolveTeamRewardTier(t *testing.T) {
    cfg := defaultTeamRewardConfig()
    require.Equal(t, "0", resolveTeamRewardPool(decimal.NewFromInt(19), cfg).String())
    require.Equal(t, "3", resolveTeamRewardPool(decimal.NewFromInt(100), cfg).String())
    require.Equal(t, "250", resolveTeamRewardPool(decimal.NewFromInt(5000), cfg).String())
    require.Equal(t, "250", resolveTeamRewardPool(decimal.NewFromInt(10000), cfg).String())
}

func TestAllocateTeamRewardAssignsRemainderDeterministically(t *testing.T) {
    got := allocateTeamReward(decimal.RequireFromString("1.00000000"), []TeamContribution{
        {UserID: 9, Amount: decimal.NewFromInt(2)},
        {UserID: 3, Amount: decimal.NewFromInt(1)},
    })
    require.Equal(t, "0.66666667", got[9].StringFixed(8))
    require.Equal(t, "0.33333333", got[3].StringFixed(8))
}
```

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service -run 'TeamReward|AllocateTeam' -count=1 -p 1`

Expected: FAIL because reward config and allocation functions do not exist.

- [ ] **Step 3: Implement fixed-precision config and allocation**

```go
type TeamRewardTier struct { Threshold, Rate decimal.Decimal }
type TeamRewardConfig struct { Enabled bool; Cap decimal.Decimal; Tiers []TeamRewardTier }
type TeamContribution struct { UserID int64; Amount decimal.Decimal }

func defaultTeamRewardConfig() TeamRewardConfig {
    return TeamRewardConfig{Enabled: true, Cap: decimal.NewFromInt(250), Tiers: []TeamRewardTier{
        {decimal.NewFromInt(20), decimal.RequireFromString("0.02")},
        {decimal.NewFromInt(100), decimal.RequireFromString("0.03")},
        {decimal.NewFromInt(500), decimal.RequireFromString("0.04")},
        {decimal.NewFromInt(2000), decimal.RequireFromString("0.05")},
    }}
}
```

Add settings for enabled, tiers JSON, cap, and a non-editable settlement start
month. The new shared-reward switch defaults to enabled; the obsolete affiliate
switch remains ignored. Reject non-increasing thresholds/rates and non-positive
caps.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service -run 'TeamReward|AllocateTeam' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit the domain model**

```powershell
git add backend/internal/service/play_team_rewards.go backend/internal/service/play_team_rewards_test.go backend/internal/service/domain_constants.go backend/internal/service/play_models.go backend/internal/service/setting_play_runtime.go
git commit -m "feat(play): define shared team rewards"
```

### Task 2: Membership And Settlement Migration

**Files:**
- Create: `backend/migrations/191_agent_team_shared_rewards.sql`
- Create: `backend/migrations/agent_team_shared_rewards_migration_test.go`

- [ ] **Step 1: Write the failing migration contract test**

Require `archived_at`, `left_at`, removal of both legacy membership uniqueness
constraints, the partial active-membership index, team-event,
settlement/allocation tables, decimal columns, and unique team-period/user
constraints. Reject destructive table drops.

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./migrations -run AgentTeamSharedRewards -count=1 -p 1`

Expected: FAIL because migration 191 is absent.

- [ ] **Step 3: Add the forward migration**

```sql
ALTER TABLE play_teams ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE play_team_members ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ;
ALTER TABLE play_team_members DROP CONSTRAINT IF EXISTS uq_play_team_members_user;
ALTER TABLE play_team_members DROP CONSTRAINT IF EXISTS uq_play_team_members_team_user;
CREATE UNIQUE INDEX IF NOT EXISTS uq_play_team_members_active_user
  ON play_team_members(user_id) WHERE left_at IS NULL;
```

Create `play_team_events` with team, actor, optional subject, event type, JSONB
detail, and timestamp columns. Create `play_team_settlements` and
`play_team_reward_allocations`. Insert default enabled reward settings only when
absent, plus `play_team_shared_reward_start_month` set to the current
`Asia/Shanghai` calendar month. Preserve every current membership as active.

- [ ] **Step 4: Run migration tests and integration setup**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./migrations -run AgentTeamSharedRewards -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit the migration**

```powershell
git add backend/migrations/191_agent_team_shared_rewards.sql backend/migrations/agent_team_shared_rewards_migration_test.go
git commit -m "feat(play): persist team reward settlements"
```

### Task 3: Membership Repository And Lifecycle

**Files:**
- Create: `backend/internal/repository/play_repo_team_test.go`
- Modify: `backend/internal/repository/play_repo_extended.go`
- Modify: `backend/internal/service/play_models.go`
- Modify: `backend/internal/service/play_extended.go`
- Modify: `backend/internal/service/play_errors.go`

- [ ] **Step 1: Write failing repository/service tests**

Cover active membership lookup, leave preserving the row, join after leave,
captain transfer, captain rejection while members remain, member removal, and
archiving a one-member team.

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository ./internal/service -run 'Team(Leave|Join|Transfer|Remove|Archive|Membership)' -count=1 -p 1`

Expected: FAIL because lifecycle methods and active-row filters do not exist.

- [ ] **Step 3: Implement transactional lifecycle operations**

Add repository methods `LeaveTeam`, `TransferTeamCaptain`, `RemoveTeamMember`,
`CountActiveTeamMembers`, and active-member row locks. Update every current team
query to require `left_at IS NULL` and every team lookup to require
`archived_at IS NULL`. In the same transaction as each create, join, leave,
transfer, remove, or archive mutation, append a typed row to `play_team_events`
with the authenticated actor and affected member; tests assert mutation rollback
when the event insert fails.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository ./internal/service -run 'Team(Leave|Join|Transfer|Remove|Archive|Membership)' -count=1 -p 1`

Run: `& 'C:\Program Files\Go\bin\go.exe' test -tags=integration ./internal/repository -run '^TestTeam' -count=1 -p 1`

Expected: both commands PASS. The integration command is required because it
covers concurrent create/join, typed events, rollback, and lifecycle
authorization against PostgreSQL.

- [ ] **Step 5: Commit lifecycle support**

```powershell
git add backend/internal/repository/play_repo_team_test.go backend/internal/repository/play_repo_team_integration_test.go backend/internal/repository/play_repo_extended.go backend/internal/service/play_models.go backend/internal/service/play_extended.go backend/internal/service/play_errors.go
git commit -m "feat(play): add team membership lifecycle"
```

### Task 4: Settlement Snapshot And Idempotent Payout

**Files:**
- Create: `backend/internal/repository/play_repo_team_rewards.go`
- Create: `backend/internal/repository/play_repo_team_rewards_test.go`
- Create: `backend/internal/service/play_team_settlement.go`
- Create: `backend/internal/service/play_team_settlement_test.go`
- Modify: `backend/internal/service/play_models.go`

- [ ] **Step 1: Write failing contribution and retry tests**

Tests must prove usage is included only inside membership intervals, use
`actual_cost`, snapshot the highest tier, cap at `$250`, and retry only failed
allocations after a simulated second-user payout failure.

```go
require.Equal(t, "30.00000000", settlement.TeamSpend.StringFixed(8))
require.Equal(t, "0.60000000", settlement.PoolAmount.StringFixed(8))
require.Equal(t, 1, ledger.Count("team_reward:7:2026-06:11"))
```

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository ./internal/service -run 'Team(Settlement|Contribution|Payout|Retry)' -count=1 -p 1`

Expected: FAIL because settlement persistence and payout do not exist.

- [ ] **Step 3: Implement snapshot creation and payout state machine**

Use `shopspring/decimal` through calculation and database scan boundaries.
Create immutable allocations before payout. Pay with the existing balance grant
transaction using `PlayRewardSourceTeamSharedReward = "team_shared_reward"` and
`team_reward:{team}:{month}:{user}`. Mark the settlement complete only when all
positive allocations are paid.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/repository ./internal/service -run 'Team(Settlement|Contribution|Payout|Retry)' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit settlement logic**

```powershell
git add backend/internal/repository/play_repo_team_rewards.go backend/internal/repository/play_repo_team_rewards_test.go backend/internal/service/play_team_settlement.go backend/internal/service/play_team_settlement_test.go backend/internal/service/play_models.go
git commit -m "feat(play): settle team rewards safely"
```

### Task 5: Runner And User/Admin APIs

**Files:**
- Create: `backend/internal/handler/play_handler_team_test.go`
- Create: `backend/internal/handler/admin/play_team_rewards_test.go`
- Modify: `backend/internal/service/play_growth_runner.go`
- Modify: `backend/internal/service/wire.go`
- Modify: `backend/internal/handler/play_handler_extended.go`
- Modify: `backend/internal/handler/admin/play_handler.go`
- Modify: `backend/internal/server/routes/play.go`
- Modify: `backend/internal/server/routes/admin.go`
- Regenerate: `backend/cmd/server/wire_gen.go`

- [ ] **Step 1: Write failing route and runner tests**

Cover `POST /play/teams/leave`, transfer, removal, settlement history, admin
configuration, admin retry, and runner behavior that settles only closed months.

- [ ] **Step 2: Run and verify RED**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler ./internal/handler/admin ./internal/service -run 'Team|PlayGrowthRunner' -count=1 -p 1`

Expected: FAIL because routes and runner settlement calls are absent.

- [ ] **Step 3: Add APIs and runner invocation**

Add explicit DTOs and domain-error mappings. Extend `PlayGrowthRunner` to accept
the existing `LeaderLockCache` and `*sql.DB`, create a UUID owner ID, and wrap
`SettleDueTeamRewardMonths(ctx, now)` with `tryAcquireSingletonLeaderLock` using
a dedicated team-settlement key and a TTL longer than one runner cycle. Keep the
unique team/month constraint and allocation idempotency keys as the second line
of defense. Settle only the immediately previous closed month, and skip it when
it is earlier than `play_team_shared_reward_start_month`, preventing retroactive
payouts for pre-deployment months. Admin retry accepts a settlement ID and only
processes unpaid allocations. Regenerate Wire after changing the runner provider
signature.

- [ ] **Step 4: Run and verify GREEN**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/handler ./internal/handler/admin ./internal/service ./internal/server -run 'Team|PlayGrowthRunner' -count=1 -p 1`

Expected: PASS.

- [ ] **Step 5: Commit APIs and scheduling**

```powershell
git add backend/internal/handler/play_handler_team_test.go backend/internal/handler/admin/play_team_rewards_test.go backend/internal/service/play_growth_runner.go backend/internal/service/wire.go backend/internal/handler/play_handler_extended.go backend/internal/handler/admin/play_handler.go backend/internal/server/routes/play.go backend/internal/server/routes/admin.go backend/cmd/server/wire_gen.go
git commit -m "feat(play): expose team rewards and lifecycle"
```

### Task 6: Team User Experience

**Files:**
- Create: `frontend/src/views/public/__tests__/AgentTeamView.spec.ts`
- Create: `frontend/src/components/admin/play/TeamRewardSettings.vue`
- Create: `frontend/src/components/admin/play/__tests__/TeamRewardSettings.spec.ts`
- Modify: `frontend/src/api/play.ts`
- Modify: `frontend/src/api/admin/play.ts`
- Modify: `frontend/src/views/public/AgentTeamView.vue`
- Modify: `frontend/src/views/user/PlayHubView.vue`
- Modify: `frontend/src/views/admin/SettingsView.vue`
- Modify: `frontend/src/i18n/locales/jisudeng-pages.zh.ts`
- Modify: `frontend/src/i18n/locales/jisudeng-pages.en.ts`

- [ ] **Step 1: Write failing frontend tests**

Mount a team with `$120` monthly spend and assert the 3% tier, estimated `$3.60`
pool, member spend shares, lifecycle controls, and settlement history. Assert
that failed allocations never render as paid.

- [ ] **Step 2: Run and verify RED**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/AgentTeamView.spec.ts src/components/admin/play/__tests__/TeamRewardSettings.spec.ts`

Expected: FAIL because the API types and controls do not exist.

- [ ] **Step 3: Implement truthful progress, lifecycle, and admin controls**

Replace token-based reward copy with actual-spend tiers while retaining Token as
an informational field. Add confirm dialogs for leave/remove/transfer, show the
monthly cap, and render allocation status from confirmed backend fields.

- [ ] **Step 4: Run tests, typecheck, and build**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/AgentTeamView.spec.ts src/components/admin/play/__tests__/TeamRewardSettings.spec.ts`

Run: `pnpm --dir frontend run typecheck`

Run: `pnpm --dir frontend run build`

Expected: PASS.

- [ ] **Step 5: Commit team UI**

```powershell
git add frontend/src/api/play.ts frontend/src/api/admin/play.ts frontend/src/views/public/AgentTeamView.vue frontend/src/views/user/PlayHubView.vue frontend/src/views/admin/SettingsView.vue frontend/src/views/public/__tests__/AgentTeamView.spec.ts frontend/src/components/admin/play/TeamRewardSettings.vue frontend/src/components/admin/play/__tests__/TeamRewardSettings.spec.ts frontend/src/i18n/locales/jisudeng-pages.zh.ts frontend/src/i18n/locales/jisudeng-pages.en.ts
git commit -m "feat(frontend): explain and manage team rewards"
```

### Task 7: Team Reward Verification

- [ ] **Step 1: Run the full Play protection suite**

Run: `& 'C:\Program Files\Go\bin\go.exe' test ./internal/service ./internal/repository ./internal/handler ./internal/server ./migrations -run 'Play|Team|Growth' -count=1 -p 1`

- [ ] **Step 2: Run frontend focused tests and build**

Run: `pnpm --dir frontend exec vitest run src/views/public/__tests__/AgentTeamView.spec.ts src/components/admin/play/__tests__/TeamRewardSettings.spec.ts`

Run: `pnpm --dir frontend run build`

- [ ] **Step 3: Verify obsolete payout path is unreachable**

Run: `rg -n "team_affiliate_bonus|AccrueBonusQuota\(ctx, captain" backend/internal frontend/src`

Expected: legacy migration/constants may remain, but no active team settlement path calls Affiliate quota.

- [ ] **Step 4: Commit any verification-only corrections**

Only commit corrections required by failed checks; do not create an empty commit.
