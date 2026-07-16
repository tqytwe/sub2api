# Growth Regression Rollback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **2026-07-16 delivery override:** Server-side browser automation is supplementary
> technical evidence only. Final production acceptance must follow
> `docs/DELIVERY_WORKFLOW.md` and be completed by the user on their local
> workstation as guest, regular user, and administrator. This override replaces
> any older browser-control completion wording below.

**Goal:** Restore the documented Growth / Play production behavior, compensate the database side effect of migration 188, deploy `play/main`, and verify all preserved recent features with the supplied normal-user account.

**Architecture:** Reverse only commits `dc102f78` and `ebce8edf`, leaving every earlier Image Studio, pricing, billing, CI, and fork customization commit intact. Add one forward-only, non-destructive migration that repairs the affected default VIP JSON while preserving custom tier structures and all migration-188 tables. Use local tests as the first gate, production public endpoints as the second gate, and authenticated browser acceptance as the final gate.

**Tech Stack:** Git, Go 1.26, PostgreSQL migrations, Vue 3, TypeScript, Vitest, pnpm, GitHub Actions, Zeabur deployment, browser-assisted production observation.

---

### Task 1: Reverse The Two Regression Commits

**Files:**
- Restore/delete: exactly the files changed by `dc102f78` and `ebce8edf`
- Preserve: `docs/superpowers/specs/2026-07-15-growth-regression-rollback-design.md`

- [ ] **Step 1: Record the clean baseline**

Run:

```bash
git status --short --branch
git diff --name-status f091ab60..dc102f78
```

Expected: the branch is clean and the second command lists the 50-file growth-world change set.

- [ ] **Step 2: Reverse both commits without committing**

Run:

```bash
git revert --no-commit dc102f78
git revert --no-commit ebce8edf
```

Expected: no conflicts; the working tree restores the code state from `f091ab60` while retaining the design commit.

- [ ] **Step 3: Verify rollback scope**

Run:

```bash
git diff --stat
git diff --name-status
git diff --check
```

Expected: only the two target commits are reversed; no Image Studio, pricing, billing, CI, or fork-integrity file changed unless it was part of those commits.

- [ ] **Step 4: Run the restored focused tests**

Run:

```bash
cd frontend
pnpm exec vitest run src/composables/__tests__/homeLiveStats.spec.ts
cd ../backend
go test ./internal/service -run 'Test(GetVIPTier|ParsePlayVIP|Play)' -count=1
```

Expected: the restored home statistics tests pass and Play service tests compile and pass.

- [ ] **Step 5: Commit the rollback**

```bash
git add -A
git commit -m "revert: restore documented growth and home behavior"
```

### Task 2: Add A Forward-Only VIP Compensation Migration

**Files:**
- Create: `backend/migrations/growth_world_rollback_migration_test.go`
- Create: `backend/migrations/189_restore_growth_rollback_defaults.sql`
- Modify: `docs/FORK_CUSTOMIZATIONS.md`
- Modify: `scripts/check-fork-integrity.sh`

- [ ] **Step 1: Write the failing migration contract test**

Create `backend/migrations/growth_world_rollback_migration_test.go`:

```go
package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrowthWorldRollbackMigrationIsForwardOnlyAndGuarded(t *testing.T) {
	content, err := FS.ReadFile("189_restore_growth_rollback_defaults.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "blindbox_pool_upgrade")
	require.Contains(t, sql, `value::jsonb @> '[{"tier":2,"label":"V2","min_recharge":200},{"tier":3,"label":"V3","min_recharge":500}]'::jsonb`)
	require.Contains(t, sql, "WITH ORDINALITY")
	require.NotContains(t, strings.ToUpper(sql), "DROP TABLE")
	require.NotContains(t, strings.ToUpper(sql), "DELETE FROM")
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```bash
cd backend
go test ./migrations -run TestGrowthWorldRollbackMigrationIsForwardOnlyAndGuarded -count=1
```

Expected: FAIL because `189_restore_growth_rollback_defaults.sql` does not exist.

- [ ] **Step 3: Add the guarded compensating migration**

Create `backend/migrations/189_restore_growth_rollback_defaults.sql`:

```sql
-- Restore the default V2/V3 blind-box perk removed by migration 188.
-- Existing growth-world tables and custom VIP tier structures are preserved.
UPDATE settings
SET value = (
    SELECT jsonb_agg(
        CASE
            WHEN (tier->>'tier')::int IN (2, 3)
              AND jsonb_typeof(tier->'perks') = 'array'
              AND NOT (tier->'perks' ? 'blindbox_pool_upgrade')
            THEN jsonb_set(
                tier,
                '{perks}',
                tier->'perks' || '["blindbox_pool_upgrade"]'::jsonb
            )
            ELSE tier
        END
        ORDER BY tier_order
    )::text
    FROM jsonb_array_elements(settings.value::jsonb)
         WITH ORDINALITY AS tiers(tier, tier_order)
)
WHERE key = 'play_vip_tiers'
  AND jsonb_typeof(value::jsonb) = 'array'
  AND value::jsonb @> '[{"tier":2,"label":"V2","min_recharge":200},{"tier":3,"label":"V3","min_recharge":500}]'::jsonb;
```

- [ ] **Step 4: Register the migration in fork integrity**

Append `189_restore_growth_rollback_defaults.sql` to the migration list in both `docs/FORK_CUSTOMIZATIONS.md` and the `MIGRATIONS` array in `scripts/check-fork-integrity.sh`.

- [ ] **Step 5: Run migration and integrity tests**

Run:

```bash
cd backend
go test ./migrations -run TestGrowthWorldRollbackMigrationIsForwardOnlyAndGuarded -count=1
cd ..
bash scripts/check-fork-integrity.sh
```

Expected: PASS; the integrity script reports no missing migration or canonical-list mismatch.

- [ ] **Step 6: Commit the compensation**

```bash
git add backend/migrations/growth_world_rollback_migration_test.go \
  backend/migrations/189_restore_growth_rollback_defaults.sql \
  docs/FORK_CUSTOMIZATIONS.md scripts/check-fork-integrity.sh
git commit -m "fix: compensate growth rollback settings"
```

### Task 3: Run The Complete Local Verification Gate

**Files:**
- Verify only; modify code only if a test identifies an in-scope regression

- [ ] **Step 1: Verify backend unit tests**

Run:

```bash
cd backend
go test ./internal/service ./internal/handler ./internal/server/routes ./migrations -count=1
```

Expected: PASS.

- [ ] **Step 2: Verify frontend statistics and critical user flows**

Run:

```bash
cd frontend
pnpm exec vitest run src/composables/__tests__/homeLiveStats.spec.ts
pnpm run typecheck
pnpm run build
```

Expected: all commands PASS and the production bundle is generated.

- [ ] **Step 3: Verify fork integrity and whitespace**

Run:

```bash
bash scripts/check-fork-integrity.sh
git diff --check origin/play/main...HEAD
git status --short --branch
```

Expected: integrity passes, no whitespace errors, and the worktree is clean.

- [ ] **Step 4: Confirm commit scope**

Run:

```bash
git log --oneline origin/play/main..HEAD
git diff --stat origin/play/main...HEAD
```

Expected: one design commit, one rollback commit, and one compensation commit; no unrelated source changes.

### Task 4: Integrate And Deploy Through `play/main`

**Files:**
- Git history only

- [ ] **Step 1: Refresh remote state**

Run:

```bash
git fetch origin
git status --short --branch
git log --oneline --decorate -5 origin/play/main
```

Expected: the repair branch is clean and based on the current remote deployment branch.

- [ ] **Step 2: Fast-forward `play/main` to the verified repair**

Run:

```bash
git switch play/main
git merge --ff-only codex/rollback-growth-regressions
```

Expected: fast-forward succeeds without a merge commit.

- [ ] **Step 3: Push the deployment branch**

Run:

```bash
bash scripts/push-github-and-deploy.sh play/main
```

Expected: `origin/play/main` advances to the verified repair commit and Zeabur starts a deployment.

- [ ] **Step 4: Wait for public deployment health**

Poll:

```bash
curl -fsS https://www.jisudeng.com/health
curl -fsS https://www.jisudeng.com/api/v1/public/home-stats
```

Expected: health returns `{"status":"ok"}` and home statistics return the documented `total_requests`, `availability_pct`, `avg_ttft_ms`, and `has_live_data` fields rather than `snapshot_id:...:unavailable`.

### Task 5: Perform Local-Workstation Production Acceptance

**Files:**
- Create after verification: `docs/superpowers/specs/2026-07-15-production-acceptance.md`

- [ ] **Step 1: Open production and log in**

After deployment health is confirmed, the user opens
`https://www.jisudeng.com/login` in their local computer browser, enters the
credentials manually, and verifies that login reaches the normal user area.
Repeat the applicable checks as an administrator, and inspect public behavior
as a guest. Do not expose passwords, tokens, or cookies in screenshots, logs,
source files, command output, or the acceptance report.

- [ ] **Step 2: Verify the home and navigation contracts**

Check the homepage statistics and the documented Growth / Play sidebar entries. Confirm no admin-only growth configuration entry appears for the normal user.

- [ ] **Step 3: Verify Play Hub and child pages read-only**

Visit `/play`, `/check-in`, `/arena`, `/blindbox`, `/quiz-quest`, and `/agent-team`. Record whether each page loads its original content and state. Do not check in, open a blindbox, submit a quiz, or change team membership.

- [ ] **Step 4: Verify preserved earlier features**

Visit `/image-studio` and `/models`. Confirm template previews, controls, model/group visibility, prices, and existing works load without blank states. Do not generate images or spend balance.

- [ ] **Step 5: Write the acceptance matrix**

Create `docs/superpowers/specs/2026-07-15-production-acceptance.md` with columns:

```markdown
| Requirement | Active source | Production evidence | Result | Follow-up |
|---|---|---|---|---|
```

Every item from Tasks 4 and 5 receives `PASS` or `FAIL` with concrete observed evidence.

### Task 6: Close Remaining Verified Gaps

**Files:**
- Modify only files directly tied to a `FAIL` row in the production acceptance matrix
- Add a focused automated test beside each repaired component

- [ ] **Step 1: Select one failing acceptance row**

State one hypothesis linking the requirement, observed production behavior, and suspected component. Do not combine independent failures.

- [ ] **Step 2: Reproduce with a failing automated test**

Add the smallest test at the owning backend or frontend module and run it to confirm FAIL.

- [ ] **Step 3: Implement the minimal fix**

Change only the owning component and rerun the focused test until PASS.

- [ ] **Step 4: Run the complete local verification gate**

Repeat Task 3, commit the fix, fast-forward `play/main`, push, wait for deployment health, and repeat the affected browser acceptance row.

- [ ] **Step 5: Finish only when every in-scope row passes**

Update the acceptance matrix with final production evidence. Stop feature development if a remaining action would spend balance, change team state, or require new product authority; report that exact action instead of performing it.
