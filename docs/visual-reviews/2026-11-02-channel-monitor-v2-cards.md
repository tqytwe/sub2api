# V2 Channel Monitor Cards

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/channelMonitorV2.ts",
    "frontend/src/api/__tests__/channelMonitorV2.spec.ts",
    "frontend/src/features/channel-monitor-v2/monitorCards.ts",
    "frontend/src/features/channel-monitor-v2/MonitorStatusCards.vue",
    "frontend/src/features/channel-monitor-v2/MonitorCandySettings.vue",
    "frontend/src/features/channel-monitor-v2/MonitorSettingsPanel.vue",
    "frontend/src/features/channel-monitor-v2/__tests__/MonitorSettingsPanel.spec.ts",
    "frontend/src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts",
    "frontend/src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts",
    "frontend/src/views/user/ChannelStatusV2View.vue",
    "frontend/src/i18n/locales/en/channelMonitorV2.ts",
    "frontend/src/i18n/locales/zh/channelMonitorV2.ts"
  ],
  "routes_or_surfaces": ["/monitor", "Administrator V2 monitor settings"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["cards", "analytics", "loading", "empty", "Candy disabled", "Candy history", "recent sample timeline", "keyboard focus"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-02-channel-monitor-v2-cards/prototype-cards.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-08-28-channel-monitor-availability/baseline-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-02-channel-monitor-v2-cards/updated-desktop.png", "docs/visual-reviews/assets/2026-11-02-channel-monitor-v2-cards/updated-mobile.png"],
  "commands": ["Vite browser review harness mounting the real MonitorStatusCards.vue with project styles and locale messages; browser screenshots via Chromium at 1440x1000 and 390x844", "pnpm exec vitest run src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts src/api/__tests__/channelMonitorV2.spec.ts", "pnpm typecheck", "pnpm design:check"],
  "checks": {"keyboard": {"status": "passed", "reason": "Native tab, select and button controls provide keyboard operation and visible focus."}, "reduced_motion": {"status": "passed", "reason": "No continuous motion added."}},
  "residual_risks": ["Captures render the actual card component with fixture data, not the authenticated /monitor route or live API. Final acceptance must verify permissions and live data at desktop and mobile widths."]
}
-->

## Scope

This review covers the V2 user view switch between cards and the existing analytics view, recent-sample timelines, Candy history presentation, and the administrator controls for Candy rules, card timeline mode, platform order, and group order.

## Baseline

The current Fork had a V2 matrix and line chart but no card layout, Candy history display, or Candy configuration controls. Existing analytics filters and V2 settings remain the source of truth.

## Prototype

The static card board at `assets/2026-11-02-channel-monitor-v2-cards/prototype-cards.png` defines the compact group card, metric strip, timeline, and Candy history order. It is reused as evidence, not shipped as runtime artwork.

## Reuse Decision

The implementation reuses the existing V2 API client, filter toolbar, matrix response, card/input/tab/badge utility classes, and locale fragments. No separate static data path or new navigation surface is introduced.

## State Coverage

Cards cover loading, empty, health states, no qualified samples, disabled Candy, waiting Candy, stale Candy, and per-result history. Analytics keeps its existing pulse/line and detail tabs. Administrator controls cover disabled-by-default new checks, validation bounds, ordering, and save/refresh persistence.

## Viewport Coverage

The card grid stacks at narrow widths and expands to two/three columns at desktop widths. Long model names and group labels truncate within stable card dimensions. Final authenticated browser capture remains required at 360, 768, 1280, and 1920 pixels.

## Evidence

- `assets/2026-11-02-channel-monitor-v2-cards/updated-desktop.png` and `updated-mobile.png` are Chromium captures of the actual `MonitorStatusCards.vue` component with project styles, translations, and API-shaped sample rows. The route/auth shell and live API are not included.
- `assets/2026-11-02-channel-monitor-v2-cards/prototype-cards.png` records the initial card layout.
- `frontend/src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts` covers timeline gaps, sample thresholds, history limits, and stale checks.
- `frontend/src/api/__tests__/channelMonitorV2.spec.ts` covers the `recent_samples` matrix parameter.

## Review notes

- Cards request `recent_samples=1` and force the platform/group matrix grouping while preserving the shared range, platform, group, and model filters.
- Analytics retains the existing matrix and line chart behavior.
- Candy remains disabled by default for newly added rules; answer previews are displayed only in the administrator surface supplied by the API.
- The card layout uses the existing project card, tab, badge, and input tokens and has no continuous animation.

## Residual Risk

This branch does not prove backend scheduling, migration compatibility, or production permission filtering. Those are covered by the backend branch and coordinated integration/deployment acceptance.
