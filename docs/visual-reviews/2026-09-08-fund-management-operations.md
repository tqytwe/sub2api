# Visual Review: Fund Management Operations

<!-- visual-review-manifest
{
  "schema_version": 1,
  "changed_files": [
    "frontend/src/views/admin/AdminFundsView.vue",
    "frontend/src/api/admin/funds.ts",
    "frontend/src/router/index.ts",
    "frontend/src/components/layout/AppSidebar.vue",
    "frontend/src/i18n/locales/zh.ts",
    "frontend/src/i18n/locales/en.ts",
    "frontend/src/i18n/locales/zh/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/en/legacy/admin-resources.ts",
    "frontend/src/i18n/locales/zh/workspaceShell.ts",
    "frontend/src/i18n/locales/en/workspaceShell.ts"
  ],
  "routes_or_surfaces": ["/admin/funds/refunds", "/admin/funds/credits", "/admin/funds/operations", "/admin/funds/classification redirect"],
  "languages_and_themes": ["zh-CN/light static review board", "en-US labels covered by locale source review"],
  "states": ["default", "account-search results", "selected account", "credit confirmation", "loading", "empty history", "completed correction", "insufficient-balance correction pending", "error"],
  "viewports": ["390x900", "1280x900"],
  "artifact_mode": "static-review-board",
  "prototype_artifacts": [
    "docs/visual-reviews/assets/fund-management-operations/prototype-1280.png",
    "docs/visual-reviews/assets/fund-management-operations/prototype-390.png"
  ],
  "baseline_artifacts": ["docs/visual-reviews/assets/fund-management-operations/baseline-static-review-board-1280.png"],
  "updated_artifacts": [
    "docs/visual-reviews/assets/fund-management-operations/updated-static-review-board-1280.png",
    "docs/visual-reviews/assets/fund-management-operations/updated-static-review-board-390.png"
  ],
  "commands": [
    "cd frontend && pnpm exec playwright screenshot --viewport-size=1280,900 file:///home/codex/worktrees/sub2api-fund-management-operations-20260908/docs/visual-reviews/assets/fund-management-operations/review-board.html ../docs/visual-reviews/assets/fund-management-operations/prototype-1280.png",
    "cd frontend && pnpm exec playwright screenshot --viewport-size=390,900 --full-page file:///home/codex/worktrees/sub2api-fund-management-operations-20260908/docs/visual-reviews/assets/fund-management-operations/review-board.html ../docs/visual-reviews/assets/fund-management-operations/prototype-390.png"
  ],
  "checks": {
    "keyboard": {"status": "not-applicable", "reason": "Static review boards cannot execute the authenticated keyboard flow; final browser acceptance is required."},
    "reduced_motion": {"status": "not-applicable", "reason": "No new motion was added; the existing refresh rotation remains transient."}
  },
  "residual_risks": [
    "The PNGs are static review boards rather than authenticated product browser captures.",
    "Before a release, verify 360, 768, 1280, and wide routes in Chinese and English, light and dark theme, keyboard navigation, 200 percent zoom, empty/error/loading states, and a real TOTP-protected correction."
  ]
}
-->

## Scope

The changed administrator surfaces are the three Fund Management tabs, its
sidebar children, the legacy classification redirect, and their Chinese and
English labels. No user-facing wallet page is changed.

## Baseline

The prior page directly accepted database user IDs for gifts and offline
recharges, displayed IDs next to accounts, and exposed an obsolete 30-dollar
signup-gift review tab. It did not provide a searchable operation history or a
traceable wrong-account correction path. During integration review, the first
replacement also incorrectly omitted existing refund rejection, secure payout
snapshot viewing, payout FX rate, and payout note fields; this revision restores
those established refund workflows before adding the new tabs.

## Prototype

The two prototype images use the established administrator sidebar, compact
tables, semantic status chips, standard fields, buttons, and panel density.
They introduce no new page shell, icon set, card family, gradient, or animated
surface. The board shows account lookup, selected account balance, confirmation
by exact email, public operation numbers, masked payment references, operation
detail, and the correction entry point.

## Reuse Decision

`AdminFundsView` retains `AppLayout`, `Icon`, shared `btn`/`input` styles, the
existing TOTP step-up controller, table density, message treatment, and sidebar
navigation. The implementation deliberately uses account email rather than
internal user keys in the visible workflow.

Refund approval, rejection, payout confirmation, and payout-snapshot reads use
the existing public refund request number and the same `TotpStepUpDialog`.
Pending correction records use a public operation number, expose retry/cancel
only for the safe gift/compensation correction path, and never offer automatic
offline-recharge reassignment.

## State Coverage

- Default and empty history: compact data table keeps actions reachable.
- Account lookup: a selected account displays email, status, username, and
  current balance without a database identifier.
- Confirmation: the administrator must type the selected email before the
  existing step-up protected mutation starts.
- Correction: completed credits expose correction; insufficient source balance
  is represented as a visible pending state without a negative balance.
- Error and loading: inputs are retained and the existing status message plus
  button disabled state reports outcome without layout shift.

## Viewport Coverage

The static review board was rendered at 390x900 and 1280x900. The narrow board
stacks the form and detail panel and makes the table horizontally reachable;
the desktop board keeps filtering and row actions in a compact operational
layout. Real authenticated 360, 768, 1280, and wide browser captures remain a
release requirement.

## Evidence And Risk

Both PNG assets were rendered by Playwright, decoded as PNG, and inspected at
390 and 1280 widths. They are static review boards only. An authenticated
administrator must complete final browser acceptance after a separately
authorized deployment; no production data, correction, compensation, or
deployment was performed by this change.

## Residual Risk

Static review boards cannot prove the live authenticated TOTP, account-search,
or correction API flows. Those need browser acceptance against a disposable
test account before any production deployment.
