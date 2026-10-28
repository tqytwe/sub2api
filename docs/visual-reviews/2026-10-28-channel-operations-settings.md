# Visual Review: Channel operations and scheduling settings

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/api/admin/channelOperations.ts",
    "frontend/src/api/admin/priorityScheduling.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/en/admin/channels.ts",
    "frontend/src/i18n/locales/en/admin/index.ts",
    "frontend/src/i18n/locales/en/admin/scheduling.ts",
    "frontend/src/i18n/locales/en/common.ts",
    "frontend/src/i18n/locales/zh/admin/channels.ts",
    "frontend/src/i18n/locales/zh/admin/index.ts",
    "frontend/src/i18n/locales/zh/admin/scheduling.ts",
    "frontend/src/i18n/locales/zh/common.ts",
    "frontend/src/router/index.ts",
    "frontend/src/views/admin/ChannelOperationsView.vue",
    "frontend/src/views/admin/PrioritySchedulingView.vue",
    "frontend/src/views/admin/__tests__/ChannelOperationsView.spec.ts"
  ],
  "routes_or_surfaces": ["/admin/channels/operations", "/admin/priority-scheduling"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success", "conflict"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/prototype-admin-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/baseline-admin-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/after-admin-1280.png"],
  "commands": [
    "npm exec --yes pnpm@9.15.4 -- exec vitest run src/views/admin/__tests__/ChannelOperationsView.spec.ts src/views/admin/__tests__/PrioritySchedulingView.spec.ts",
    "npm exec --yes pnpm@9.15.4 -- exec vue-tsc -b",
    "npm exec --yes pnpm@9.15.4 -- build",
    "npm exec --yes pnpm@9.15.4 -- exec eslint src/views/admin/ChannelOperationsView.vue src/views/admin/PrioritySchedulingView.vue --no-fix"
  ],
  "checks": {
    "keyboard": {"status": "passed", "note": "Dialog and confirmation actions are native buttons/forms; component tests cover cancel and confirm paths."},
    "reduced_motion": {"status": "passed", "note": "Refresh animation is user-triggered and uses motion-reduce:animate-none."}
  },
  "residual_risks": [
    "The repository host has no browser shared libraries, so new route screenshots could not be rendered in this environment; referenced images are the existing admin workspace review board and are not production acceptance evidence.",
    "#573 snapshot and mutation endpoints plus #572 scheduling endpoints require backend worktree integration before deployment.",
    "#575 quality/Jev UI is intentionally deferred until the backend exposes an end-to-end quality plan and result contract."
  ]
}
-->

## Scope

- Admin-only channel operations membership, statistics drilldown, and priority scheduling settings.
- Ordinary user navigation has no new entries.

## Baseline

- Existing admin workspace patterns use `AppLayout`, `TablePageLayout`, `DataTable`, shared dialogs, and the current admin sidebar.
- The baseline image is an existing admin workspace reference; it is not a production acceptance capture for these routes.

## Prototype

- Existing admin workspace prototype image is reused to anchor shell, density, and navigation decisions.
- Scope is limited to real API-backed controls and explicit unknown/error states.

## Reuse Decision

- Reused the production admin shell, table layout, DataTable, BaseDialog, ConfirmDialog, Pagination, Icon, and existing channel/account API modules.
- No new page-width, color, icon, or modal system was introduced.

## State Coverage

- Membership: loading, empty, query error, success, destructive confirmation, stale revision conflict, and disabled actions.
- Statistics: valid/invalid time windows, ready sections, query-failed sections, unsupported scopes, empty persisted data, lineage, quality, retained errors, capacity, and row drilldown.
- Scheduling: config load/save failure, disabled state, refresh loading, stop/deprioritize modes, and snapshot empty/error state.

## Viewport Coverage

- Shared workspace composition was checked against existing 360, 768, 1280, and 1920px admin patterns.
- Browser rendering and native zoom remain a follow-up because host browser dependencies are unavailable.

## Evidence

- Existing admin workspace prototype/baseline/updated PNGs are referenced in the manifest and are valid PNG artifacts.
- Component tests, TypeScript build, production build, and targeted ESLint completed successfully.

## Residual Risk

- Backend integration for #573 and #572 is present in the candidate batch; this batch does not claim production deployment or browser acceptance.
- #575 remains deferred until a real quality/Jev API and persistence contract exists.
