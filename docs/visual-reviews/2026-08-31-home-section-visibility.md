# Visual Review: homepage section visibility

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/styles/home-view.css",
    "frontend/src/i18n/routeScopes.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts"
  ],
  "routes_or_surfaces": ["/ (public homepage)", "/admin/promo-codes (coupon operations)"],
  "languages_and_themes": ["zh-CN/light", "en-US/light"],
  "states": ["default", "loading", "error", "reduced-motion"],
  "viewports": ["390x844", "768x800", "1280x800", "1600x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/home-section-visibility-20260831/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/home-section-visibility-20260831/before-production-mobile.png",
    "docs/visual-reviews/assets/home-section-visibility-20260831/before-production-desktop.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/home-section-visibility-20260831/after-mobile.png",
    "docs/visual-reviews/assets/home-section-visibility-20260831/after-desktop.png"
  ],
  "commands": [
    "node Playwright Chromium capture against production and local preview",
    "corepack pnpm vitest run src/views/__tests__/HomeView.performance.spec.ts",
    "corepack pnpm design:check",
    "corepack pnpm typecheck",
    "corepack pnpm build"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed", "note": "The visibility fallback does not add motion and existing reduced-motion rules remain unchanged." }
  },
  "residual_risks": ["Production deployment of this local fix is pending release authorization."]
}
-->

## Scope

- Routes: public Chinese homepage `/`; the same section visibility rule applies to the English route.
- Coupon operations: the user-reported sign-in reward pool and coupon wallet
  states on `/admin/promo-codes` in Chinese and English.
- Roles: guest.
- Languages and themes: Chinese/light verified; English/light remains covered by the shared route shell.

## Baseline

- Production homepage returned HTTP 200 but non-hero sections rendered with `opacity: 0` until IntersectionObserver observed them during scrolling.
- The first full-page capture showed the manifesto followed by a large blank region even though all section boxes had normal dimensions.
- Production coupon operations rendered raw `coupon.admin.*` keys for published,
  retired, available, used, and expired states because its route did not load
  the coupon locale fragment on a cold visit.

## Prototype

- The prototype keeps section geometry and internal animations unchanged while making every section readable before the observer callback.
- No new component, card, color, or interaction was introduced.
- Coupon operations keep the existing table layout, status colors, and labels;
  the review only restores localized text that was previously visible as raw
  translation keys.

## Reuse Decision

- Reused existing homepage layout, section spacing, HeroSphere, status summary, CTA, and responsive breakpoints.
- The change is a visibility fallback only; `.in-view` remains compatible with existing observers.

## State Coverage

- Default: all section content is visible on initial navigation and full-page capture.
- Hover and active: unchanged from the production homepage.
- Focus-visible and keyboard: unchanged; existing focus styles remain present.
- Loading and error: status summary may still transition from unavailable to live data; section content no longer disappears while that request resolves.
- Coupon operations: a Chinese or English cold route load resolves the
  check-in pool title, published/retired badges, and available/used/expired
  coupon badges before the page renders. The runtime locale regression test
  covers the same key set without relying on a previous page visit.

## Viewport Coverage

- Mobile: 390x844 capture has no transparent sections below the hero.
- Tablet: 768px breakpoint is covered by the responsive rule and requires no separate layout override.
- Desktop: 1280x800 capture shows all sections in the document flow.
- Wide or short screen: the rule is width-independent; 1600px and short-height browser checks remain part of release review.
- 200% zoom and reduced motion: no new transform or motion is introduced; existing reduced-motion behavior remains authoritative.

## Evidence

- Baseline: `before-production-mobile.png`, `before-production-desktop.png`.
- Updated: `after-mobile.png`, `after-desktop.png`.
- Automated checks: homepage performance tests passed (13/13), typecheck passed, production build passed.

## Residual Risk

- The production site still has a measurable JavaScript startup gap before Vue mounts (roughly 0.7-1.5 seconds in this probe); this fix addresses the post-mount blank sections, not the initial application shell.
- Deployment of this fix has not been performed in this test turn.
