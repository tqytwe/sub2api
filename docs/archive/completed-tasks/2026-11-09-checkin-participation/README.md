# Unqualified check-ins record participation without extra energy

Task ID: `PLAY-CHECKIN-PARTICIPATION-20261109`. Owner: current task submitter.
Baseline: `play/main@4cce2a54d33c8063100cf8aea2f77e46964714b7` (PR #422).
Branch: `fix/checkin-participation-only-20261109`.
Backend state: server verification passed for this scope. Product state: server verified, pending PR/deployment and final local acceptance.

## Authorization and boundaries

The user explicitly selected: cancel the extra growth energy; users who do not meet qualification only record check-in without an award; qualified future check-ins still use the existing pool. This is independent of the preceding governance blank-page repair, already deployed by #422.

Remove the fixed one-point `play_growth_energy_ledger` check-in grant. Keep the daily unique claim, streak, frozen milestone configuration and immutable qualification snapshot in their existing transaction. Unqualified seventh-day check-ins issue neither extra energy nor cash milestones. Qualifying after today's check-in does not grant a second claim or retroactively pay its milestones; future qualified check-ins use the configured coupon/redeem-code/balance pool. Historical snapshots and ledger entries are unchanged. Quiz/blindbox fallback and configured farm task progress/energy calculation are unchanged; this scope removes the additional one-point check-in grant.

## Contract and interface

The existing `growth_energy_enabled` activity flag is now false for check-in and takes precedence over the shared qualification `reward_mode` bucket. The shared v1 eligibility classification still describes qualification evidence used by multiple activities, not proof of a check-in award. No new field, enum migration, schema change or old-snapshot rewrite is needed. Result `reward_type=none`, all money fields zero and no growth-energy grant field means participation only. Qualification reasons and progress remain available. Reward/milestone/boost promises are suppressed for current unqualified status; future qualified behavior is unchanged.

User CheckInView and PlayHubView show participation-only copy, keep the check-in action and acknowledge the recorded streak without a zero-dollar or energy award toast. The existing admin check-in tab displays actual zero-energy participation separately from historical positive-energy grants. Its filter is labelled reward eligibility because it queries immutable qualification evidence. Existing coupon menu/check-in-pool configuration, repair permissions and accounting remain unchanged.

## TDD and targeted verification

RED: service and real HTTP handler still grant one energy point and the status flag is true; user check-in/hub promise energy despite an explicit false flag; admin labels zero-energy participation as energy. GREEN: all targeted frontend and service/handler tests pass. Additional lifecycle regression covers unqualified seventh day, same-day qualification duplicate rejection, next-day qualified pool reward and preserved snapshot history.

`./scripts/test-checkin-contract.sh` passed against disposable real PostgreSQL and Redis, with live Vue/HTTP tests: inactive/unverified/too-new participation grants zero energy and zero cash, qualified balance pool remains 0.5, seventh-day/makeup remain 1.5, duplicated concurrent requests leave a single check-in/snapshot and no double grant. Refresh/read-back, stale-tab conflict, permission failures and feature switches are included. Read-only production was not used as a write target.

## Reviews

Specification review: passed for the stated check-in-only behavior. Existing entries retained; actual no-award results and user/admin copy agree; qualified pool and milestones preserved; history, farm, quiz and blindbox are outside this grant removal. No unrelated feature or migration was added.

Code quality review: passed for this scope. The daily unique claim and transaction remain before awards; no energy ledger insert is reachable for unqualified check-in; immutable snapshots stay linked. Explicit false overrides shared qualification mode; older true/missing flags retain compatible behavior, including unrelated quiz energy. Admin labels use actual positive ledger energy to preserve history rather than recategorizing all snapshots. Existing step-up repair cannot award unqualified milestones because it checks the original snapshot. No secret or production data in fixtures. [Visual review](../../../visual-reviews/2026-11-09-checkin-participation.md).

## Evidence and release status

- UI: CheckInView, PlayHubView, AdminCheckinRecords and existing zh/en locale resources.
- API: existing check-in/status/hub and admin records; no route/permission changes.
- Schema: not applicable. Existing check-in/snapshot/ledger schema is retained.
- Tests: relevant service/handler, user/hub/admin components and real PostgreSQL/HTTP/Vue contract passed.
- Full gates passed: `make test` (backend default tests/golangci-lint; frontend lint/typecheck and 547 files / 4,253 tests passed, baseline 3 files / 8 tests skipped), `make test-backend-unit`, `make build`, Fork integrity, design governance, document links, handoff checker and nine handoff tests. No check was relaxed. The historical milestone/repair integration regression also passed.
- PR/head/CI/merge: pending.
- Exact deployment SHA, completion time, health, asset and production aggregate: pending.
- Final local visitor/user/administrator browser acceptance: pending; no connected local-browser tool. Isolated screenshots are development evidence only.
- Rollback trigger: altered eligibility, duplicate grants, broken qualified pool, unexpected schema/data write or deployment SHA/health mismatch. Use a reviewed source revert, never delete rewards or mutate history.
