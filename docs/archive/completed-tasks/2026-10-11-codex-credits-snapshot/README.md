# Codex credits snapshot synchronization — isolated evidence

Status: review; source frozen at `b7faef8f1f896b5113dee07278ce50e336d9ff99`; no PR, deployment or production acceptance is claimed here.

Business files: `frontend/src/components/account/OpenAIQuotaResetCell.vue` and its existing `OpenAIQuotaResetCell.spark_shadow.spec.ts`.
Baseline: `8364b462feda15f3bf8f3bb2029314d2193b9dba`.
Branch/worktree: `fix/codex-credits-snapshot-follow`, `/workspace/quota-frontend`.
Frozen component SHA256: `64e7ca2468ac90f895f6178ff6c436369148249b719513ced7ca49b1adf5a881`.
Frozen spec SHA256: `faa53bbfbc7cde236de398e8c10b0984dab89dadd49f8f4f9ae1e469c8d3ce11`.

## Contract and review

Same-ID persisted snapshots update the existing balance display. Freshness ordering includes unknown snapshots. Semantically repeated snapshots do not invalidate an in-flight newer query. Old query/prop results cannot roll back newer data. Generation guards reject responses after switching account, ABA reuse or unmount, including stale loading finalizers. No automatic query, force loop, reset, content generation or billed probe is introduced.

Independent agent `frontend_review` completed specification review followed by quality review. Initial review found repeat-snapshot invalidation and lost unknown timestamps; both received new RED cases before correction. Final review passed scoped code, retaining visual evidence as a separate gate at that time. Independent visual follow-up review also passed. The parent accepted the explicitly authorized evidence follow-up while retaining disclosure of late prototype chronology and inherited active/reduced-motion limitations; no additional approval is pending. No shared handoff snapshot is modified by this task.

## Test evidence

| Evidence | Result |
| --- | --- |
| [Initial RED](./evidence/quota-frontend-red.log.gz) | 7 failed, 30 passed, before implementation. |
| [Nested reference RED](./evidence/quota-frontend-nested-red.log.gz) | 1 failed, 40 passed. |
| [Independent-review RED](./evidence/quota-frontend-review-red.log.gz) | 3 failed, 41 passed. |
| [Older query RED](./evidence/quota-frontend-old-query-red.log.gz) | 1 failed, 44 passed. |
| [Final targeted GREEN](./evidence/quota-frontend-green.log.gz) | 46 passed; includes inverse old-unknown ordering. |
| Targeted ESLint and diff check | Exit 0. |
| `vue-tsc --noEmit` | Exit 0 on the identical frozen component; subsequent test-only additions are excluded by repository tsconfig. Evidence is reused, not represented as another full run. |
| [Design gate after visual follow-up](./evidence/quota-frontend-design-after.log.gz) | Passed against baseline 8364b462; no gate changes or exceptions. |
| [Documentation links](./evidence/quota-frontend-doclinks.log.gz) | Passed, 289 files. |
| [Original design gate](./evidence/quota-frontend-design.log.gz) | Failed missing visual evidence; retained after follow-up. |
| [Active-state observation](./evidence/interaction-active-observation-failed.log.gz) | Native `:active` false under harness; not reclassified as passed. |

Full local gates are coordinated sequentially with the parent. The initial `make test` passed all default Go packages (service 143.539s), then failed because another task held the global golangci runner lock. [The original failure](./evidence/make-test-initial-lock-failure.log.gz) is retained. Default Go tests were not repeated; [serial lint recovery](./evidence/quota-frontend-backend-lint.log.gz) completed with zero issues using `--allow-serial-runners` (not parallel runners).

The full frontend command passed lint/typecheck, then reported **503 test files / 3841 tests passed, one failure, three files / eight tests skipped**. The unchanged `RequestLedgerView.session.http.spec.ts:118` asserted a server response was destroyed after a fixed 30ms wait and observed false. [Original full-run failure](./evidence/quota-frontend-make-test-frontend-initial-failure.log.gz) is retained; this run is not relabeled a full-suite pass. Without modifying that suite or its source, four separate single-worker runs each passed all 26 tests: [run 1](./evidence/quota-frontend-ledger-retry.log.gz), [run 2](./evidence/quota-frontend-ledger-retry-1.log.gz), [run 3](./evidence/quota-frontend-ledger-retry-2.log.gz), [run 4](./evidence/quota-frontend-ledger-retry-3.log.gz). Combined coverage reuses the first run's unchanged passing files and these targeted recoveries. Load-related timing is a hypothesis, not an established root cause; the original timing-sensitive assertion remains a CI risk outside this repair's scope.

[Tagged unit](./evidence/quota-frontend-unit.log.gz) passed (service 206.986s). [Full backend/frontend build](./evidence/quota-frontend-build.log.gz) passed. [Fork integrity](./evidence/quota-frontend-fork.log.gz) passed. All were run sequentially with `GOMAXPROCS=2`, `GOFLAGS=-p=2`, `GOMEMLIMIT=2GiB`, shared public Go package cache and a task-specific lint cache. Public package cache reuse does not substitute the other branch's backend gate results; these are this worktree's own runs. Final PR CI remains pending.

## Real rendered artifacts

See [visual review](../../../visual-reviews/2026-10-11-codex-credits-snapshot.md) for the complete manifest, chronology and limitations.

Capture uses actual baseline/current SFCs, child components, repository Tailwind/CSS and translations. The synthetic fixture is production-bundled with Vite, then served only by Playwright request fulfillment from disk. Real API exports are replaced by local stubs at build time; reset throws. No dev server or production requests are used.

```bash
PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs \
  node docs/archive/completed-tasks/2026-10-11-codex-credits-snapshot/capture.mjs \
  /workspace/quota-frontend /workspace/quota-frontend-baseline

PLAYWRIGHT_MODULE=/path/to/playwright/index.mjs \
  node docs/archive/completed-tasks/2026-10-11-codex-credits-snapshot/check-interactions.mjs \
  /tmp/quota-capture-<run>/after/dist docs/visual-reviews/assets/codex-credits-snapshot
```

Requires installed frontend dependencies, Playwright and system Chromium. Baseline worktree uses the same locked dependency installation. Chromium is `/usr/bin/chromium`; the main script supports `CHROMIUM_PATH`. The prototype is baseline component state rendering captured during the authorized evidence follow-up; it is not evidence of pre-code compliance. Existing reduced-motion spinner behavior and unconfirmed native active state are explicitly retained risks.

## Remaining gates

1. Parent visual/engineering-artifact review passed; late prototype chronology explicitly accepted as disclosed evidence follow-up.
2. Local full coverage is complete through the explicitly documented split recovery above; original whole-run failures remain failures. Unit/build/Fork integrity passed.
3. Source freeze committed; ordinary merge of main `16a11abd29b1f37f3be28820b948f2d8400e4e70` preserves both source hashes and main backend/CI/scripts exactly: [merge proof](./merge-proof.json). Post-merge [quota/routing/session intersection](./evidence/quota-frontend-merge-targeted.log.gz) passed 4 files / 116 tests. [Design](./evidence/quota-frontend-merge-design.log.gz) and [documentation links](./evidence/quota-frontend-merge-doclinks.log.gz) passed. Prior local gates and visual evidence are reused where bytes are unchanged; inherited identifier backend changes rely on main evidence, not a claimed rerun. Coordinate draft PR targeting `play/main`; final-head CI pending.
4. Merge/deploy and user-local-browser acceptance remain outside this agent's authority.
