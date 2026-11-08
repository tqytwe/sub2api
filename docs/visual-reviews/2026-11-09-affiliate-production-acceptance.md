# Visual review: affiliate production acceptance corrections

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/affiliates/AdminAffiliateOverviewView.vue",
    "frontend/src/i18n/locales/en/admin/affiliateOperations.ts",
    "frontend/src/i18n/locales/zh/admin/affiliateOperations.ts"
  ],
  "routes_or_surfaces": ["/admin/affiliates/overview", "/affiliate"],
  "languages_and_themes": ["zh and en", "light and dark shared themes"],
  "states": ["default", "empty leaderboard", "partial error", "retry", "preview with complete total"],
  "viewports": ["360x1000", "768x1000", "1280x1000", "1920x1000"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-operations-prototype.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-light.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-light.png"],
  "commands": [
    "./node_modules/.bin/vitest run src/views/admin/affiliates/__tests__/AdminAffiliateOverviewView.spec.ts src/views/user/__tests__/AffiliateView.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts",
    "./node_modules/.bin/vue-tsc --noEmit",
    "node ../scripts/check-frontend-design-governance.mjs"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Existing native controls, links and shared DataTable focus behavior are unchanged."},
    "reduced_motion": {"status": "passed", "notes": "No animation or continuous motion was introduced."}
  },
  "residual_risks": [
    "The existing prototype and baseline artifacts are static review boards, not production browser captures.",
    "Final guest, user and administrator production acceptance still requires the user's local browser and deployment evidence."
  ]
}
-->

## Scope

The administrator overview now labels the complete outstanding total separately from the five-row preview. The user and administrator leaderboard surfaces retain their existing layout, top-ten limit, live refresh behavior and identity rules.

## Baseline

The baseline is the existing administrator operations review board at `docs/visual-reviews/assets/2026-11-08-affiliate-admin-1280-light.png`, with the exception heading showing only one ambiguous number. The user leaderboard baseline is the existing `/affiliate` implementation covered by its focused view tests.

## Prototype

The accepted prototype is the existing operations prototype at `docs/visual-reviews/assets/2026-11-08-affiliate-operations-prototype.png`. This correction keeps the established section layout and adds only explicit total-versus-preview text.

## Reuse Decision

The change reuses the existing AppLayout, DataTable, link styles, typography tokens and translation namespaces. No new page shell, card, icon or interaction pattern was introduced.

## State Coverage

- A successful exception response shows both the total number of outstanding records and the number of preview rows.
- A failed exception response shows the existing retry state without presenting a misleading zero count.
- An empty leaderboard retains the existing explicit empty state.
- Existing focus, loading, retry and reduced-motion behavior remains unchanged.

## Viewport Coverage

The shared responsive layout and DataTable behavior remain in use at 360, 768, 1280 and 1920 px. The new count text is short, wraps with the existing flex heading, and has no fixed width or viewport-specific styling.

## Evidence

Focused Vue tests cover the total-versus-preview label, the 2026-11 empty user leaderboard, the 2026-08 masked user identity and the 2026-08 complete administrator identity. The backend integration test covers the shared August cohort and 25% conversion calculation.

## Residual Risk

No production browser screenshot or database/runtime evidence is claimed by this review. Those remain deployment acceptance gates.
