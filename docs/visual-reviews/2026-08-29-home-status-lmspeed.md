# Visual Review: v182-home-status-lmspeed

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/index.html",
    "frontend/src/api/publicHomeStats.ts",
    "frontend/src/api/publicStatus.ts",
    "frontend/src/components/home/HeroSphere.vue",
    "frontend/src/components/home/LmspeedBadge.vue",
    "frontend/src/components/home/LmspeedProviderProof.vue",
    "frontend/src/components/layout/PublicContentLayout.vue",
    "frontend/src/composables/useHomeLiveStats.ts",
    "frontend/src/i18n/locales/jisudeng-home.en.ts",
    "frontend/src/i18n/locales/jisudeng-home.zh.ts",
    "frontend/src/i18n/locales/jisudeng-pages.en.ts",
    "frontend/src/i18n/locales/jisudeng-pages.zh.ts",
    "frontend/src/styles/home-view.css",
    "frontend/src/utils/homeLiveStats.ts",
    "frontend/src/views/HomeView.vue",
    "frontend/src/views/public/PublicStatusView.vue"
  ],
  "routes_or_surfaces": ["Chinese home (/)", "English home (/en)", "/status", "/en/status", "home HeroSphere"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "focus-visible", "unavailable", "reduced-motion", "theme-switch"],
  "viewports": ["360x800", "768x900", "1280x800", "1600x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/v182-home-status/prototype-1440.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/v182-home-status/baseline-production-1440.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/v182-home-status/updated-browser-360-zh-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-360-zh-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-360-en-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-360-en-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-768-zh-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-768-zh-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-768-en-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-768-en-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1280-zh-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1280-zh-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1280-en-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1280-en-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1600-zh-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1600-zh-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1600-en-light.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1600-en-dark.png",
    "docs/visual-reviews/assets/v182-home-status/updated-browser-1280-zh-light-reduced-motion.png"
  ],
  "commands": [
    "Playwright Core with Chromium 1234 against http://127.0.0.1:4173: set localStorage theme before navigation, capture / and /en at 360x800, 768x900, 1280x800, and 1600x1000 in light and dark themes",
    "Playwright Chromium reduced-motion probe at 1280x800: assert zero /earth/ requests and a focus-visible solid outline on .home-status-summary-link",
    "pnpm exec vitest run src/composables/__tests__/homeLiveStats.spec.ts src/components/home/__tests__/HeroSphere.performance.spec.ts src/views/__tests__/HomeView.performance.spec.ts src/views/public/__tests__/PublicStatusView.spec.ts src/i18n/__tests__/homeTruthfulness.spec.ts",
    "pnpm design:check",
    "go test ./internal/service ./internal/repository ./internal/handler ./internal/server ./migrations -run 'Test(PublicStatusSnapshotWorker|PublicStatusSnapshotReadiness|PublicStatusSnapshotRefresh|PublicStatusOpsAggregation|OpsAggregationFinalizes|OpsRepositoryAdvances)' -count=1"
  ],
  "checks": {
    "keyboard": {"status": "passed", "notes": "Local Chromium focused .home-status-summary-link at 1280px; it matched :focus-visible and computed a solid outline."},
    "reduced_motion": {"status": "passed", "notes": "Local Chromium at 1280px with prefers-reduced-motion reduced made zero /earth/ requests; the browser capture is listed in updated artifacts."}
  },
  "residual_risks": [
    "The Chromium captures use the local Vite frontend without a running backend. They accurately exercise the homepage layout, dark-palette redraw, unavailable presentation, focus and reduced-motion boundary, but public settings and status API requests returned local 500 responses; fresh, delayed, loading, retry and populated-metric states still require an integrated backend capture.",
    "Production values, Zeabur deployment SHA, migration execution, 200 percent browser zoom, real-device performance, and guest/user/admin acceptance are not established by these local browser captures."
  ]
}
-->

## Scope

- Routes: Chinese and English home plus public status routes.
- Surfaces: first-screen actions, cumulative-call/data-watermark disclosure,
  first-party status entry, snapshot status metrics, and the HeroSphere
  static/low-load boundary.
- Explicit removal: all homepage LMSPEED status branding, anchor, badge,
  provider proof, external image/link, and noscript fallback.

## Baseline

- The prior homepage mixed product actions with LMSPEED external branding and
  summary performance claims that did not expose data window or sample size.
- The baseline PNG is a production-era static reference. It is used only to
  compare hierarchy and removal scope; it is not a claim about current live
  production state.

## Prototype

- The prototype and initial static board show the revised hierarchy: product
  value, create-key/docs actions, then first-party data transparency. Status
  metrics name their window, samples, and data-watermark boundary.
- Desktop treats the globe as optional enhancement. Constrained environments
  use a stable poster before any geography fetch, so CTA layout is not delayed.
- Eligible desktop enhancement resources are queued from an idle callback (or a
  bounded timeout) only while the page is visible and intersecting. The idle
  callback is cancelled on unmount and constrained modes avoid all geography
  requests.

## Reuse Decision

- `HomeView` retains the established public homepage typography and button
  system. `PublicStatusView` uses `PublicContentLayout` for header, footer,
  page background, frame width, and responsive gutters; it owns only its
  metrics content.
- Existing semantic status, focus, surface, and text tokens are used. The two
  status-view breakpoint comments are narrow audited governance allowances:
  they only reflow the inner grid and do not claim route-shell ownership.
- No new card family, icon family, decorative gradient, or continuous status
  animation was introduced.

## State Coverage

- Default/fresh: displays processed calls, thirty-day availability, 24-hour
  TTFT P50/P95, samples, and `data_through`.
- Delayed/unavailable: changes the explicit state label and does not retain a
  green claim when business data becomes stale, lacks samples, or is absent.
- The local browser has no connected backend, so all captures deliberately
  exercise the unavailable state; it does not make a green availability or
  latency claim. Unit tests cover loading, error and empty geometry while an
  integrated backend capture remains a release gate.
- Chromium programmatically focused the status link at 1280px. It matched
  `:focus-visible` and had a solid computed outline. No pointer-only status
  action is added.
- Reduced-motion/save-data/mobile/low-memory/low-core: HeroSphere uses its
  poster and performs no Earth LOD fetch.
- The aggregation job starts with a fixed 30-day completeness bootstrap when
  its durable watermark is missing. It writes no watermark, and the public
  state stays unavailable, until every 24-hour source chunk has completed.
  The worker cadence is then an immediate pass followed by five-minute boundary
  runs. Missing snapshots are recovered newest-first from only the recent
  six-hour horizon, at most four windows per run; both the worker pre-check and
  the snapshot SQL require the aggregation watermark to cover the target hour.

## Viewport Coverage

- Actual Chromium screenshots cover 360x800, 768x900, 1280x800 and 1600x1000
  for Chinese and English in both light and dark themes. The desktop globe is
  visible in the right-side background at 1280/1600 without crossing CTA or
  status content; constrained mobile keeps it low-contrast and noninteractive.
- The screenshot matrix also includes a 1280px Chinese light reduced-motion
  capture. The live browser probe made no `/earth/` request in that mode.
  200 percent browser zoom and integrated status API states remain release
  gates rather than implied by this local capture.

## Evidence

- All listed `updated-browser-*` files are decoded Chromium PNG captures of
  the Vite application, not reconstructed boards. Their dimensions were
  verified with the repository PNG decoder and `file` metadata.
- The dark canvas uses a reviewed Canvas2D-only color conversion and observes
  `html.dark`; the visual matrix includes both initial dark rendering and a
  unit-tested runtime theme-switch redraw.
- Focused Vitest covers the freshness model, HeroSphere performance boundary,
  homepage removal/truthfulness, and public status rendering.
- The backend contract and migration evidence are in
  `docs/V182_P1_HOME_STATUS_CONTRACT.md`; the public endpoint reads only a
  persisted snapshot and does not calculate raw TTFT percentiles in HTTP.
- The migration baseline records 373 repository SQL files (262-268 pending)
  against 373 production lineage rows, including seven retired daily-card
  migrations that must not be recreated.

## Residual Risk

- This is a real local browser matrix, but it is not an integrated or
  production capture. The local backend returned 500 for public settings and
  status requests, so displayed values are intentionally unavailable rather
  than a substitute for fresh/delayed data validation.
- The poster/desktop enhancement must still be measured against actual
  production device and network profiles, including first-interactive timing,
  200 percent zoom and the deployed asset hash.
