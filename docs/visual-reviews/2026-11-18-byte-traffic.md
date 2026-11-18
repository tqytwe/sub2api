# Visual Review: HTTP body traffic usage

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/usage/UsageFilters.vue",
    "frontend/src/components/common/Select.vue",
    "frontend/src/components/usage/TrafficPanel.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en/traffic.ts",
    "frontend/src/i18n/locales/zh/traffic.ts",
    "frontend/src/views/admin/UsageView.vue",
    "frontend/src/views/admin/ops/OpsDashboard.vue",
    "frontend/src/views/admin/ops/components/OpsNetworkChart.vue",
    "frontend/src/views/user/UsageView.vue"
  ],
  "routes_or_surfaces": [
    "/usage traffic tab",
    "/admin/usage traffic tab",
    "/admin/ops network series"
  ],
  "languages_and_themes": [
    "en/light",
    "zh/light",
    "en/dark",
    "zh/dark"
  ],
  "states": [
    "summary and real Chart.js trend",
    "completed and partial request records",
    "admin delayed delivery health",
    "deleted API key choices",
    "Ops measured gaps and unavailable qdisc",
    "keyboard controls",
    "local table scroll"
  ],
  "viewports": [
    "360x844",
    "390x844",
    "768x900",
    "1280x900",
    "1920x1080"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/prototype-1280.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/baseline-usage-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/baseline-admin-usage-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/baseline-admin-ops-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/baseline-admin-usage-360-en-dark.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-usage-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-usage-390-zh-dark.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-admin-usage-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-admin-usage-390-zh-dark.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-admin-ops-1280-en-light.png",
    "docs/visual-reviews/assets/2026-11-18-byte-traffic/updated-admin-ops-390-zh-dark.png"
  ],
  "commands": [
    "pnpm exec vite build --minify false --outDir /data/tmp/item16-browser-dist",
    "node docs/visual-reviews/assets/2026-11-18-byte-traffic/replay.mjs baseline BASELINE_DIST ARTIFACT_DIR",
    "node docs/visual-reviews/assets/2026-11-18-byte-traffic/replay.mjs updated UPDATED_DIST ARTIFACT_DIR",
    "pnpm design:check"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "Chromium activates traffic tabs with Enter, opens searchable API-key Select with Enter and closes with Escape, expands native trend details with Space, and switches the Ops instance selector with ArrowDown."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "All contexts request reduced motion and verify the browser media query. Screenshots disable animations."
    }
  },
  "residual_risks": [
    "Browser captures execute the actual compiled Vue routes with synthetic API responses and synthetic identities. They do not prove production authentication, live ingestion, persistence, or end-to-end traffic totals. User local-computer production acceptance remains required.",
    "At 360px in English, the pre-existing ModelDistributionChart source/metric buttons on the admin usage page extend the document by 19px. The same overflow is checked against the #433 baseline; the traffic and network panels contain their own overflow.",
    "Loading, empty, failure/retry and cursor pagination are covered by component tests; the saved browser captures focus on populated traffic, partial reads, delayed recording health, and Ops network samples.",
    "The original prototype and old static boards remain development aids. Only explicitly listed baseline_artifacts and updated_artifacts are application browser evidence."
  ]
}
-->

## Scope

The user and admin Usage pages gain an HTTP/SSE body traffic tab. Admin Ops gains a network series chart. Byte totals are diagnostic values and remain separate from token billing.

## Baseline

The three baseline captures use the production Vite bundle built from unmodified #433, commit `3dcc3c200dc1a475723766d5b3feb948069ddce9`, at `/data/tmp/v2212-item14-baseline-dist`. They show the actual Usage and Ops routes before traffic recording UI. The parent integration agent confirmed the source identity of this reused bundle. The old `baseline-1280.png` and HTML board remain static development aids.

## Prototype

The retained `prototype-1280.png` board was captured before implementation and proposes period/API-key controls, byte summaries, a trend and request records. It remains a prototype rather than application evidence.

## Reuse Decision

Traffic is a tab on the existing Usage pages. Admin identity filtering reuses `UsageFilters`; the user view reuses `Select`; the trend uses the installed Chart.js integration. Ops uses its existing dashboard area. Its SVG paths render measured network series, not interface icons. The route shell and layout remain shared.

## State Coverage

The replay uses synthetic authenticated user and admin identities and fixture responses matching `traffic.ts` and `opsNetwork.ts`. Real Vue rendering and Chart.js pixels are verified, including completed and partially read requests, historical/deleted keys and delayed delivery health. Ops curves include missing retransmission samples and unavailable qdisc observations; the keyboard-selected second instance demonstrates a valid qdisc series. Onboarding is marked complete for the fixture identities using the existing application storage key.

Browser checks activate traffic tabs with Enter, expand the trend table with Space, open/close the user Select with Enter/Escape, and switch the Ops instance with ArrowDown while asserting the SVG changes. No browser exceptions are accepted. Hover/active/focus visuals use existing shared controls; success, disabled, loading, empty, error/retry and pagination behavior remain supported by the targeted component tests, with only populated/default, partial and delayed-health states saved here.

## Viewport Coverage

All three updated routes are checked at 360×844 (English/dark), 390×844 (Chinese/dark), 768×900 (Chinese/light), 1280×900 (English/light), and 1920×1080 (English/dark), using reduced motion. Full-page screenshots at 390 and 1280 show each route. The narrow traffic table scrolls in its own wrapper. The traffic and network panels have no internal overflow escaping their width.

The pre-existing English ModelDistributionChart buttons on admin Usage cause 19px document overflow at 360px. The replay explicitly checks that same baseline allowance rather than reporting a clean whole-page result. This component is untouched by item 16 and is a separate retained issue.

## Evidence

Only the manifest's application PNGs are browser evidence. Their source is the actual compiled Vue application, served through Playwright routing at `https://traffic-review.invalid`; all external network requests are blocked. The fixture contains no real users, credentials or API usage. The updated bundle is from item 16 over #432, commit `6385c821143b2b8141bdf08106f4390b7fbace4d`, with this worktree's uncommitted feature changes. Baseline and updated backend revisions are not claimed to have been deployed.

Replay from the repository root, after building the desired worktree, with Node and Playwright available:

```sh
cd frontend
pnpm exec vite build --minify false --outDir /data/tmp/item16-browser-dist
cd ..
PLAYWRIGHT_MODULE=/path/to/playwright-core/index.mjs CHROMIUM_PATH=/path/to/chrome \
  node docs/visual-reviews/assets/2026-11-18-byte-traffic/replay.mjs \
  updated /data/tmp/item16-browser-dist docs/visual-reviews/assets/2026-11-18-byte-traffic
```

Use `baseline` with a verified pre-feature bundle to replay baseline captures. This run used `/data/cache/npm/_npx/e41f203b7505f1fb/node_modules/playwright-core/index.mjs` and `/data/cache/playwright/chromium-1194/chrome-linux/chrome`. Optional `SURFACE=/admin/ops` restricts replay to one route. Locale comes from `?lang=en` or `?lang=zh`, matching the application's URL locale rules.

## Residual Risk

Fixture browser checks establish layout, control behavior and rendering on the build under review. Production authentication, data ingestion, byte reconciliation, live failure/retry behavior and the combined integration build still need their own acceptance. The repository requires final user acceptance from a local computer against production. The known 360px English admin chart overflow remains outside this feature.
