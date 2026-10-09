# Account group policies and retained-history totals

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/components/account/AccountGroupModelLimits.vue",
    "frontend/src/components/account/AccountTodayStatsCell.vue",
    "frontend/src/components/account/EditAccountModal.vue",
    "frontend/src/components/account/groupAllowedModels.ts",
    "frontend/src/types/index.ts",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts",
    "frontend/src/i18n/locales/zh/admin/overview.ts",
    "frontend/src/i18n/locales/en/admin/overview.ts"
  ],
  "routes_or_surfaces": ["/admin/accounts", "/admin/groups"],
  "languages_and_themes": ["zh-CN/light", "zh-CN/dark"],
  "states": ["default", "hover", "active", "focus-visible", "loading", "disabled", "empty", "error", "success"],
  "viewports": ["1280x900", "1600x1000"],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": ["docs/visual-reviews/assets/account-policy-ui/prototype-edit.png", "docs/visual-reviews/assets/account-policy-ui/prototype-stats.png"],
  "baseline_artifacts": ["docs/visual-reviews/assets/account-policy-ui/baseline-edit.png", "docs/visual-reviews/assets/account-policy-ui/baseline-accounts.png"],
  "updated_artifacts": ["docs/visual-reviews/assets/account-policy-ui/updated-edit-1600-light.png", "docs/visual-reviews/assets/account-policy-ui/updated-edit-1280-dark.png", "docs/visual-reviews/assets/account-policy-ui/updated-stats-1280-light.png", "docs/visual-reviews/assets/account-policy-ui/updated-stats-1600-dark.png", "docs/visual-reviews/assets/account-policy-ui/updated-copy-1280-light.png"],
  "commands": ["node account-policy-live-acceptance.cjs (Playwright browser, local fixture supervisor; production frontend build, HTTP server, PostgreSQL 18.1, Redis 8.4)", "pnpm design:check", "pnpm lint:check", "pnpm typecheck"],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": { "status": "passed" }
  },
  "residual_risks": ["Desktop-only scope requested: mobile and Canvas excluded.", "Local automated browser evidence is not the user-local production acceptance required by DELIVERY_WORKFLOW.md.", "English locale keys are tested; English visual acceptance is not yet claimed."]
}
-->

## Scope

Existing standard-mode administrator account editor, optional today-statistics column, and existing Group Copy Accounts explanation. No page layout, TPS, tooltip width, pricing, surcharge, memory fixture, upgrade source, or release safeguards changed. Simple-mode account detail intentionally omits policies and composite groups; policy editing is therefore unavailable there. The existing simple-mode composite filtering contract remains unchanged.

## Baseline

The production-style account dialog, neighboring Groups dialog, GroupSelector, input/btn classes, BaseDialog, table and stats cell were inspected. Baseline screenshots use the unchanged production frontend and synthetic read-only API data solely for layout inspection. They do not prove persistence. Existing account policies were absent from the editor and retained-history totals were absent from the statistics cell.

## Prototype

The two prototype images were captured before the corresponding visible implementation. The account-editor prototype inserts an explicitly proposed form section into the rendered baseline; the stats prototype adds proposed totals to the existing table. These are browser-rendered design prototypes, not implemented-feature or persistence evidence. The user-authorized boundary is the two gaps and the subsequently reported Copy Accounts data-loss fix; the existing visual system is retained.

## Reuse Decision

Reuse BaseDialog, GroupSelector, input/input-label/input-hint, btn-primary/btn-secondary and existing statistics formatting. The new section uses plain labelled textareas instead of triggering remote model discovery. Empty input has explicit unrestricted semantics, saved values come from fetched account details, and no additional page/frame/card pattern is introduced.

## State Coverage

- Default and saved: full detail supplies all current standard-mode bindings, with saved policy shown separately from draft input.
- Focus/keyboard, hover/active: native textareas and shared buttons; no custom animation or tooltip.
- Loading/disabled: update and policy inputs lock while submitting; synchronous guard prevents repeated submissions.
- Empty: blank policy explicitly removes the additional limit. Empty filtered list remains the existing account-table state.
- Error: per-group byte/count validation; real database notification failure rolls back, retains inputs, and permits retry.
- Success: real HTTP save, database query, detail GET, scheduler Redis refresh, page reload and reopen checked. Cancel sends no write.
- Legacy stats: missing lifetime fields omitted; explicit zero supported. Existing loading/error states retained.

## Viewport Coverage

1280×900 and 1600×1000, Chinese light/dark, reduced-motion browser context. Mobile/Canvas explicitly excluded by the task. Screenshots of the final build and checks are recorded in the delivery evidence.

## Evidence

Updated screenshots are from the production frontend build connected to the actual local HTTP server, fresh migrated PostgreSQL and Redis; no API interception is used. Test credentials remain in memory/local fixture environment and are absent from screenshots and recorded evidence. See the archived acceptance record for exact tested source and gate results.

## Residual Risk

The main thread coordinates production order. This branch is draft-only: no merge, deployment-success assertion, or final user-local production acceptance is performed here. Account exports still omit group policies and do not constitute a policy backup.
