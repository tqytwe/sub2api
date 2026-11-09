# Invitation reconciliation correction

Baseline: `origin/play/main@22596af39e33368804a96614d680ddbee1cf2282`.
Branch: `fix/affiliate-cohort-reconciliation-20261109`.
Worktree: `/data/projects/sub2api-affiliate-reconciliation-20261109`.
Scope: repair evidence-backed historical paid invitations, separate registration/binding/qualification statistics, retain top-ten privacy and live refresh. No wallet, rebate policy, farm, or activity-rule changes. Related customization: FORK-PLAY-003 and its invitation operations/privacy behavior in FORK_CUSTOMIZATIONS.md.

## Production investigation — 2026-11-09 01:53–01:59 UTC

The user supplied local logged-in browser evidence: November monthly binding and paid counts both zero, conversion zero, pending 166, empty user monthly leaderboard. This is pre-fix administrator-as-user evidence, not final ordinary-user acceptance.

Zeabur project `6a51de14c2881a93656fa4c5`, environment `6a51de14104975fcb46761c3`, PostgreSQL service `6a51e192c2881a93656fa67f`. Credentials stayed in the existing CLI configuration and service environment. All queries ran via service exec in explicit read-only transactions, 7-second statement / 2-second lock limits, ended with ROLLBACK, and returned only aggregates. The first attempted SET used malformed quoting and failed before BEGIN executed; its psql process exited. Subsequent transactions completed and rolled back.

- 406 ordinary invitation relationships, all missing historical binding time; no missing paid-fact rows.
- Four existing relationships also have verified campaign-attribution evidence. They are deduplicated with ordinary relationships, not added as four extra invitees.
- Six invitees registered in November (Shanghai boundary `[2026-10-31T16:00:00Z, 2026-11-30T16:00:00Z)`): two on November 2, two on November 3, one each on November 5/7.
- Four November first payers, belonging to three inviters, have active accounts, no applicable high-risk cases, positive verified net payment, a unique first-order rebate to the same inviter, and matching AFFILIATE_REBATE_APPLIED audit amounts. No earlier pending payment.
- All four were incorrectly pending historical_unknown and net_paid=0 because migration 350 required bound_at to compute net payment and did not inspect existing first-order rebate proof. Neither frontend polling nor month formatting caused the empty ranking.
- 92 historical people have first-order rebate links (amount, audit and uniqueness checked by candidate query); ten are currently excluded by risk or disabled inviter status. State filtering remains independent of financial evidence.
- Historical ordinary relationships include 135 disabled relationships. Pending 166 is a qualification bucket after disabled/risk precedence, not a count of all raw relationships.

## Fix and compatibility

Migration 352 adds safe JSON audit amount matching, requires exactly one positive regular rebate on the verified lifetime-first order, a matching actual payment-order owner, identical invitee/inviter, and matching completed rebate audit. Historical conflict/earlier pending payment/removal of first-payment evidence remain pending/excluded. The proof establishes attribution at the first payment, never a fabricated binding timestamp. Lifetime-first month remains fixed across refund and repayment. Existing rows are reprojected idempotently; ledger and wallet amounts are never changed. Ledger/audit mutation triggers refresh the affected user's fact after payment fulfillment; leaderboard polls still read the materialized facts and current risk state.

The additive `reconciliation` API object exposes raw deduplicated relations, selected-month registrations/current effective registration cohort, known/unknown binding dates, disjoint qualification buckets, missing projections and pending payment clues. The latter clues overlap state buckets. The user endpoint keeps authenticated ownership; administrator scope is permission protected. Existing summary fields remain; conversion explicitly uses registration cohort and bound-date count is relabeled confirmed bindings.

Risk/rollback: migration replaces the projection function and installs per-user read-model triggers; this adds per-mutation SQL work and advisory locking. No new global scan runs on polls. Historical SQL bytes remain unchanged. If migration or projection health fails, stop acceptance and review a source revert; do not guess binding dates or change funds. Indexes on order IDs/user ledger lookups already exist. Migration updates only the honor fact table, not source payment/financial rows.

## TDD and reviews

RED: repository test failed because reconciliation was absent; registration-cohort conversion test failed (1 vs expected 1/3). Frontend tests failed for missing reconciliation, tab-return refresh and empty-ranking invitation link. Historical migration regression failed because migration 352 did not exist; malformed audit test then failed because a prefix numeric regex accepted invalid JSON.
GREEN: focused PostgreSQL migration/repository suites passed; focused frontend suites passed (37 tests: administrator overview, user affiliate, refresh scheduler, locale completeness). Repository test verifies missing projections and Shanghai midnight boundaries. User test starts with an empty ranking and displays a newly verified payer after the next five-second poll. Final `make test` passed: backend default suite, golangci-lint (0 issues), frontend lint/typecheck, 546 passing frontend files / 4,244 passing tests, 3 files / 8 tests skipped as in baseline.

Specification review: relation dates separate from paid qualifications; recover only provable first payment; top ten and masking unchanged; unknown history remains pending; ordinary rebate balances untouched. Specification review passed for the implemented scope. Registration cohort conversion now uses actual user creation time, while binding counts and paid-month ranking keep their independent definitions.
Code quality review passed after replacing numeric-prefix regex matching with safe full JSON parsing; invalid audit bodies cannot establish attribution. First-payment uniqueness, order owner, inviter/invitee identity, conflict, earlier pending payments, refund/re-payment, audit removal and source ledger immutability are covered. Existing per-invitee advisory lock and first-paid lock are retained. Permission ownership remains enforced by the existing handler; no endpoint or permission is added. Full-suite gates passed; PR CI, exact production deployment and post-fix local browser acceptance remain pending.

## Delivery gates

Commit / PR: pending.
Focused PostgreSQL migration/repository tests passed; 37 focused frontend tests passed. `make test-backend-unit` passed. `make build` passed (backend binary, locale validation, Vue type build, Vite production build). `make test` and `./scripts/check-fork-integrity.sh` passed. Frontend full suite: 546 files / 4,244 tests passed, 3 files / 8 tests skipped. An initial overly parallel validation attempt exhausted server memory: lint was killed by the OOM killer and unrelated timed network/build tests timed out. Those runs were rejected; validation was restarted with limited Go package/process concurrency. No test thresholds or unrelated business code were changed.
Exact deployed SHA / Zeabur deployment / health: pending.
Post-fix local browser guest/user/administrator acceptance: pending; cannot be substituted with server browser or static boards.
