# Administrator check-in account search and continuity evidence

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/play/AdminCheckinRecords.vue",
    "frontend/src/api/admin/play.ts",
    "frontend/src/components/admin/play/__tests__/AdminCheckinRecords.spec.ts"
  ],
  "routes_or_surfaces": [
    "/admin/play-ops"
  ],
  "languages_and_themes": [
    "zh/light",
    "zh/dark",
    "en/light",
    "en/dark"
  ],
  "states": [
    "account search",
    "expanded daily evidence",
    "later makeup",
    "no previous record",
    "reward split",
    "loading",
    "disabled",
    "empty",
    "error/retry",
    "repair regression",
    "keyboard focus"
  ],
  "viewports": [
    "360x1000",
    "768x1000",
    "1280x1000",
    "1920x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-checkin-account-evidence/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-checkin-account-evidence/baseline-1280-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-12-checkin-account-evidence/updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-12-checkin-account-evidence/updated-360-dark.png"
  ],
  "commands": [
    "node /data/tmp/checkin-account-browser.cjs baseline # compiled component browser baseline and static prototype before implementation",
    "node /data/tmp/checkin-account-browser.cjs updated # Playwright compiled-component screenshot matrix",
    "make test",
    "make build",
    "./scripts/check-fork-integrity.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Native details receives focus and opens with Enter at each width/theme/locale."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "All component browser scenarios request reduced motion; no new animation."
    }
  },
  "residual_risks": [
    "Synthetic compiled-component screenshots are supplementary; user-local production acceptance remains required."
  ]
}
-->

## Scope

Improve the existing admin check-in report, without changing reward decisions, balances, sign-in history, routes or schema. Query email/username fragments (case insensitive, literal percent/underscore), preserve internal `user_id` API compatibility and repair targets. Numeric account text remains account text. Existing admin authorization applies to the additive fields.

## Baseline

Verified production base `641785fe1d70ca3cd7beeecb0c46b7e4221fc9ab` (#426). Existing report requires an ID and shows a counter, not its daily evidence. Adjacent coupon audience form and shared input/dialog patterns were inspected. Baseline capture uses the actual compiled component/CSS with synthetic data, no production login or identity.

## Prototype

The prototype PNG was saved before frontend implementation; the user's request authorizes the bounded existing-report change. Retain the existing filter/table/repair layout. Promote account email and username, show reward-time versus snapshot-time streak, expand date/record-time evidence in place, explain earlier gaps and later makeup. No new page or design pattern.

## Reuse Decision

Reuse existing `.input`, `.btn`, table scroll region, eligibility labels, TOTP repair dialog and native keyboard-accessible details. No shared component changes or new shell/navigation. Monetary amounts remain decimal strings and use the existing USD unit.

## State Coverage

Regression tests cover account query/reset, stable pagination, query failure/retry, no matching records and existing audited repair confirmation. New tests cover continuity across date/page filters, actual gaps, another user's intervening record, stale stored counters, later makeup/as-of fencing, earliest-known record and cash-ledger decomposition. An earlier unknown registration period is not called a missed day. Daily evidence marks records created after the chosen check-in, rather than retroactively claiming continuity at reward time. No historical data or financial writes.

## Viewport Coverage

Compiled Vue/Tailwind matrix: 360, 768, 1280, 1920 × zh/en × light/dark. Native details opens with keyboard Enter; reduced motion enabled, zero page overflow/runtime errors. The existing table retains its labeled horizontal scroll region. Summary reuses native hover/active/focus behavior; async success/loading/disabled/empty/error and repair states retain regression coverage. No separate 200% browser zoom assertion yet.

## Evidence

Baseline/prototype and two updated screenshots are linked in the manifest. The browser harness uses synthetic responses and never calls production or starts a development server. Full `make test` passed: backend tests and golangci-lint (0 issues), frontend lint/typecheck and 551 files / 4,277 tests; existing skips: 3 files / 8 tests. Focused PostgreSQL regression passed, and the original repository query fails the new account/evidence test. Production read-only aggregate independently confirms the disputed record has 30 actual dates, 30 reward-time consecutive days, zero later records, daily ledger 2 and milestone ledger 5 USD; ROLLBACK completed. `make test-backend-unit`, `make build`, production-equivalent `CGO_ENABLED=0 go build -tags=embed`, Fork integrity, design governance and documentation link checks all passed. Exact CI/deployment evidence belongs to the associated PR.

## Review

Specification review: account-first management and actual date continuity implemented within the existing report; earlier gaps, later records and reward decomposition are visible. No new reward policy or page. Code review: bind account text literally, preserve internal ID API compatibility, fence all date/ledger evidence by as-of, derive continuity across query filters/pages, retain exact decimal strings and untouched financial/repair logic. Both reviews passed after checking native details keyboard behavior and mobile table reachability.

## Residual Risk

User-local administrator acceptance is pending release. Server screenshots cannot establish real login permissions or whole-site reward correctness. Continuity is derived from recorded dates and creation times as of the query fence; it does not invent missing historical eligibility or replace frozen reward-rule evidence.
