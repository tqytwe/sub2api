# Visual Review: quality group rule operations

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/admin/account/ScheduledTestsPanel.vue",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/en/admin/resources.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/zh/admin/resources.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/router/index.ts",
    "frontend/src/views/admin/AccountQualityView.vue"
  ],
  "routes_or_surfaces": ["/admin/account-quality", "/admin/accounts scheduled tests", "admin sidebar"],
  "languages_and_themes": ["zh/light", "zh/dark", "en/light", "en/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success", "confirm-delete"],
  "viewports": ["360x800", "768x900", "1280x900", "1920x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/quality-group-rules-20261031/prototype-quality-group-rules.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/quality-group-rules-20261031/baseline-scheduled-tests-production.png",
    "docs/visual-reviews/assets/quality-group-rules-20261031/baseline-account-edit-production.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/quality-group-rules-20261031/updated-quality-group-rules-review-board.png"
  ],
  "commands": [
    "npm exec --yes --package=pnpm@9.15.9 -- pnpm --dir frontend design:check",
    "npm exec --yes --package=pnpm@9.15.9 -- pnpm --dir frontend test:run -- src/views/admin/__tests__/AccountQualityView.spec.ts",
    "npm exec --yes --package=pnpm@9.15.9 -- pnpm --dir frontend typecheck"
  ],
  "checks": {
    "keyboard": {"status": "passed", "reason": "Native form controls and existing shared button/dialog behavior are retained; browser keyboard capture remains a final acceptance item."},
    "reduced_motion": {"status": "passed", "reason": "No new continuous motion was added; existing refresh feedback uses the shared reduced-motion utility."}
  },
  "residual_risks": [
    "The provided baseline PNGs are user-supplied production references; the updated PNG is a static review board, not an authenticated browser screenshot.",
    "Final browser acceptance on the deployed /admin/account-quality route remains required for admin, ordinary-user and guest roles.",
    "No production data, credentials or request bodies are included in these artifacts."
  ]
}
-->

## Scope

- Routes: `/admin/account-quality`; the existing account scheduled-test surface; the administrator sidebar entry.
- Roles: administrator only. The route metadata and server admin group enforce administrator access; ordinary users do not receive this navigation entry.
- Languages and themes: Chinese and English, light and dark.

## Baseline

- The supplied production references show the existing account edit and scheduled-test surfaces without group quality rule management.
- The new surface reuses the existing admin workspace shell and scheduled-test controls rather than introducing a separate page pattern.
- The baseline images are user-supplied references and are retained only as visual context.

## Prototype

- The prototype board defines a compact admin section for group filters, a quality rule form, coverage count, status, edit, pause and deletion choices.
- The scope is limited to API-backed group quality templates and the existing account-quality route. It does not add a user-facing entry or request/response body capture.
- The artifact is a static review board, not a browser capture or production acceptance result.

## Reuse Decision

- Reused `AppLayout`, existing button/input/select/textarea classes, the shared icon registry, scheduled-test ownership controls, localization resources and the existing admin route metadata.
- No new page-level width, background, scrolling container or decorative visual system was introduced.
- No design-system exception was added.

## State Coverage

- Default: list of templates and per-account quality plans.
- Hover, active and focus-visible: existing shared controls and native form focus behavior.
- Loading and disabled: list loading, save loading, per-template busy state and disabled actions.
- Empty and error: empty template list, global history error, API load/save/delete errors and retry for history/presets.
- Success and confirmation: created/updated/deleted notices and explicit confirmation before deleting plans.

## Viewport Coverage

- Mobile and tablet use the existing responsive grid and wrapped action rows at 360px and 768px.
- Desktop and wide views use the admin workspace frame at 1280px and 1920px.
- 200% zoom, dark theme, language switching and reduced motion remain final browser acceptance checks.

## Evidence

- Baseline: user-supplied production screenshots are stored under the review asset directory.
- Prototype and updated artifacts are static review-board PNGs generated from the documented layout; they do not claim authenticated browser rendering.
- The relevant Vitest, typecheck and design governance commands are listed in the manifest and are rerun in the candidate worktree.

## Residual Risk

- Final acceptance must be performed by an administrator in the deployed environment and must verify ordinary-user/guest denial, responsive layout and API error states.
- The static record does not mark the migration or production rollout complete.
