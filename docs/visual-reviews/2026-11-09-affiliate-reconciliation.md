# Visual review: invitation reconciliation and empty paid rankings

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/affiliates/AdminAffiliateOverviewView.vue",
    "frontend/src/views/user/AffiliateView.vue",
    "frontend/src/i18n/locales/en/admin/affiliateOperations.ts",
    "frontend/src/i18n/locales/zh/admin/affiliateOperations.ts",
    "frontend/src/i18n/locales/en/affiliateOperations.ts",
    "frontend/src/i18n/locales/zh/affiliateOperations.ts"
  ],
  "routes_or_surfaces": ["/admin/affiliates/overview", "/affiliate"],
  "languages_and_themes": ["zh and en", "light and dark"],
  "states": ["default", "empty", "error", "retry", "loading", "disabled", "focus-visible", "hover", "active", "success"],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-light.png", "docs/visual-reviews/assets/2026-11-08-affiliate-user-farm-review.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-360-light.png",
    "docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-768-light.png",
    "docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-1280-light.png",
    "docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-1280-dark.png",
    "docs/visual-reviews/assets/2026-11-09-affiliate-reconciliation-1920-light.png"
  ],
  "commands": ["pnpm --dir frontend design:check", "pnpm --dir frontend typecheck", "pnpm --dir frontend exec vitest run src/views/admin/affiliates/__tests__/AdminAffiliateOverviewView.spec.ts src/views/user/__tests__/AffiliateView.spec.ts"],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Shared native button switches to the invitation tab; existing router links and focus styles are reused. Interaction is covered by Vue tests; browser keyboard acceptance remains pending."},
    "reduced_motion": {"status": "passed", "notes": "No animation introduced. Existing shared reduced-motion rules apply."}
  },
  "residual_risks": ["These images render a static design board, not the running Vue application or a production screenshot. Responsive and theme review of the actual application requires the user's local browser after deployment."]
}
-->

## Scope

Repair the confirmed invitation-operations regression only. Separate raw registration counts and paid eligibility; preserve farm, activity rules and wallet policy.

## Baseline

The user reported four monthly summary cards and an empty paid ranking on production on 2026-11-09. Existing static administrator and user review boards establish the section hierarchy and typography. The target overview, the adjacent invitation-record page, shared StatCard and DataTable, and user affiliate tabs were inspected before edits. No new navigation or page shell is added.

## Prototype

The new prototype was generated before page implementation. It separates confirmed bindings, actual registration dates, qualification totals and evidence gaps, and adds a route from an empty paid leaderboard to existing invitation details. Counts on the board are illustrative and must not be treated as production evidence. The already-authorized scope is to explain zero values and restore only evidence-backed historical first payments.

## Reuse Decision

Keep AppLayout, StatCard, shared grid and definition-list treatment already used by account funds, native links/buttons, and user gw styles. Reconciliation is one section, not nested cards. The server returns counts rather than private identities. No page widths, icon variants, colors, animations or new components were introduced.

## State Coverage

Default/success show explicit all-time versus selected-month labels. Loading/disabled use existing overview loading behavior. Error/retry retain section errors and do not fabricate counts. Empty rankings explain registration versus eligible payment and offer My invitations. The button is keyboard reachable and its tab change is covered by a Vue interaction test. Overview refreshes on a visible-tab return; leaderboard polling retains existing request ordering and retry behavior.

Hover, active and focus-visible reuse existing button/link styles. There is no new motion. Static boards cover narrow, tablet, desktop, wide, and a dark palette; they are development aids, not proof that the real Vue layout was rendered at each size. Locale completeness and actual Vue interaction/type tests supplement these boards.

## Viewport Coverage

Static boards at 360, 768, 1280 and 1920 px and a dark palette support layout review. Actual application viewport acceptance remains pending.

## Evidence

The referenced prototype preceded implementation. Updated static review boards describe the intended revised composition; Vue tests exercise the actual components and no static artifact is claimed as a live production capture.

## Residual Risk

Local authenticated production browser screenshots, actual responsive/theme checks, and final guest/user/administrator acceptance remain required. Server-side static boards, test fixtures, health and API checks cannot replace that evidence.
