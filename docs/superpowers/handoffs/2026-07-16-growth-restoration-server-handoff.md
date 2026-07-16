# Growth Restoration Server Handoff

Date: 2026-07-16

## Objective

Complete and verify three approved production corrections:

1. Restore the configurable seven-tier blindbox pool.
2. Replace the obsolete captain-only team affiliate reward with monthly shared
   balance rewards.
3. Replace fabricated public-home counters with authoritative statistics and
   reconcile public, admin, and user values after deployment.

Do not deploy until all implementation plans, reviews, and full verification
are complete.

## Repository State

- Repository: https://github.com/tqytwe/sub2api
- Target base: `origin/play/main`
- Working branch: `codex/growth-restoration`
- Base commit included: `e376e2950` (docs iframe and docs icon fix)
- Design commit: `3355db049`
- Plan commit: `af5aef00b`
- Current implementation commit: `1c3094ea4`

The branch was rebased onto the current `origin/play/main`. At handoff time the
working tree was clean.

## Approved Documents

Designs:

- `docs/superpowers/specs/2026-07-16-configurable-blindbox-pool-design.md`
- `docs/superpowers/specs/2026-07-16-agent-team-shared-rewards-design.md`
- `docs/superpowers/specs/2026-07-16-truthful-home-stats-design.md`

Implementation plans:

- `docs/superpowers/plans/2026-07-16-configurable-blindbox-pool.md`
- `docs/superpowers/plans/2026-07-16-agent-team-shared-rewards.md`
- `docs/superpowers/plans/2026-07-16-truthful-home-stats.md`

## Completed Work

- All three designs are approved.
- All three plans were self-reviewed and committed.
- A clean isolated worktree baseline passed:
  - `scripts/check-fork-integrity.sh`
  - backend server build with `CGO_ENABLED=0` and `-p 1`
  - frontend `vue-tsc -b`
  - frontend `vite build`
- Blindbox Task 1 was implemented with TDD in `1c3094ea4`:
  - approved default pool and validation;
  - runtime setting key and fallback;
  - pool fields on service models;
  - injectable crypto-random draw source;
  - focused tests.
- Blindbox Task 1 specification review returned `SPEC COMPLIANT`.

## Mandatory First Action

Blindbox Task 1 has NOT passed code-quality review. Fix these findings before
starting Blindbox Task 2:

1. Important: `backend/internal/service/play_blindbox_pool.go` compares the
   expected reward and RTP cap with exact `float64` ordering. A mathematically
   valid boundary such as `0.07 == 0.1 * 0.7` can be rejected. Add a failing
   exact-cap test first, then use robust decimal arithmetic or a justified
   scale-aware tolerance.
2. Important: invalid non-empty pool configuration falls back silently. Preserve
   the approved fallback, but emit an operational warning without logging raw
   configuration. Missing configuration is a normal fallback and must not warn.
   Make the parse/validation diagnostic testable.
3. Minor: test all seven reward tier boundaries through
   `PlayService.pickBlindboxReward` with injected draws. Also test random-source
   failure and out-of-range draws through that service path.

After fixing:

- run focused tests;
- commit the fix separately;
- rerun specification review;
- rerun code-quality review;
- proceed only when Critical and Important findings are zero.

## Remaining Execution Order

Use subagent-driven development. For every task, use a fresh implementer,
specification reviewer, and code-quality reviewer. The implementer must use TDD
and commit one task at a time.

1. Finish Blindbox Task 1 review fixes.
2. Blindbox Tasks 2 through 6.
3. Agent Team Tasks 1 through 7.
4. Truthful Home Statistics Tasks 1 through 5.
5. Final cross-feature review and full verification.
6. Push for review, deploy through the existing production workflow, then run
   database/API/UI acceptance.

Do not run independent implementation agents in parallel because the plans
share service models, settings, routes, admin UI, and generated Wire files.

## Critical Production Facts

- Production already contains `play_blindbox_pool_json`, but the old runtime
  ignores it.
- The approved pool costs USD 0.50 and has rewards:
  - USD 0.05 / 40%
  - USD 0.20 / 30%
  - USD 0.50 / 18%
  - USD 1 / 8%
  - USD 3 / 3%
  - USD 10 / 0.9%
  - USD 20 / 0.1%
- Team reward rules use monthly `actual_cost`: USD 20/2%, USD 100/3%,
  USD 500/4%, USD 2000/5%, capped at USD 250 and split by contribution.
- The obsolete team payout path can pass `source_user_id=0`, violate a foreign
  key, and still report payment after an earlier ledger transaction. The new
  system must not use affiliate quota or user ID zero.
- The public home page currently fabricates a 12,847,360 request baseline while
  the real API/database count is orders of magnitude lower.
- Channel monitoring traffic may legitimately create usage rows for a configured
  user/key. Acceptance must distinguish real monitor requests from statistics
  defects by matching timestamps, API keys, models, and request metadata.

## Server Bootstrap

For an existing checkout:

```bash
git fetch origin
git switch codex/growth-restoration || \
  git switch --track -c codex/growth-restoration origin/codex/growth-restoration
git pull --ff-only
git status --short --branch
git log -5 --oneline
```

Expected handoff HEAD after this document is committed and pushed will be
recorded in the final local handoff message. Do not continue if the branch is
dirty or the expected commit is absent.

Create an isolated server worktree using the
`superpowers:using-git-worktrees` skill, then install dependencies and run a
server baseline before editing.

## Server Codex Startup Prompt

```text
Continue the approved sub2api growth-restoration implementation from the remote
branch codex/growth-restoration.

First read completely:
- docs/superpowers/handoffs/2026-07-16-growth-restoration-server-handoff.md
- all three referenced design specs
- all three referenced implementation plans

Required workflow:
1. Use superpowers:using-git-worktrees.
2. Use superpowers:subagent-driven-development.
3. Use strict TDD: every production behavior needs a failing test observed
   before implementation.
4. For each plan task, use a fresh implementer, then a specification reviewer,
   then a code-quality reviewer. Fix and re-review all Critical and Important
   findings before moving on.
5. Preserve fork customizations and run scripts/check-fork-integrity.sh at major
   checkpoints.
6. Do not deploy until all three plans and full builds pass.

Immediate task:
Fix the three open Blindbox Task 1 quality-review findings listed in the handoff
document. Commit the fixes separately, rerun focused tests, specification
review, and code-quality review. Only then continue Blindbox Task 2 and the
remaining tasks in the documented order.

Production acceptance after deployment:
- capture read-only database truth at bounded timestamps;
- compare public, admin, and test-user APIs to SQL;
- inspect public, admin, and user pages in a browser;
- reconcile blindbox/team ledgers and balances separately from usage metrics;
- identify channel-monitor requests by timestamp/key/model metadata;
- never persist login credentials in Git or logs.

Continue autonomously until implementation and verification are complete, unless
a real blocker requires user authority. Report exact commits, test commands,
deployment result, and database/API/UI reconciliation evidence.
```

## Acceptance Guardrails

- Do not edit already-applied migration files. Add forward migrations only.
- Do not expose raw settings or credentials in logs or public APIs.
- Do not claim a reward is paid before the balance and allocation transaction is
  confirmed.
- Do not manufacture statistics when data is missing.
- Do not persist production or test-user passwords in repository files.
- Do not mutate production statistics during reconciliation.
