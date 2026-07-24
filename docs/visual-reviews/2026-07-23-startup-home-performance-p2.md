# Startup And Home Performance P2 Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/App.vue",
    "frontend/src/views/HomeView.vue",
    "frontend/src/utils/startupChecks.ts",
    "frontend/src/utils/homeContent.ts"
  ],
  "routes_or_surfaces": [
    "home",
    "login",
    "setup",
    "custom home content"
  ],
  "languages_and_themes": [
    "zh-CN light static board",
    "zh-CN dark static board",
    "en-US light static board",
    "en-US dark static board"
  ],
  "states": [
    "production HTML has injected public settings and skips startup setup probe",
    "static or dev fallback without injected settings keeps setup probe",
    "setup route keeps its own setup-status guard",
    "default home route avoids loading DOMPurify until custom inline HTML exists",
    "URL custom home content renders through iframe without sanitizer import"
  ],
  "viewports": [
    "390x844",
    "1280x820",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/startup-home-performance-p2/prototype-startup-home-performance-p2.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/startup-home-performance-p2/baseline-startup-home-performance-p2.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/startup-home-performance-p2/updated-startup-home-performance-p2.png"
  ],
  "commands": [
    "pnpm --dir frontend exec vitest run src/__tests__/App.startup.spec.ts src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/startupChecks.spec.ts src/utils/__tests__/homeContent.spec.ts",
    "pnpm --dir frontend run lint:check",
    "pnpm --dir frontend run build",
    "node build-output signal check for index preload and HomeView vendor-markdown imports",
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
    "This phase intentionally changes startup requests and route chunk dependencies, not visible layout. Browser acceptance should still inspect home and login in Chinese and English, light and dark.",
    "The static review board is not a browser screenshot. Final local acceptance remains required after deployment."
  ]
}
-->

## Scope

This review covers a startup performance change on public entry pages. The route layout, colors, spacing, controls and copy remain unchanged.

## Baseline

Before this phase, `App.vue` probed `/setup/status` on every mount even when production HTML had already injected public settings. `HomeView.vue` also statically imported the sanitizer used only by optional custom inline home HTML.

Baseline artifact: `docs/visual-reviews/assets/startup-home-performance-p2/baseline-startup-home-performance-p2.png`.

## Prototype

The prototype keeps setup detection for static/dev fallback and the setup route, while skipping the redundant production startup probe. Optional custom home HTML sanitization moves behind a dynamic import.

Prototype artifact: `docs/visual-reviews/assets/startup-home-performance-p2/prototype-startup-home-performance-p2.png`.

## Reuse Decision

The implementation reuses the existing setup route guard, app store injected public settings state, and home custom content surface. No new visual component pattern is introduced.

## State Coverage

The review covers the production startup state with injected settings, static/dev fallback without injected settings, setup route ownership, default GTM home content, URL-based custom home content, and inline custom HTML sanitization.

## Viewport Coverage

The static board covers mobile, desktop and wide desktop intent. The implementation changes request timing and dynamic imports only, so existing responsive CSS and text wrapping continue to own layout behavior.

## Evidence

Updated artifact: `docs/visual-reviews/assets/startup-home-performance-p2/updated-startup-home-performance-p2.png`.

Automated evidence:

- `pnpm --dir frontend exec vitest run src/__tests__/App.startup.spec.ts src/views/__tests__/HomeView.performance.spec.ts src/utils/__tests__/startupChecks.spec.ts src/utils/__tests__/homeContent.spec.ts`: 4 files passed, 12 tests passed.
- `pnpm --dir frontend run lint:check`: design governance passed with 1 evidence record; ESLint passed.
- `pnpm --dir frontend run build`: production build passed. Known Vite static/dynamic import and large chunk warnings remain unchanged.
- Build signal check: `index.html` is 2369 bytes, modulepreloads are `vendor-vue`, `vendor-i18n`, `vendor-core`, `preloadsFullLocale=false`; `HomeView-BDdEDmge.js` is 58318 bytes and has `homeStaticVendorMarkdown=false`, `homeDynamicVendorMarkdown=true`.
- `git diff --check`: passed.

## Residual Risk

Deployment proof can verify request and bundle behavior. Final browser acceptance should still cover Chinese and English public entry pages in light and dark mode.
