# Visual review: participation-only unqualified check-ins

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/user/CheckInView.vue",
    "frontend/src/views/user/PlayHubView.vue",
    "frontend/src/components/admin/play/AdminCheckinRecords.vue",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": [
    "/check-in",
    "/play",
    "/admin/play check-in tab"
  ],
  "languages_and_themes": [
    "zh light/dark; en locale keys and tests"
  ],
  "states": [
    "default",
    "success",
    "empty",
    "error",
    "loading",
    "disabled"
  ],
  "viewports": [
    "360x900",
    "768x900",
    "1280x900",
    "1920x900"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-prototype-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-prototype-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-prototype-1280-light.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-baseline-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-baseline-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-baseline-1280-light.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-1920-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-360-dark.png",
    "docs/visual-reviews/assets/2026-11-09-participation-checkin-updated-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-1920-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-360-dark.png",
    "docs/visual-reviews/assets/2026-11-09-participation-hub-updated-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-1920-light.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-360-dark.png",
    "docs/visual-reviews/assets/2026-11-09-participation-admin-updated-1280-dark.png"
  ],
  "commands": [
    "pnpm --dir frontend design:check",
    "node docs/archive/completed-tasks/2026-11-09-checkin-participation/component-browser-check.cjs updated",
    "./scripts/test-checkin-contract.sh"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "notes": "Existing native buttons, filters, repair checkbox and protected repair submit retained. No focus, dialog or keyboard handler changes."
    },
    "reduced_motion": {
      "status": "passed",
      "notes": "No animation changes; inherited play reduced-motion styles retained."
    }
  },
  "residual_risks": [
    "Isolated real components with synthetic API data, not authenticated production routes; icons/dialogs/router are stubs. Final local production browser acceptance remains pending. Server fallback fonts are used."
  ]
}
-->

## Scope

User approved removing the additional one-point check-in growth-energy grant. Unqualified check-ins retain participation and streaks with no reward; future qualified check-ins retain the published coupon/redeem/balance pool. Reward configuration remains in the existing coupon menu. History, quiz/blindbox and farm reward algorithms remain unchanged.

## Baseline

Inspect CheckInView, adjacent PlayHubView, existing AdminCheckinRecords and shared AppLayout/buttons. Current production shows growth-energy mode and the administrator labels the qualification bucket as a reward. Real component fixtures capture the old state using synthetic records. Production screenshots containing account details are not committed.

## Prototype

Saved real component prototype captures before modifying visible implementation. Reuse the same page and banner, change the unqualified check-in message to participation only, keep the check-in action, hide reward promises, and show historical energy separately from zero-reward participation in the existing admin table. No new menu or layout.

## Reuse Decision

Keep existing play tokens, layout, typography, buttons, qualification progress, table, filters and protected repair flow. Preserve old energy strings for older server compatibility and quiz/blindbox reuse. An explicit false activity grant flag takes precedence over the shared qualification classification.

## State Coverage

Tests cover available participation-only action, successful no-reward result without a zero-cash or energy toast, governance-blocked qualified state, real refresh/retry and duplicate handling, current zero-energy records and historical positive-energy records. Loading, native disabled controls, errors and empty table behavior remain covered by existing tests. Shared hover/active/focus and reduced-motion are unchanged. No new interactive control, dialog or icon.

## Viewport Coverage

Real Chromium fixtures execute the actual Vue scripts against synthetic asynchronous APIs and compile repository CSS. Screenshots at 360, 768, 1280 and 1920px, plus dark at 360/1280px, have no page overflow. The admin table keeps its own existing horizontal scroll. English keys are symmetric; no new styling or page shell. Server fonts and stubbed layout do not establish production visual acceptance.

## Evidence

The archived component-browser-check.cjs and proposal.cjs reproduce component evidence without a server or production session. The prototype transforms the inspected source in memory; the updated pass reads the implemented component. The actual PostgreSQL/HTTP/Vue contract test verifies persisted counts, snapshots, no grants, duplicates and refreshed user status independently of the screenshot fixture.

## Residual Risk

After exact-SHA deployment the local user browser must check no-reward check-in, refresh, hub copy and qualified pool rewards; the local administrator browser checks the existing check-in table and coupon pool. This thread has no connected local browser tool; server captures do not substitute for final acceptance. Existing configured farm task completion and energy formulas are outside the additional one-point grant removal.
