# Admin Users Bulk Actions Visual Review

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/UsersView.vue",
    "frontend/src/components/admin/user/BulkUserActionDialog.vue",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh.ts"
  ],
  "routes_or_surfaces": [
    "/admin/users",
    "administrator batch disable and batch delete dialogs"
  ],
  "languages_and_themes": [
    "zh-CN light static prototype",
    "zh-CN and en-US implementation strings",
    "light and dark existing token paths"
  ],
  "states": [
    "no selection",
    "cross-page selection",
    "reason required",
    "preview loading",
    "preview ready",
    "HMAC-signed preview token",
    "protected administrator skipped",
    "stale preview",
    "step-up required",
    "execution partial success",
    "failed-user retry selection",
    "execution complete"
  ],
  "viewports": [
    "360x800",
    "390x844",
    "768x900",
    "1280x800",
    "1920x1080"
  ],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png",
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-390.png"
  ],
  "baseline_artifacts": [
    "docs/visual-reviews/assets/admin-users-bulk-actions/baseline-admin-users-selection-1440.png"
  ],
  "updated_artifacts": [
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png",
    "docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-390.png"
  ],
  "commands": [
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1440,900 --screenshot <baseline-artifact> file://<prototype>?mode=baseline",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 1440,900 --screenshot <prototype-artifact> file://<prototype>?mode=preview",
    "xvfb-run -a firefox --no-remote --profile <temporary-profile> --window-size 390,844 --screenshot <mobile-artifact> file://<prototype>?mode=preview",
    "pnpm --dir frontend design:check",
    "pnpm --dir frontend lint:check",
    "pnpm --dir frontend typecheck",
    "pnpm --dir frontend test:run",
    "pnpm --dir frontend build"
  ],
  "checks": {
    "keyboard": {
      "status": "passed",
      "reason": "The implementation reuses BaseDialog and TotpStepUpDialog focus ownership, and component tests exercise the preview and step-up execution path."
    },
    "reduced_motion": {
      "status": "passed",
      "reason": "The prototype introduces no decorative or continuous motion."
    }
  },
  "residual_risks": [
    "The prototype artifacts are browser-rendered static review boards, not authenticated product screenshots.",
    "Final administrator acceptance must select users across pages, preview both actions, complete TOTP, and verify complete and partial-result states in the user's production browser."
  ]
}
-->

## Scope

- Route: `/admin/users`.
- Roles: administrator only.
- Languages and themes: Chinese and English strings with existing light and dark semantic tokens.
- Behavior: explicit cross-page user IDs only, maximum 500 users, HMAC-signed server-side preview, administrator protection, TOTP step-up, per-user execution results and retry-safe selection refresh.

## Baseline

The existing page supports cross-page selection and exposes only the batch limits action. Disable and delete are available one user at a time, which makes incident response slow and increases the chance of inconsistent manual handling.

The baseline artifact shows the selected-user toolbar without an open dialog. It uses simulated `example.test` accounts and contains no production data.

## Prototype

- Desktop: `docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-1440.png`.
- Mobile: `docs/visual-reviews/assets/admin-users-bulk-actions/prototype-admin-users-bulk-actions-390.png`.
- Approval status: implementation continuation requested by the user on 2026-07-24.
- Scope boundary: add batch disable and batch delete only; preserve single-user actions and the existing batch-limits flow.

The selected-state toolbar keeps the current operational density and adds two adjacent actions. The destructive delete action uses the existing danger treatment without introducing a parallel visual system.

## Reuse Decision

- Reuse `AppLayout`, `TablePageLayout`, `DataTable`, `BaseDialog`, `TotpStepUpDialog`, `Icon`, buttons, inputs, badges, toast and `useTableSelection`.
- Add one user-domain dialog component because preview, typed delete confirmation, TOTP and partial results are a coherent reusable flow.
- No design-system exception is required.

## State Coverage

- Default: no bulk actions until at least one user is selected.
- Selection: selected count persists across pages.
- Validation: reason is required and delete confirmation must exactly match the preview's eligible count.
- Loading: preview and execution buttons retain their labels and prevent duplicate requests.
- Protected: administrators are listed as skipped and can never be executed.
- Stale: a changed selection or server `409` invalidates the preview and asks for a new one.
- Success and partial success: completed, skipped and failed IDs remain visible before the dialog closes; failed IDs stay selected after refresh for a direct retry.
- Step-up: both destructive actions use the shared TOTP dialog before execution.

## Viewport Coverage

- Mobile: actions wrap below the selected count; the dialog becomes a full-height flow with reachable footer actions.
- Tablet: standard dialog width with wrapping metrics.
- Desktop and wide: toolbar remains single-row when space permits and dialog uses the shared wide width.
- 200% zoom: content scrolls inside the dialog; no action relies on hover.
- Reduced motion: no new motion is required.

## Evidence

The current artifacts are static prototype evidence generated with Firefox under Xvfb. Product behavior is covered by component, API, typecheck, lint and production-build checks. Authenticated production screenshots remain part of the user's local-browser acceptance rather than server-side evidence.

## Residual Risk

Authenticated production screenshots cannot be produced from the server and do not replace final local-browser acceptance. The user should verify cross-page selection, administrator exclusion, TOTP interaction, API Key impact, partial failure feedback and refreshed table state after deployment.
