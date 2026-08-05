# Admin Users VIP Batch Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/UsersView.vue",
    "frontend/src/api/admin/users.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts"
  ],
  "routes_or_surfaces": ["/admin/users"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark", "en-US/light", "en-US/dark"],
  "states": [
    "VIP tier filter and paid amount column",
    "standard exclusive group grant/revoke preview",
    "CSV preview classifications",
    "expired or mismatched preview error"
  ],
  "viewports": ["390x844", "1280x820"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/baseline-admin-users-selection-1440.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png"],
  "commands": [
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend vitest run src/views/admin/__tests__/UsersView.spec.ts src/i18n/__tests__/userLocaleKeys.spec.ts",
    "pnpm --dir frontend build"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Existing table selection and shared Select controls remain keyboard accessible."},
    "reduced_motion": {"status": "passed", "reason": "The overlay uses existing transitions and no new motion requirement."}
  },
  "residual_risks": [
    "Authenticated browser acceptance of CSV upload and protected writes remains required before deployment.",
    "Production health and migration checks are intentionally not performed in this worktree."
  ]
}
-->

## Scope

The existing administrator user list gains a localized VIP filter, a paid
membership column, and a compact overlay for previewing and submitting standard
exclusive-group changes or CSV email imports.

## Baseline

The existing user page keeps its shared `TablePageLayout`, table selection,
filter settings, and existing batch actions. No existing user-management
control is removed.

## Prototype

The reviewed surface is the existing table with one optional VIP column and a
fixed, dismissible batch overlay. The overlay remains within the page route and
does not change the route shell dimensions.

## Reuse Decision

The change reuses the existing `Select`, `Icon`, button, dark-mode, and table
components. No new visual library or standalone page shell was introduced.

## State Coverage

The review covers empty and populated VIP values, both grant and revoke
actions, CSV preview classifications, loading/disabled submit controls, and
localized preview or submit failures.

## Viewport Coverage

The target review sizes are `390x844` and `1280x820`. The overlay uses a
responsive max width and padding so the selection and file controls remain
inside the viewport at both sizes.

## Evidence

Static source review, `pnpm typecheck`, the UsersView and locale tests, and the
production frontend build passed. Browser authentication, real CSV upload, and
server mutation acceptance remain deployment-stage checks.

## Review Notes

The overlay is mounted inside the existing `TablePageLayout` route and uses the
same shared `Select`, button, spacing, and dark-mode tokens. It does not replace
the page shell or alter existing table actions. Chinese and English copy are
kept in separate locale objects with matching keys.

## Residual Risk

No production database, migration, deployment, or protected administrator route
was accessed from this worktree. Those checks must be completed on the release
candidate before rollout.
