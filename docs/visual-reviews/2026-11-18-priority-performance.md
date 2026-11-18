# 优先调度性能分析与绑定管理

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": ["frontend/src/views/admin/PrioritySchedulingView.vue", "frontend/src/api/admin/priorityScheduling.ts", "frontend/src/i18n/locales/zh/admin/scheduling.ts", "frontend/src/i18n/locales/en/admin/scheduling.ts"],
  "routes_or_surfaces": ["/admin/priority-scheduling: account-wide performance analysis, score recompute, per-group sticky binding inspection/cleanup"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "focus-visible", "loading", "disabled", "empty", "error", "analysis results", "recompute results", "sticky bindings", "clear confirmation", "clear success", "clear failure"],
  "viewports": ["360x900", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "browser-capture",
  "baseline_artifacts": ["docs/visual-reviews/assets/2026-11-18-priority-performance/baseline-360-light.png", "docs/visual-reviews/assets/2026-11-18-priority-performance/baseline-1280-dark.png"],
  "prototype_artifacts": ["docs/visual-reviews/assets/2026-11-18-priority-performance/prototype-360-light.png", "docs/visual-reviews/assets/2026-11-18-priority-performance/prototype-1280-dark.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/2026-11-18-priority-performance/updated-360-light.png", "docs/visual-reviews/assets/2026-11-18-priority-performance/updated-1280-dark.png"],
  "commands": ["Playwright Chromium capture: node /data/tmp/v2212-priority-visual.mjs baseline", "Playwright Chromium capture: node /data/tmp/v2212-priority-visual.mjs prototype", "Playwright Chromium capture: node /data/tmp/v2212-priority-visual.mjs updated", "pnpm exec vitest run src/views/admin/__tests__/PrioritySchedulingPerformance.spec.ts src/views/admin/__tests__/PrioritySchedulingView.spec.ts --maxWorkers=1 --minWorkers=1"],
  "checks": {"keyboard": {"status": "passed", "reason": "Native select and button controls are used; the component workflow tests exercise their actions."}, "reduced_motion": {"status": "passed", "reason": "The page uses no continuous animation or motion-dependent control."}},
  "residual_risks": ["Chromium rendered the compiled real Vue view with synthetic API data; it is development evidence, not a production browser acceptance.", "The Go test/build gates are running in the coordinator worktree and remain pending for this isolated contribution.", "Final keyboard and user-local guest/user/admin acceptance remain pending after cumulative integration and deployment."]
}
-->

## Scope

Fixed upstream `v2.21.2` task 8 closes the admin workflow around shared account performance: choose an OpenAI group and model, inspect scores and latency trends, compare a read-only recompute, and inspect or explicitly clear only that group's sticky session bindings. No new route or user-facing navigation is added.

## Baseline

Before implementation, the page offered a scheduling snapshot and cache weight only. It had no group-scoped analysis, model catalog, independent recompute result, latency trend table, or sticky-binding controls. Baseline screenshots use the compiled current view and neutral synthetic data.

## Prototype

The prototype keeps existing AppLayout, section, button, select, DataTable, Icon and localization patterns. Group selection scopes model discovery and analysis; latency values share the analysis result; recompute remains visually separate from the observed scheduler snapshot; sticky cleanup is group-scoped with an explicit confirmation and a statement that response ownership/continuation records remain untouched.

## Reuse Decision

Reused the existing admin groups API, shared `btn`/`input` styles, `DataTable`, `Icon.vue`, current scheduling section hierarchy, semantic border/status tokens and locale modules. No new chart, modal, navigation item, page width or icon was introduced. The compact trend table exposes counts and P90 values; unavailable error samples render as `—`, not zero.

## State Coverage

Component tests cover selected-group model loading and analysis, a measured zero cache rate, a separate diagnostic diff that does not replace the live snapshot, and explicit confirmation before clearing bindings. Existing tests retain config load/save, cold snapshot, stopped candidate and retry behavior. Loading/disabled/error/empty/success states are rendered inline. Browser captures cover the result, recompute and binding states. Keyboard behavior uses native select/button controls; a production keyboard pass remains pending.

## Viewport Coverage

Chromium checked 360, 768, 1280 and 1920 pixel widths in Chinese/English, light/dark themes, with reduced motion. All 16 combinations had no page-level horizontal overflow, page errors, NaN/invalid dates, raw empty arrays, or untranslated scheduling keys. Assertions checked the displayed 1800 ms peak bucket P90 and localized empty/non-empty reasons. The latency table keeps headers and numeric units together and scrolls within its section on narrow screens. At 360px, actions wrap and the two-column binding table stays within its section.

## Evidence

The PNGs are decoded Chromium captures of the compiled production Vue component with stubbed synthetic API responses and no production requests. Both targeted priority scheduling Vitest files passed: 2 files, 9 tests. `vue-tsc --noEmit` was killed by SIGKILL while the coordinator was using the constrained runner; it did not produce a typecheck result. Go tests/build are intentionally left to the coordinator.

## Residual Risk

The trend repository filters by exact effective model and selected group and uses the same completed, instrumented timing population as account performance. The displayed account summary is explicitly the maximum bucket P90, not a global percentile. The current source population contains only successful completed requests, so error-sample counts are returned as unknown. Production validation, full frontend typecheck/design gate, Go gates, route authorization checks, and local three-identity browser acceptance remain required before release.

## Review correction

The first updated screenshot contained a stale synthetic API field (`p90_ttft_ms` instead of `peak_bucket_p90_ttft_ms`) and exposed the shared DataTable fallback `[]` when the reasons slot was empty. That image was invalid evidence and has been replaced. A failing component test reproduced the empty-array and untranslated-reason output before the fix. Empty reasons now show an explicit localized state, and all reason codes emitted by the scheduler have Chinese and English labels. The corrected 1280px dark and 360px light screenshots were reopened and inspected: the trend shows 1800 ms, zero errors remain unknown (`—`), and empty and measured-performance reasons render readable text. These remain synthetic development captures, not production acceptance.
