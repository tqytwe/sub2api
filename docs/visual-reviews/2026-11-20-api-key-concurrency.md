# Visual review for API key concurrency and waiting

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/user/KeysView.vue", "frontend/src/components/keys/BulkEditKeysModal.vue", "frontend/src/i18n/locales/en/dashboard.ts", "frontend/src/i18n/locales/zh/dashboard.ts"],
  "routes_or_surfaces": ["/keys", "Create and Edit API key", "Bulk edit API keys"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": ["default", "create dialog", "bulk selection", "bulk concurrency input", "keyboard focus and dismissal"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-20-api-key-concurrency/prototype-bulk-entry-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-20-api-key-concurrency/baseline-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-20-api-key-concurrency/updated-360-light.png", "docs/visual-reviews/assets/2026-11-20-api-key-concurrency/updated-1280-dark.png", "docs/visual-reviews/assets/2026-11-20-api-key-concurrency/updated-1280-key-create.png", "docs/visual-reviews/assets/2026-11-20-api-key-concurrency/updated-1280-key-bulk.png"],
  "commands": ["Playwright Chromium fixture with synthetic API", "node .item11-visual-review.mjs --baseline", "node .item11-visual-review.mjs"],
  "checks": { "keyboard": { "status": "passed" }, "reduced_motion": { "status": "passed" } },
  "residual_risks": ["Captures used an isolated Chromium fixture with synthetic data and a simplified shell. Final acceptance requires the authenticated production UI on the user's browser.", "Concurrency column and form implementation predated this review. The prototype covers the batch entry wiring added during this review."]
}
-->

## Scope

User API keys, including single key and bulk limit controls.

## Baseline

The `/keys` page at `cbfe082f8` showed current use without the configured key limit or waiting count. The baseline renders that revision's `KeysView` with the current shared `DataTable` and a synthetic key. The updated page shows active requests and waiting requests, exposes the key limit on create and edit, and restores the fixed upstream revision's batch editing entry.

## Prototype

The bulk entry prototype was rendered against the baseline before adding the selection controls to `KeysView`. It proposes the existing checkbox, selected count and action buttons from upstream `58f461b08`. The concurrency column and inputs had already been implemented when this review started.
## Reuse Decision

The implementation reuses `DataTable`, `BaseDialog`, `Icon`, `BulkEditKeysModal`, and the existing button and field classes.

## Viewport Coverage

Chromium rendered 360, 768, 1280 and 1920 px in both languages and themes with reduced motion enabled. The document had no horizontal overflow and the fixture had no console errors.
## State Coverage

The create dialog opened with limit `0`. Esc closed it and restored focus to its trigger. Selecting a row opened the real bulk dialog and allowed editing its concurrency field. Loading, error, invalid input, clearing a limit and partial bulk failure are exercised in the Vue tests.
## Residual Risk

The captures do not cover the entire authenticated shell, every hover state, or 200% zoom.
## Evidence

The Vite bundle was built in memory and loaded through Playwright request interception, with no running application or production connection.
