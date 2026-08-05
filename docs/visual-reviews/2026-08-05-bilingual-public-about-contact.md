# Bilingual Public About and Contact Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/router/index.ts",
    "frontend/src/router/publicNavigation.ts",
    "frontend/src/utils/publicLocaleRoute.ts",
    "frontend/src/utils/routeSeo.ts",
    "frontend/src/router/__tests__/publicNavigation.spec.ts",
    "frontend/src/utils/__tests__/publicLocaleRoute.spec.ts",
    "frontend/src/utils/__tests__/routeSeo.spec.ts",
    "frontend/src/views/public/AboutView.vue",
    "frontend/src/views/public/ContactView.vue",
    "backend/internal/web/embed_on.go",
    "backend/internal/web/embed_test.go"
  ],
  "routes_or_surfaces": [
    "/about",
    "/contact",
    "/en/about",
    "/en/contact",
    "homepage English About and Contact navigation",
    "public locale switcher"
  ],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark", "en-US/dark"],
  "states": ["guest", "authenticated user", "administrator"],
  "viewports": ["360x800", "768x1024", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/home-mobile-navigation/prototype-360.jpg"],
  "baseline_artifacts": ["docs/visual-reviews/assets/home-mobile-navigation/baseline-360.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/home-mobile-navigation/updated-1280.png"],
  "commands": [
    "pnpm --dir frontend run design:check",
    "pnpm --dir frontend run typecheck",
    "pnpm --dir frontend vitest run src/router/__tests__/publicNavigation.spec.ts src/utils/__tests__/publicLocaleRoute.spec.ts src/utils/__tests__/routeSeo.spec.ts",
    "go test -tags embed ./backend/internal/web"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "The existing homepage menu and public toolbar retain focus-visible and Escape behavior."},
    "reduced_motion": {"status": "not-applicable", "reason": "This change adds route and metadata contracts without new motion."}
  },
  "residual_risks": [
    "The existing home screenshots cover the shared navigation surface; production browser click-through remains part of final deployment acceptance."
  ]
}
-->

## Scope

English homepage navigation now has stable `/en/about` and `/en/contact` destinations instead of query-localized Chinese URLs. Chinese routes retain `/about` and `/contact`, and both pairs expose reciprocal canonical and hreflang metadata.

## Baseline

English homepage About and Contact links used Chinese paths with `?lang=en`, so the visible locale and crawler metadata could disagree.

## Prototype

The reviewed route shape uses dedicated `/en/about` and `/en/contact` pages with reciprocal links to the Chinese routes.

## Reuse Decision

The existing AboutView, ContactView, public toolbar, locale switcher, and page frames are reused. Only route names, locale targets, and SEO contracts change.

## Review Notes

The existing homepage responsive navigation, toolbar controls, theme behavior, and auth-state presentation are reused. The route-only addition does not change visual layout, spacing, or component hierarchy. Existing 360px and 1280px homepage artifacts cover the shared header and mobile menu surfaces.

## State Coverage

Guest, authenticated-user, and administrator navigation states retain the same toolbar and console visibility while English About and Contact targets become stable localized routes.

## Viewport Coverage

The shared homepage navigation remains covered at 360x800, 768x1024, and 1280x820; the route-only change introduces no viewport-specific layout.

## Evidence

The existing homepage prototype, baseline, and updated artifacts are reused for the unchanged visual surface. Navigation and SEO contract tests cover route destinations, locale switching, canonical URLs, hreflang, and structured data.

## Acceptance

The route contract tests assert English navigation names and locale-switch targets. Frontend and embedded backend SEO tests assert language, canonical, hreflang, and structured-data output for all four routes. Production acceptance must click both links in guest, regular-user, and administrator sessions at phone, tablet, and desktop widths.

## Residual Risk

Production browser click-through and authenticated-state visual evidence remain part of final deployment acceptance.
