# Visual Review: public-nav-wide-pages

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/styles/public-pages.css",
    "frontend/src/views/public/ContactView.vue",
    "frontend/src/views/public/ModelsView.vue"
  ],
  "routes_or_surfaces": ["/models", "/docs", "/contact", "/about", "/blindbox", "/arena", "/quiz-quest", "/agent-team"],
  "languages_and_themes": ["zh-CN/light", "en-US/light", "zh-CN/dark"],
  "states": ["guest", "signed-in table", "empty", "error", "responsive"],
  "viewports": ["390x844", "1366x900", "1920x1080", "1920x2400"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/public-nav-wide/models-page-prototype.png",
    "docs/visual-reviews/assets/public-nav-wide/public-nav-wide-board.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/public-nav-wide/models-page-before.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/public-nav-wide/models-page-updated-board.png",
    "docs/visual-reviews/assets/public-nav-wide/public-nav-updated-board.png"
  ],
  "commands": [
    "firefox --headless --screenshot ...models-page-wide.png --window-size 1920,1080 file:///.../models-page-prototype/index.html",
    "firefox --headless --screenshot ...public-nav-wide-board-full.png --window-size 1920,2400 file:///.../public-nav-wide-prototype/index.html",
    "pnpm typecheck",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": { "status": "not-applicable", "reason": "This change adjusts layout, table grouping, and width rules without changing focus order or interactive behavior." },
    "reduced_motion": { "status": "not-applicable", "reason": "No motion, animation, or timed transition was added by this change." }
  },
  "residual_risks": [
    "Final browser screenshot with live backend model data remains required before production acceptance."
  ]
}
-->

## Scope

- Routes: `/models`, `/docs`, `/contact`, plus a review-only decision for `/about` and Play public pages.
- Roles: guest model pricing view, signed-in pricing table structure, public documentation reader, public contact page.
- Languages and themes: Chinese and English strings were updated for the public model price grouping; dark mode receives semantic emerald price highlighting.

## Baseline

- Current behavior: `/models` used a narrow, small table and included an `适用场景` column that the user asked to remove.
- Baseline screenshot or recording: `docs/visual-reviews/assets/public-nav-wide/models-page-before.png`.
- Inconsistencies observed: the pricing table did not use wide-screen space well, and official/site prices were visually too similar.

## Prototype

- Prototype design image: `docs/visual-reviews/assets/public-nav-wide/models-page-prototype.png` and `docs/visual-reviews/assets/public-nav-wide/public-nav-wide-board.png`.
- Approval status: user approved both the model price prototype and the public navigation width system direction.
- Scope boundary: apply wide workspace rules to `/models`, `/docs`, and `/contact`; keep `/about` reading width and Play pages' independent experience.

## Reuse Decision

- Shared layouts and components reused: existing `public-pages.css`, `PublicPageToolbar`, `SupportContactPanel`, and `SupportFloatingCard`.
- New shared pattern, if any: wide public workspace width using `1344px`, plus grouped official price and our price cells on `/models`.
- Design-system exception, if any: none; new color treatment uses Tailwind semantic emerald utilities and existing page tokens.

## State Coverage

- Default: model, docs, and contact public pages keep existing content flow with wider alignment.
- Hover and active: no new hover or active interaction was introduced.
- Focus-visible and keyboard: focus order is unchanged because existing links, buttons, and inputs remain in DOM order.
- Loading, disabled, empty, error and success: existing loading and empty/error states remain in place; model table column count changed only for populated rows.

## Viewport Coverage

- Mobile: models hero collapses to one column at `max-width: 900px`; table spacing tightens at `max-width: 640px`.
- Tablet: contact page collapses to one column at `max-width: 900px`; docs sidebar already collapses through existing rules.
- Desktop: `/models`, `/docs`, and `/contact` align to `1344px` content rhythm.
- Wide or short screen: static review board covers 1920px width and shows model, docs, contact, and reading/experience decisions.
- 200% zoom and reduced motion: no fixed overlay or motion was added; final browser acceptance should re-check live data rows.

## Evidence

- Updated screenshot or recording: `docs/visual-reviews/assets/public-nav-wide/models-page-updated-board.png` and `docs/visual-reviews/assets/public-nav-wide/public-nav-updated-board.png`.
- Automated visual or overlap checks: design governance validates changed visual files and artifacts; `vue-tsc` validates template/type correctness.
- Commands run: `pnpm typecheck`, `pnpm design:check`, and Firefox headless static review screenshots.

## Residual Risk

- Known limitations: the current artifacts are static review boards; live `/models` with production data still needs browser screenshot verification before release acceptance.
- Follow-up owner: frontend release verifier.
