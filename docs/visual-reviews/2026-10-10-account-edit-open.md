# Account editor load failure recovery

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AccountsView.vue",
    "frontend/src/i18n/locales/zh/admin/accounts.ts",
    "frontend/src/i18n/locales/en/admin/accounts.ts"
  ],
  "routes_or_surfaces": [
    "/admin/accounts"
  ],
  "languages_and_themes": [
    "zh-CN/light",
    "zh-CN/dark"
  ],
  "states": [
    "default",
    "hover",
    "active",
    "focus-visible",
    "loading",
    "disabled",
    "empty",
    "error",
    "success"
  ],
  "viewports": [
    "1280x900",
    "1600x1000"
  ],
  "artifact_mode": "browser-capture",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/account-policy-ui/updated-edit-1600-light.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/account-edit-open/red-chunk-retry.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/account-edit-open/green-chunk-retry.png",
    "docs/visual-reviews/assets/account-edit-open/green-persistent-error.png",
    "docs/visual-reviews/assets/account-edit-open/green-edit-light.png",
    "docs/visual-reviews/assets/account-edit-open/green-edit-dark.png"
  ],
  "commands": [
    "pnpm build (Playwright browser tests below)",
    "node repro-green.cjs",
    "node persistent-chunk.cjs",
    "node matrix.cjs (six data scenarios)",
    "node matrix-tail.cjs (HTTP failures, permissions, themes)",
    "pnpm design:check",
    "pnpm lint:check",
    "pnpm typecheck"
  ],
  "checks": {
    "keyboard": { "status": "passed" },
    "reduced_motion": {
      "status": "passed"
    }
  },
  "residual_risks": [
    "Synthetic local data and production builds are technical evidence, not production user acceptance.",
    "The injected module failure proves a recovery defect, not the unique cause of the user's production incident.",
    "Desktop-only scope; mobile and Canvas excluded. English keys tested, no English visual acceptance claimed."
  ]
}
-->

## Scope

Restore the existing account editor after an asynchronous module load failure. Reuse the existing shared Toast and per-route, session-bounded chunk recovery. No modal fields, styles, page frame, API, or persistence code changes. The user authorized the minimal repair and excluded mobile/Canvas.

## Baseline

Before implementation, inspect the existing Accounts dialog, neighboring Groups dialog and shared BaseDialog/Toast. The prior account-policy editor screenshot is the already accepted design reference. The baseline built-app capture shows the actual account table after a successful detail GET and one blocked EditAccountModal JavaScript fetch: subsequent clicks still fail to open the editor even after the asset network fault is removed.

## Prototype

The existing `account-policy-ui/updated-edit-1600-light.png` is deliberately reused as the restoration prototype, not presented as a newly generated design. The agreed boundary is to reopen this existing editor and use the existing error Toast when its module cannot load. No new visual pattern is introduced.

## Reuse Decision

Reuse `defineAsyncComponent`, the existing `recoverFromChunkLoadError` helper, app-store error Toast, and current modal lifecycle. Vue handles async loader failures internally, so global browser error/rejection listeners do not receive this failure. The local handler closes the failed instance, displays a translated error and attempts the existing bounded page reload. An unmounted-view guard settles late loader failures without touching the newly navigated page.

## State Coverage

- Default/success: OpenAI API key and OAuth editors reopen with saved fields after real HTTP update, PostgreSQL verification, detail GET and page reload.
- Loading/error: a delayed module failure after navigating to Users does not reload the new page or show a stale Toast (unit and rebuilt-browser RED→GREEN). One transient blocked module is recovered; a persistent failure triggers exactly one automatic reload, then a visible Chinese error. Explicit page reload after network recovery succeeds.
- Disabled/submission: existing synchronous submitting guard still emits only one PUT for two immediate submissions.
- Cancel: no PUT and unchanged whole-account database hash; reopening reads existing details.
- Detail error: injected HTTP 503 produces the existing API message, no dialog/write; a following real HTTP GET opens the editor.
- Optional profile error: an injected TLS profile 503 does not block the dialog.
- Null/legacy: complete, JSON-null and older partial account data all render and save. No runtime page errors in these data or HTTP-error scenarios.
- Hover, active and focus-visible: unchanged shared buttons and form controls; no bespoke style added. Keyboard Enter opens, Tab moves to another editor control, and keyboard cancel closes without a write (actual browser checked).
- Empty: a failed loader leaves no mounted editor; empty/null fields are exercised. Empty-table visual behavior is unchanged.

## Viewport Coverage

Chinese 1600×1000 light and 1280×900 dark, with reduced motion. Manually inspected actual screenshots: readable content, existing internal modal scroll and action footer retained. The light capture deliberately retains the prior detail-failure Toast while showing successful reopening. Mobile, 768px and Canvas excluded by task scope; no new responsive layout introduced.

## Evidence

See [acceptance record](../archive/completed-tasks/2026-10-10-account-edit-open/README.md). The screenshots use the built frontend and actual local Go server, migrated PostgreSQL 18.1 and Redis 8.4. Only explicit failure tests intercept assets or selected failing GET responses; the six save/reload scenarios use real HTTP and SQL throughout. Synthetic credentials are never shown or archived.

## Residual Risk

Production public access from this environment failed (HTTP 403 / browser certificate error), so no authenticated production browser fault trace is available. The observed loader recovery defect is proven locally; it cannot establish the sole production incident cause. Production deployment and user-local acceptance are coordinated by the main thread after this draft PR. Account export still omits group policies and is not a policy backup.
