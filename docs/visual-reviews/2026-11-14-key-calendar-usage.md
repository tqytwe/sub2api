# Visual Review: Key Calendar Consumption

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/usage.ts",
    "frontend/src/components/common/DateRangePicker.vue",
    "frontend/src/views/user/KeysView.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/dashboard.ts",
    "frontend/src/i18n/locales/zh/dashboard.ts"
  ],
  "routes_or_surfaces": ["/keys existing consumption filter and table column; actual components with synthetic API/layout"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default 30 calendar days", "selected range", "loading unknown", "failed unknown", "successful zero", "pagination", "keyboard open", "calendar-only presets"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-calendar-usage/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-calendar-usage/prototype-1280-dark.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-calendar-usage/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-calendar-usage/baseline-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-14-key-calendar-usage/updated-360-light.png", "docs/visual-reviews/assets/2026-11-14-key-calendar-usage/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-14-key-calendar-usage/updated-360-picker.png", "docs/visual-reviews/assets/2026-11-14-key-calendar-usage/updated-360-error.png"],
  "commands": ["node /data/tmp/v2212-key-calendar-visual.mjs baseline # Playwright browser capture", "node /data/tmp/v2212-key-calendar-visual.mjs prototype # Playwright browser capture", "node /data/tmp/v2212-key-calendar-visual.mjs updated # Playwright browser capture"],
  "checks": {"keyboard": {"status": "passed"}, "reduced_motion": {"status": "passed"}},
  "residual_risks": ["Browser captures compile the real KeysView, DateRangePicker, TablePageLayout and table with synthetic APIs and outer layout; they are not authenticated route or production acceptance.", "Final integrated typecheck/build and independent reviews remain release gates. Real user-local production acceptance remains pending deployment."]
}
-->

## Scope

Port fixed upstream v2.21.2 calendar consumption to the existing `/keys` page.
Default to 30 inclusive calendar days; allow an explicit date range and preserve it
while paging, sorting and filtering. Keep the current key management workflow and
Fork billing calculation. There is no new route, table or migration.

## Baseline

The existing KeysView showed a fixed last-30-days amount without a date selector,
and rendered unknown or failed consumption as zero. Browser baseline images were
captured before editing the page. The adjacent UsageView and the shared date picker,
table, layout, icons and existing abort flow were inspected.

## Prototype

Before page implementation, a browser compiled the existing page with an in-memory
prototype that reused DateRangePicker in the filter area and changed the fixed
period label. The boundary is the existing page's date filter and consumption
column, as authorized for this upstream package. These images are development
design evidence, not a separate product prototype or production screenshot.

## Reuse Decision

Reuse DateRangePicker, TablePageLayout, DataTable, Icon and the existing refresh
button and error toast. Add a backwards-compatible optional date-picker flag to
hide the rolling-hour preset on this calendar-only consumer. Other pages retain
their existing preset defaults. No page width, palette or layout system changes.

## State Coverage

Browser checks cover actual page rendering, successful zero and nonzero amounts,
the mobile calendar menu, keyboard Enter, Today selection and failed consumption.
Tests cover range persistence through pagination, invalid ranges, loading/error
unknown values, stale results, cancellation and successful refresh.

The shared picker supplies hover, active, focus and disabled interaction styling;
no new button or interaction style is introduced. Empty key lists retain the
existing empty state. No new animation is added; captures use reduced motion.

## Viewport Coverage

Baseline, prototype and updated page each use 360, 768, 1280 and 1920px at 900px
height in both languages and themes. Each matrix has no page overflow or browser
errors. Table scrolling remains owned by the existing shared components.
The narrow updated menu remains inside the viewport. Full authenticated route
checks and user-local acceptance remain outstanding release verification.

## Evidence

The temporary Playwright script compiles actual Vue components in memory and
serves only intercepted `https://key-calendar-review.invalid/` fixtures. No
application server, production API, login or real credential is used. Synthetic
fixtures are marked on the captured pages. Images are under
`assets/2026-11-14-key-calendar-usage/`.

## Residual Risk

The integrating maintainer must retain the parallel Key model-policy and live
concurrency changes during the three-way merge and verify the final page together.
The backend must continue to verify ownership and preserve Fork charged-cost SQL.
Deployment and production acceptance are not established by these images.
