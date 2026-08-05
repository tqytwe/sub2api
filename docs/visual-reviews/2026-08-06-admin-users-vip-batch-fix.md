# Admin Users VIP Batch Fix Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/UsersView.vue",
    "frontend/src/api/admin/users.ts",
    "frontend/src/utils/vipTier.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": ["/admin/users"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": [
    "all configured VIP tier options",
    "selected-user and current-filter preview",
    "CSV-only preview",
    "CSV duplicate, missing, disabled, and invalid classifications",
    "stable retry submission state"
  ],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/baseline-admin-users-selection-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "commands": [
    "pnpm typecheck",
    "pnpm vitest run src/views/admin/__tests__/UsersView.spec.ts src/api/__tests__/admin.users.spec.ts src/utils/__tests__/vipTier.spec.ts",
    "pnpm build"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Existing shared Select, native checkbox, file input, and buttons retain keyboard interaction."},
    "reduced_motion": {"status": "passed", "reason": "No new animation or motion behavior was added."}
  },
  "residual_risks": [
    "Authenticated browser acceptance of each configured tier and a real CSV upload remains required after deployment.",
    "The static review does not execute production group writes or modify membership ledger data."
  ]
}
-->

## Scope

This review covers the VIP filter and exclusive-group batch overlay after the
type normalization, complete tier list, CSV-only preview, categorized preview
counts, and retry-safe submission changes.

## Baseline

The baseline overlay required a selected user or the current-filter checkbox,
so a CSV-only import could not be previewed. Its summary hid duplicate,
missing, disabled, and invalid CSV classifications, and the tier control only
exposed the first few levels.

## Prototype

The updated overlay keeps the existing compact layout while adding a
categorized preview grid. Numeric tier values cover the configured business
range, and the file input becomes a complete independent preview target.

## Reuse Decision

The page continues to use the existing table frame, shared Select control,
buttons, native file input, dark-mode tokens, and localized copy. No route
shell or new visual library was introduced.

## State Coverage

The overlay now supports explicit selected users, all users matching the
current filters, and CSV-only input. Preview counts distinguish actionable,
already processed, missing, disabled, duplicate, and invalid entries.

## Viewport Coverage

The existing responsive overlay remains constrained by its current max width
and padding at 390x844 and 1280x820. The category grid collapses to one column
on narrow screens and does not change the page shell or table geometry.

## Evidence

Static source review, type checking, focused component/API/utility tests, the
frontend lint/design check, and the production frontend build are required for
this change. Authenticated browser acceptance of CSV upload and protected
writes remains a deployment-stage check.

## Review Notes

The tier control uses numeric values for the complete V0-V6 business range,
while the server still validates the configured tier set. Chinese and English
preview labels are kept in matching locale objects.

## Residual Risk

The visual evidence is a static review board. Production database contents,
historical membership reconciliation, and actual group mutations are outside
this deployment and remain unchanged.
