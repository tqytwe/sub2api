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
    "docs/visual-reviews/assets/account-edit-open/red-chunk-retry.png",
    "docs/visual-reviews/assets/account-edit-open/same-page-draft-red.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/account-edit-open/same-page-draft-green.png",
    "docs/visual-reviews/assets/account-edit-open/green-persistent-error.png",
    "docs/visual-reviews/assets/account-edit-open/green-edit-light.png",
    "docs/visual-reviews/assets/account-edit-open/green-edit-dark.png"
  ],
  "commands": [
    "pnpm build (Playwright browser tests below)",
    "EXPECT_DRAFT_SAFE=1 node same-page-draft.cjs",
    "node repro-green.cjs (explicit manual reload)",
    "node persistent-chunk.cjs",
    "node matrix.cjs (complete real HTTP and SQL matrix)",
    "EXPECT_RACE_FIXED=1 node request-race-browser.cjs",
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

Restore the existing account editor after an asynchronous module load failure, and keep its save target tied to the latest selected account while detail requests finish out of order. Reuse the existing shared Toast and manual refresh prompt; editor failures never automatically reload another unsaved dialog. No modal fields, styles, page frame, API contract, or persistence code changes. The user authorized the minimal repair and excluded mobile/Canvas.

## Baseline

Before implementation, inspect the existing Accounts dialog, neighboring Groups dialog and shared BaseDialog/Toast. The prior account-policy editor screenshot is the already accepted design reference. The baseline built-app capture shows the actual account table after a successful detail GET and one blocked EditAccountModal JavaScript fetch: subsequent clicks still fail to open the editor even after the asset network fault is removed.

## Prototype

The existing `account-policy-ui/updated-edit-1600-light.png` is deliberately reused as the restoration prototype, not presented as a newly generated design. The agreed boundary is to reopen this existing editor and use the existing error Toast when its module cannot load. No new visual pattern is introduced.

## Reuse Decision

Reuse `defineAsyncComponent`, app-store error Toast, and current modal lifecycle. Vue handles async loader failures internally. The local handler closes the failed instance and prompts users to finish unsaved work before explicitly refreshing. No automatic reload occurs: holding Edit, opening Create and entering a draft before the old module rejects must preserve that draft. An unmounted-view guard settles late loader failures without touching the newly navigated page.

## State Coverage

- Default/success: OpenAI API key and OAuth editors reopen with saved fields after real HTTP update, PostgreSQL verification, detail GET and page reload.
- Loading/error: a delayed module failure after navigating to Users does not reload the new page or show a stale Toast (unit and rebuilt-browser RED→GREEN). Transient and persistent failures trigger zero automatic reloads and a visible Chinese error. Explicit page reload after network recovery succeeds; cancel/reopen works. An actual Create draft survives a late Edit module failure without account writes.
- Disabled/submission: existing synchronous submitting guard still emits only one PUT for two immediate submissions.
- Cancel: no PUT and unchanged whole-account database hash; reopening reads existing details. In the follow-up race test, cancelling B invalidates delayed A, so A cannot reopen the editor.
- Latest selection: actual delayed A HTTP response followed by B cannot replace B or discard its draft; a real save targets B in PUT, SQL and GET. The original candidate reproduced the wrong-target save before the generation guard.
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
