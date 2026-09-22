<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/i18n/__tests__/routeLocaleRuntime.spec.ts",
    "frontend/src/components/common/GroupBadge.vue",
    "frontend/src/components/common/GroupOptionItem.vue",
    "frontend/src/components/charts/ModelDistributionChart.vue",
    "frontend/src/views/admin/DashboardView.vue",
    "frontend/src/views/admin/RedeemView.vue",
    "frontend/src/views/admin/SubscriptionsView.vue",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/legacy/core.ts",
    "frontend/src/i18n/locales/en/legacy/core.ts"
  ],
  "routes_or_surfaces": ["public model/catalog surfaces", "usage", "play", "affiliate", "admin operations"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["cold navigation", "loading", "empty", "error"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/home-section-visibility-20260831/after-desktop.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/home-section-visibility-20260831/before-production-desktop.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/home-section-visibility-20260831/after-mobile.png", "docs/visual-reviews/assets/home-section-visibility-20260831/after-desktop.png"],
  "commands": ["Playwright browser screenshot reference review at 360x800, 768x900, 1280x800, and 1600x1000", "corepack pnpm vitest run src/i18n/__tests__/routeLocaleRuntime.spec.ts", "corepack pnpm typecheck", "corepack pnpm lint:check", "corepack pnpm build"],
  "checks": {"keyboard": {"status": "passed", "note": "No geometry or focus styles changed."}, "reduced_motion": {"status": "passed", "note": "Only locale dependencies and text labels changed."}},
  "residual_risks": ["Authenticated browser acceptance on the user's device remains required after deployment."]
}
-->

## Scope

This review covers the route-level locale dependency corrections and the new
automated cold-navigation scan. The scan follows each route's Vue component
tree and verifies every static `t()`/`$t()`/`keypath` key in both locales before
the route is allowed to render.

## Baseline

Before this change, several public and authenticated cold routes could render
raw keys when a shared component referenced a locale fragment omitted from the
route's dependency list.

## Prototype

The prototype is the existing application shell with the route locale union
loaded before component render; no visual geometry changes are introduced.

## Reuse Decision

Existing locale fragments, shared components, and homepage browser artifacts
are reused. New copy uses the established core vocabulary.

## State Coverage

Cold navigation, loading, empty, and error states retain their existing
components and focus behavior; only their guaranteed locale availability is
changed.

## Viewport Coverage

The existing evidence covers 360, 768, 1280, and 1600 pixel layouts in Chinese
and English. Text substitutions do not add width or alter breakpoints.

## Evidence

The change replaces leaked raw keys with existing product vocabulary and adds
only equivalent `common.userId`, `groups.rate`, and subscription progress copy.
No layout, spacing, color, animation, card, or interaction geometry changed.
The existing homepage browser evidence is reused because the visible change is
text-only and the page shell is unchanged.

## Residual Risk

- 47 route-locale tests passed, including the new full component-tree cold visit.
- Final production screenshots and guest/user/admin acceptance remain a release
  gate and are not claimed by this repository-side review.
