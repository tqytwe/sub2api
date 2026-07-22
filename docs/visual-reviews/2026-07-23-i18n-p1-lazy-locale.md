# I18n P1 Lazy Locale Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/index.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/core.ts",
    "frontend/src/i18n/locales/zh/core.ts",
    "frontend/src/i18n/locales/merge.ts",
    "frontend/src/router/index.ts"
  ],
  "routes_or_surfaces": [
    "home",
    "login",
    "register",
    "wallet",
    "admin users",
    "admin settings"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark static board",
    "en-US light static board",
    "en-US dark static board"
  ],
  "states": [
    "core locale loaded for public entry routes",
    "full locale loaded before complex routes render",
    "language switch reloads the current route scope",
    "missing key and fallback leakage checks"
  ],
  "viewports": [
    "390x844",
    "1280x820",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/i18n-p1-lazy-locale/prototype-i18n-p1-lazy-locale.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/i18n-p1-lazy-locale/baseline-i18n-p1-lazy-locale.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/i18n-p1-lazy-locale/updated-i18n-p1-lazy-locale.png"
  ],
  "commands": [
    "python3 generated static review boards with PIL",
    "pnpm --dir frontend exec vitest run src/i18n/__tests__/lazyLocaleScope.spec.ts src/i18n/__tests__/localesMessageCompile.spec.ts src/i18n/__tests__/localesNoKeyCollision.spec.ts",
    "pnpm --dir frontend run lint:check",
    "pnpm --dir frontend run build",
    "git diff --check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed"
    },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "This phase changes locale loading order, not layout. Final browser checks should still cover Chinese and English on home, login, register, wallet and at least one admin route.",
    "Most non-entry routes intentionally load the full locale bundle before rendering; deeper domain-level locale splitting remains a later optimization."
  ]
}
-->

## Scope

This review covers the visible copy-loading behavior changed by P1. The UI layout, spacing, colors, component hierarchy and interactions are intentionally unchanged.

## Baseline

The previous startup path loaded the full `zh` or `en` locale chunk before the app could mount. That kept copy complete but pulled admin, wallet, Play, payment and other low-frequency text into public entry pages.

Baseline artifact: `docs/visual-reviews/assets/i18n-p1-lazy-locale/baseline-i18n-p1-lazy-locale.png`.

## Prototype

The prototype loads a lightweight core locale for `/`, `/home`, `/login`, `/register`, `/setup` and `/key-usage`. Other routes load the full locale before title resolution and component rendering, preserving bilingual completeness.

Prototype artifact: `docs/visual-reviews/assets/i18n-p1-lazy-locale/prototype-i18n-p1-lazy-locale.png`.

## Reuse Decision

The implementation reuses the existing `vue-i18n` instance and existing locale objects. It adds only a shared merge helper and core bundle entry points; no visible component or style system changes are introduced.

## State Coverage

The review covers Chinese and English entry pages, register banner copy, route title resolution, and full-locale upgrade before wallet/admin routes. Missing-key tests protect against raw key leakage in the entry scope.

## Viewport Coverage

The static board covers mobile, desktop and wide desktop intent. P1 changes when locale messages load, not component dimensions or responsive CSS; existing route layouts continue to own text wrapping, spacing and theme behavior.

## Evidence

Updated artifact: `docs/visual-reviews/assets/i18n-p1-lazy-locale/updated-i18n-p1-lazy-locale.png`.

Automated evidence is provided by locale scope tests, full locale compile tests, locale collision tests, lint, production build and whitespace checks.

## Residual Risk

Browser acceptance after deployment should still check Chinese and English entry pages plus one user route and one admin route. The next phase can split full locale by domain instead of using the conservative full-route upgrade.
