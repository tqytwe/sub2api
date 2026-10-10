# Visual Review: read-only account quota refresh

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountUsageCell.vue",
    "frontend/src/views/admin/AccountsView.vue"
  ],
  "routes_or_surfaces": [
    "/admin/accounts"
  ],
  "languages_and_themes": [
    "zh-CN/light (unchanged template)",
    "en-US/dark (unchanged template)"
  ],
  "states": [
    "passive-refresh",
    "manual-refresh",
    "missing-window",
    "query-error"
  ],
  "viewports": [
    "360x800",
    "768x1024",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/prototype.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/baseline.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/2026-10-10-readonly-quota/updated.png"
  ],
  "commands": [
    "pnpm exec vitest run src/components/account/__tests__/AccountUsageCell.spec.ts src/utils/__tests__/accountUsageRefresh.spec.ts",
    "python3 + Pillow static contract review boards"
  ],
  "checks": {
    "keyboard": {
      "status": "not-applicable",
      "reason": "No template, controls, handlers for keyboard, or focus styles changed."
    },
    "reduced_motion": {
      "status": "not-applicable",
      "reason": "No animation, motion, layout, or style changes."
    }
  },
  "residual_risks": [
    "Static boards document refresh contracts only; production browser acceptance remains with the root release task. No screenshot or responsive visual acceptance is claimed."
  ]
}
-->

## Scope

Functional refresh scheduling only, for administrators on /admin/accounts. No template, CSS, labels, balance formatting, or controls change. User authorized removal of the feedback loop. The file-based design gate requires evidence for any Vue change.

## Baseline

The AccountUsageCell watcher promoted passive snapshot writes to force=true. RED component tests reproduce this feedback. baseline.png is a static contract board, not a screenshot of production.

## Prototype

prototype.png records the intended request behavior and scope boundary. Existing controls and render tree are reused; no visual redesign is proposed.

## Reuse Decision

Reviewed AccountUsageCell, adjacent OpenCodeGoUsageCell, and shared UsageProgressBar. Preserve their components, templates and styling. No new visual pattern.

## State Coverage

Default, hover, active, focus, disabled and loading render paths are unchanged. Component and parent-view tests cover passive snapshot updates, browser-cache bypass without upstream force, and explicit manual refresh. Backend fake upstream tests cover errors and absent windows. No browser interaction result is claimed.

## Viewport Coverage

The same watcher executes at 360, 768, 1280 and 1920 widths. No viewport CSS changes. Static boards are not responsive screenshots; 200% zoom and theme acceptance remain part of final browser verification.

## Evidence

The targeted frontend tests cover passive/manual refresh and bypassing a populated page cache without forcing the upstream. updated.png describes the resulting contract. Backend tests use a fake upstream only; production has not been queried or modified by this task.

## Residual Risk

Final browser acceptance is outstanding and belongs to the root release task. This record must not be treated as a production screenshot or evidence that the reported upstream credit loss has been attributed.

