# v2.17.7 moderation admin sync visual review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/moderation/ModerationUsersSummary.vue",
    "frontend/src/components/common/DateRangePicker.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/index.ts",
    "frontend/src/i18n/locales/en/admin/moderationUsers.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/index.ts",
    "frontend/src/i18n/locales/zh/admin/moderationUsers.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/views/admin/RiskControlView.vue",
    "frontend/src/views/admin/UsageView.vue"
  ],
  "routes_or_surfaces": ["/admin/risk-control", "/admin/usage"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "loading", "empty", "error", "success", "disabled", "focus-visible", "conflict"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/prototype-admin-1280.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/baseline-admin-1280.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/admin-routing-detail/after-admin-1280.png"],
  "commands": [
    "PATH=/opt/node-v22.23.4-linux-x64/bin:/opt/node-v22.23.4-linux-x64/lib/node_modules/corepack/shims:$PATH pnpm --dir frontend test:run",
    "PATH=/opt/node-v22.23.4-linux-x64/bin:/opt/node-v22.23.4-linux-x64/lib/node_modules/corepack/shims:$PATH pnpm --dir frontend typecheck",
    "PATH=/opt/node-v22.23.4-linux-x64/bin:/opt/node-v22.23.4-linux-x64/lib/node_modules/corepack/shims:$PATH pnpm --dir frontend build"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Authenticated browser capture remains pending."},
    "reduced_motion": {"status": "not-applicable", "reason": "No browser capture is available in this environment."}
  },
  "residual_risks": [
    "The referenced PNGs are shared static admin workspace review-board artifacts, not authenticated production screenshots; local browser acceptance remains required.",
    "The record covers implementation and state contracts; it does not claim that an administrator has accepted the deployed route."
  ]
}
-->

## Scope

- `/admin/risk-control`: administrator-only moderation user list, details, exemptions, reset and versioned unban actions.
- `/admin/usage`: existing date-range control integration used by the synchronized admin usage surface.
- Ordinary user navigation and API access remain outside this surface.

## Baseline

- Existing admin shell, table density, date-range control, `BaseDialog`, shared buttons and semantic status colors are the baseline.
- The baseline image is an existing admin workspace reference; it is not a production acceptance capture for these routes.

## Prototype

- The existing admin workspace review board is reused to anchor shell, density, controls and dialog composition.
- No new visual pattern or decorative treatment was introduced. The scope is limited to API-backed moderation controls and explicit loading, empty, error and conflict states.

## Reuse Decision

- Reused the production admin shell, table conventions, date-range control, dialog, form controls, localization scopes and existing risk-control page.
- The four exemption controls, pagination and detail actions use existing component and token conventions.

## State Coverage

- List: loading, empty, query error, pagination and stale response protection.
- Detail: loading, readback error, ownership-aware disabled state, exemption changes, reset, unban, success and version conflict.
- Keyboard and focus behavior are covered by native controls and component tests but are not claimed as browser acceptance.

## Viewport Coverage

- The shared admin composition is recorded for 360, 768, 1280 and 1920px review targets.
- Actual authenticated browser rendering, dark theme, language switching and 200% zoom remain follow-up acceptance work.

## Evidence

- The referenced PNGs are valid shared static review-board artifacts and are deliberately not presented as production screenshots.
- Frontend tests, type checking and production build are run in the candidate worktree; production acceptance requires a later authenticated browser session.

## Residual Risk

- Before merge/deployment, an administrator must verify the actual routes in the deployed build, including ordinary-user absence, admin authorization, responsive layout and action conflict handling.
- This record does not mark the migration or production rollout complete.
