# Pelican Preview Layout and V2 Sample Timeline

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/common/BaseDialog.vue",
    "frontend/src/components/user/pelican/PelicanGroupHistoryDialog.vue",
    "frontend/src/features/channel-monitor-v2/monitorCards.ts"
  ],
  "routes_or_surfaces": ["/monitor group artwork history dialog and recent sample timeline"],
  "languages_and_themes": ["zh-CN/light"],
  "states": ["selected artwork", "history loading", "history empty", "history error", "low-volume request bucket", "unscored bucket", "meets sample threshold"],
  "viewports": ["1280x900", "1920x1055"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/pelican-group-showcase/prototype-result-preview.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/pelican-group-showcase/updated-monitor-pelican-modal-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/pelican-preview-size-fix/layout-desktop-1280.png",
    "docs/visual-reviews/assets/pelican-preview-size-fix/layout-desktop-1920.png"
  ],
  "commands": [
    "Chromium headless screenshot of a static layout review board based on the existing Pelican dialog prototype and caller-provided 90dvh/flex layout contract",
    "pnpm exec vitest run src/components/common/__tests__/BaseDialog.spec.ts src/components/user/pelican/__tests__/PelicanGroupHistoryDialog.spec.ts src/features/channel-monitor-v2/__tests__/monitorCards.spec.ts",
    "go test ./internal/repository -run 'TestChannelMonitorV2(RecentSampleBucketsDoNotFilterByHealthThreshold|RecentSampleModelFilterUsesFirstAvailableParameter|RecentSampleModelClauseMatchesDisplayFilter)'"
  ],
  "checks": {
    "keyboard": { "status": "passed", "reason": "Existing dialog focus and history controls remain native keyboard-operable elements; the fix adds no control." },
    "reduced_motion": { "status": "not-applicable", "reason": "The fix changes modal sizing and sample classification, not motion behavior." }
  },
  "residual_risks": ["Review-board PNGs are labeled static and are not application screenshots. Authenticated desktop and mobile layout still require the user's local browser after deployment."]
}
-->

## Scope

The existing `/monitor` group history dialog keeps its model selector and run list while the artwork stage fills the available dialog height. Recent sample bars retain buckets that have traffic below the health threshold and mark those buckets unscored.

## Baseline

The previous dialog capture at `assets/pelican-group-showcase/updated-monitor-pelican-modal-1280.png` shows a short dialog with a collapsed artwork stage. Its existing caller supplied `h-[90dvh]` and a flex body, but `BaseDialog` did not declare or apply those attributes. The recent-sample repository query also filtered buckets using the health threshold before returning them.

## Prototype

`assets/pelican-group-showcase/prototype-result-preview.png` is the existing approved layout boundary: history remains beside a large sandboxed artwork canvas. This change does not add a route or alter the history workflow.

## Reuse Decision

The fix uses the existing `BaseDialog` class contract, current model history list, and existing sandboxed artwork preview. No new page or control was added.

## State Coverage

The component tests cover history selection and artwork loading. The `BaseDialog` test verifies that content and body classes reach the panel and body. Timeline tests cover actual low-volume buckets as `unknown` while qualified buckets keep their health state. No interaction or animation behavior changes.

## Viewport Coverage

The static layout boards show the intended desktop geometry at 1280px and 1920px. The screenshots are explicitly review boards, not product captures. Mobile CSS is checked by the shared responsive layout contract, but a 390px application screenshot remains pending local browser acceptance.

## Evidence

- Baseline: `assets/pelican-group-showcase/updated-monitor-pelican-modal-1280.png`.
- Prototype: `assets/pelican-group-showcase/prototype-result-preview.png`.
- Static geometry review: `assets/pelican-preview-size-fix/layout-desktop-1280.png` and `layout-desktop-1920.png`.
- Focused component and repository tests verify the implementation contract; they do not substitute for the deployed route.

## Residual Risk

The static boards demonstrate the intended available space and are not rendered from the authenticated app. The user's local browser must verify the deployed modal at desktop and mobile widths, including a real generated artwork and a selected history entry.
