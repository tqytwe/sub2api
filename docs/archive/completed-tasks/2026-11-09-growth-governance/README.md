# Growth governance tab and check-in energy investigation

Task ID: `PLAY-GOVERNANCE-20261109`. Owner: current task submitter.
Baseline: `origin/play/main@010b5f5e764a92bb1f6c5bf3810aa9a96e3990dd` (PR #421).
Branch: `fix/checkin-reward-governance-20261109`.
Backend state: verified for this readiness JSON scope; product state: verified in server development, pending deployment and local product acceptance.

## Scope and findings

The existing check-in qualification fallback awards one growth-energy point when a user lacks reward eligibility. This branch bypasses the configured check-in coupon pool, stores a qualification snapshot and appends the growth-energy ledger. It is independent of cash and farm energy. Production requires qualification, but does not require legacy governance approval. Reward configuration remains in the existing coupon menu's check-in-pool tab.

Read-only production aggregation on 2026-11-09 about 18:44 Asia/Shanghai: seven check-ins used energy mode with `no_recent_activity`; thirteen used redeemable mode. Seven check-in growth-energy ledger entries total seven points. One check-in pool is published, eleven retired; no governance approvals. The read-only transaction explicitly rolled back; no production records were modified. The user decision on removing or retaining this fallback is pending; this patch changes no reward issuance or qualification rule and does not alter historical awards.

The governance tab crashes when readiness has no blockers: Go serializes its nil slice as `null`, while the template reads `.length`. The page's existing tests omitted the readiness endpoint and missed this real payload. Also, the independent readiness section was accidentally nested in the conditional cohort section and could disappear when cohort evidence was absent.

## Implementation and TDD

RED: component test reproduces `Cannot read properties of null (reading 'length')`; absent-cohort test cannot find the status section. Backend JSON regression sees `blocking_reasons:null` instead of an empty list. GREEN: initialize blocking reasons to an empty slice; tolerate older servers' null in the template; move readiness outside the cohort condition; require the actual API call instead of silently omitting it. Related component, route and API tests: 38 passed; targeted service tests passed. Error/retry and blocked/nonempty reasons are included.

UI/API association: `GrowthGovernanceOperations.vue` → existing `/admin/play/growth/reward-readiness` → `PlayService.GetRewardReadiness`. Schema: not applicable; no migration or new persistence. Existing reward pools, qualification snapshots and energy ledger are read-only investigation inputs. API field correction: no-blocker responses now contain `[]`; old clients already expect an array; new UI tolerates older null payloads. Permission, routes, currencies, pool probabilities, legacy approval actions and issuance transactions are unchanged. Rollback: reviewed source revert if the tab or readiness contract regresses; no data rollback.

## Reviews

Specification review passed for this patch: reproduce and repair blank governance tab, preserve the existing pool entry, explain energy source, and keep the pending reward-policy choice explicit. No additional eligibility, budget or approval gate is added. An absent cohort disables approval without hiding independent reward diagnostics.

Code quality review passed for this patch: readonly readiness paths only; nonempty reason codes preserved; new/old server compatibility; no authorization or financial write changes; refresh generations and error/retry behavior retained. Regression tests exercise the real endpoint instead of a missing optional mock. The visual fixture uses synthetic records, no credentials or production data. [Visual review](../../../visual-reviews/2026-11-09-growth-governance.md).

## Verification and release

- Targeted tests: 38 frontend tests; related backend service tests passed.
- Complete server gates passed: `make test` (backend tests and golangci-lint; frontend lint/typecheck and 547 files / 4,252 tests passed, 3 files / 8 tests skipped as in baseline), `make test-backend-unit`, `make build`, `./scripts/check-fork-integrity.sh`, `pnpm design:check`, document links and nine handoff tests. No gate was skipped or relaxed.
- PR/head/CI/merge: pending.
- Exact production deployment SHA, finished timestamp and health: pending.
- Local browser visitor/user/administrator acceptance: pending. No local browser tool is connected to this thread; isolated server Chromium fixtures are development evidence only.
- Reward-policy decision: pending user reply; existing policy retained in this patch.

The archived component-browser-check.cjs generates baseline, prototype and updated images without a server. It needs installed project frontend dependencies, Playwright and Chromium; environment paths reflect this development machine. It stubs API data, translation interpolation and dialogs/icons and cannot prove authenticated production behavior.
